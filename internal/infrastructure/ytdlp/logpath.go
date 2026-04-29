package ytdlp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func BuildLogPath(downloadDirectory string, jobID string) (string, error) {
	trimmed := strings.TrimSpace(downloadDirectory)
	if trimmed == "" {
		return "", fmt.Errorf("download directory not found")
	}
	logDir := filepath.Join(trimmed, "xiadown", "yt-dlp", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s.log", strings.TrimSpace(jobID))
	if filename == ".log" {
		filename = fmt.Sprintf("ytdlp-%d.log", time.Now().Unix())
	}
	return filepath.Join(logDir, filename), nil
}

func DefaultLogPath(downloadDirectory string, jobID string) string {
	trimmed := strings.TrimSpace(downloadDirectory)
	if trimmed == "" {
		return ""
	}
	filename := fmt.Sprintf("%s.log", strings.TrimSpace(jobID))
	if filename == ".log" {
		return ""
	}
	return filepath.Join(trimmed, "xiadown", "yt-dlp", "logs", filename)
}

func ResolveLogPathFromMetadata(metadataJSON string) string {
	payload := parseJSONMap(metadataJSON)
	if len(payload) == 0 {
		return ""
	}
	logEntry, ok := payload["log"].(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(getString(logEntry, "path"))
}

func parseJSONMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}
