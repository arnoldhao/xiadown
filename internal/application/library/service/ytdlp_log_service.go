package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"xiadown/internal/application/library/dto"
)

const defaultYTDLPLogMaxBytes = 1 << 20

func (service *LibraryService) GetYTDLPOperationLog(ctx context.Context, request dto.GetYTDLPOperationLogRequest) (dto.GetYTDLPOperationLogResponse, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return dto.GetYTDLPOperationLogResponse{}, fmt.Errorf("operation id is required")
	}
	operation, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.GetYTDLPOperationLogResponse{}, err
	}
	if operation.Kind != "download" {
		return dto.GetYTDLPOperationLogResponse{}, fmt.Errorf("operation kind is not download")
	}
	logPath := resolveYTDLPLogPathFromOutputJSON(operation.OutputJSON)
	if logPath == "" {
		if downloadDir, err := service.resolveDownloadDirectory(ctx); err == nil {
			logPath = defaultOperationLogPath(downloadDir, operation.ID)
		}
	}
	if strings.TrimSpace(logPath) == "" {
		return dto.GetYTDLPOperationLogResponse{OperationID: operation.ID}, fmt.Errorf("log path not found")
	}
	content, truncated, err := readYTDLPLogFile(logPath, request.MaxBytes, request.TailLines)
	if err != nil {
		return dto.GetYTDLPOperationLogResponse{OperationID: operation.ID, Path: logPath}, err
	}
	return dto.GetYTDLPOperationLogResponse{OperationID: operation.ID, Path: logPath, Content: content, Truncated: truncated}, nil
}

func resolveYTDLPLogPathFromOutputJSON(outputJSON string) string {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(outputJSON), &payload); err != nil {
		return ""
	}
	if logPath := nestedLogPath(payload["log"]); logPath != "" {
		return logPath
	}
	if logPath := nestedLogPath(payload["metadata"]); logPath != "" {
		return logPath
	}
	return ""
}

func nestedLogPath(raw any) string {
	entry, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	if logEntry, ok := entry["log"].(map[string]any); ok {
		if path, ok := logEntry["path"].(string); ok {
			return strings.TrimSpace(path)
		}
	}
	if path, ok := entry["path"].(string); ok {
		return strings.TrimSpace(path)
	}
	return ""
}

func defaultOperationLogPath(downloadDirectory string, operationID string) string {
	trimmed := strings.TrimSpace(downloadDirectory)
	if trimmed == "" {
		return ""
	}
	filename := strings.TrimSpace(operationID) + ".log"
	if filename == ".log" {
		return ""
	}
	return trimmed + "/xiadown/yt-dlp/logs/" + filename
}

func readYTDLPLogFile(path string, maxBytes int, tailLines int) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", false, err
	}
	if maxBytes <= 0 {
		maxBytes = defaultYTDLPLogMaxBytes
	}
	truncated := false
	if size := info.Size(); size > int64(maxBytes) {
		truncated = true
		offset := size - int64(maxBytes)
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return "", false, err
		}
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", truncated, err
	}
	if truncated {
		if index := bytes.IndexByte(data, '\n'); index >= 0 && index+1 < len(data) {
			data = data[index+1:]
		}
	}
	content := string(data)
	if tailLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > tailLines {
			lines = lines[len(lines)-tailLines:]
			truncated = true
		}
		content = strings.Join(lines, "\n")
	}
	return content, truncated, nil
}
