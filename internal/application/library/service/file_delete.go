package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func deleteLocalFileIfExists(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	cleaned := filepath.Clean(trimmed)
	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("refusing to delete directory: %s", cleaned)
	}
	return os.Remove(cleaned)
}

type libraryFileLatestOperationUpdate struct {
	expected string
	next     string
}

type libraryFileDeleteOptions struct {
	fileID                string
	deleteLocal           bool
	onlyIfMissing         bool
	skipIfDeleted         bool
	suppressFileEvent     bool
	eventCategory         string
	eventActor            string
	eventOperationID      string
	latestOperationUpdate *libraryFileLatestOperationUpdate
}

func (service *LibraryService) markLibraryFileDeleted(ctx context.Context, fileID string, deleteLocal bool) error {
	_, _, err := service.markLibraryFileDeletedWithOptions(ctx, libraryFileDeleteOptions{
		fileID:      fileID,
		deleteLocal: deleteLocal,
	})
	return err
}

func (service *LibraryService) markLibraryFileDeletedWithOptions(
	ctx context.Context,
	options libraryFileDeleteOptions,
) (library.LibraryFile, bool, error) {
	fileID := strings.TrimSpace(options.fileID)
	if service == nil || service.files == nil || fileID == "" {
		return library.LibraryFile{}, false, library.ErrFileNotFound
	}

	unlockLocalTrack := service.lockListenLocalTrackMutation(fileID)
	defer unlockLocalTrack()

	// Delete requests are commonly built from List/Get snapshots. Reload only
	// after taking the same per-file lock used by relink and metadata writes so a
	// stale request cannot remove the old path or save an obsolete record over a
	// relink that committed while it waited.
	item, err := service.files.Get(ctx, fileID)
	if err != nil {
		return library.LibraryFile{}, false, err
	}
	if options.skipIfDeleted && item.State.Deleted {
		return item, false, nil
	}
	if options.onlyIfMissing {
		path := strings.TrimSpace(item.Storage.LocalPath)
		if item.State.Deleted || path == "" || !localFileDefinitelyMissing(path) {
			return item, false, nil
		}
	}
	before := item
	localFileWasPresent := localRegularFileExists(item.Storage.LocalPath)
	if update := options.latestOperationUpdate; update != nil {
		current := strings.TrimSpace(item.LatestOperationID)
		expected := strings.TrimSpace(update.expected)
		next := strings.TrimSpace(update.next)
		// This is a compare-and-set merge, not a stale snapshot overlay. It keeps
		// a genuinely newer operation reference while still supporting transcode
		// cleanup's intentional LatestOperationID update.
		if next != "" && (current == expected || current == next) {
			item.LatestOperationID = next
		}
	}
	if options.deleteLocal {
		if err := deleteLocalFileIfExists(item.Storage.LocalPath); err != nil {
			return library.LibraryFile{}, false, err
		}
	}
	if service != nil && service.subtitles != nil {
		if err := service.subtitles.DeleteByFileID(ctx, item.ID); err != nil {
			return library.LibraryFile{}, false, err
		}
	}
	if service != nil && service.localTracks != nil {
		if err := service.localTracks.Delete(ctx, item.ID); err != nil {
			return library.LibraryFile{}, false, err
		}
	}
	now := service.now()
	item.State.Status = "deleted"
	item.State.Deleted = true
	item.UpdatedAt = now
	if err := service.saveLibraryFilePreservingDisplayName(ctx, item); err != nil {
		return library.LibraryFile{}, false, err
	}
	if !options.suppressFileEvent {
		category := strings.TrimSpace(options.eventCategory)
		if category == "" {
			category = "file_lifecycle"
		}
		changes := []dto.FileFieldChangeDTO{{
			Field:  "fileLifecycle",
			Before: fileLifecycleValue(before),
			After:  "deleted",
		}}
		if options.deleteLocal && localFileWasPresent {
			changes = append(changes, dto.FileFieldChangeDTO{
				Field: "localFile", Before: "present", After: "deleted",
			})
		}
		eventOperationID := strings.TrimSpace(options.eventOperationID)
		if eventOperationID == "" {
			eventOperationID = fileEventOperationID(item)
		}
		if _, err := service.appendLibraryFileEvent(ctx, appendLibraryFileEventParams{
			EventType:   libraryFileEventDeleted,
			Category:    category,
			Actor:       options.eventActor,
			OperationID: eventOperationID,
			FileID:      item.ID,
			LibraryID:   item.LibraryID,
			Before:      fileEventSnapshot(before),
			After:       fileEventSnapshot(item),
			Changes:     changes,
			DeleteFile:  options.deleteLocal,
			OccurredAt:  now,
		}); err != nil {
			return library.LibraryFile{}, false, err
		}
	}
	if err := service.touchLibrary(ctx, item.LibraryID, now); err != nil {
		return library.LibraryFile{}, false, err
	}
	if err := service.syncCatalogProjection(ctx, item.LibraryID); err != nil {
		return library.LibraryFile{}, false, err
	}
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
	return item, true, nil
}
