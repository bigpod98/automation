package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func isAllowedAudio(path string, allowed map[string]struct{}) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := allowed[ext]
	return ok
}

func findSingleAudioFile(dir string, allowed map[string]struct{}) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read claim dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if isAllowedAudio(path, allowed) {
			files = append(files, path)
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("claim must contain one audio file")
	}
	if len(files) > 1 {
		return "", fmt.Errorf("claim must contain exactly one audio file, found %d", len(files))
	}

	return files[0], nil
}

func findSingleMetadataFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read claim dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".json" || ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("claim must contain one metadata file (.json/.yaml/.yml)")
	}
	if len(files) > 1 {
		return "", fmt.Errorf("claim must contain exactly one metadata file, found %d", len(files))
	}

	return files[0], nil
}

func findSingleMarkdownFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read claim dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("claim must contain one markdown file (*.md)")
	}
	if len(files) > 1 {
		return "", fmt.Errorf("claim must contain exactly one markdown file, found %d", len(files))
	}

	return files[0], nil
}

func parseMetadata(path string, dest any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(b, dest); err != nil {
			return fmt.Errorf("invalid JSON metadata: %w", err)
		}
		return nil
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(b, dest); err != nil {
			return fmt.Errorf("invalid YAML metadata: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported metadata file extension for %q", path)
	}
}
