package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/bigpod98/automation/internal/app"
)

func main() {
	var configPath string
	var once bool
	var queue string

	flag.StringVar(&configPath, "config", "config.yaml", "Path to YAML configuration file")
	flag.BoolVar(&once, "once", false, "Process one polling cycle and exit")
	flag.StringVar(&queue, "queue", "", "Run only this queue (jivetalking, jivedrop_standalone, jivedrop_hugo, jivefire_standalone)")
	flag.Parse()

	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("failed loading config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.StartWorkers(ctx, cfg, once, queue); err != nil {
		log.Fatalf("worker error: %v", err)
	}
}
