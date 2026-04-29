package library

import (
	"strings"
	"time"
)

const (
	DreamFMLocalTrackAvailable = "available"
	DreamFMLocalTrackMissing   = "missing"
)

type DreamFMLocalTrack struct {
	FileID         string
	LibraryID      string
	LocalPath      string
	Title          string
	Author         string
	CoverLocalPath string
	Format         string
	AudioCodec     string
	DurationMs     *int64
	SizeBytes      *int64
	ModTimeUnix    int64
	Availability   string
	LastCheckedAt  time.Time
	ProbeError     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DreamFMLocalTrackParams struct {
	FileID         string
	LibraryID      string
	LocalPath      string
	Title          string
	Author         string
	CoverLocalPath string
	Format         string
	AudioCodec     string
	DurationMs     *int64
	SizeBytes      *int64
	ModTimeUnix    int64
	Availability   string
	LastCheckedAt  *time.Time
	ProbeError     string
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

type DreamFMLocalTrackListOptions struct {
	Query              string
	IncludeUnavailable bool
	Limit              int
	Offset             int
}

func NewDreamFMLocalTrack(params DreamFMLocalTrackParams) (DreamFMLocalTrack, error) {
	fileID := strings.TrimSpace(params.FileID)
	libraryID := strings.TrimSpace(params.LibraryID)
	localPath := strings.TrimSpace(params.LocalPath)
	title := strings.TrimSpace(params.Title)
	availability := strings.TrimSpace(params.Availability)
	if availability == "" {
		availability = DreamFMLocalTrackAvailable
	}
	if fileID == "" || libraryID == "" || localPath == "" || title == "" {
		return DreamFMLocalTrack{}, ErrInvalidLibraryFile
	}
	switch availability {
	case DreamFMLocalTrackAvailable, DreamFMLocalTrackMissing:
	default:
		return DreamFMLocalTrack{}, ErrInvalidLibraryFile
	}
	now := time.Now().UTC()
	lastCheckedAt := now
	if params.LastCheckedAt != nil && !params.LastCheckedAt.IsZero() {
		lastCheckedAt = params.LastCheckedAt.UTC()
	}
	createdAt := now
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	return DreamFMLocalTrack{
		FileID:         fileID,
		LibraryID:      libraryID,
		LocalPath:      localPath,
		Title:          title,
		Author:         strings.TrimSpace(params.Author),
		CoverLocalPath: strings.TrimSpace(params.CoverLocalPath),
		Format:         strings.TrimSpace(params.Format),
		AudioCodec:     strings.TrimSpace(params.AudioCodec),
		DurationMs:     params.DurationMs,
		SizeBytes:      params.SizeBytes,
		ModTimeUnix:    params.ModTimeUnix,
		Availability:   availability,
		LastCheckedAt:  lastCheckedAt,
		ProbeError:     strings.TrimSpace(params.ProbeError),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}
