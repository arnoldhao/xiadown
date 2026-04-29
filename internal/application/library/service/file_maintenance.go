package service

import (
	"context"
	"os"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
)

const missingLocalFileError = "missing_local_file"

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
	for _, item := range items {
		if item.State.Deleted || strings.TrimSpace(item.Storage.LocalPath) == "" {
			continue
		}
		response.Checked++
		if localFileExists(item.Storage.LocalPath) {
			if item.State.LastError == missingLocalFileError {
				item.State.LastError = ""
			}
			item.State.LastChecked = now.Format(time.RFC3339)
			item.UpdatedAt = now
			if err := service.files.Save(ctx, item); err != nil {
				return response, err
			}
			service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
			continue
		}
		response.Missing++
		item.State.LastError = missingLocalFileError
		item.State.LastChecked = now.Format(time.RFC3339)
		item.UpdatedAt = now
		if err := service.files.Save(ctx, item); err != nil {
			return response, err
		}
		service.syncDreamFMLocalTrackFromFile(ctx, item, nil)
		service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
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
		if localFileExists(item.Storage.LocalPath) {
			continue
		}
		if err := service.markLibraryFileDeleted(ctx, item, false); err != nil {
			return response, err
		}
		response.Removed++
	}
	return response, nil
}

func localFileExists(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && info != nil && !info.IsDir()
}
