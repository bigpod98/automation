package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	libfilerunner "github.com/bigpod98/libfilerunner-go/pkg"
	jivedropautomation "github.com/linuxmatters/jivedrop/pkg/automation"
	jivefireautomation "github.com/linuxmatters/jivefire/pkg/automation"
	jivetalkingautomation "github.com/linuxmatters/jivetalking/pkg/automation"
)

type queueOps struct {
	runOnceOrchestration func(context.Context) (libfilerunner.RunOnceResult, error)
	completed            func(context.Context, string) error
	failed               func(context.Context, string) (string, error)
}

type claimLocalizer func(context.Context, string) (string, func(), error)

type queueWorker struct {
	name      string
	ops       queueOps
	localize  claimLocalizer
	batch     int
	poll      time.Duration
	handler   func(context.Context, string) error
	logf      func(string, ...any)
	onceOnly  bool
	queueKind string
	status    *StatusStore
}

func StartWorkers(ctx context.Context, cfg Config, once bool, queue string) error {
	allowed := make(map[string]struct{}, len(cfg.AllowedAudioExtensions))
	for _, ext := range cfg.AllowedAudioExtensions {
		allowed[ext] = struct{}{}
	}

	store := NewStatusStore()

	workers, inspectors, err := buildWorkers(cfg, allowed, once, queue, store)
	if err != nil {
		return err
	}
	if len(workers) == 0 {
		return fmt.Errorf("no queues are enabled")
	}

	if cfg.APIListen != "" {
		srv := newAPIServer(cfg.APIListen, store, inspectors)
		if err := srv.start(ctx); err != nil {
			return err
		}
		log.Printf("[api] listening on %s", cfg.APIListen)
	}

	errCh := make(chan error, len(workers))
	for i := range workers {
		w := workers[i]
		go func() {
			errCh <- w.run(ctx)
		}()
	}

	var firstErr error
	for range workers {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func buildWorkers(cfg Config, allowed map[string]struct{}, once bool, queue string, store *StatusStore) ([]queueWorker, map[string]QueueInspector, error) {
	var workers []queueWorker
	inspectors := make(map[string]QueueInspector)

	validQueues := map[string]struct{}{
		"jivetalking":         {},
		"jivedrop_standalone": {},
		"jivedrop_hugo":       {},
		"jivefire_standalone": {},
	}
	if queue != "" {
		if _, ok := validQueues[queue]; !ok {
			return nil, nil, fmt.Errorf("unknown queue %q: must be one of jivetalking, jivedrop_standalone, jivedrop_hugo, jivefire_standalone", queue)
		}
	}

	var materializer *s3Materializer
	if cfg.QueueBackend == "s3" {
		m, err := newS3Materializer(cfg.S3)
		if err != nil {
			return nil, nil, err
		}
		materializer = m
	}

	if cfg.Queues.Jivetalking.Enabled && (queue == "" || queue == "jivetalking") {
		ops, localize, err := queueBackend(cfg, cfg.Queues.Jivetalking, false, materializer)
		if err != nil {
			return nil, nil, fmt.Errorf("jivetalking queue setup: %w", err)
		}
		if err := os.MkdirAll(cfg.Queues.Jivetalking.OutputDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("jivetalking output dir: %w", err)
		}
		store.register("jivetalking")
		inspectors["jivetalking"] = buildInspector(cfg, cfg.Queues.Jivetalking, false, materializer)

		workers = append(workers, queueWorker{
			name:      "jivetalking",
			queueKind: cfg.QueueBackend,
			ops:       ops,
			localize:  localize,
			batch:     cfg.BatchSize,
			poll:      cfg.PollInterval,
			logf:      log.Printf,
			status:    store,
			handler: func(ctx context.Context, claimPath string) error {
				if !isAllowedAudio(claimPath, allowed) {
					return fmt.Errorf("unsupported audio file extension: %s", claimPath)
				}

				processed, err := jivetalkingautomation.ProcessFile(claimPath)
				if err != nil {
					return err
				}

				if _, err := moveFileToDir(processed, cfg.Queues.Jivetalking.OutputDir); err != nil {
					return err
				}

				return nil
			},
			onceOnly: once,
		})
	}

	if cfg.Queues.JivedropStandalone.Enabled && (queue == "" || queue == "jivedrop_standalone") {
		ops, localize, err := queueBackend(cfg, cfg.Queues.JivedropStandalone, true, materializer)
		if err != nil {
			return nil, nil, fmt.Errorf("jivedrop standalone queue setup: %w", err)
		}
		if err := os.MkdirAll(cfg.Queues.JivedropStandalone.OutputDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("jivedrop standalone output dir: %w", err)
		}
		store.register("jivedrop-standalone")
		inspectors["jivedrop-standalone"] = buildInspector(cfg, cfg.Queues.JivedropStandalone, true, materializer)

		workers = append(workers, queueWorker{
			name:      "jivedrop-standalone",
			queueKind: cfg.QueueBackend,
			ops:       ops,
			localize:  localize,
			batch:     cfg.BatchSize,
			poll:      cfg.PollInterval,
			logf:      log.Printf,
			status:    store,
			handler: func(ctx context.Context, claimPath string) error {
				audioPath, err := findSingleAudioFile(claimPath, allowed)
				if err != nil {
					return err
				}
				metaPath, err := findSingleMetadataFile(claimPath)
				if err != nil {
					return err
				}

				var meta JivedropStandaloneMetadata
				if err := parseMetadata(metaPath, &meta); err != nil {
					return err
				}
				if strings.TrimSpace(meta.Title) == "" {
					return fmt.Errorf("metadata.title is required")
				}
				if strings.TrimSpace(meta.Num.String()) == "" {
					return fmt.Errorf("metadata.num is required")
				}
				if strings.TrimSpace(meta.Cover) == "" {
					return fmt.Errorf("metadata.cover is required and must be a URL")
				}

				coverPath, err := downloadCover(ctx, meta.Cover, claimPath, cfg.CoverDownloadTimeout, cfg.CoverDownloadMaxBytes)
				if err != nil {
					return fmt.Errorf("cover download failed: %w", err)
				}
				defer os.Remove(coverPath)

				_, err = jivedropautomation.RunStandalone(jivedropautomation.StandaloneInput{
					AudioPath: audioPath,
					OutputDir: cfg.Queues.JivedropStandalone.OutputDir,
					Title:     meta.Title,
					Num:       meta.Num.String(),
					CoverPath: coverPath,
					Artist:    meta.Artist,
					Album:     meta.Album,
					Date:      meta.Date,
					Comment:   meta.Comment,
					Stereo:    meta.Stereo,
				})
				return err
			},
			onceOnly: once,
		})
	}

	if cfg.Queues.JivedropHugo.Enabled && (queue == "" || queue == "jivedrop_hugo") {
		ops, localize, err := queueBackend(cfg, cfg.Queues.JivedropHugo, true, materializer)
		if err != nil {
			return nil, nil, fmt.Errorf("jivedrop hugo queue setup: %w", err)
		}
		if err := os.MkdirAll(cfg.Queues.JivedropHugo.OutputDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("jivedrop hugo output dir: %w", err)
		}
		store.register("jivedrop-hugo")
		inspectors["jivedrop-hugo"] = buildInspector(cfg, cfg.Queues.JivedropHugo, true, materializer)

		workers = append(workers, queueWorker{
			name:      "jivedrop-hugo",
			queueKind: cfg.QueueBackend,
			ops:       ops,
			localize:  localize,
			batch:     cfg.BatchSize,
			poll:      cfg.PollInterval,
			logf:      log.Printf,
			status:    store,
			handler: func(ctx context.Context, claimPath string) error {
				audioPath, err := findSingleAudioFile(claimPath, allowed)
				if err != nil {
					return err
				}
				markdownPath, err := findSingleMarkdownFile(claimPath)
				if err != nil {
					return err
				}

				_, err = jivedropautomation.RunHugo(jivedropautomation.HugoInput{
					AudioPath:    audioPath,
					MarkdownPath: markdownPath,
					OutputDir:    cfg.Queues.JivedropHugo.OutputDir,
				})
				return err
			},
			onceOnly: once,
		})
	}

	if cfg.Queues.JivefireStandalone.Enabled && (queue == "" || queue == "jivefire_standalone") {
		ops, localize, err := queueBackend(cfg, cfg.Queues.JivefireStandalone, true, materializer)
		if err != nil {
			return nil, nil, fmt.Errorf("jivefire standalone queue setup: %w", err)
		}
		if err := os.MkdirAll(cfg.Queues.JivefireStandalone.OutputDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("jivefire standalone output dir: %w", err)
		}
		store.register("jivefire-standalone")
		inspectors["jivefire-standalone"] = buildInspector(cfg, cfg.Queues.JivefireStandalone, true, materializer)

		workers = append(workers, queueWorker{
			name:      "jivefire-standalone",
			queueKind: cfg.QueueBackend,
			ops:       ops,
			localize:  localize,
			batch:     cfg.BatchSize,
			poll:      cfg.PollInterval,
			logf:      log.Printf,
			status:    store,
			handler: func(ctx context.Context, claimPath string) error {
				audioPath, err := findSingleAudioFile(claimPath, allowed)
				if err != nil {
					return err
				}
				metaPath, err := findSingleMetadataFile(claimPath)
				if err != nil {
					return err
				}

				var meta JivefireStandaloneMetadata
				if err := parseMetadata(metaPath, &meta); err != nil {
					return err
				}

				base := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath)) + ".mp4"
				outputTarget := strings.TrimSpace(meta.Output)
				if outputTarget == "" {
					outputTarget = base
				}

				outputPath := outputTarget
				if !filepath.IsAbs(outputPath) {
					outputPath = filepath.Join(cfg.Queues.JivefireStandalone.OutputDir, outputPath)
				}

				if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
					return err
				}

				var bgPath string
				if meta.BackgroundImage != "" {
					p, err := downloadCover(ctx, meta.BackgroundImage, claimPath, cfg.CoverDownloadTimeout, cfg.CoverDownloadMaxBytes)
					if err != nil {
						return fmt.Errorf("background_image download failed: %w", err)
					}
					defer os.Remove(p)
					bgPath = p
				}

				var thumbPath string
				if meta.ThumbnailImage != "" {
					p, err := downloadCover(ctx, meta.ThumbnailImage, claimPath, cfg.CoverDownloadTimeout, cfg.CoverDownloadMaxBytes)
					if err != nil {
						return fmt.Errorf("thumbnail_image download failed: %w", err)
					}
					defer os.Remove(p)
					thumbPath = p
				}

				return jivefireautomation.Run(jivefireautomation.Input{
					AudioPath:        audioPath,
					OutputPath:       outputPath,
					Episode:          meta.Episode.Int(),
					Title:            meta.Title,
					Channels:         meta.Channels.Int(),
					BarColor:         meta.BarColor,
					TextColor:        meta.TextColor,
					BackgroundImage:  bgPath,
					ThumbnailImage:   thumbPath,
					NoPreview:        meta.NoPreview,
					RequestedEncoder: meta.Encoder,
				})
			},
			onceOnly: once,
		})
	}

	return workers, inspectors, nil
}

func buildInspector(cfg Config, q WorkerQueueConfig, selectDirs bool, materializer *s3Materializer) QueueInspector {
	if cfg.QueueBackend == "s3" {
		return materializer.newInspector(q, selectDirs)
	}
	return newDirectoryInspector(q.QueueDir, selectDirs)
}

func queueBackend(cfg Config, q WorkerQueueConfig, selectDirs bool, materializer *s3Materializer) (queueOps, claimLocalizer, error) {
	if cfg.QueueBackend == "directory" {
		runner, err := newDirectoryRunner(q.QueueDir, selectDirs, cfg.BatchSize)
		if err != nil {
			return queueOps{}, nil, err
		}
		ops := queueOps{
			runOnceOrchestration: runner.RunOnceOrchestration,
			completed:            runner.Completed,
			failed:               runner.Failed,
		}
		return ops, func(_ context.Context, claim string) (string, func(), error) {
			return claim, func() {}, nil
		}, nil
	}

	runner, err := newS3Runner(cfg.S3, q, selectDirs, cfg.BatchSize)
	if err != nil {
		return queueOps{}, nil, err
	}
	if materializer == nil {
		return queueOps{}, nil, fmt.Errorf("s3 materializer is not configured")
	}

	ops := queueOps{
		runOnceOrchestration: runner.RunOnceOrchestration,
		completed:            runner.Completed,
		failed:               runner.Failed,
	}

	if selectDirs {
		return ops, materializer.materializeDir, nil
	}
	return ops, materializer.materializeFile, nil
}

func (w *queueWorker) run(ctx context.Context) error {
	w.logf("[%s] worker started (backend=%s)", w.name, w.queueKind)
	defer w.logf("[%s] worker stopped", w.name)

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		if err := w.runBatch(ctx); err != nil {
			w.logf("[%s] batch error: %v", w.name, err)
		}

		if w.onceOnly {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.poll):
		}
	}
}

func (w *queueWorker) runBatch(ctx context.Context) error {
	processed := 0
	for {
		if w.batch > 0 && processed >= w.batch {
			return nil
		}

		claim, err := w.ops.runOnceOrchestration(ctx)
		if err != nil {
			return err
		}
		if !claim.Found {
			return nil
		}

		w.logf("[%s] claimed %s", w.name, claim.InProgress)
		w.status.setProcessing(w.name, claim.InProgress)
		localClaim, cleanup, err := w.localize(ctx, claim.InProgress)
		if err != nil {
			w.status.setIdle(w.name)
			failedPath, failErr := w.ops.failed(ctx, claim.InProgress)
			if failErr != nil {
				w.logf("[%s] localize failed and queue failed transition failed: %v (localize err: %v)", w.name, failErr, err)
				processed++
				continue
			}
			w.logf("[%s] localize failed %s -> %s (%v)", w.name, claim.InProgress, failedPath, err)
			processed++
			continue
		}

		handlerErr := w.handler(ctx, localClaim)
		cleanup()
		w.status.setIdle(w.name)

		if handlerErr != nil {
			failedPath, failErr := w.ops.failed(ctx, claim.InProgress)
			if failErr != nil {
				w.logf("[%s] handler failed and queue failed transition failed: %v (handler err: %v)", w.name, failErr, handlerErr)
				processed++
				continue
			}
			w.logf("[%s] failed %s -> %s (%v)", w.name, claim.InProgress, failedPath, handlerErr)
			processed++
			continue
		}

		if err := w.ops.completed(ctx, claim.InProgress); err != nil {
			w.logf("[%s] completed handler but failed to mark complete for %s: %v", w.name, claim.InProgress, err)
			processed++
			continue
		}

		w.logf("[%s] completed %s", w.name, claim.InProgress)
		processed++
	}
}

func newDirectoryRunner(queueDir string, selectDirs bool, batchSize int) (*libfilerunner.DirectoryRunner, error) {
	selectTarget := libfilerunner.SelectTargetFiles
	if selectDirs {
		selectTarget = libfilerunner.SelectTargetDirectories
	}

	runner, err := libfilerunner.NewDirectoryRunner(libfilerunner.DirectoryConfig{
		InputDir:      filepath.Join(queueDir, "input"),
		InProgressDir: filepath.Join(queueDir, "in-progress"),
		FailedDir:     filepath.Join(queueDir, "failed"),
		BatchSize:     batchSize,
		SelectTarget:  selectTarget,
	})
	if err != nil {
		return nil, err
	}

	if err := runner.EnsureDirectories(); err != nil {
		return nil, err
	}

	return runner, nil
}

func newS3Runner(s3cfg S3Config, q WorkerQueueConfig, selectDirs bool, batchSize int) (*libfilerunner.S3Runner, error) {
	selectTarget := libfilerunner.SelectTargetFiles
	if selectDirs {
		selectTarget = libfilerunner.SelectTargetDirectories
	}

	return libfilerunner.NewS3Runner(libfilerunner.S3Config{
		Region:           strings.TrimSpace(s3cfg.Region),
		Bucket:           strings.TrimSpace(s3cfg.Bucket),
		InputPrefix:      strings.TrimSpace(q.InputPrefix),
		InProgressPrefix: strings.TrimSpace(q.InProgressPrefix),
		FailedPrefix:     strings.TrimSpace(q.FailedPrefix),
		BatchSize:        batchSize,
		SelectTarget:     selectTarget,
	})
}
