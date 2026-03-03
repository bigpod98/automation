package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	libfilerunner "github.com/bigpod98/libfilerunner-go/pkg"
)

type config struct {
	Backend       string
	InputDir      string
	InProgressDir string
	FailedDir     string
	PollInterval  time.Duration

	S3Region           string
	S3Bucket           string
	S3InputPrefix      string
	S3InProgressPrefix string
	S3FailedPrefix     string

	AzureAccountURL       string
	AzureContainer        string
	AzureInputPrefix      string
	AzureInProgressPrefix string
	AzureFailedPrefix     string

	JiveTalkingBin string
	JiveFireBin    string
	JiveDropBin    string
}

type jobSpec struct {
	InputAudio    string `json:"input_audio"`
	OutputDir     string `json:"output_dir,omitempty"`
	EpisodeNumber string `json:"episode_number"`
	Title         string `json:"title"`
	ShowTitle     string `json:"show_title,omitempty"`
	CoverArt      string `json:"cover_art"`

	JiveTalking stepSpec       `json:"jivetalking,omitempty"`
	JiveFire    jiveFireSpec   `json:"jivefire,omitempty"`
	JiveDrop    jiveDropSpec   `json:"jivedrop,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type stepSpec struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	ExtraArgs []string `json:"extra_args,omitempty"`
}

type jiveFireSpec struct {
	stepSpec
	OutputPath string   `json:"output_path,omitempty"`
	Channels   int      `json:"channels,omitempty"`
	Encoder    string   `json:"encoder,omitempty"`
	NoPreview  bool     `json:"no_preview,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
}

type jiveDropSpec struct {
	stepSpec
	OutputPath string   `json:"output_path,omitempty"`
	Artist     string   `json:"artist,omitempty"`
	Album      string   `json:"album,omitempty"`
	Date       string   `json:"date,omitempty"`
	Comment    string   `json:"comment,omitempty"`
	Stereo     bool     `json:"stereo,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
}

type jobResult struct {
	InputAudio      string `json:"input_audio"`
	ProcessedAudio  string `json:"processed_audio"`
	VideoOutput     string `json:"video_output,omitempty"`
	PodcastOutput   string `json:"podcast_output,omitempty"`
	CompletedAtUnix int64  `json:"completed_at_unix"`
}

type queueRunner interface {
	RunOnce(ctx context.Context, handler libfilerunner.Handler) (libfilerunner.RunOnceResult, error)
}

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

func processJob(ctx context.Context, cfg config, fileJob libfilerunner.FileJob) error {
	log.Printf("processing job file: %s", fileJob.Name)

	r, err := fileJob.Open()
	if err != nil {
		return fmt.Errorf("open job file: %w", err)
	}
	defer r.Close()

	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read job file: %w", err)
	}

	var job jobSpec
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&job); err != nil {
		return fmt.Errorf("decode job json: %w", err)
	}

	if err := validateJob(job); err != nil {
		return err
	}

	processedAudio := job.InputAudio
	if enabled(job.JiveTalking.Enabled, true) {
		jiveTalkingArgs := append([]string{}, job.JiveTalking.ExtraArgs...)
		jiveTalkingArgs = append(jiveTalkingArgs, job.InputAudio)
		if err := runCommand(ctx, cfg.JiveTalkingBin, jiveTalkingArgs...); err != nil {
			return fmt.Errorf("jivetalking failed: %w", err)
		}
		processedAudio = processedAudioPath(job.InputAudio)
		if _, err := os.Stat(processedAudio); err != nil {
			return fmt.Errorf("processed file missing (%s): %w", processedAudio, err)
		}
	}

	outputDir := job.OutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(job.InputAudio)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir (%s): %w", outputDir, err)
	}

	baseName := strings.TrimSuffix(filepath.Base(processedAudio), filepath.Ext(processedAudio))
	result := jobResult{
		InputAudio:     job.InputAudio,
		ProcessedAudio: processedAudio,
	}

	if enabled(job.JiveFire.Enabled, true) {
		videoOut := job.JiveFire.OutputPath
		if videoOut == "" {
			videoOut = filepath.Join(outputDir, baseName+".mp4")
		}

		showTitle := job.ShowTitle
		if showTitle == "" {
			showTitle = "Podcast"
		}

		jiveFireArgs := []string{processedAudio, videoOut, "--episode", job.EpisodeNumber, "--title", showTitle}
		if job.JiveFire.Channels == 1 || job.JiveFire.Channels == 2 {
			jiveFireArgs = append(jiveFireArgs, "--channels", fmt.Sprintf("%d", job.JiveFire.Channels))
		}
		if job.JiveFire.Encoder != "" {
			jiveFireArgs = append(jiveFireArgs, "--encoder", job.JiveFire.Encoder)
		}
		if job.JiveFire.NoPreview {
			jiveFireArgs = append(jiveFireArgs, "--no-preview")
		}
		jiveFireArgs = append(jiveFireArgs, job.JiveFire.ExtraArgs...)

		if err := runCommand(ctx, cfg.JiveFireBin, jiveFireArgs...); err != nil {
			return fmt.Errorf("jivefire failed: %w", err)
		}
		result.VideoOutput = videoOut
	}

	if enabled(job.JiveDrop.Enabled, true) {
		podcastOut := job.JiveDrop.OutputPath
		if podcastOut == "" {
			podcastOut = filepath.Join(outputDir, baseName+".mp3")
		}

		jiveDropArgs := []string{
			processedAudio,
			"--title", job.Title,
			"--num", job.EpisodeNumber,
			"--cover", job.CoverArt,
			"--output-path", podcastOut,
		}
		if job.JiveDrop.Artist != "" {
			jiveDropArgs = append(jiveDropArgs, "--artist", job.JiveDrop.Artist)
		}
		if job.JiveDrop.Album != "" {
			jiveDropArgs = append(jiveDropArgs, "--album", job.JiveDrop.Album)
		}
		if job.JiveDrop.Date != "" {
			jiveDropArgs = append(jiveDropArgs, "--date", job.JiveDrop.Date)
		}
		if job.JiveDrop.Comment != "" {
			jiveDropArgs = append(jiveDropArgs, "--comment", job.JiveDrop.Comment)
		}
		if job.JiveDrop.Stereo {
			jiveDropArgs = append(jiveDropArgs, "--stereo")
		}
		jiveDropArgs = append(jiveDropArgs, job.JiveDrop.ExtraArgs...)

		if err := runCommand(ctx, cfg.JiveDropBin, jiveDropArgs...); err != nil {
			return fmt.Errorf("jivedrop failed: %w", err)
		}
		result.PodcastOutput = podcastOut
	}

	result.CompletedAtUnix = time.Now().Unix()
	resultPath := filepath.Join(outputDir, baseName+".automation-result.json")
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result payload: %w", err)
	}
	if err := os.WriteFile(resultPath, append(payload, '\n'), 0o644); err != nil {
		return fmt.Errorf("write result payload: %w", err)
	}

	log.Printf("job complete: %s", resultPath)
	return nil
}

func validateJob(job jobSpec) error {
	if strings.TrimSpace(job.InputAudio) == "" {
		return errors.New("job validation: input_audio is required")
	}
	if strings.TrimSpace(job.EpisodeNumber) == "" {
		return errors.New("job validation: episode_number is required")
	}
	if strings.TrimSpace(job.Title) == "" {
		return errors.New("job validation: title is required")
	}
	if enabled(job.JiveDrop.Enabled, true) && strings.TrimSpace(job.CoverArt) == "" {
		return errors.New("job validation: cover_art is required when jivedrop is enabled")
	}
	if _, err := os.Stat(job.InputAudio); err != nil {
		return fmt.Errorf("job validation: input_audio not found: %w", err)
	}
	if enabled(job.JiveDrop.Enabled, true) {
		if _, err := os.Stat(job.CoverArt); err != nil {
			return fmt.Errorf("job validation: cover_art not found: %w", err)
		}
	}
	return nil
}

func runCommand(ctx context.Context, bin string, args ...string) error {
	log.Printf("run: %s %s", bin, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w; output: %s", err, strings.TrimSpace(string(out)))
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed != "" {
		log.Printf("%s output: %s", bin, trimmed)
	}
	return nil
}

func processedAudioPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)
	return base + "-processed" + ext
}

func enabled(val *bool, defaultValue bool) bool {
	if val == nil {
		return defaultValue
	}
	return *val
}

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
