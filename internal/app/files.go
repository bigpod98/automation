package app

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func uniqueDestination(dir, fileName string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	base := filepath.Base(fileName)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	}

	for i := 1; ; i++ {
		next := filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))
		if _, err := os.Stat(next); os.IsNotExist(err) {
			return next, nil
		}
	}
}

func moveFileToDir(srcPath, dstDir string) (string, error) {
	dstPath, err := uniqueDestination(dstDir, filepath.Base(srcPath))
	if err != nil {
		return "", err
	}

	if err := os.Rename(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("failed to move %q to %q: %w", srcPath, dstPath, err)
	}

	return dstPath, nil
}

func resolveOptionalPath(path string, baseDir string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func ensureURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("cover URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("cover URL must include host")
	}
	return nil
}

func downloadCover(ctx context.Context, rawURL, destDir string, timeout time.Duration, maxBytes int64) (string, error) {
	if err := ensureURL(rawURL); err != nil {
		return "", err
	}

	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed creating cover download request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed downloading cover URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cover URL returned HTTP %d", resp.StatusCode)
	}

	ext := filepath.Ext(strings.ToLower(resp.Request.URL.Path))
	if ext == "" {
		if contentType := resp.Header.Get("Content-Type"); contentType != "" {
			if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
				ext = exts[0]
			}
		}
	}
	if ext == "" {
		ext = ".img"
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(destDir, "cover-*.tmp"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp cover file: %w", err)
	}
	tmpPath := tmp.Name()

	limited := io.LimitReader(resp.Body, maxBytes+1)
	written, copyErr := io.Copy(tmp, limited)
	closeErr := tmp.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed writing downloaded cover: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed closing downloaded cover: %w", closeErr)
	}
	if written > maxBytes {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("cover file exceeds max size of %d bytes", maxBytes)
	}

	return tmpPath, nil
}
