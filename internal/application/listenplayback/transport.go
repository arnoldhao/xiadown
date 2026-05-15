package listenplayback

import "context"

type VideoLoadStrategy int

const (
	VideoLoadStandard VideoLoadStrategy = iota
	VideoLoadPreferInPlaceWhenSameVideoID
	VideoLoadForceFullPageWhenSameVideoID
)

type Transport interface {
	LoadVideo(ctx context.Context, request PlayRequest, strategy VideoLoadStrategy) error
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Seek(ctx context.Context, seconds float64) error
	SetVolume(ctx context.Context, volume float64, muted bool) error
	Next(ctx context.Context) error
	Previous(ctx context.Context) error
	CurrentVideoID(ctx context.Context) string
}

type LibraryClient interface {
	Radio(ctx context.Context, videoID string, limit int) ([]Track, error)
	MixQueue(ctx context.Context, playlistID string, startVideoID string) (MixQueueResult, error)
	MixQueueContinuation(ctx context.Context, continuation string) (MixQueueResult, error)
	TrackMetadata(ctx context.Context, videoID string) (Track, error)
}

type MixQueueResult struct {
	Tracks            []Track
	ContinuationToken string
}

type SessionStore interface {
	SavePlaybackSession(ctx context.Context, session RestoredPlaybackSession) error
	LoadPlaybackSession(ctx context.Context) (RestoredPlaybackSession, bool, error)
	ClearPlaybackSession(ctx context.Context) error
}
