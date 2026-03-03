package main

import (
	"context"
	"fmt"
	"strings"

	libfilerunner "github.com/bigpod98/libfilerunner-go/pkg"
)

type queueRunner interface {
	RunOnce(ctx context.Context, handler libfilerunner.Handler) (libfilerunner.RunOnceResult, error)
}

func createRunner(cfg config) (queueRunner, string, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Backend)) {
	case "directory", "dir", "local":
		runner, err := libfilerunner.NewDirectoryRunner(libfilerunner.DirectoryConfig{
			InputDir:      cfg.InputDir,
			InProgressDir: cfg.InProgressDir,
			FailedDir:     cfg.FailedDir,
		})
		if err != nil {
			return nil, "", err
		}
		if err := runner.EnsureDirectories(); err != nil {
			return nil, "", err
		}
		desc := fmt.Sprintf("%s -> %s -> %s", cfg.InputDir, cfg.InProgressDir, cfg.FailedDir)
		return runner, desc, nil
	case "s3":
		runner, err := libfilerunner.NewS3Runner(libfilerunner.S3Config{
			Region:           cfg.S3Region,
			Bucket:           cfg.S3Bucket,
			InputPrefix:      cfg.S3InputPrefix,
			InProgressPrefix: cfg.S3InProgressPrefix,
			FailedPrefix:     cfg.S3FailedPrefix,
		})
		if err != nil {
			return nil, "", err
		}
		desc := fmt.Sprintf("s3://%s [%s -> %s -> %s]", cfg.S3Bucket, cfg.S3InputPrefix, cfg.S3InProgressPrefix, cfg.S3FailedPrefix)
		return runner, desc, nil
	case "azure", "azureblob", "blob":
		runner, err := libfilerunner.NewAzureBlobRunner(libfilerunner.AzureBlobConfig{
			AccountURL:       cfg.AzureAccountURL,
			Container:        cfg.AzureContainer,
			InputPrefix:      cfg.AzureInputPrefix,
			InProgressPrefix: cfg.AzureInProgressPrefix,
			FailedPrefix:     cfg.AzureFailedPrefix,
		})
		if err != nil {
			return nil, "", err
		}
		desc := fmt.Sprintf("azure://%s [%s -> %s -> %s]", cfg.AzureContainer, cfg.AzureInputPrefix, cfg.AzureInProgressPrefix, cfg.AzureFailedPrefix)
		return runner, desc, nil
	default:
		return nil, "", fmt.Errorf("unsupported backend %q (supported: directory, s3, azureblob)", cfg.Backend)
	}
}
