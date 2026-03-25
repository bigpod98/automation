package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// QueueInspector lists pending and in-progress items for a single queue without
// claiming them.
type QueueInspector interface {
	PendingItems(ctx context.Context) ([]string, error)
	InProgressItems(ctx context.Context) ([]string, error)
}

// --- Directory backend ---

type directoryInspector struct {
	inputDir      string
	inProgressDir string
	selectDirs    bool
}

func newDirectoryInspector(queueDir string, selectDirs bool) QueueInspector {
	return &directoryInspector{
		inputDir:      filepath.Join(queueDir, "input"),
		inProgressDir: filepath.Join(queueDir, "in-progress"),
		selectDirs:    selectDirs,
	}
}

func (d *directoryInspector) PendingItems(ctx context.Context) ([]string, error) {
	return readDirNames(d.inputDir, d.selectDirs)
}

func (d *directoryInspector) InProgressItems(ctx context.Context) ([]string, error) {
	return readDirNames(d.inProgressDir, d.selectDirs)
}

func readDirNames(dir string, wantDirs bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if wantDirs == e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// --- S3 backend ---

type s3Inspector struct {
	client           *s3.Client
	bucket           string
	inputPrefix      string
	inProgressPrefix string
	selectDirs       bool
}

// newS3Inspector is called from s3Materializer so they share the same client.
func (m *s3Materializer) newInspector(q WorkerQueueConfig, selectDirs bool) QueueInspector {
	return &s3Inspector{
		client:           m.client,
		bucket:           m.bucket,
		inputPrefix:      strings.TrimSpace(q.InputPrefix),
		inProgressPrefix: strings.TrimSpace(q.InProgressPrefix),
		selectDirs:       selectDirs,
	}
}

func (i *s3Inspector) PendingItems(ctx context.Context) ([]string, error) {
	return i.listTopLevel(ctx, i.inputPrefix)
}

func (i *s3Inspector) InProgressItems(ctx context.Context) ([]string, error) {
	return i.listTopLevel(ctx, i.inProgressPrefix)
}

// listTopLevel lists direct children of a prefix.
// For file queues it returns object keys; for directory queues it returns
// common prefixes (the virtual sub-directories).
func (i *s3Inspector) listTopLevel(ctx context.Context, prefix string) ([]string, error) {
	prefix = ensureTrailingSlash(prefix)
	delimiter := aws.String("/")

	var items []string
	var token *string

	for {
		out, err := i.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &i.bucket,
			Prefix:            &prefix,
			Delimiter:         delimiter,
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}

		if i.selectDirs {
			for _, cp := range out.CommonPrefixes {
				if cp.Prefix == nil {
					continue
				}
				name := strings.TrimPrefix(*cp.Prefix, prefix)
				name = strings.TrimSuffix(name, "/")
				if name != "" {
					items = append(items, name)
				}
			}
		} else {
			for _, obj := range out.Contents {
				if obj.Key == nil {
					continue
				}
				name := strings.TrimPrefix(*obj.Key, prefix)
				if name != "" {
					items = append(items, name)
				}
			}
		}

		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}

	return items, nil
}
