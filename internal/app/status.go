package app

import (
	"sync"
	"time"
)

type WorkerState string

const (
	WorkerStateIdle       WorkerState = "idle"
	WorkerStateProcessing WorkerState = "processing"
)

type WorkerStatus struct {
	Name         string      `json:"name"`
	State        WorkerState `json:"state"`
	CurrentClaim string      `json:"current_claim,omitempty"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type StatusStore struct {
	mu      sync.RWMutex
	workers map[string]*WorkerStatus
}

func NewStatusStore() *StatusStore {
	return &StatusStore{workers: make(map[string]*WorkerStatus)}
}

func (s *StatusStore) register(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[name] = &WorkerStatus{
		Name:      name,
		State:     WorkerStateIdle,
		UpdatedAt: time.Now(),
	}
}

func (s *StatusStore) setProcessing(name, claim string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.workers[name]; ok {
		w.State = WorkerStateProcessing
		w.CurrentClaim = claim
		w.UpdatedAt = time.Now()
	}
}

func (s *StatusStore) setIdle(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.workers[name]; ok {
		w.State = WorkerStateIdle
		w.CurrentClaim = ""
		w.UpdatedAt = time.Now()
	}
}

func (s *StatusStore) all() []WorkerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]WorkerStatus, 0, len(s.workers))
	for _, w := range s.workers {
		out = append(out, *w)
	}
	return out
}

func (s *StatusStore) get(name string) (WorkerStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.workers[name]
	if !ok {
		return WorkerStatus{}, false
	}
	return *w, true
}
