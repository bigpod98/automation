package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	libfilerunner "github.com/bigpod98/libfilerunner-go/pkg"
)

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
