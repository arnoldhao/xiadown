package library

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const (
	ListenLocalMusicMutationFamilyState  = "state"
	ListenLocalMusicMutationFamilyManage = "manage"

	ListenLocalMusicEntityTrackState     = "track_state"
	ListenLocalMusicEntityLyricDocument  = "lyric_document"
	ListenLocalMusicEntityLyricSelection = "lyric_selection"
)

var (
	ErrInvalidListenLocalMusicMutation     = errors.New("invalid Listen Local Music mutation")
	ErrListenLocalMusicIdempotencyConflict = errors.New("Listen Local Music idempotency conflict")
	ErrListenLocalMusicRevisionConflict    = errors.New("Listen Local Music revision conflict")
	ErrListenLocalMusicDependencyPending   = errors.New("Listen Local Music mutation dependency pending")
	ErrListenLocalMusicContentChanged      = errors.New("Listen Local Music content identity changed")
)

type ListenLocalMusicRevisionConflictError struct {
	CurrentRevision int64
	Current         json.RawMessage
}

func (err *ListenLocalMusicRevisionConflictError) Error() string {
	return ErrListenLocalMusicRevisionConflict.Error()
}

func (err *ListenLocalMusicRevisionConflictError) Unwrap() error {
	return ErrListenLocalMusicRevisionConflict
}

type ListenLocalMusicTrackState struct {
	SubjectID               string    `json:"subjectId"`
	TrackID                 string    `json:"trackId"`
	Revision                int64     `json:"revision"`
	Favorite                bool      `json:"favorite"`
	FavoriteRevision        int64     `json:"favoriteRevision"`
	PositionMs              int64     `json:"positionMs"`
	PlaySessionID           string    `json:"playSessionId,omitempty"`
	ContentIdentityRevision int64     `json:"contentIdentityRevision"`
	ProgressRevision        int64     `json:"progressRevision"`
	CumulativeListenedMs    int64     `json:"cumulativeListenedDurationMs"`
	PlayCount               int64     `json:"playCount"`
	SkipCount               int64     `json:"skipCount"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

type ListenLocalMusicLyricDocument struct {
	ID              string    `json:"id"`
	TrackID         string    `json:"trackId"`
	Revision        int64     `json:"revision"`
	SourceKind      string    `json:"sourceKind"`
	ProviderID      string    `json:"providerId,omitempty"`
	ProviderTrackID string    `json:"providerTrackId,omitempty"`
	TimingKind      string    `json:"timingKind"`
	Language        string    `json:"language,omitempty"`
	ContentHash     string    `json:"contentHash,omitempty"`
	Availability    string    `json:"availability"`
	LicensePolicy   string    `json:"licensePolicy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type ListenLocalMusicLyricSelection struct {
	SubjectID  string    `json:"subjectId"`
	TrackID    string    `json:"trackId"`
	DocumentID string    `json:"documentId"`
	OffsetMs   int64     `json:"offsetMs"`
	Revision   int64     `json:"revision"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ListenLocalMusicMutation struct {
	SubjectID           string
	ActorDeviceID       string
	Family              string
	MutationID          string
	RequestHash         string
	Type                string
	EntityID            string
	ExpectedRevision    int64
	DependsOnMutationID string
	Payload             json.RawMessage
	OccurredAt          time.Time
}

type ListenLocalMusicMutationResult struct {
	MutationID string          `json:"mutationId"`
	Type       string          `json:"type"`
	EntityID   string          `json:"entityId"`
	Revision   int64           `json:"revision"`
	Replayed   bool            `json:"replayed"`
	Result     json.RawMessage `json:"result"`
}

type ListenLocalMusicPlayEvent struct {
	SubjectID                    string
	ActorDeviceID                string
	EventID                      string
	RequestHash                  string
	PlaySessionID                string
	Sequence                     int64
	TrackID                      string
	ContentIdentityRevision      int64
	CumulativeListenedDurationMs int64
	PositionMs                   int64
	Terminal                     bool
	Completed                    bool
	EndReason                    string
	DeviceOccurredAt             *time.Time
	ReceivedAt                   time.Time
}

type ListenLocalMusicPlayEventResult struct {
	EventID                      string                      `json:"eventId"`
	PlaySessionID                string                      `json:"playSessionId"`
	Sequence                     int64                       `json:"acknowledgedSequence"`
	CumulativeListenedDurationMs int64                       `json:"cumulativeListenedDurationMs"`
	PositionMs                   int64                       `json:"positionMs"`
	Terminal                     bool                        `json:"terminal"`
	Accepted                     bool                        `json:"accepted"`
	Replayed                     bool                        `json:"replayed"`
	TrackStateRevision           int64                       `json:"trackStateRevision"`
	TrackState                   *ListenLocalMusicTrackState `json:"trackState,omitempty"`
}

type ListenLocalMusicWriteRepository interface {
	ApplyMutation(context.Context, ListenLocalMusicMutation) (ListenLocalMusicMutationResult, error)
	ApplyPlayEvent(context.Context, ListenLocalMusicPlayEvent) (ListenLocalMusicPlayEventResult, error)
}
