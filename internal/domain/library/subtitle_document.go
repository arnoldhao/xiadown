package library

import (
	"strings"
	"time"
)

type SubtitleDocument struct {
	ID              string
	FileID          string
	LibraryID       string
	Format          string
	OriginalContent string
	WorkingContent  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SubtitleDocumentParams struct {
	ID              string
	FileID          string
	LibraryID       string
	Format          string
	OriginalContent string
	WorkingContent  string
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
}

func NewSubtitleDocument(params SubtitleDocumentParams) (SubtitleDocument, error) {
	id := strings.TrimSpace(params.ID)
	fileID := strings.TrimSpace(params.FileID)
	libraryID := strings.TrimSpace(params.LibraryID)
	format := strings.TrimSpace(params.Format)
	if id == "" || fileID == "" || libraryID == "" || format == "" {
		return SubtitleDocument{}, ErrInvalidSubtitleDocument
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	working := params.WorkingContent
	if strings.TrimSpace(working) == "" {
		working = params.OriginalContent
	}
	return SubtitleDocument{
		ID:              id,
		FileID:          fileID,
		LibraryID:       libraryID,
		Format:          format,
		OriginalContent: params.OriginalContent,
		WorkingContent:  working,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}
