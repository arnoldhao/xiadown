package listenplayback

type PlaybackState string

const (
	PlaybackStateIdle      PlaybackState = "idle"
	PlaybackStateLoading   PlaybackState = "loading"
	PlaybackStatePlaying   PlaybackState = "playing"
	PlaybackStatePaused    PlaybackState = "paused"
	PlaybackStateBuffering PlaybackState = "buffering"
	PlaybackStateEnded     PlaybackState = "ended"
	PlaybackStateError     PlaybackState = "error"
)

func (state PlaybackState) IsPlaying() bool {
	return state == PlaybackStatePlaying
}

type RepeatMode string

const (
	RepeatModeOff RepeatMode = "off"
	RepeatModeAll RepeatMode = "all"
	RepeatModeOne RepeatMode = "one"
)

type QueueKind string

const (
	QueueKindNone     QueueKind = "none"
	QueueKindRadio    QueueKind = "radio"
	QueueKindPlaylist QueueKind = "playlist"
	QueueKindMix      QueueKind = "mix"
)

type FeedbackTokens struct {
	Add    string `json:"add,omitempty"`
	Remove string `json:"remove,omitempty"`
}

type TrackArtist struct {
	Name     string `json:"name"`
	BrowseID string `json:"browseId,omitempty"`
}

const (
	TrackArtistSourceAPILinked         = "api-linked"
	TrackArtistSourceAPILinkedMultiple = "api-linked-multiple"
	TrackArtistSourceAPIText           = "api-text"
	TrackArtistSourceAPIMetadata       = "api-metadata"
	TrackArtistSourceObserved          = "observed"
)

type Track struct {
	ID                     string         `json:"id"`
	VideoID                string         `json:"videoId"`
	Title                  string         `json:"title"`
	Artist                 string         `json:"artist"`
	Artists                []TrackArtist  `json:"artists,omitempty"`
	ArtistBrowseID         string         `json:"artistBrowseId,omitempty"`
	ArtistSource           string         `json:"artistSource,omitempty"`
	DurationLabel          string         `json:"durationLabel,omitempty"`
	DurationSeconds        float64        `json:"durationSeconds,omitempty"`
	ThumbnailURL           string         `json:"thumbnailUrl,omitempty"`
	MusicVideoType         string         `json:"musicVideoType,omitempty"`
	HasVideo               bool           `json:"hasVideo,omitempty"`
	VideoAvailabilityKnown bool           `json:"videoAvailabilityKnown,omitempty"`
	LikeStatus             string         `json:"likeStatus,omitempty"`
	InLibrary              bool           `json:"inLibrary,omitempty"`
	FeedbackTokens         FeedbackTokens `json:"feedbackTokens,omitempty"`
}

type QueueSnapshot struct {
	Queue        []Track `json:"queue"`
	CurrentIndex int     `json:"currentIndex"`
}

type QueueState struct {
	Kind        QueueKind `json:"kind"`
	Title       string    `json:"title"`
	Items       []Track   `json:"items"`
	SeedVideoID string    `json:"seedVideoId,omitempty"`
	PlaylistID  string    `json:"playlistId,omitempty"`
}

type Snapshot struct {
	Version              uint64        `json:"version"`
	State                PlaybackState `json:"state"`
	CurrentTrack         *Track        `json:"currentTrack,omitempty"`
	Progress             float64       `json:"progress"`
	Duration             float64       `json:"duration"`
	Volume               float64       `json:"volume"`
	VolumeBeforeMute     float64       `json:"volumeBeforeMute"`
	Muted                bool          `json:"muted"`
	ShuffleEnabled       bool          `json:"shuffleEnabled"`
	RepeatMode           RepeatMode    `json:"repeatMode"`
	Queue                []Track       `json:"queue"`
	QueueKind            QueueKind     `json:"queueKind"`
	QueueTitle           string        `json:"queueTitle"`
	CurrentIndex         int           `json:"currentIndex"`
	PendingPlayVideoID   string        `json:"pendingPlayVideoId,omitempty"`
	ShowMiniPlayer       bool          `json:"showMiniPlayer"`
	CanUndoQueue         bool          `json:"canUndoQueue"`
	CanRedoQueue         bool          `json:"canRedoQueue"`
	CanAutoloadPending   bool          `json:"canAutoloadPending"`
	CurrentTimeMs        int           `json:"currentTimeMs,omitempty"`
	ObservedAudioQuality string        `json:"observedPlaybackAudioQuality,omitempty"`
}

type PlayOptions struct {
	StartSeconds     float64
	RestartFromStart bool
	ForceReload      bool
}

type PlayRequest struct {
	Track            Track
	StartSeconds     float64
	RestartFromStart bool
	ForceReload      bool
	Volume           float64
	Muted            bool
}

type ObservedTrack struct {
	ObservedVideoID string
	Title           string
	Artist          string
	ThumbnailURL    string
	LikeStatus      string
	TrackChanged    bool
	MetadataSource  string
}

type RestoredPlaybackSession struct {
	Queue            []Track    `json:"queue"`
	QueueKind        QueueKind  `json:"queueKind,omitempty"`
	QueueTitle       string     `json:"queueTitle,omitempty"`
	CurrentIndex     int        `json:"currentIndex"`
	CurrentVideoID   string     `json:"currentVideoId,omitempty"`
	Progress         float64    `json:"progress"`
	Duration         float64    `json:"duration"`
	ShuffleEnabled   bool       `json:"shuffleEnabled"`
	RepeatMode       RepeatMode `json:"repeatMode"`
	Volume           float64    `json:"volume"`
	VolumeBeforeMute float64    `json:"volumeBeforeMute,omitempty"`
	Muted            bool       `json:"muted"`
}
