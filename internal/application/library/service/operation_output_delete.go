package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const operationDetachedOutputFileIDsKey = "detachedOutputFileIds"

// DeleteOperationOutput removes one file from a task's durable output
// projection. The Library file remains usable unless DeleteFile is explicitly
// requested; this keeps a task-list edit separate from the file lifecycle.
func (service *LibraryService) DeleteOperationOutput(
	ctx context.Context,
	request dto.DeleteOperationOutputRequest,
) (dto.LibraryOperationDTO, error) {
	operationID := strings.TrimSpace(request.OperationID)
	fileID := strings.TrimSpace(request.FileID)
	if operationID == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("operationId is required")
	}
	if fileID == "" {
		return dto.LibraryOperationDTO{}, fmt.Errorf("fileId is required")
	}
	if service == nil || service.operations == nil || service.files == nil {
		return dto.LibraryOperationDTO{}, fmt.Errorf("library service is not configured")
	}
	service.operationOutputMutationMu.Lock()
	defer service.operationOutputMutationMu.Unlock()

	operation, err := service.operations.Get(ctx, operationID)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	if operation.Status != library.OperationStatusSucceeded {
		return dto.LibraryOperationDTO{}, fmt.Errorf(
			"operation status %q does not support output deletion",
			operation.Status,
		)
	}

	remaining, linkedByOutput := withoutOperationOutputFile(operation.OutputFiles, fileID)
	outputSnapshot := findOperationOutputFile(operation.OutputFiles, fileID)
	alreadyDetached := containsDetachedOperationOutputFileID(operation.OutputJSON, fileID)
	fileItem, fileErr := service.files.Get(ctx, fileID)
	if fileErr != nil && fileErr != library.ErrFileNotFound {
		return dto.LibraryOperationDTO{}, fileErr
	}
	linkedByFile := fileErr == nil &&
		fileItem.LibraryID == operation.LibraryID &&
		(fileItem.Origin.OperationID == operationID || fileItem.LatestOperationID == operationID)
	if !linkedByOutput && !linkedByFile && !alreadyDetached {
		return dto.LibraryOperationDTO{}, fmt.Errorf("file %q is not an output of operation %q", fileID, operationID)
	}
	fileAlreadyDeleted := fileErr == nil && libraryFileSoftDeleted(fileItem)
	if alreadyDetached && !linkedByOutput && (!request.DeleteFile || fileErr != nil || fileAlreadyDeleted) {
		return toOperationDTO(operation), nil
	}
	beforeSnapshot := operationOutputFileEventSnapshot(outputSnapshot)
	if fileErr == nil {
		beforeSnapshot = fileEventSnapshot(fileItem)
	}
	localFileWasPresent := fileErr == nil && localRegularFileExists(fileItem.Storage.LocalPath)
	outputJSON, err := withDetachedOperationOutput(
		operation.OutputJSON,
		fileID,
		fileItem.Storage.LocalPath,
	)
	if err != nil {
		return dto.LibraryOperationDTO{}, err
	}

	// Delete the local file before detaching the record. An OS error therefore
	// propagates to the caller while the task projection is still unchanged.
	fileLifecycleChanged := false
	afterFileItem := fileItem
	if request.DeleteFile && fileErr == nil && !fileAlreadyDeleted {
		updated, changed, err := service.markLibraryFileDeletedWithOptions(ctx, libraryFileDeleteOptions{
			fileID:            fileID,
			deleteLocal:       true,
			suppressFileEvent: true,
		})
		if err != nil {
			return dto.LibraryOperationDTO{}, err
		}
		afterFileItem = updated
		fileLifecycleChanged = changed
	}

	operation.OutputFiles = remaining
	operation.OutputJSON = outputJSON
	operation.Metrics = service.rebuildOperationMetricsFromOutputs(
		ctx,
		remaining,
		operation.StartedAt,
		operation.FinishedAt,
	)
	if err := service.operations.Save(ctx, operation); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	changes := make([]dto.FileFieldChangeDTO, 0, 3)
	if !alreadyDetached || linkedByOutput {
		changes = append(changes, dto.FileFieldChangeDTO{
			Field: "taskAssociation", Before: "attached", After: "detached",
		})
	}
	if fileLifecycleChanged {
		changes = append(changes, dto.FileFieldChangeDTO{
			Field: "fileLifecycle", Before: "active", After: "deleted",
		})
		if localFileWasPresent {
			changes = append(changes, dto.FileFieldChangeDTO{
				Field: "localFile", Before: "present", After: "deleted",
			})
		}
	}
	afterSnapshot := cloneFileEventSnapshot(beforeSnapshot)
	if fileErr == nil {
		afterSnapshot = fileEventSnapshot(afterFileItem)
	}
	if len(changes) > 0 {
		if _, err := service.appendLibraryFileEvent(ctx, appendLibraryFileEventParams{
			EventType:   libraryFileEventOperationOutputDetach,
			Category:    "task_output",
			Actor:       libraryFileEventActorDesktop,
			OperationID: operation.ID,
			FileID:      fileID,
			LibraryID:   operation.LibraryID,
			Before:      beforeSnapshot,
			After:       afterSnapshot,
			Changes:     changes,
			DeleteFile:  request.DeleteFile,
			OccurredAt:  service.now(),
		}); err != nil {
			return dto.LibraryOperationDTO{}, err
		}
	}
	if err := service.touchLibrary(ctx, operation.LibraryID, service.now()); err != nil {
		return dto.LibraryOperationDTO{}, err
	}
	service.publishOperationUpdate(toOperationDTO(operation))
	return toOperationDTO(operation), nil
}

func findOperationOutputFile(items []library.OperationOutputFile, fileID string) library.OperationOutputFile {
	trimmedFileID := strings.TrimSpace(fileID)
	for _, item := range items {
		if strings.TrimSpace(item.FileID) == trimmedFileID {
			return item
		}
	}
	return library.OperationOutputFile{FileID: trimmedFileID}
}

func withoutOperationOutputFile(
	items []library.OperationOutputFile,
	fileID string,
) ([]library.OperationOutputFile, bool) {
	trimmedFileID := strings.TrimSpace(fileID)
	result := make([]library.OperationOutputFile, 0, len(items))
	removed := false
	for _, item := range items {
		if strings.TrimSpace(item.FileID) == trimmedFileID {
			removed = true
			continue
		}
		result = append(result, item)
	}
	return result, removed
}

func detachedOperationOutputFileIDs(outputJSON string) []string {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(outputJSON)), &payload); err != nil {
		return nil
	}
	values, ok := payload[operationDetachedOutputFileIDsKey].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		text, ok := value.(string)
		trimmed := strings.TrimSpace(text)
		if !ok || trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func containsDetachedOperationOutputFileID(outputJSON string, fileID string) bool {
	trimmedFileID := strings.TrimSpace(fileID)
	for _, candidate := range detachedOperationOutputFileIDs(outputJSON) {
		if candidate == trimmedFileID {
			return true
		}
	}
	return false
}

func withDetachedOperationOutput(
	outputJSON string,
	fileID string,
	localPath string,
) (string, error) {
	trimmed := strings.TrimSpace(outputJSON)
	if trimmed == "" {
		trimmed = "{}"
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", fmt.Errorf("decode operation output: %w", err)
	}
	if payload == nil {
		payload = map[string]any{}
	}

	detached := detachedOperationOutputFileIDs(trimmed)
	if !containsString(detached, fileID) {
		detached = append(detached, strings.TrimSpace(fileID))
	}
	payload[operationDetachedOutputFileIDsKey] = detached

	if values, ok := payload["outputFiles"].([]any); ok {
		filtered := make([]any, 0, len(values))
		for _, value := range values {
			object, isObject := value.(map[string]any)
			candidate, _ := object["fileId"].(string)
			if isObject && strings.TrimSpace(candidate) == strings.TrimSpace(fileID) {
				continue
			}
			filtered = append(filtered, value)
		}
		payload["outputFiles"] = filtered
	}

	path := strings.TrimSpace(localPath)
	if path != "" {
		for _, key := range operationOutputArtifactStringKeys {
			value, ok := payload[key].(string)
			if ok && sameCleanPath(value, path) {
				delete(payload, key)
			}
		}
		for _, key := range operationOutputFileArtifactListKeys {
			values, ok := payload[key].([]any)
			if !ok {
				continue
			}
			filtered := make([]any, 0, len(values))
			for _, value := range values {
				candidate, isString := value.(string)
				if isString && sameCleanPath(candidate, path) {
					continue
				}
				filtered = append(filtered, value)
			}
			payload[key] = filtered
		}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode operation output: %w", err)
	}
	return string(encoded), nil
}

func containsString(values []string, candidate string) bool {
	trimmed := strings.TrimSpace(candidate)
	for _, value := range values {
		if strings.TrimSpace(value) == trimmed {
			return true
		}
	}
	return false
}
