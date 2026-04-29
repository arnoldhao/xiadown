package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func (service *LibraryService) markLibraryFileDeleted(ctx context.Context, item library.LibraryFile, deleteLocal bool) error {
	if deleteLocal {
		if err := deleteLocalFileIfExists(item.Storage.LocalPath); err != nil {
			return err
		}
	}
	if service != nil && service.subtitles != nil {
		if err := service.subtitles.DeleteByFileID(ctx, item.ID); err != nil {
			return err
		}
	}
	if service != nil && service.localTracks != nil {
		if err := service.localTracks.Delete(ctx, item.ID); err != nil {
			return err
		}
	}
	now := service.now()
	item.State.Status = "deleted"
	item.State.Deleted = true
	item.UpdatedAt = now
	if err := service.files.Save(ctx, item); err != nil {
		return err
	}
	if err := service.touchLibrary(ctx, item.LibraryID, now); err != nil {
		return err
	}
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
	return nil
}
