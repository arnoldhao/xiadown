package library

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const ListenLocalPlaylistNameMaxLength = 120

// ListenLocalPlaylist is an app-owned playlist for indexed local media. It is
// intentionally separate from provider playlists such as YouTube Music.
type ListenLocalPlaylist struct {
	ID        string
	Name      string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListenLocalPlaylistParams struct {
	ID        string
	Name      string
	Revision  int64
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type ListenLocalTrackDisplaySnapshot struct {
	Title      string
	Author     string
	Album      string
	DurationMs *int64
}

type ListenLocalPlaylistItem struct {
	ID                   string
	PlaylistID           string
	FileID               string
	Position             int
	Revision             int64
	TrackDisplaySnapshot ListenLocalTrackDisplaySnapshot
	AddedAt              time.Time
}

type ListenLocalPlaylistItemParams struct {
	ID                   string
	PlaylistID           string
	FileID               string
	Position             int
	Revision             int64
	TrackDisplaySnapshot ListenLocalTrackDisplaySnapshot
	AddedAt              *time.Time
}

func NewListenLocalPlaylist(params ListenLocalPlaylistParams) (ListenLocalPlaylist, error) {
	id := strings.TrimSpace(params.ID)
	name := strings.TrimSpace(params.Name)
	if id == "" || name == "" || len([]rune(name)) > ListenLocalPlaylistNameMaxLength {
		return ListenLocalPlaylist{}, ErrInvalidListenLocalPlaylist
	}
	revision := params.Revision
	if revision == 0 {
		revision = 1
	}
	if revision < 1 {
		return ListenLocalPlaylist{}, ErrInvalidListenLocalPlaylist
	}
	now := time.Now().UTC()
	createdAt := now
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	return ListenLocalPlaylist{
		ID:        id,
		Name:      name,
		Revision:  revision,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func NewListenLocalPlaylistItem(playlistID, fileID string, position int, addedAt time.Time) (ListenLocalPlaylistItem, error) {
	return NewListenLocalPlaylistItemWithParams(ListenLocalPlaylistItemParams{
		ID: uuid.NewString(), PlaylistID: playlistID, FileID: fileID, Position: position, AddedAt: &addedAt,
	})
}

func NewListenLocalPlaylistItemWithParams(params ListenLocalPlaylistItemParams) (ListenLocalPlaylistItem, error) {
	id := strings.TrimSpace(params.ID)
	playlistID := strings.TrimSpace(params.PlaylistID)
	fileID := strings.TrimSpace(params.FileID)
	revision := params.Revision
	if revision == 0 {
		revision = 1
	}
	if id == "" || playlistID == "" || fileID == "" || params.Position < 0 || revision < 1 {
		return ListenLocalPlaylistItem{}, ErrInvalidListenLocalPlaylist
	}
	addedAt := time.Time{}
	if params.AddedAt != nil {
		addedAt = *params.AddedAt
	}
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}
	snapshot := params.TrackDisplaySnapshot
	snapshot.Title = strings.TrimSpace(snapshot.Title)
	snapshot.Author = strings.TrimSpace(snapshot.Author)
	snapshot.Album = strings.TrimSpace(snapshot.Album)
	if snapshot.DurationMs != nil {
		value := max(*snapshot.DurationMs, 0)
		snapshot.DurationMs = &value
	}
	return ListenLocalPlaylistItem{
		ID:                   id,
		PlaylistID:           playlistID,
		FileID:               fileID,
		Position:             params.Position,
		Revision:             revision,
		TrackDisplaySnapshot: snapshot,
		AddedAt:              addedAt.UTC(),
	}, nil
}

func ListenLocalTrackSnapshot(track ListenLocalTrack) ListenLocalTrackDisplaySnapshot {
	return ListenLocalTrackDisplaySnapshot{
		Title: track.Title, Author: track.Author, Album: track.Album, DurationMs: track.DurationMs,
	}
}
