package service

import (
	"path/filepath"
	"strings"

	"xiadown/internal/domain/library"
)

func resolveLibraryFileTitle(item library.LibraryFile, fallback string) string {
	return firstNonEmpty(
		item.Metadata.Title,
		item.DisplayName,
		fallback,
		resolveLibraryNameFromFile(item.Storage.LocalPath),
		resolveLibraryNameFromFile(item.Name),
	)
}

func resolveLibraryFileDisplayName(item library.LibraryFile) string {
	return firstNonEmpty(
		item.DisplayName,
		item.Metadata.Title,
		item.Name,
		resolveLibraryNameFromFile(item.Storage.LocalPath),
	)
}

func resolveLibraryFileName(item library.LibraryFile) string {
	localPath := strings.TrimSpace(item.Storage.LocalPath)
	if localPath == "" {
		return resolveLibraryNameFromFile(item.Name)
	}
	base := strings.TrimSpace(filepath.Base(localPath))
	if base == "" || base == "." {
		return resolveLibraryNameFromFile(item.Name)
	}
	return base
}

func resolveStoredFileName(path string, fallback string) string {
	return firstNonEmpty(
		resolveLibraryNameFromFile(path),
		resolveLibraryNameFromFile(fallback),
	)
}

func buildDownloadFileMetadata(operation library.LibraryOperation, title string) library.FileMetadata {
	return library.FileMetadata{
		Title:     strings.TrimSpace(title),
		Author:    strings.TrimSpace(operation.Meta.Uploader),
		Extractor: strings.TrimSpace(operation.Meta.Platform),
	}
}

func buildTranscodeFileMetadata(sourceFile library.LibraryFile, fallbackTitle string) library.FileMetadata {
	return library.FileMetadata{
		Title:     resolveLibraryFileTitle(sourceFile, fallbackTitle),
		Author:    strings.TrimSpace(sourceFile.Metadata.Author),
		Extractor: strings.TrimSpace(sourceFile.Metadata.Extractor),
	}
}
