package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

const missingLocalFileError = "missing_local_file"

type localFilePresence uint8

const (
	localFilePresenceIndeterminate localFilePresence = iota
	localFilePresenceAvailable
	localFilePresenceMissing
)

type libraryFilePresenceResult struct {
	item     library.LibraryFile
	presence localFilePresence
	eligible bool
	updated  bool
}

// inspectAndPersistLibraryFilePresence reloads the file while holding the same
// per-file shard as rename, relink, delete, and metadata edits. Availability
// activity and returned projections therefore use the committed display name,
// never the title from an earlier List snapshot.
func (service *LibraryService) inspectAndPersistLibraryFilePresence(
	ctx context.Context,
	fileID string,
	updateState bool,
	persistAvailableCheck bool,
	now time.Time,
) (libraryFilePresenceResult, error) {
	result := libraryFilePresenceResult{}
	if service == nil || service.files == nil {
		return result, nil
	}
	unlock := service.lockListenLocalTrackMutation(fileID)
	defer unlock()

	item, err := service.files.Get(ctx, fileID)
	if err != nil {
		if errors.Is(err, library.ErrFileNotFound) {
			return result, nil
		}
		return result, err
	}
	if item.State.Deleted || strings.TrimSpace(item.Storage.LocalPath) == "" {
		return result, nil
	}
	result.item = item
	result.eligible = true
	result.presence = inspectLocalFilePresence(item.Storage.LocalPath)
	if !updateState || result.presence == localFilePresenceIndeterminate {
		return result, nil
	}

	before := item
	switch result.presence {
	case localFilePresenceAvailable:
		if !persistAvailableCheck && item.State.LastError != missingLocalFileError {
			return result, nil
		}
		if item.State.LastError == missingLocalFileError {
			item.State.LastError = ""
		}
	case localFilePresenceMissing:
		item.State.LastError = missingLocalFileError
	}
	item.State.LastChecked = now.Format(time.RFC3339)
	item.UpdatedAt = now
	if err := service.saveLibraryFilePreservingDisplayName(ctx, item); err != nil {
		return result, err
	}
	if err := service.appendLibraryFileAvailabilityTransition(ctx, before, item, now); err != nil {
		return result, err
	}
	result.item = item
	result.updated = true
	return result, nil
}

func (service *LibraryService) VerifyLibraryFiles(ctx context.Context) (dto.VerifyLibraryFilesResponse, error) {
	response := dto.VerifyLibraryFilesResponse{}
	if service == nil || service.files == nil {
		return response, nil
	}
	items, err := service.files.List(ctx)
	if err != nil {
		return response, err
	}
	now := service.now()
	updatedItems := make([]library.LibraryFile, 0)
	for _, candidate := range items {
		presence, err := service.inspectAndPersistLibraryFilePresence(ctx, candidate.ID, true, true, now)
		if err != nil {
			return response, err
		}
		if !presence.eligible {
			continue
		}
		response.Checked++
		if presence.presence == localFilePresenceMissing {
			response.Missing++
			service.syncListenLocalTrackFromFile(ctx, presence.item, nil)
		}
		if presence.updated {
			updatedItems = append(updatedItems, presence.item)
		}
	}
	if len(updatedItems) > 0 {
		if err := service.syncCatalogProjection(ctx, libraryFileLibraryIDs(updatedItems)...); err != nil {
			return response, err
		}
		for _, item := range updatedItems {
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
		}
	}
	return response, nil
}

func (service *LibraryService) ClearMissingLibraryFiles(ctx context.Context) (dto.ClearMissingLibraryFilesResponse, error) {
	response := dto.ClearMissingLibraryFilesResponse{}
	if service == nil || service.files == nil {
		return response, nil
	}
	items, err := service.files.List(ctx)
	if err != nil {
		return response, err
	}
	for _, item := range items {
		if item.State.Deleted || strings.TrimSpace(item.Storage.LocalPath) == "" {
			continue
		}
		response.Checked++
		if !localFileDefinitelyMissing(item.Storage.LocalPath) {
			continue
		}
		_, deleted, err := service.markLibraryFileDeletedWithOptions(ctx, libraryFileDeleteOptions{
			fileID:        item.ID,
			onlyIfMissing: true,
			eventCategory: "maintenance",
		})
		if err != nil {
			return response, err
		}
		if deleted {
			response.Removed++
		}
	}
	return response, nil
}

// ClearSelectedMissingLibraryFiles removes only the selected stale library
// records. It never deletes a local file. markLibraryFileDeletedWithOptions
// reloads and rechecks each record while holding the same per-file lock used by
// relink and metadata mutations, so a stale scan cannot clear a repaired path.
func (service *LibraryService) ClearSelectedMissingLibraryFiles(
	ctx context.Context,
	request dto.ClearSelectedMissingLibraryFilesRequest,
) (dto.ClearMissingLibraryFilesResponse, error) {
	fileIDs := normalizeFileIDs(request.FileIDs)
	if len(fileIDs) == 0 {
		return dto.ClearMissingLibraryFilesResponse{}, fmt.Errorf("fileIds is required")
	}
	response := dto.ClearMissingLibraryFilesResponse{Checked: len(fileIDs)}
	for _, fileID := range fileIDs {
		_, deleted, err := service.markLibraryFileDeletedWithOptions(ctx, libraryFileDeleteOptions{
			fileID:        fileID,
			onlyIfMissing: true,
			skipIfDeleted: true,
			eventCategory: "maintenance",
			// deleteLocal intentionally remains false: this operation clears only
			// XiaDown's library record and related indexes.
		})
		if err != nil {
			return response, err
		}
		if deleted {
			response.Removed++
		}
	}
	return response, nil
}

func localFileExists(path string) bool {
	return inspectLocalFilePresence(path) == localFilePresenceAvailable
}

func localFileDefinitelyMissing(path string) bool {
	return inspectLocalFilePresence(path) == localFilePresenceMissing
}

func inspectLocalFilePresence(path string) localFilePresence {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return localFilePresenceIndeterminate
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		if os.IsNotExist(err) {
			return localFilePresenceMissing
		}
		return localFilePresenceIndeterminate
	}
	if info == nil || info.IsDir() {
		return localFilePresenceIndeterminate
	}
	return localFilePresenceAvailable
}
