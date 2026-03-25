package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Materializer struct {
	bucket     string
	stagingDir string
	client     *s3.Client
}

func newS3Materializer(cfg S3Config) (*s3Materializer, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{}
	if strings.TrimSpace(cfg.Region) != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(strings.TrimSpace(cfg.Region)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	if err := os.MkdirAll(cfg.StagingDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create s3 staging dir: %w", err)
	}

	return &s3Materializer{
		bucket:     strings.TrimSpace(cfg.Bucket),
		stagingDir: cfg.StagingDir,
		client:     s3.NewFromConfig(awsCfg),
	}, nil
}

func (m *s3Materializer) materializeFile(ctx context.Context, key string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp(m.stagingDir, "claim-file-")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	localPath := filepath.Join(tmpDir, path.Base(key))

	if err := m.downloadObject(ctx, key, localPath); err != nil {
		cleanup()
		return "", nil, err
	}

	return localPath, cleanup, nil
}

func (m *s3Materializer) materializeDir(ctx context.Context, prefix string) (string, func(), error) {
	prefix = ensureTrailingSlash(prefix)
	tmpDir, err := os.MkdirTemp(m.stagingDir, "claim-dir-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	keys, err := m.listKeys(ctx, prefix)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if len(keys) == 0 {
		cleanup()
		return "", nil, fmt.Errorf("no objects found under claimed prefix %q", prefix)
	}

	for _, key := range keys {
		rel := strings.TrimPrefix(key, prefix)
		if rel == "" {
			continue
		}
		localPath := filepath.Join(tmpDir, filepath.FromSlash(rel))
		if err := m.downloadObject(ctx, key, localPath); err != nil {
			cleanup()
			return "", nil, err
		}
	}

	return tmpDir, cleanup, nil
}

func (m *s3Materializer) listKeys(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	var token *string

	for {
		out, err := m.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &m.bucket,
			Prefix:            &prefix,
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects for prefix %q failed: %w", prefix, err)
		}

		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key
			if strings.HasSuffix(key, "/") {
				continue
			}
			keys = append(keys, key)
		}

		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}

	return keys, nil
}

func (m *s3Materializer) downloadObject(ctx context.Context, key, dst string) error {
	out, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &m.bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("get object %q failed: %w", key, err)
	}
	defer out.Body.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("downloading object %q failed: %w", key, err)
	}

	return nil
}

func ensureTrailingSlash(s string) string {
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}
