package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	libfilerunner "github.com/bigpod98/libfilerunner-go/pkg"
)

func main() {
	cfg := readConfig()
	if err := validateBackendConfig(cfg); err != nil {
		log.Fatalf("invalid backend config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner, queueDescription, err := createRunner(cfg)
	if err != nil {
		log.Fatalf("create runner: %v", err)
	}

	log.Printf("automation started (backend=%s queue=%s poll=%s)", cfg.Backend, queueDescription, cfg.PollInterval)
	for {
		if err := ctx.Err(); err != nil {
			log.Printf("stopping: %v", err)
			return
		}

		result, runErr := runner.RunOnce(ctx, func(runCtx context.Context, fileJob libfilerunner.FileJob) error {
			return processJob(runCtx, cfg, fileJob)
		})
		if runErr != nil {
			log.Printf("runner error: %v", runErr)
			time.Sleep(cfg.PollInterval)
			continue
		}
		if !result.Found {
			time.Sleep(cfg.PollInterval)
		}
	}
}
