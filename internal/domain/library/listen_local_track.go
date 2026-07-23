package library

import (
	"strings"
	"time"
)

const (
	ListenLocalTrackAvailable = "available"
	ListenLocalTrackMissing   = "missing"
)

type ListenLocalTrack struct {
	FileID    string
	LibraryID string
	Revision  int64
	// ContentIdentityRevision identifies the logical audio timeline. It is
	// deliberately independent from metadata/resource revisions so tag-only
	// rewrites and equivalent representations do not invalidate resume state.
	ContentIdentityRevision int64
	// ContentIdentitySignature is a private fingerprint of the encoded audio
	// timeline. It is persisted locally but never exposed through paired APIs.
	// This separates a same-duration content replacement from tag/artwork-only
	// byte rewrites that should preserve playback progress.
	ContentIdentitySignature string `json:"-"`
	// MetadataRevision is the conflict/cache identity for editable descriptive
	// tags. ResourceRevision identifies the current set/version of byte
	// resources. Both are separate from the all-observable entity Revision.
	MetadataRevision int64
	ResourceRevision int64
	LocalPath        string
	Title            string
	Author           string
	Album            string
	AlbumArtist      string
	Genre            string
	TrackNumber      int
	DiscNumber       int
	Year             int
	CoverLocalPath   string
	Format           string
	AudioCodec       string
	DurationMs       *int64
	SizeBytes        *int64
	ModTimeUnix      int64
	Availability     string
	LastCheckedAt    time.Time
	ProbeError       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ListenLocalTrackParams struct {
	FileID                   string
	LibraryID                string
	Revision                 int64
	ContentIdentityRevision  int64
	ContentIdentitySignature string
	MetadataRevision         int64
	ResourceRevision         int64
	LocalPath                string
	Title                    string
	Author                   string
	Album                    string
	AlbumArtist              string
	Genre                    string
	TrackNumber              int
	DiscNumber               int
	Year                     int
	CoverLocalPath           string
	Format                   string
	AudioCodec               string
	DurationMs               *int64
	SizeBytes                *int64
	ModTimeUnix              int64
	Availability             string
	LastCheckedAt            *time.Time
	ProbeError               string
	CreatedAt                *time.Time
	UpdatedAt                *time.Time
}

type ListenLocalTrackListOptions struct {
	FileIDs            []string
	Query              string
	Artist             string
	Album              string
	Sort               string
	IncludeUnavailable bool
	Limit              int
	Offset             int
}

func NewListenLocalTrack(params ListenLocalTrackParams) (ListenLocalTrack, error) {
	fileID := strings.TrimSpace(params.FileID)
	libraryID := strings.TrimSpace(params.LibraryID)
	localPath := strings.TrimSpace(params.LocalPath)
	title := strings.TrimSpace(params.Title)
	availability := strings.TrimSpace(params.Availability)
	if availability == "" {
		availability = ListenLocalTrackAvailable
	}
	if fileID == "" || libraryID == "" || localPath == "" || title == "" {
		return ListenLocalTrack{}, ErrInvalidLibraryFile
	}
	revision := params.Revision
	if revision == 0 {
		revision = 1
	}
	contentIdentityRevision := params.ContentIdentityRevision
	if contentIdentityRevision == 0 {
		contentIdentityRevision = 1
	}
	metadataRevision := params.MetadataRevision
	if metadataRevision == 0 {
		metadataRevision = 1
	}
	resourceRevision := params.ResourceRevision
	if resourceRevision == 0 {
		resourceRevision = 1
	}
	if revision < 1 || contentIdentityRevision < 1 || metadataRevision < 1 || resourceRevision < 1 {
		return ListenLocalTrack{}, ErrInvalidLibraryFile
	}
	switch availability {
	case ListenLocalTrackAvailable, ListenLocalTrackMissing:
	default:
		return ListenLocalTrack{}, ErrInvalidLibraryFile
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
	return ListenLocalTrack{
		FileID:                   fileID,
		LibraryID:                libraryID,
		Revision:                 revision,
		ContentIdentityRevision:  contentIdentityRevision,
		ContentIdentitySignature: strings.TrimSpace(params.ContentIdentitySignature),
		MetadataRevision:         metadataRevision,
		ResourceRevision:         resourceRevision,
		LocalPath:                localPath,
		Title:                    title,
		Author:                   strings.TrimSpace(params.Author),
		Album:                    strings.TrimSpace(params.Album),
		AlbumArtist:              strings.TrimSpace(params.AlbumArtist),
		Genre:                    strings.TrimSpace(params.Genre),
		TrackNumber:              max(params.TrackNumber, 0),
		DiscNumber:               max(params.DiscNumber, 0),
		Year:                     max(params.Year, 0),
		CoverLocalPath:           strings.TrimSpace(params.CoverLocalPath),
		Format:                   strings.TrimSpace(params.Format),
		AudioCodec:               strings.TrimSpace(params.AudioCodec),
		DurationMs:               params.DurationMs,
		SizeBytes:                params.SizeBytes,
		ModTimeUnix:              params.ModTimeUnix,
		Availability:             availability,
		LastCheckedAt:            lastCheckedAt,
		ProbeError:               strings.TrimSpace(params.ProbeError),
		CreatedAt:                createdAt,
		UpdatedAt:                updatedAt,
	}, nil
}
