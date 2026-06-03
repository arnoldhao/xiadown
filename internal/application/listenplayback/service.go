package listenplayback

import (
	"context"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	queueUndoMaxCount          = 10
	radioQueueLimit            = 24
	mixFetchThresholdRemaining = 10
	restoredSeekTolerance      = 1.5
	repeatOneRecoveryThrottle  = 450 * time.Millisecond
	playbackSnapshotMinDelta   = 500 * time.Millisecond
)

type PlayerService struct {
	mu sync.Mutex

	transport Transport
	library   LibraryClient
	store     SessionStore
	random    func(limit int) int

	state PlaybackState
	err   string

	currentTrack    Track
	hasCurrentTrack bool

	progress                     float64
	currentTimeMs                int
	duration                     float64
	observedPlaybackAudioQuality string

	volume           float64
	volumeBeforeMute float64
	muted            bool

	shuffleEnabled bool
	repeatMode     RepeatMode

	queue        []Track
	queueKind    QueueKind
	queueTitle   string
	currentIndex int

	queueOrderBeforeShuffle []Track

	queueUndo []QueueSnapshot
	queueRedo []QueueSnapshot

	forwardSkipIndexStack []int

	showMiniPlayer               bool
	pendingPlayVideoID           string
	hasUserInteractedThisSession bool

	pendingRestoredSeek         float64
	pendingRestoredLoadDeferred bool
	restoringPlaybackSession    bool
	autoResumeAfterRestoredSeek bool

	songNearingEnd           bool
	appInitiatedPlayback     bool
	suppressAutoplayAfterEnd bool
	lastRepeatOneRecoveryAt  time.Time

	mixContinuationToken string
	fetchingMoreMixSongs bool

	metadataEnrichmentInFlight  map[string]struct{}
	metadataEnrichmentAttempted map[string]struct{}

	snapshotVersion                 uint64
	lastPublishedPlaybackState      PlaybackState
	lastPublishedPlaybackProgress   float64
	lastPublishedPlaybackDuration   float64
	lastPublishedPlaybackNearingEnd bool
	nextSubscriberID                uint64
	subscribers                     map[uint64]SnapshotListener
}

type SnapshotListener func(Snapshot)

type Option func(*PlayerService)

func WithLibraryClient(client LibraryClient) Option {
	return func(service *PlayerService) {
		service.library = client
	}
}

func WithSessionStore(store SessionStore) Option {
	return func(service *PlayerService) {
		service.store = store
	}
}

func WithRandomIndex(random func(limit int) int) Option {
	return func(service *PlayerService) {
		if random != nil {
			service.random = random
		}
	}
}

func WithUserInteractionUnlocked() Option {
	return func(service *PlayerService) {
		service.hasUserInteractedThisSession = true
	}
}

func NewPlayerService(transport Transport, options ...Option) *PlayerService {
	service := &PlayerService{
		transport:        transport,
		state:            PlaybackStateIdle,
		volume:           1,
		volumeBeforeMute: 1,
		repeatMode:       RepeatModeOff,
		queueKind:        QueueKindNone,
		currentIndex:     0,
		random: func(limit int) int {
			if limit <= 0 {
				return 0
			}
			return int(time.Now().UnixNano() % int64(limit))
		},
		subscribers: make(map[uint64]SnapshotListener),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *PlayerService) Subscribe(listener SnapshotListener) func() {
	if service == nil || listener == nil {
		return func() {}
	}
	service.mu.Lock()
	service.nextSubscriberID++
	id := service.nextSubscriberID
	if service.subscribers == nil {
		service.subscribers = make(map[uint64]SnapshotListener)
	}
	service.subscribers[id] = listener
	service.mu.Unlock()
	return func() {
		service.mu.Lock()
		delete(service.subscribers, id)
		service.mu.Unlock()
	}
}

func (service *PlayerService) Snapshot(ctx context.Context) Snapshot {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.snapshotLocked(ctx)
}

func (service *PlayerService) PublishSnapshot(ctx context.Context) Snapshot {
	if service == nil {
		return Snapshot{}
	}
	service.mu.Lock()
	service.snapshotVersion++
	snapshot := service.snapshotLocked(ctx)
	service.recordPublishedPlaybackStateLocked()
	listeners := make([]SnapshotListener, 0, len(service.subscribers))
	for _, listener := range service.subscribers {
		if listener != nil {
			listeners = append(listeners, listener)
		}
	}
	service.mu.Unlock()
	for _, listener := range listeners {
		listener(snapshot)
	}
	return snapshot
}

func (service *PlayerService) recordPublishedPlaybackStateLocked() {
	service.lastPublishedPlaybackState = service.state
	service.lastPublishedPlaybackProgress = service.progress
	service.lastPublishedPlaybackDuration = service.duration
	service.lastPublishedPlaybackNearingEnd = service.songNearingEnd
}

func (service *PlayerService) snapshotLocked(ctx context.Context) Snapshot {
	queue := cloneTracks(service.queue)
	var current *Track
	if service.hasCurrentTrack {
		track := service.currentTrack
		current = &track
		if len(queue) > 0 {
			index := safeQueueIndex(service.currentIndex, len(queue))
			if tracksReferToSameQueueItem(queue[index], track) {
				queue[index] = track
			}
		}
	}
	return Snapshot{
		Version:              service.snapshotVersion,
		State:                service.state,
		CurrentTrack:         current,
		Progress:             service.progress,
		Duration:             service.duration,
		Volume:               service.volume,
		VolumeBeforeMute:     service.volumeBeforeMute,
		Muted:                service.muted,
		ShuffleEnabled:       service.shuffleEnabled,
		RepeatMode:           service.repeatMode,
		Queue:                queue,
		QueueKind:            service.queueKind,
		QueueTitle:           service.queueTitle,
		CurrentIndex:         service.currentIndex,
		PendingPlayVideoID:   service.pendingPlayVideoID,
		ShowMiniPlayer:       service.showMiniPlayer,
		CanUndoQueue:         len(service.queueUndo) > 0,
		CanRedoQueue:         len(service.queueRedo) > 0,
		CanAutoloadPending:   service.shouldAutoloadPendingVideoLocked(ctx),
		CurrentTimeMs:        service.currentTimeMs,
		ObservedAudioQuality: service.observedPlaybackAudioQuality,
	}
}

func (service *PlayerService) State() PlaybackState {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.state
}

func (service *PlayerService) CurrentTrack() (Track, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.currentTrack, service.hasCurrentTrack
}

func (service *PlayerService) Queue() ([]Track, int) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return cloneTracks(service.queue), service.currentIndex
}

func (service *PlayerService) ShuffleEnabled() bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.shuffleEnabled
}

func (service *PlayerService) RepeatMode() RepeatMode {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.repeatMode
}

func (service *PlayerService) ConfirmPlaybackStarted() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.confirmPlaybackStartedLocked()
}

func (service *PlayerService) RecordPlaybackIntent() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.recordPlaybackIntentLocked()
}

func (service *PlayerService) recordPlaybackIntentLocked() {
	service.showMiniPlayer = false
	service.hasUserInteractedThisSession = true
}

func (service *PlayerService) confirmPlaybackStartedLocked() {
	service.recordPlaybackIntentLocked()
	service.state = PlaybackStatePlaying
}

func (service *PlayerService) MiniPlayerDismissed() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.showMiniPlayer = false
	if service.state == PlaybackStateLoading {
		service.state = PlaybackStateIdle
	}
}

func (service *PlayerService) ToggleShuffle() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.setShuffleEnabledLocked(!service.shuffleEnabled, true)
}

func (service *PlayerService) SetShuffleEnabled(enabled bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.setShuffleEnabledLocked(enabled, true)
}

func (service *PlayerService) CycleRepeatMode() RepeatMode {
	service.mu.Lock()
	defer service.mu.Unlock()
	switch service.repeatMode {
	case RepeatModeOff:
		service.repeatMode = RepeatModeAll
	case RepeatModeAll:
		service.repeatMode = RepeatModeOne
	default:
		service.repeatMode = RepeatModeOff
	}
	return service.repeatMode
}

func (service *PlayerService) SetRepeatMode(mode RepeatMode) {
	service.mu.Lock()
	defer service.mu.Unlock()
	switch mode {
	case RepeatModeAll, RepeatModeOne:
		service.repeatMode = mode
	default:
		service.repeatMode = RepeatModeOff
	}
}

func (service *PlayerService) MarkPlaybackEnded() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.markPlaybackEndedLocked()
}

func (service *PlayerService) markPlaybackEndedLocked() {
	service.state = PlaybackStateEnded
	service.songNearingEnd = false
}

func (service *PlayerService) clearForwardSkipNavigationStackLocked() {
	service.forwardSkipIndexStack = nil
}

func (service *PlayerService) pushForwardSkipStackIfLeavingIndexLocked(newIndex int) {
	from := service.currentIndex
	if from == newIndex {
		return
	}
	if from < 0 || from >= len(service.queue) || newIndex < 0 || newIndex >= len(service.queue) {
		return
	}
	service.forwardSkipIndexStack = append(service.forwardSkipIndexStack, from)
	if len(service.forwardSkipIndexStack) > 20 {
		service.forwardSkipIndexStack = append([]int(nil), service.forwardSkipIndexStack[len(service.forwardSkipIndexStack)-20:]...)
	}
}

func (service *PlayerService) shouldAutoloadPendingVideoLocked(ctx context.Context) bool {
	if service.pendingRestoredLoadDeferred {
		return false
	}
	if service.pendingPlayVideoID == "" {
		return false
	}
	if service.transport == nil {
		return true
	}
	return strings.TrimSpace(service.transport.CurrentVideoID(ctx)) != service.pendingPlayVideoID
}

func cloneTracks(tracks []Track) []Track {
	if len(tracks) == 0 {
		return nil
	}
	clone := make([]Track, len(tracks))
	copy(clone, tracks)
	return clone
}

func tracksEqual(left []Track, right []Track) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !trackEqual(left[index], right[index]) {
			return false
		}
	}
	return true
}

func trackEqual(left Track, right Track) bool {
	return reflect.DeepEqual(left, right)
}

func safeQueueIndex(index int, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func clampSeconds(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return value
}

func clampVolume(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func normalizedVideoID(value string) string {
	return strings.TrimSpace(value)
}
