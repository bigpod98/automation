package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	QueueBackend string `yaml:"queue_backend"`

	PollIntervalRaw        string   `yaml:"poll_interval"`
	BatchSize              int      `yaml:"batch_size"`
	AllowedAudioExtensions []string `yaml:"allowed_audio_extensions"`

	CoverDownloadTimeoutRaw string `yaml:"cover_download_timeout"`
	CoverDownloadMaxBytes   int64  `yaml:"cover_download_max_bytes"`

	APIListen string `yaml:"api_listen"`

	S3     S3Config    `yaml:"s3"`
	Queues QueueConfig `yaml:"queues"`

	PollInterval         time.Duration `yaml:"-"`
	CoverDownloadTimeout time.Duration `yaml:"-"`
}

type S3Config struct {
	Region     string `yaml:"region"`
	Bucket     string `yaml:"bucket"`
	StagingDir string `yaml:"staging_dir"`
}

type QueueConfig struct {
	Jivetalking        WorkerQueueConfig `yaml:"jivetalking"`
	JivedropStandalone WorkerQueueConfig `yaml:"jivedrop_standalone"`
	JivedropHugo       WorkerQueueConfig `yaml:"jivedrop_hugo"`
	JivefireStandalone WorkerQueueConfig `yaml:"jivefire_standalone"`
}

type WorkerQueueConfig struct {
	Enabled   bool   `yaml:"enabled"`
	QueueDir  string `yaml:"queue_dir"`
	OutputDir string `yaml:"output_dir"`

	InputPrefix      string `yaml:"input_prefix"`
	InProgressPrefix string `yaml:"in_progress_prefix"`
	FailedPrefix     string `yaml:"failed_prefix"`
}

func DefaultConfig() Config {
	return Config{
		QueueBackend:           "s3",
		PollIntervalRaw:        "5s",
		BatchSize:              5,
		AllowedAudioExtensions: []string{".wav", ".flac", ".mp3", ".m4a", ".aac", ".ogg"},

		APIListen:               ":8080",
		CoverDownloadTimeoutRaw: "30s",
		CoverDownloadMaxBytes:   15 * 1024 * 1024,
		S3: S3Config{
			Region:     "",
			Bucket:     "",
			StagingDir: "queues/staging",
		},
		Queues: QueueConfig{
			Jivetalking: WorkerQueueConfig{
				Enabled:          true,
				QueueDir:         "queues/jivetalking",
				OutputDir:        "queues/jivetalking/output",
				InputPrefix:      "automation/jivetalking/input/",
				InProgressPrefix: "automation/jivetalking/in-progress/",
				FailedPrefix:     "automation/jivetalking/failed/",
			},
			JivedropStandalone: WorkerQueueConfig{
				Enabled:          true,
				QueueDir:         "queues/jivedrop/standalone",
				OutputDir:        "queues/jivedrop/standalone/output",
				InputPrefix:      "automation/jivedrop/standalone/input/",
				InProgressPrefix: "automation/jivedrop/standalone/in-progress/",
				FailedPrefix:     "automation/jivedrop/standalone/failed/",
			},
			JivedropHugo: WorkerQueueConfig{
				Enabled:          true,
				QueueDir:         "queues/jivedrop/hugo",
				OutputDir:        "queues/jivedrop/hugo/output",
				InputPrefix:      "automation/jivedrop/hugo/input/",
				InProgressPrefix: "automation/jivedrop/hugo/in-progress/",
				FailedPrefix:     "automation/jivedrop/hugo/failed/",
			},
			JivefireStandalone: WorkerQueueConfig{
				Enabled:          true,
				QueueDir:         "queues/jivefire/standalone",
				OutputDir:        "queues/jivefire/standalone/output",
				InputPrefix:      "automation/jivefire/standalone/input/",
				InProgressPrefix: "automation/jivefire/standalone/in-progress/",
				FailedPrefix:     "automation/jivefire/standalone/failed/",
			},
		},
	}
}

func LoadConfig(configPath string) (Config, error) {
	cfg := DefaultConfig()

	b, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("failed to parse config %q: %w", configPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to read config %q: %w", configPath, err)
	}

	if err := cfg.normalise(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) normalise() error {
	poll, err := time.ParseDuration(strings.TrimSpace(c.PollIntervalRaw))
	if err != nil {
		return fmt.Errorf("invalid poll_interval: %w", err)
	}
	if poll <= 0 {
		return fmt.Errorf("poll_interval must be > 0")
	}

	coverTimeout, err := time.ParseDuration(strings.TrimSpace(c.CoverDownloadTimeoutRaw))
	if err != nil {
		return fmt.Errorf("invalid cover_download_timeout: %w", err)
	}
	if coverTimeout <= 0 {
		return fmt.Errorf("cover_download_timeout must be > 0")
	}

	if c.BatchSize < 0 {
		return fmt.Errorf("batch_size must be >= 0")
	}
	if c.CoverDownloadMaxBytes <= 0 {
		return fmt.Errorf("cover_download_max_bytes must be > 0")
	}

	c.QueueBackend = strings.ToLower(strings.TrimSpace(c.QueueBackend))
	if c.QueueBackend == "" {
		c.QueueBackend = "s3"
	}
	if c.QueueBackend != "s3" && c.QueueBackend != "directory" {
		return fmt.Errorf("queue_backend must be either s3 or directory")
	}
	if c.QueueBackend == "s3" {
		if strings.TrimSpace(c.S3.Bucket) == "" {
			return fmt.Errorf("s3.bucket is required when queue_backend is s3")
		}
		if strings.TrimSpace(c.S3.StagingDir) == "" {
			return fmt.Errorf("s3.staging_dir is required when queue_backend is s3")
		}
	}

	exts := make([]string, 0, len(c.AllowedAudioExtensions))
	for _, ext := range c.AllowedAudioExtensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts = append(exts, ext)
	}
	if len(exts) == 0 {
		return fmt.Errorf("allowed_audio_extensions must contain at least one extension")
	}

	c.AllowedAudioExtensions = exts
	c.PollInterval = poll
	c.CoverDownloadTimeout = coverTimeout

	for name, q := range map[string]WorkerQueueConfig{
		"queues.jivetalking":         c.Queues.Jivetalking,
		"queues.jivedrop_standalone": c.Queues.JivedropStandalone,
		"queues.jivedrop_hugo":       c.Queues.JivedropHugo,
		"queues.jivefire_standalone": c.Queues.JivefireStandalone,
	} {
		if !q.Enabled {
			continue
		}
		if strings.TrimSpace(q.OutputDir) == "" {
			return fmt.Errorf("%s.output_dir is required when enabled", name)
		}
		if c.QueueBackend == "directory" && strings.TrimSpace(q.QueueDir) == "" {
			return fmt.Errorf("%s.queue_dir is required when queue_backend is directory", name)
		}
		if c.QueueBackend == "s3" {
			if strings.TrimSpace(q.InputPrefix) == "" {
				return fmt.Errorf("%s.input_prefix is required when queue_backend is s3", name)
			}
			if strings.TrimSpace(q.InProgressPrefix) == "" {
				return fmt.Errorf("%s.in_progress_prefix is required when queue_backend is s3", name)
			}
			if strings.TrimSpace(q.FailedPrefix) == "" {
				return fmt.Errorf("%s.failed_prefix is required when queue_backend is s3", name)
			}
		}
	}

	c.Queues.Jivetalking.QueueDir = filepath.Clean(c.Queues.Jivetalking.QueueDir)
	c.Queues.Jivetalking.OutputDir = filepath.Clean(c.Queues.Jivetalking.OutputDir)
	c.Queues.JivedropStandalone.QueueDir = filepath.Clean(c.Queues.JivedropStandalone.QueueDir)
	c.Queues.JivedropStandalone.OutputDir = filepath.Clean(c.Queues.JivedropStandalone.OutputDir)
	c.Queues.JivedropHugo.QueueDir = filepath.Clean(c.Queues.JivedropHugo.QueueDir)
	c.Queues.JivedropHugo.OutputDir = filepath.Clean(c.Queues.JivedropHugo.OutputDir)
	c.Queues.JivefireStandalone.QueueDir = filepath.Clean(c.Queues.JivefireStandalone.QueueDir)
	c.Queues.JivefireStandalone.OutputDir = filepath.Clean(c.Queues.JivefireStandalone.OutputDir)
	c.S3.StagingDir = filepath.Clean(c.S3.StagingDir)

	return nil
}
