package service

import (
	"context"
	"encoding/json"
	"strings"

	"xiadown/internal/domain/library"
)

const operationThumbnailPreviewPathKey = "thumbnailPreviewPath"

func withOperationThumbnailPreviewPath(outputJSON string, thumbnailPath string) (string, bool) {
	trimmedPath := strings.TrimSpace(thumbnailPath)
	if trimmedPath == "" {
		return outputJSON, false
	}

	payload := map[string]any{}
	if strings.TrimSpace(outputJSON) != "" {
		_ = json.Unmarshal([]byte(outputJSON), &payload)
		if payload == nil {
			payload = map[string]any{}
		}
	}

	if existing, ok := payload[operationThumbnailPreviewPathKey].(string); ok && strings.TrimSpace(existing) == trimmedPath {
		return outputJSON, false
	}
	payload[operationThumbnailPreviewPathKey] = trimmedPath

	encoded, err := json.Marshal(payload)
	if err != nil {
		return outputJSON, false
	}
	return string(encoded), true
}

func extractOperationThumbnailPreviewPath(outputJSON string) string {
	if strings.TrimSpace(outputJSON) == "" {
		return ""
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(outputJSON), &payload); err != nil {
		return ""
	}
	thumbnailPath := strings.TrimSpace(getString(payload, operationThumbnailPreviewPathKey))
	if thumbnailPath == "" || !pathExists(thumbnailPath) {
		return ""
	}
	return thumbnailPath
}

func (service *LibraryService) persistedYTDLPThumbnailPreviewPath(
	ctx context.Context,
	operation library.LibraryOperation,
) string {
	previewPath := strings.TrimSpace(extractOperationThumbnailPreviewPath(operation.OutputJSON))
	if previewPath != "" {
		return previewPath
	}
	if service == nil || service.operations == nil || strings.TrimSpace(operation.ID) == "" {
		return ""
	}
	latest, err := service.operations.Get(ctx, operation.ID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(extractOperationThumbnailPreviewPath(latest.OutputJSON))
}
