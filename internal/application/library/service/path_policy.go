package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (service *LibraryService) isAgentSource(source string) bool {
	return strings.EqualFold(strings.TrimSpace(source), "agent")
}

func (service *LibraryService) resolveAgentPath(ctx context.Context, path string, allowTemp bool, requireExists bool) (string, error) {
	roots, err := service.resolveAgentAllowedRoots(ctx, allowTemp)
	if err != nil {
		return "", err
	}
	return resolveAllowedPath(path, roots, requireExists)
}

func (service *LibraryService) resolveAgentAllowedRoots(ctx context.Context, allowTemp bool) ([]string, error) {
	roots := make([]string, 0, 3)
	if baseDir, err := libraryBaseDir(); err == nil {
		roots = append(roots, baseDir)
	}
	if downloadDir, err := service.resolveDownloadDirectory(ctx); err == nil {
		downloadRoot := filepath.Join(downloadDir, "xiadown")
		roots = append(roots, downloadRoot)
	}
	if allowTemp {
		if tempDir := strings.TrimSpace(os.TempDir()); tempDir != "" {
			roots = append(roots, tempDir)
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no allowed roots resolved for agent path")
	}
	return roots, nil
}

func resolveAllowedPath(path string, roots []string, requireExists bool) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(trimmed) {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", err
		}
		if requireExists {
			if _, err := os.Stat(abs); err != nil {
				return "", err
			}
		}
		if isPathWithinRoots(abs, roots) {
			return abs, nil
		}
		return "", fmt.Errorf("path is outside allowed roots")
	}

	for _, root := range roots {
		rootTrimmed := strings.TrimSpace(root)
		if rootTrimmed == "" {
			continue
		}
		candidate := filepath.Join(rootTrimmed, trimmed)
		if requireExists {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if isPathWithinRoots(abs, roots) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path must be absolute or under allowed roots")
}

func isPathWithinRoots(path string, roots []string) bool {
	for _, root := range roots {
		rootTrimmed := strings.TrimSpace(root)
		if rootTrimmed == "" {
			continue
		}
		rootAbs, err := filepath.Abs(rootTrimmed)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
			return true
		}
	}
	return false
}
