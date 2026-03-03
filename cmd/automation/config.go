package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func readConfig() config {
	var cfg config
	flag.StringVar(&cfg.Backend, "backend", envOrDefault("AUTOMATION_BACKEND", "s3"), "libfilerunner backend: directory, s3, or azureblob")

	flag.StringVar(&cfg.InputDir, "input-dir", envOrDefault("AUTOMATION_INPUT_DIR", "./queue/input"), "job queue input directory")
	flag.StringVar(&cfg.InProgressDir, "in-progress-dir", envOrDefault("AUTOMATION_INPROGRESS_DIR", "./queue/in-progress"), "job queue in-progress directory")
	flag.StringVar(&cfg.FailedDir, "failed-dir", envOrDefault("AUTOMATION_FAILED_DIR", "./queue/failed"), "job queue failed directory")

	flag.StringVar(&cfg.S3Region, "s3-region", envOrDefault("AUTOMATION_S3_REGION", ""), "s3 queue region")
	flag.StringVar(&cfg.S3Bucket, "s3-bucket", envOrDefault("AUTOMATION_S3_BUCKET", ""), "s3 queue bucket")
	flag.StringVar(&cfg.S3InputPrefix, "s3-input-prefix", envOrDefault("AUTOMATION_S3_INPUT_PREFIX", "queue/input"), "s3 input prefix")
	flag.StringVar(&cfg.S3InProgressPrefix, "s3-in-progress-prefix", envOrDefault("AUTOMATION_S3_INPROGRESS_PREFIX", "queue/in-progress"), "s3 in-progress prefix")
	flag.StringVar(&cfg.S3FailedPrefix, "s3-failed-prefix", envOrDefault("AUTOMATION_S3_FAILED_PREFIX", "queue/failed"), "s3 failed prefix")

	flag.StringVar(&cfg.AzureAccountURL, "azure-account-url", envOrDefault("AUTOMATION_AZURE_ACCOUNT_URL", ""), "azure blob account URL")
	flag.StringVar(&cfg.AzureContainer, "azure-container", envOrDefault("AUTOMATION_AZURE_CONTAINER", ""), "azure blob container")
	flag.StringVar(&cfg.AzureInputPrefix, "azure-input-prefix", envOrDefault("AUTOMATION_AZURE_INPUT_PREFIX", "queue/input"), "azure blob input prefix")
	flag.StringVar(&cfg.AzureInProgressPrefix, "azure-in-progress-prefix", envOrDefault("AUTOMATION_AZURE_INPROGRESS_PREFIX", "queue/in-progress"), "azure blob in-progress prefix")
	flag.StringVar(&cfg.AzureFailedPrefix, "azure-failed-prefix", envOrDefault("AUTOMATION_AZURE_FAILED_PREFIX", "queue/failed"), "azure blob failed prefix")

	flag.DurationVar(&cfg.PollInterval, "poll-interval", durationOrDefault("AUTOMATION_POLL_INTERVAL", 2*time.Second), "poll interval when queue is empty")

	flag.StringVar(&cfg.JiveTalkingBin, "jivetalking-bin", envOrDefault("JIVETALKING_BIN", "jivetalking"), "jivetalking executable path")
	flag.StringVar(&cfg.JiveFireBin, "jivefire-bin", envOrDefault("JIVEFIRE_BIN", "jivefire"), "jivefire executable path")
	flag.StringVar(&cfg.JiveDropBin, "jivedrop-bin", envOrDefault("JIVEDROP_BIN", "jivedrop"), "jivedrop executable path")
	flag.Parse()
	return cfg
}

func validateBackendConfig(cfg config) error {
	switch strings.ToLower(strings.TrimSpace(cfg.Backend)) {
	case "directory", "dir", "local":
		if strings.TrimSpace(cfg.InputDir) == "" || strings.TrimSpace(cfg.InProgressDir) == "" || strings.TrimSpace(cfg.FailedDir) == "" {
			return errors.New("directory backend requires input-dir, in-progress-dir, and failed-dir")
		}
	case "s3":
		if strings.TrimSpace(cfg.S3Region) == "" {
			return errors.New("s3 backend requires s3-region")
		}
		if strings.TrimSpace(cfg.S3Bucket) == "" {
			return errors.New("s3 backend requires s3-bucket")
		}
	case "azure", "azureblob", "blob":
		if strings.TrimSpace(cfg.AzureAccountURL) == "" {
			return errors.New("azureblob backend requires azure-account-url")
		}
		if strings.TrimSpace(cfg.AzureContainer) == "" {
			return errors.New("azureblob backend requires azure-container")
		}
	default:
		return fmt.Errorf("unsupported backend %q (supported: directory, s3, azureblob)", cfg.Backend)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Printf("invalid duration in %s=%q, using %s", key, val, fallback)
		return fallback
	}
	return d
}
