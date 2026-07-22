package service

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const libraryTaskHealthUnavailable = "unavailable"

const (
	databaseIntegrityPending     = "pending"
	databaseIntegrityHealthy     = "healthy"
	databaseIntegrityFailed      = "failed"
	databaseIntegrityUnavailable = "unavailable"
)

func (service *LibraryService) GetDatabaseIntegrityStatus(
	_ context.Context,
) dto.DatabaseIntegrityStatusDTO {
	result := dto.DatabaseIntegrityStatusDTO{State: databaseIntegrityUnavailable}
	if service == nil || service.databaseIntegrity == nil {
		return result
	}
	state, checkedAt, detail := service.databaseIntegrity()
	switch strings.TrimSpace(state) {
	case databaseIntegrityPending, databaseIntegrityHealthy, databaseIntegrityFailed:
		result.State = strings.TrimSpace(state)
	default:
		result.State = databaseIntegrityUnavailable
	}
	result.CheckedAt = strings.TrimSpace(checkedAt)
	if result.State == databaseIntegrityFailed {
		result.Detail = strings.TrimSpace(detail)
	}
	return result
}

// GetLibraryMaintenanceSnapshot inventories the three user-actionable states
// owned by the legacy Library boundary: definitely missing local files,
// recoverable soft-deleted file records, and succeeded tasks that no longer
// have a usable primary output. Catalog trash is layered onto this snapshot by
// the Catalog boundary because it owns logical item lifecycle and revisions.
func (service *LibraryService) GetLibraryMaintenanceSnapshot(
	ctx context.Context,
) (dto.LibraryMaintenanceSnapshotDTO, error) {
	missing, _, err := service.listMissingLibraryFileItems(ctx, true, nil)
	if err != nil {
		return dto.LibraryMaintenanceSnapshotDTO{}, err
	}
	result := dto.LibraryMaintenanceSnapshotDTO{
		CheckedFiles:      missing.Checked,
		MissingFiles:      missing.Missing,
		DeletedFiles:      []dto.DeletedLibraryFileDTO{},
		TaskIssues:        []dto.LibraryTaskMaintenanceDTO{},
		DatabaseIntegrity: service.GetDatabaseIntegrityStatus(ctx),
	}
	if service == nil || service.files == nil {
		return result, nil
	}
	files, err := service.files.List(ctx)
	if err != nil {
		return dto.LibraryMaintenanceSnapshotDTO{}, err
	}
	for _, item := range files {
		if !libraryFileRecoverableDeleted(item) {
			continue
		}
		missingDTO := toMissingLibraryFileDTO(item)
		result.DeletedFiles = append(result.DeletedFiles, dto.DeletedLibraryFileDTO{
			FileID:     missingDTO.FileID,
			LibraryID:  missingDTO.LibraryID,
			Name:       missingDTO.Name,
			Kind:       missingDTO.Kind,
			OldPath:    missingDTO.OldPath,
			Format:     missingDTO.Format,
			CanRestore: localRegularFileExists(item.Storage.LocalPath),
			UpdatedAt:  missingDTO.UpdatedAt,
		})
	}
	sort.SliceStable(result.DeletedFiles, func(i, j int) bool {
		if result.DeletedFiles[i].UpdatedAt != result.DeletedFiles[j].UpdatedAt {
			return result.DeletedFiles[i].UpdatedAt > result.DeletedFiles[j].UpdatedAt
		}
		return result.DeletedFiles[i].FileID < result.DeletedFiles[j].FileID
	})
	if service.operations == nil {
		return result, nil
	}
	operations, err := service.operations.List(ctx)
	if err != nil {
		return dto.LibraryMaintenanceSnapshotDTO{}, err
	}
	result.CheckedTasks = len(operations)
	result.TaskIssues = libraryTaskMaintenanceIssues(operations, files)
	return result, nil
}

// RestoreDeletedLibraryFiles restores metadata only when the current path is
// still a regular file. Every selection is reloaded under the same per-file
// lock used by delete/relink, so stale maintenance results cannot resurrect a
// missing or concurrently changed record.
func (service *LibraryService) RestoreDeletedLibraryFiles(
	ctx context.Context,
	request dto.RestoreDeletedLibraryFilesRequest,
) (dto.RestoreDeletedLibraryFilesResponse, error) {
	fileIDs := normalizeFileIDs(request.FileIDs)
	if len(fileIDs) == 0 {
		return dto.RestoreDeletedLibraryFilesResponse{}, nil
	}
	result := dto.RestoreDeletedLibraryFilesResponse{Checked: len(fileIDs)}
	if service == nil || service.files == nil {
		result.Skipped = len(fileIDs)
		return result, nil
	}
	restoredItems := make([]library.LibraryFile, 0, len(fileIDs))
	libraryIDs := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		unlock := service.lockListenLocalTrackMutation(fileID)
		item, err := service.files.Get(ctx, fileID)
		if err != nil {
			unlock()
			return result, err
		}
		if !libraryFileRecoverableDeleted(item) || !localRegularFileExists(item.Storage.LocalPath) {
			unlock()
			result.Skipped++
			continue
		}
		now := service.now()
		before := item
		item.State.Deleted = false
		item.State.Status = "active"
		item.State.LastError = ""
		item.State.LastChecked = now.Format(time.RFC3339)
		item.UpdatedAt = now
		if _, err := service.saveLibraryFileWithEvent(ctx, before, item, appendLibraryFileEventParams{
			EventType:   libraryFileEventRestored,
			Category:    "maintenance",
			OperationID: fileEventOperationID(item),
			FileID:      item.ID,
			LibraryID:   item.LibraryID,
			Before:      fileEventSnapshot(before),
			After:       fileEventSnapshot(item),
			Changes: []dto.FileFieldChangeDTO{{
				Field: "fileLifecycle", Before: "deleted", After: "active",
			}},
			OccurredAt: now,
		}); err != nil {
			unlock()
			return result, err
		}
		unlock()
		if service.libraries != nil {
			if err := service.touchLibrary(ctx, item.LibraryID, now); err != nil {
				return result, err
			}
		}
		restoredItems = append(restoredItems, item)
		libraryIDs = append(libraryIDs, item.LibraryID)
		result.Restored++
	}
	if len(restoredItems) == 0 {
		return result, nil
	}
	if err := service.syncCatalogProjection(ctx, libraryIDs...); err != nil {
		return result, err
	}
	for _, item := range restoredItems {
		service.syncListenLocalTrackFromFile(ctx, item, nil)
		// The record may have been deleted again while the batch projection ran.
		// Re-read and publish under the per-file lock so a stale active event can
		// never be emitted after a newer delete event.
		unlock := service.lockListenLocalTrackMutation(item.ID)
		current, getErr := service.files.Get(ctx, item.ID)
		if getErr == nil && !libraryFileSoftDeleted(current) && localRegularFileExists(current.Storage.LocalPath) {
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, current))
		}
		unlock()
	}
	return result, nil
}

func libraryFileSoftDeleted(item library.LibraryFile) bool {
	return item.State.Deleted || strings.EqualFold(strings.TrimSpace(item.State.Status), "deleted")
}

func localRegularFileExists(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}
	info, err := os.Stat(trimmed)
	return err == nil && info != nil && info.Mode().IsRegular()
}

func libraryTaskMaintenanceIssues(
	operations []library.LibraryOperation,
	files []library.LibraryFile,
) []dto.LibraryTaskMaintenanceDTO {
	filesByID := make(map[string]library.LibraryFile, len(files))
	filesByOperation := make(map[string][]library.LibraryFile)
	filesByRootFileID := make(map[string][]library.LibraryFile)
	for _, file := range files {
		filesByID[file.ID] = file
		operationID := strings.TrimSpace(file.Origin.OperationID)
		if operationID != "" {
			filesByOperation[operationID] = append(filesByOperation[operationID], file)
		}
		if rootFileID := strings.TrimSpace(file.Lineage.RootFileID); rootFileID != "" {
			filesByRootFileID[rootFileID] = append(filesByRootFileID[rootFileID], file)
		}
	}
	issues := make([]dto.LibraryTaskMaintenanceDTO, 0)
	for _, operation := range operations {
		if operation.Status != library.OperationStatusSucceeded {
			continue
		}
		candidates := make(map[string]library.LibraryFile)
		declaredOutputs := make(map[string]library.OperationOutputFile)
		for _, output := range operation.OutputFiles {
			fileID := strings.TrimSpace(output.FileID)
			if fileID == "" || libraryTaskAuxiliaryKind(output.Kind) {
				continue
			}
			declaredOutputs[fileID] = output
			if file, exists := filesByID[fileID]; exists {
				candidates[file.ID] = file
			}
		}
		for _, file := range filesByOperation[operation.ID] {
			candidates[file.ID] = file
		}
		for declaredFileID := range declaredOutputs {
			for _, file := range filesByRootFileID[declaredFileID] {
				candidates[file.ID] = file
			}
		}
		outputCount := 0
		availableCount := 0
		deletedCount := 0
		unavailableCount := 0
		seenOutputs := make(map[string]struct{}, len(candidates)+len(declaredOutputs))
		for fileID, output := range declaredOutputs {
			seenOutputs[fileID] = struct{}{}
			file, exists := candidates[fileID]
			if !exists {
				outputCount++
				unavailableCount++
				if output.Deleted {
					deletedCount++
				}
				continue
			}
			if !libraryTaskPrimaryOutput(file) {
				continue
			}
			outputCount++
			if libraryFileSoftDeleted(file) {
				deletedCount++
				continue
			}
			if catalogFileAvailable(file) {
				availableCount++
				continue
			}
			unavailableCount++
		}
		for fileID, file := range candidates {
			if _, declared := seenOutputs[fileID]; declared {
				continue
			}
			if !libraryTaskPrimaryOutput(file) {
				continue
			}
			outputCount++
			if libraryFileSoftDeleted(file) {
				deletedCount++
				continue
			}
			if catalogFileAvailable(file) {
				availableCount++
				continue
			}
			unavailableCount++
		}
		if availableCount > 0 {
			// A healthy transcode or other playable replacement owns current
			// availability even when an original/intermediate output was deleted.
			continue
		}
		if outputCount == 0 && operation.Metrics.FileCount == 0 {
			continue
		}
		if outputCount == 0 {
			outputCount = operation.Metrics.FileCount
			unavailableCount = outputCount
		}
		issues = append(issues, dto.LibraryTaskMaintenanceDTO{
			OperationID:            operation.ID,
			Name:                   operation.DisplayName,
			ExecutionStatus:        string(operation.Status),
			Health:                 libraryTaskHealthUnavailable,
			OutputCount:            outputCount,
			AvailableOutputCount:   availableCount,
			DeletedOutputCount:     deletedCount,
			UnavailableOutputCount: unavailableCount,
		})
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Name != issues[j].Name {
			return issues[i].Name < issues[j].Name
		}
		return issues[i].OperationID < issues[j].OperationID
	})
	return issues
}

func libraryTaskPrimaryOutput(file library.LibraryFile) bool {
	return !libraryTaskAuxiliaryKind(string(file.Kind))
}

func libraryTaskAuxiliaryKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case string(library.FileKindThumbnail), string(library.FileKindSubtitle):
		return true
	default:
		return false
	}
}
