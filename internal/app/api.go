package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"time"
)

type apiServer struct {
	store      *StatusStore
	inspectors map[string]QueueInspector
	server     *http.Server
}

func newAPIServer(listen string, store *StatusStore, inspectors map[string]QueueInspector) *apiServer {
	a := &apiServer{store: store, inspectors: inspectors}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /queues", a.handleQueues)
	mux.HandleFunc("GET /queues/{name}/pending", a.handlePending)
	mux.HandleFunc("GET /queues/{name}/in-progress", a.handleInProgress)
	mux.HandleFunc("GET /queues/{name}/worker", a.handleWorker)

	a.server = &http.Server{
		Addr:    listen,
		Handler: mux,
	}
	return a
}

// start binds the listener synchronously (so port errors surface immediately),
// then serves in a background goroutine that shuts down when ctx is cancelled.
func (a *apiServer) start(ctx context.Context) error {
	ln, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return fmt.Errorf("API server listen on %s: %w", a.server.Addr, err)
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutCtx)
	}()

	go func() {
		if err := a.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			// Nothing we can do at this point; log via standard output.
			fmt.Printf("[api] server error: %v\n", err)
		}
	}()

	return nil
}

// --- response types ---

type queueSummary struct {
	Name   string      `json:"name"`
	Worker WorkerStatus `json:"worker"`
}

type itemsResponse struct {
	Queue string   `json:"queue"`
	Items []string `json:"items"`
	Count int      `json:"count"`
}

// --- handlers ---

// GET /queues
// Returns worker state for every registered queue, sorted by name.
func (a *apiServer) handleQueues(w http.ResponseWriter, r *http.Request) {
	statuses := a.store.all()
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	summaries := make([]queueSummary, len(statuses))
	for i, s := range statuses {
		summaries[i] = queueSummary{Name: s.Name, Worker: s}
	}

	writeJSON(w, http.StatusOK, summaries)
}

// GET /queues/{name}/pending
// Lists items waiting in the input queue.
func (a *apiServer) handlePending(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	insp, ok := a.inspectors[name]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("queue %q not found", name))
		return
	}

	items, err := insp.PendingItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, itemsResponse{Queue: name, Items: orEmpty(items), Count: len(items)})
}

// GET /queues/{name}/in-progress
// Lists items currently claimed and being processed.
func (a *apiServer) handleInProgress(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	insp, ok := a.inspectors[name]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("queue %q not found", name))
		return
	}

	items, err := insp.InProgressItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, itemsResponse{Queue: name, Items: orEmpty(items), Count: len(items)})
}

// GET /queues/{name}/worker
// Returns the live state of the worker for the named queue.
func (a *apiServer) handleWorker(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	status, ok := a.store.get(name)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("queue %q not found", name))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
