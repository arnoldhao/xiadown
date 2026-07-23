package library

import (
	"context"
	"errors"
	"time"
)

const (
	ListenLocalMusicWorkspaceID = "music-default"
	ListenLocalMusicSubjectID   = "music-owner"

	ListenLocalMusicEntityTrack        = "track"
	ListenLocalMusicEntityPlaylist     = "playlist"
	ListenLocalMusicEntityPlaylistItem = "playlist_item"
	ListenLocalMusicEntityMembership   = "membership"

	ListenLocalMusicResourceOriginal               = "original"
	ListenLocalMusicResourcePlaybackRepresentation = "playbackRepresentation"
	ListenLocalMusicResourceArtwork                = "artwork"
)

var ErrListenLocalMusicSyncResetRequired = errors.New("Listen Local Music synchronization reset required")

var (
	ErrListenLocalMusicCompatibleRepresentationUnavailable = errors.New("Listen Local Music compatible representation unavailable")
	ErrListenLocalMusicCompatibleRepresentationConflict    = errors.New("Listen Local Music compatible representation idempotency conflict")
)

const (
	ListenLocalMusicCompatibleRepresentationUnsupported = "unsupported"
	ListenLocalMusicCompatibleRepresentationGenerating  = "generating"
	ListenLocalMusicCompatibleRepresentationFailed      = "failed"
	ListenLocalMusicCompatibleRepresentationReady       = "ready"
)

type ListenLocalMusicSyncPosition struct {
	Epoch         string
	HighWater     int64
	MinimumCursor int64
}

type ListenLocalMusicSyncResetError struct {
	Position ListenLocalMusicSyncPosition
}

func (err *ListenLocalMusicSyncResetError) Error() string {
	return ErrListenLocalMusicSyncResetRequired.Error()
}

func (err *ListenLocalMusicSyncResetError) Unwrap() error {
	return ErrListenLocalMusicSyncResetRequired
}

// ListenLocalMusicResource is an internal, resolved resource. LocalPath and
// FileID are deliberately consumed only by the fixed authenticated content
// route; the paired JSON mapper must never serialize them.
type ListenLocalMusicResource struct {
	ID           string
	FileID       string
	Revision     int64
	Kind         string
	MediaType    string
	Container    string
	Codec        string
	ByteLength   *int64
	ETag         string
	Checksum     string
	Availability string
	LocalPath    string
	// ModTimeUnixNano is an internal TOCTOU guard used between descriptor
	// resolution and no-follow open. Strong byte identity is carried by
	// Checksum; this value is never serialized.
	ModTimeUnixNano int64
}

type ListenLocalMusicTrackProjection struct {
	Track             ListenLocalTrack
	CatalogItemID     string
	PlaybackResources []ListenLocalMusicResource
	ArtworkResource   *ListenLocalMusicResource
}

type ListenLocalMusicPlaylistProjection struct {
	Playlist ListenLocalPlaylist
	Items    []ListenLocalPlaylistItem
}

type ListenLocalMusicCanonicalEntity struct {
	EntityType     string
	EntityID       string
	Revision       int64
	Track          *ListenLocalMusicTrackProjection
	Playlist       *ListenLocalPlaylist
	PlaylistItem   *ListenLocalPlaylistItem
	Membership     *ListenLocalMusicMembership
	TrackState     *ListenLocalMusicTrackState
	LyricDocument  *ListenLocalMusicLyricDocument
	LyricSelection *ListenLocalMusicLyricSelection
	DeletedAt      *time.Time
}

type ListenLocalMusicSnapshotQuery struct {
	Epoch       string
	HighWater   int64
	AfterType   string
	AfterEntity string
	Limit       int
}

type ListenLocalMusicSnapshotPage struct {
	Entities   []ListenLocalMusicCanonicalEntity
	Position   ListenLocalMusicSyncPosition
	NextType   string
	NextEntity string
	HasMore    bool
}

type ListenLocalMusicChange struct {
	Sequence   int64
	EntityType string
	EntityID   string
	Operation  string
	Revision   int64
	OccurredAt time.Time
	Entity     *ListenLocalMusicCanonicalEntity
}

type ListenLocalMusicChangeQuery struct {
	Epoch string
	After int64
	Limit int
}

type ListenLocalMusicChangePage struct {
	Changes  []ListenLocalMusicChange
	Position ListenLocalMusicSyncPosition
	Cursor   int64
	HasMore  bool
}

type ListenLocalMusicPlaylistQuery struct {
	Epoch   string
	AfterID string
	Limit   int
}

type ListenLocalMusicPlaylistPage struct {
	Items   []ListenLocalMusicPlaylistProjection
	NextID  string
	HasMore bool
}

// ListenLocalMusicCompatibleRepresentationStatus is deliberately opaque. It
// reports only the lifecycle needed by a paired Music client; operation IDs,
// local paths, command output, and tool diagnostics remain Desktop-private.
type ListenLocalMusicCompatibleRepresentationStatus struct {
	Status    string
	ErrorCode string
}

// ListenLocalMusicCompatibleRepresentationCoordinator is the narrow bridge
// from the paired Music API to Desktop's existing managed transcode service.
// RequestID is a client-generated UUID retained by the operation journal so a
// transport retry cannot enqueue a second transcode.
type ListenLocalMusicCompatibleRepresentationCoordinator interface {
	GetIOSCompatibleRepresentationStatuses(context.Context, []string) (map[string]ListenLocalMusicCompatibleRepresentationStatus, error)
	RequestIOSCompatibleRepresentation(context.Context, string, string) (ListenLocalMusicCompatibleRepresentationStatus, error)
}

// ListenLocalMusicReadRepository is the paired-device read boundary. It only
// exposes entities already present in the Music index and resources proven to
// belong to a selected Track; it cannot enumerate arbitrary Library files.
type ListenLocalMusicReadRepository interface {
	GetSyncPosition(context.Context) (ListenLocalMusicSyncPosition, error)
	ListSnapshot(context.Context, ListenLocalMusicSnapshotQuery) (ListenLocalMusicSnapshotPage, error)
	ListChanges(context.Context, ListenLocalMusicChangeQuery) (ListenLocalMusicChangePage, error)
	GetTrackProjection(context.Context, string) (ListenLocalMusicTrackProjection, error)
	ListPlaylistProjections(context.Context, ListenLocalMusicPlaylistQuery) (ListenLocalMusicPlaylistPage, error)
	ResolveTrackResource(context.Context, string, string) (ListenLocalMusicResource, error)
}
