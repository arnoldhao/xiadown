package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/listenplayback"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	listenLivePlayerWindowName = "listen-youtube-live-player"
	listenLivePlayerEventName  = "listen:youtube-live-player"
	listenLivePlayerSource     = "listen-youtube-live-player"

	listenYouTubeOrigin   = "https://www.youtube.com"
	listenYouTubeClientID = "com.dreamapp.xiadown"
)

type ListenLivePlayerHandler struct {
	player      *ListenYouTubeLivePlayer
	coordinator *listenplayback.PlaybackCoordinator
}

// ListenLivePlaybackSessionRequest makes every public transport command
// conditional on the provider-owned coordinator session that created the UI.
// A stale workspace can therefore never control the provider that currently
// owns the shared live player.
type ListenLivePlaybackSessionRequest struct {
	Provider  listenplayback.PlaybackProvider `json:"provider"`
	SessionID string                          `json:"sessionId"`
}

type ListenLivePlaybackSessionSeekRequest struct {
	Provider  listenplayback.PlaybackProvider `json:"provider"`
	SessionID string                          `json:"sessionId"`
	Seconds   float64                         `json:"seconds"`
}

type ListenLivePlaybackSessionVolumeRequest struct {
	Provider  listenplayback.PlaybackProvider `json:"provider"`
	SessionID string                          `json:"sessionId"`
	Volume    float64                         `json:"volume"`
	Muted     bool                            `json:"muted"`
}

// ListenLivePlaybackControlRequest is the capability-gated command envelope
// used by the YouTube workspace. Unlike the legacy transport methods, these
// commands reject a stale provider/session identity instead of silently doing
// nothing, so an old UI can never mutate the newly focused player.
type ListenLivePlaybackControlRequest struct {
	Provider  listenplayback.PlaybackProvider `json:"provider"`
	SessionID string                          `json:"sessionId"`
	Command   string                          `json:"command"`
	Value     string                          `json:"value,omitempty"`
	Volume    float64                         `json:"volume,omitempty"`
	Muted     bool                            `json:"muted,omitempty"`
}

type ListenLivePlayerOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ListenLivePlayerControls struct {
	Like         bool `json:"like"`
	Dislike      bool `json:"dislike"`
	Captions     bool `json:"captions"`
	AudioTrack   bool `json:"audioTrack"`
	Quality      bool `json:"quality"`
	Volume       bool `json:"volume"`
	PlaybackRate bool `json:"playbackRate"`
}

type ListenLivePlayerSelections struct {
	Rating         string `json:"rating,omitempty"`
	CaptionID      string `json:"captionId,omitempty"`
	AudioTrackID   string `json:"audioTrackId,omitempty"`
	QualityID      string `json:"qualityId,omitempty"`
	PlaybackRateID string `json:"playbackRateId,omitempty"`
}

// ListenLivePlayerStatus extends the shared playback status with controls that
// are discovered from the currently loaded YouTube movie_player. The fields
// deliberately default to unavailable until the bridge reports a working
// getter/setter pair or a concrete, enabled DOM control. Volume is the one
// exception: XiaDown owns that bridge and can persist a request while YouTube
// replaces its movie_player or media element.
type ListenLivePlayerStatus struct {
	ListenPlayerStatus
	Controls            ListenLivePlayerControls   `json:"controls"`
	CaptionOptions      []ListenLivePlayerOption   `json:"captionOptions"`
	AudioTrackOptions   []ListenLivePlayerOption   `json:"audioTrackOptions"`
	QualityOptions      []ListenLivePlayerOption   `json:"qualityOptions"`
	PlaybackRateOptions []ListenLivePlayerOption   `json:"playbackRateOptions"`
	Selections          ListenLivePlayerSelections `json:"selections"`
}

func NewListenLivePlayerHandler(
	player *ListenYouTubeLivePlayer,
	coordinator ...*listenplayback.PlaybackCoordinator,
) *ListenLivePlayerHandler {
	handler := &ListenLivePlayerHandler{player: player}
	if len(coordinator) > 0 {
		handler.coordinator = coordinator[0]
	}
	return handler
}

func (handler *ListenLivePlayerHandler) ServiceName() string {
	return "ListenLivePlayerHandler"
}

func (handler *ListenLivePlayerHandler) Play(ctx context.Context, request ListenPlayerPlayRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	if handler.coordinator != nil {
		volume := clampListenVolume(request.Volume)
		_, err := handler.coordinator.StartSession(ctx, listenplayback.PlaybackSessionRequest{
			Focus: listenplayback.PlaybackFocusPersistent,
			Item: listenplayback.MediaItem{
				ID:           request.VideoID,
				Kind:         listenplayback.MediaKindAudio,
				Source:       listenplayback.PlaybackSource{Provider: listenplayback.PlaybackProviderStream, ID: request.VideoID, Live: true},
				Title:        request.Title,
				Artist:       request.Artist,
				CanonicalURL: listenYouTubeOrigin + "/watch?v=" + request.VideoID,
			},
			StartSeconds: request.StartSeconds,
			Volume:       &volume,
			Muted:        request.Muted,
			ForceReload:  request.ForceReload,
		})
		return err
	}
	return handler.player.StartPlaybackSession(
		listenplayback.PlaybackProviderStream,
		"legacy-stream",
		request,
	)
}

func (handler *ListenLivePlayerHandler) Pause(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	identity, ok := handler.legacyStreamIdentity()
	if !ok {
		return nil
	}
	return handler.player.PausePlaybackSession(identity.Provider, identity.SessionID)
}

func (handler *ListenLivePlayerHandler) Resume(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	identity, ok := handler.legacyStreamIdentity()
	if !ok {
		return nil
	}
	return handler.player.ResumePlaybackSession(identity.Provider, identity.SessionID)
}

func (handler *ListenLivePlayerHandler) Replay(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	identity, ok := handler.legacyStreamIdentity()
	if !ok {
		return nil
	}
	return handler.player.ReplayPlaybackSession(identity.Provider, identity.SessionID)
}

func (handler *ListenLivePlayerHandler) Seek(_ context.Context, request ListenPlayerSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	identity, ok := handler.legacyStreamIdentity()
	if !ok {
		return nil
	}
	return handler.player.SeekPlaybackSession(identity.Provider, identity.SessionID, request.Seconds)
}

func (handler *ListenLivePlayerHandler) SetVolume(_ context.Context, request ListenPlayerVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	identity, ok := handler.legacyStreamIdentity()
	if !ok {
		return nil
	}
	return handler.player.SetPlaybackSessionVolume(
		identity.Provider,
		identity.SessionID,
		request.Volume,
		request.Muted,
	)
}

func (handler *ListenLivePlayerHandler) Reset(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	identity, ok := handler.legacyStreamIdentity()
	if !ok {
		return nil
	}
	return handler.player.ResetPlaybackSession(identity.Provider, identity.SessionID)
}

func (handler *ListenLivePlayerHandler) PauseSession(_ context.Context, request ListenLivePlaybackSessionRequest) error {
	return handler.controlSession(request, (*ListenYouTubeLivePlayer).PausePlaybackSession)
}

func (handler *ListenLivePlayerHandler) ResumeSession(_ context.Context, request ListenLivePlaybackSessionRequest) error {
	return handler.controlSession(request, (*ListenYouTubeLivePlayer).ResumePlaybackSession)
}

func (handler *ListenLivePlayerHandler) ReplaySession(_ context.Context, request ListenLivePlaybackSessionRequest) error {
	return handler.controlSession(request, (*ListenYouTubeLivePlayer).ReplayPlaybackSession)
}

func (handler *ListenLivePlayerHandler) SeekSession(_ context.Context, request ListenLivePlaybackSessionSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.SeekPlaybackSession(request.Provider, request.SessionID, request.Seconds)
}

func (handler *ListenLivePlayerHandler) SetSessionVolume(_ context.Context, request ListenLivePlaybackSessionVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.SetPlaybackSessionVolume(
		request.Provider,
		request.SessionID,
		request.Volume,
		request.Muted,
	)
}

func (handler *ListenLivePlayerHandler) ControlSession(_ context.Context, request ListenLivePlaybackControlRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.ControlYouTubePlaybackSession(request)
}

func (handler *ListenLivePlayerHandler) ResetSession(_ context.Context, request ListenLivePlaybackSessionRequest) error {
	return handler.controlSession(request, (*ListenYouTubeLivePlayer).ResetPlaybackSession)
}

func (handler *ListenLivePlayerHandler) controlSession(
	request ListenLivePlaybackSessionRequest,
	command func(*ListenYouTubeLivePlayer, listenplayback.PlaybackProvider, string) error,
) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	if !isListenLiveProvider(request.Provider) || strings.TrimSpace(request.SessionID) == "" {
		return nil
	}
	return command(handler.player, request.Provider, request.SessionID)
}

func (handler *ListenLivePlayerHandler) ShowWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.ShowWindow()
}

func (handler *ListenLivePlayerHandler) HideWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.HideWindow()
}

func (handler *ListenLivePlayerHandler) ShowVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.ShowVideoWindow()
}

func (handler *ListenLivePlayerHandler) HideVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.HideVideoWindow()
}

func (handler *ListenLivePlayerHandler) HideVideoWindowForSequence(_ context.Context, request ListenEmbeddedVideoHideRequest) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("listen live player unavailable")
	}
	return handler.player.HideVideoWindowForSequence(request.Sequence)
}

func (handler *ListenLivePlayerHandler) ShowEmbeddedVideo(_ context.Context, rect ListenEmbeddedVideoRect) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("listen live player unavailable")
	}
	return handler.player.ShowEmbeddedVideo(rect)
}

func (handler *ListenLivePlayerHandler) HideEmbeddedVideo(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.HideEmbeddedVideo()
}

func (handler *ListenLivePlayerHandler) HideEmbeddedVideoForSequence(_ context.Context, request ListenEmbeddedVideoHideRequest) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("listen live player unavailable")
	}
	return handler.player.HideEmbeddedVideoForSequence(request.Sequence)
}

func (handler *ListenLivePlayerHandler) RequestEmbeddedVideoFullscreen(_ context.Context, request ListenLivePlaybackSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.RequestEmbeddedVideoFullscreen(request.Provider, request.SessionID)
}

func (handler *ListenLivePlayerHandler) ExitEmbeddedVideoFullscreen(_ context.Context, request ListenLivePlaybackSessionRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.ExitEmbeddedVideoFullscreen(request.Provider, request.SessionID)
}

func (handler *ListenLivePlayerHandler) ShowAirPlayPicker(_ context.Context, anchor ListenAirPlayAnchor) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.ShowAirPlayPicker(anchor)
}

func (handler *ListenLivePlayerHandler) StartLyricsPoll(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return nil
}

func (handler *ListenLivePlayerHandler) StopLyricsPoll(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return nil
}

func (handler *ListenLivePlayerHandler) Status(_ context.Context) (ListenLivePlayerStatus, error) {
	if handler == nil || handler.player == nil {
		return ListenLivePlayerStatus{}, fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Status(), nil
}

func (handler *ListenLivePlayerHandler) legacyStreamIdentity() (ListenLivePlaybackSessionRequest, bool) {
	if handler == nil || handler.player == nil {
		return ListenLivePlaybackSessionRequest{}, false
	}
	provider, sessionID := handler.player.CurrentPlaybackIdentity()
	if provider != listenplayback.PlaybackProviderStream || strings.TrimSpace(sessionID) == "" {
		return ListenLivePlaybackSessionRequest{}, false
	}
	return ListenLivePlaybackSessionRequest{Provider: provider, SessionID: sessionID}, true
}

func isListenLiveProvider(provider listenplayback.PlaybackProvider) bool {
	return provider == listenplayback.PlaybackProviderStream || provider == listenplayback.PlaybackProviderYouTube
}

type ListenYouTubeLivePlayer struct {
	app     *application.App
	windows *WindowManager
	cookies listenPlayerCookieProvider

	commandMu                      sync.Mutex
	mu                             sync.Mutex
	window                         *application.WebviewWindow
	closeHook                      func()
	bridgeHook                     func()
	fullscreenHook                 func()
	unfullscreenHook               func()
	currentVideo                   string
	loadedProvider                 listenplayback.PlaybackProvider
	loadedLanguage                 string
	currentState                   string
	activated                      bool
	targetVolume                   float64
	targetMuted                    bool
	requestTitle                   string
	requestArtist                  string
	observedTitle                  string
	observedArtist                 string
	observedThumb                  string
	advertising                    bool
	adLabel                        string
	errorCode                      string
	errorMessage                   string
	currentTime                    float64
	duration                       float64
	bufferedTime                   float64
	lastPlayAt                     time.Time
	videoVisible                   bool
	embeddedVisible                bool
	embeddedRect                   ListenEmbeddedVideoRect
	embeddedRefreshToken           uint64
	embeddedSequence               uint64
	embeddedResizeWaiters          map[uint64]chan bool
	embeddedFullscreen             listenEmbeddedVideoFullscreenRequests
	embeddedFullscreenActive       bool
	embeddedFullscreenTransition   bool
	embeddedFullscreenVersion      uint64
	embeddedFullscreenMonitor      uint64
	embeddedFullscreenNativeSeen   bool
	embeddedNativeWindowFullscreen bool
	embeddedNativeFullscreenWaiter chan bool
	playbackProvider               listenplayback.PlaybackProvider
	playbackSessionID              string
	controls                       ListenLivePlayerControls
	captionOptions                 []ListenLivePlayerOption
	audioTrackOptions              []ListenLivePlayerOption
	qualityOptions                 []ListenLivePlayerOption
	playbackRateOptions            []ListenLivePlayerOption
	selections                     ListenLivePlayerSelections
	nextPlaybackListener           uint64
	playbackListeners              map[uint64]listenplayback.PlaybackBackendEventListener
}

func NewListenYouTubeLivePlayer(app *application.App, windows *WindowManager, cookies listenPlayerCookieProvider) *ListenYouTubeLivePlayer {
	return &ListenYouTubeLivePlayer{
		app:               app,
		windows:           windows,
		cookies:           cookies,
		currentState:      "idle",
		targetVolume:      1,
		playbackListeners: make(map[uint64]listenplayback.PlaybackBackendEventListener),
	}
}

func (player *ListenYouTubeLivePlayer) StartPlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	request ListenPlayerPlayRequest,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	if provider != listenplayback.PlaybackProviderStream && provider != listenplayback.PlaybackProviderYouTube {
		return fmt.Errorf("listen live player cannot play provider %q", provider)
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("listen live playback session id is required")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	player.playbackProvider = provider
	player.playbackSessionID = sessionID
	player.mu.Unlock()
	if err := player.Play(request); err != nil {
		player.mu.Lock()
		if player.playbackProvider == provider && player.playbackSessionID == sessionID {
			player.playbackProvider = ""
			player.playbackSessionID = ""
		}
		player.mu.Unlock()
		return err
	}
	return nil
}

func (player *ListenYouTubeLivePlayer) CurrentPlaybackIdentity() (listenplayback.PlaybackProvider, string) {
	if player == nil {
		return "", ""
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.playbackProvider, player.playbackSessionID
}

func (player *ListenYouTubeLivePlayer) ResumePlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) error {
	return player.controlPlaybackSession(provider, sessionID, (*ListenYouTubeLivePlayer).Resume)
}

func (player *ListenYouTubeLivePlayer) PausePlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) error {
	return player.controlPlaybackSession(provider, sessionID, (*ListenYouTubeLivePlayer).Pause)
}

func (player *ListenYouTubeLivePlayer) ReplayPlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) error {
	return player.controlPlaybackSession(provider, sessionID, (*ListenYouTubeLivePlayer).Replay)
}

func (player *ListenYouTubeLivePlayer) ResetPlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) error {
	return player.controlPlaybackSession(provider, sessionID, (*ListenYouTubeLivePlayer).Reset)
}

func (player *ListenYouTubeLivePlayer) SeekPlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	seconds float64,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	if !player.playbackIdentityMatches(provider, sessionID) {
		return nil
	}
	return player.Seek(seconds)
}

func (player *ListenYouTubeLivePlayer) SetPlaybackSessionVolume(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	volume float64,
	muted bool,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	if !player.playbackIdentityMatches(provider, sessionID) {
		return nil
	}
	return player.SetVolume(volume, muted)
}

const (
	listenLiveControlLike           = "like"
	listenLiveControlDislike        = "dislike"
	listenLiveControlToggleCaptions = "toggle-captions"
	listenLiveControlSelectCaption  = "select-caption"
	listenLiveControlSelectAudio    = "select-audio-track"
	listenLiveControlSelectQuality  = "select-quality"
	listenLiveControlPlaybackRate   = "select-playback-rate"
	listenLiveControlSetVolume      = "set-volume"
)

func (player *ListenYouTubeLivePlayer) ControlYouTubePlaybackSession(
	request ListenLivePlaybackControlRequest,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Command = strings.TrimSpace(request.Command)
	request.Value = strings.TrimSpace(request.Value)
	if request.Provider != listenplayback.PlaybackProviderYouTube || request.SessionID == "" {
		return fmt.Errorf("youtube playback provider and session are required")
	}

	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if player.playbackProvider != request.Provider || player.playbackSessionID != request.SessionID {
		player.mu.Unlock()
		return fmt.Errorf("stale youtube playback session")
	}
	if err := player.validateYouTubeControlLocked(request); err != nil {
		player.mu.Unlock()
		return err
	}
	window := player.window
	if request.Command == listenLiveControlSetVolume {
		player.targetVolume = clampListenVolume(request.Volume)
		player.targetMuted = request.Muted
		request.Volume = player.targetVolume
	}
	player.mu.Unlock()
	if window == nil {
		return fmt.Errorf("youtube player window unavailable")
	}
	execListenYouTubeMusicJS(window, listenYouTubeLiveControlScript(request))
	return nil
}

func (player *ListenYouTubeLivePlayer) validateYouTubeControlLocked(
	request ListenLivePlaybackControlRequest,
) error {
	available := false
	switch request.Command {
	case listenLiveControlLike:
		available = player.controls.Like
	case listenLiveControlDislike:
		available = player.controls.Dislike
	case listenLiveControlToggleCaptions:
		available = player.controls.Captions
	case listenLiveControlSelectCaption:
		available = player.controls.Captions && listenLiveOptionExists(player.captionOptions, request.Value, true)
	case listenLiveControlSelectAudio:
		available = player.controls.AudioTrack && listenLiveOptionExists(player.audioTrackOptions, request.Value, false)
	case listenLiveControlSelectQuality:
		available = player.controls.Quality && listenLiveOptionExists(player.qualityOptions, request.Value, false)
	case listenLiveControlPlaybackRate:
		available = player.controls.PlaybackRate && listenLiveOptionExists(player.playbackRateOptions, request.Value, false)
	case listenLiveControlSetVolume:
		// Volume is owned by XiaDown's bridge. The bridge stores the request and
		// reapplies it when YouTube replaces or later exposes its video element,
		// so an instantaneous movie_player capability snapshot must not reject it.
		available = listenYouTubeVideoIDPattern.MatchString(player.currentVideo)
	default:
		return fmt.Errorf("unsupported youtube player command %q", request.Command)
	}
	if !available {
		return fmt.Errorf("youtube player command %q unavailable", request.Command)
	}
	return nil
}

func listenLiveOptionExists(options []ListenLivePlayerOption, value string, allowEmpty bool) bool {
	if allowEmpty && value == "" {
		return true
	}
	for _, option := range options {
		if option.ID == value {
			return true
		}
	}
	return false
}

func (player *ListenYouTubeLivePlayer) controlPlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	command func(*ListenYouTubeLivePlayer) error,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	if !player.playbackIdentityMatches(provider, sessionID) {
		return nil
	}
	return command(player)
}

func (player *ListenYouTubeLivePlayer) playbackIdentityMatches(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) bool {
	sessionID = strings.TrimSpace(sessionID)
	if !isListenLiveProvider(provider) || sessionID == "" {
		return false
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.playbackProvider == provider && player.playbackSessionID == sessionID
}

func (player *ListenYouTubeLivePlayer) SubscribePlaybackEvents(
	listener listenplayback.PlaybackBackendEventListener,
) func() {
	if player == nil || listener == nil {
		return func() {}
	}
	player.mu.Lock()
	if player.playbackListeners == nil {
		player.playbackListeners = make(map[uint64]listenplayback.PlaybackBackendEventListener)
	}
	player.nextPlaybackListener++
	id := player.nextPlaybackListener
	player.playbackListeners[id] = listener
	player.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			player.mu.Lock()
			delete(player.playbackListeners, id)
			player.mu.Unlock()
		})
	}
}

func (player *ListenYouTubeLivePlayer) Play(request ListenPlayerPlayRequest) error {
	if player == nil || player.app == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	request = normalizeListenPlayerPlayRequest(request)
	if !listenYouTubeVideoIDPattern.MatchString(request.VideoID) {
		return fmt.Errorf("invalid youtube video id")
	}
	cookies := player.playbackCookies(context.Background())

	player.mu.Lock()
	player.targetVolume = request.Volume
	player.targetMuted = request.Muted
	player.currentState = "loading"
	player.requestTitle = request.Title
	player.requestArtist = request.Artist
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.advertising = false
	player.adLabel = ""
	player.errorCode = ""
	player.errorMessage = ""
	player.controls = ListenLivePlayerControls{}
	player.captionOptions = nil
	player.audioTrackOptions = nil
	player.qualityOptions = nil
	player.playbackRateOptions = nil
	player.selections = ListenLivePlayerSelections{}
	player.currentTime = 0
	player.duration = 0
	player.bufferedTime = 0
	player.lastPlayAt = time.Now()
	window := player.window
	videoVisible := player.videoVisible
	embeddedVisible := player.embeddedVisible
	embeddedRect := player.embeddedRect
	fullscreenOwnsPresentation := listenEmbeddedVideoFullscreenOwnsPresentation(
		player.embeddedFullscreenActive,
		player.embeddedFullscreenTransition,
	) || player.embeddedNativeWindowFullscreen
	applyInlineGeometry := listenEmbeddedVideoCanApplyInlineGeometry(
		embeddedVisible,
		player.embeddedFullscreenActive,
		player.embeddedFullscreenTransition,
		player.embeddedNativeWindowFullscreen,
	)
	provider := player.playbackProvider
	sameVideo := listenYouTubeLiveCanReuseDocument(
		player.currentVideo,
		player.loadedProvider,
		player.loadedLanguage,
		provider,
		request,
	)
	createdWindow := window == nil
	if window == nil {
		window = player.createWindowLocked(request)
	}
	player.currentVideo = request.VideoID
	player.loadedProvider = provider
	player.loadedLanguage = request.Language
	player.mu.Unlock()

	player.dispatchPlaybackState("loading", "play-requested")

	if window == nil {
		return fmt.Errorf("listen live player window unavailable")
	}
	if applyInlineGeometry {
		listenClaimEmbeddedVideoOwner(window)
		_, _ = player.showEmbeddedVideoWindow(window, embeddedRect)
	}
	if videoVisible {
		if embeddedVisible {
			if fullscreenOwnsPresentation && !listenEmbeddedVideoFullscreenAllowsHostGeometry() {
				execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript())
			} else {
				execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(embeddedRect))
			}
		} else {
			window.Show()
		}
	}
	if createdWindow {
		loadListenYouTubeMusicURL(window, listenYouTubePlaybackURL(provider, request.VideoID, request.Language), cookies)
		scheduleListenYouTubeCookieSync(window, player.cookies)
		if embeddedVisible {
			player.scheduleEmbeddedVideoModeRefresh(window)
		}
		return nil
	}
	if sameVideo {
		execListenYouTubeMusicJS(window, listenYouTubeLiveSameVideoPlayScript(request))
		if embeddedVisible {
			player.scheduleEmbeddedVideoModeRefresh(window)
		}
		return nil
	}

	execListenYouTubeMusicJS(window, listenYouTubeLivePrepareLoadScript(request))
	loadListenYouTubeMusicURL(window, listenYouTubePlaybackURL(provider, request.VideoID, request.Language), cookies)
	scheduleListenYouTubeCookieSync(window, player.cookies)
	if embeddedVisible {
		player.scheduleEmbeddedVideoModeRefresh(window)
	}
	return nil
}

func (player *ListenYouTubeLivePlayer) Pause() error {
	player.dispatchPlaybackState("paused", "pause-requested")
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeLivePauseScript())
	return nil
}

func (player *ListenYouTubeLivePlayer) Resume() error {
	player.dispatchPlaybackState("buffering", "resume-requested")
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeLiveResumeScript())
	return nil
}

func (player *ListenYouTubeLivePlayer) Replay() error {
	player.mu.Lock()
	videoID := player.currentVideo
	volume := player.targetVolume
	muted := player.targetMuted
	player.mu.Unlock()
	player.dispatchPlaybackState("buffering", "replay-requested")
	script := listenYouTubeLiveReplayScript(videoID, volume, muted)
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, script)
	return nil
}

func (player *ListenYouTubeLivePlayer) Seek(seconds float64) error {
	player.dispatchPlaybackState("buffering", "seek-requested")
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeLiveSeekScript(seconds))
	return nil
}

func (player *ListenYouTubeLivePlayer) SetVolume(volume float64, muted bool) error {
	volume = clampListenVolume(volume)

	player.mu.Lock()
	player.targetVolume = volume
	player.targetMuted = muted
	window := player.window
	state := player.currentState
	player.mu.Unlock()

	player.dispatchPlaybackState(state, "volume-requested")
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
	return nil
}

func (player *ListenYouTubeLivePlayer) Reset() error {
	if player == nil {
		return nil
	}
	player.mu.Lock()
	window := player.window
	closeHook := player.closeHook
	bridgeHook := player.bridgeHook
	fullscreenHook := player.fullscreenHook
	unfullscreenHook := player.unfullscreenHook
	playbackProvider := player.playbackProvider
	playbackSessionID := player.playbackSessionID
	videoID := player.currentVideo
	volume := player.targetVolume
	muted := player.targetMuted
	player.window = nil
	player.closeHook = nil
	player.bridgeHook = nil
	player.fullscreenHook = nil
	player.unfullscreenHook = nil
	player.currentVideo = ""
	player.loadedProvider = ""
	player.loadedLanguage = ""
	player.currentState = "idle"
	player.activated = false
	player.videoVisible = false
	player.embeddedVisible = false
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedRefreshToken += 1
	fullscreenWaiter := player.clearEmbeddedFullscreenStateLocked()
	player.requestTitle = ""
	player.requestArtist = ""
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.advertising = false
	player.adLabel = ""
	player.errorCode = ""
	player.errorMessage = ""
	player.currentTime = 0
	player.duration = 0
	player.bufferedTime = 0
	player.lastPlayAt = time.Time{}
	playbackEvent := listenplayback.PlaybackBackendEvent{
		Provider:  playbackProvider,
		SessionID: playbackSessionID,
		State:     listenplayback.PlaybackStateIdle,
		Volume:    volume,
		Muted:     muted,
		HasTiming: true,
		HasVolume: true,
	}
	playbackListeners := player.playbackListenersLocked()
	player.playbackProvider = ""
	player.playbackSessionID = ""
	player.controls = ListenLivePlayerControls{}
	player.captionOptions = nil
	player.audioTrackOptions = nil
	player.qualityOptions = nil
	player.playbackRateOptions = nil
	player.selections = ListenLivePlayerSelections{}
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(fullscreenWaiter, false)

	if closeHook != nil {
		closeHook()
	}
	if bridgeHook != nil {
		bridgeHook()
	}
	if fullscreenHook != nil {
		fullscreenHook()
	}
	if unfullscreenHook != nil {
		unfullscreenHook()
	}
	if window != nil {
		if window.IsFullscreen() {
			window.UnFullscreen()
			deadline := time.Now().Add(3 * time.Second)
			for window.IsFullscreen() && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
		}
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
		execListenYouTubeMusicJS(window, listenYouTubeLivePauseScript())
		window.SetURL(listenYouTubeMusicBlankURL)
		window.Close()
	}
	player.dispatch(map[string]any{
		"source":           listenLivePlayerSource,
		"type":             "state",
		"state":            "idle",
		"reason":           "reset",
		"videoId":          videoID,
		"observedVideoId":  videoID,
		"requestedVideoId": videoID,
		"currentTime":      0,
		"duration":         0,
		"volume":           volume,
		"muted":            muted,
		"provider":         playbackProvider,
		"sessionId":        playbackSessionID,
	})
	player.notifyPlaybackListeners(playbackEvent, playbackListeners)
	return nil
}

func (player *ListenYouTubeLivePlayer) ShowWindow() error {
	return player.ShowVideoWindow()
}

func (player *ListenYouTubeLivePlayer) HideWindow() error {
	return player.HideVideoWindow()
}

func (player *ListenYouTubeLivePlayer) ShowVideoWindow() error {
	player.mu.Lock()
	player.videoVisible = true
	player.embeddedVisible = false
	player.embeddedRefreshToken += 1
	volume := player.targetVolume
	muted := player.targetMuted
	window := player.window
	player.mu.Unlock()
	if window == nil {
		return nil
	}
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	window.SetTitle("Listen Live")
	window.SetMinSize(320, 180)
	window.SetSize(720, 405)
	window.Show()
	window.Focus()
	execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
	return nil
}

func (player *ListenYouTubeLivePlayer) HideVideoWindow() error {
	_, err := player.HideVideoWindowForSequence(0)
	return err
}

func (player *ListenYouTubeLivePlayer) HideVideoWindowForSequence(sequence uint64) (bool, error) {
	if player == nil {
		return false, nil
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if sequence > 0 && sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if sequence > player.embeddedSequence {
		player.embeddedSequence = sequence
	}
	player.videoVisible = false
	player.embeddedVisible = false
	player.embeddedRefreshToken += 1
	window := player.window
	nativeWindowFullscreen := player.embeddedNativeWindowFullscreen &&
		listenEmbeddedVideoUsesNativeWindowFullscreen()
	generation := player.embeddedFullscreenMonitor
	elementFullscreen := !nativeWindowFullscreen && (player.embeddedFullscreenActive ||
		player.embeddedFullscreenTransition ||
		player.embeddedFullscreenNativeSeen)
	player.embeddedFullscreenTransition = nativeWindowFullscreen || elementFullscreen
	player.mu.Unlock()
	if window == nil {
		player.mu.Lock()
		waiter := player.clearEmbeddedFullscreenStateLocked()
		player.mu.Unlock()
		completeListenNativeFullscreenWaiter(waiter, false)
		return false, nil
	}
	if nativeWindowFullscreen {
		execListenYouTubeMusicJS(window, listenYouTubeLiveExitVideoModeScript())
		if window.IsFullscreen() {
			window.UnFullscreen()
		} else {
			player.restoreEmbeddedAfterNativeWindowFullscreenLocked(window, generation)
		}
		return true, nil
	}
	if elementFullscreen {
		_ = requestListenEmbeddedVideoFullscreen(
			window,
			listenLivePlayerSource,
			&player.embeddedFullscreen,
			false,
		)
	}
	if window.IsFullscreen() {
		window.UnFullscreen()
	}
	player.mu.Lock()
	waiter := player.clearEmbeddedFullscreenStateLocked()
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(waiter, false)
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	execListenYouTubeMusicJS(window, listenYouTubeLiveExitVideoModeScript())
	hideListenYouTubeMediaWindow(window)
	return true, nil
}

func (player *ListenYouTubeLivePlayer) ShowEmbeddedVideo(rect ListenEmbeddedVideoRect) (bool, error) {
	if player == nil {
		return false, nil
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	rect = normalizeListenEmbeddedVideoRect(rect)
	player.mu.Lock()
	if rect.Sequence > 0 && rect.Sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if rect.Sequence > player.embeddedSequence {
		player.embeddedSequence = rect.Sequence
	}
	wasEmbeddedVisible := player.embeddedVisible
	player.videoVisible = true
	player.embeddedVisible = true
	player.embeddedRect = rect
	volume := player.targetVolume
	muted := player.targetMuted
	playbackProvider := player.playbackProvider
	window := player.window
	fullscreenOwnsPresentation := listenEmbeddedVideoFullscreenOwnsPresentation(
		player.embeddedFullscreenActive,
		player.embeddedFullscreenTransition,
	)
	player.mu.Unlock()
	if window == nil {
		return false, nil
	}
	if nativeOwnsPresentation, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow()); known && nativeOwnsPresentation {
		fullscreenOwnsPresentation = true
	}
	// WebKit owns the native frame during macOS element fullscreen, so changing
	// its geometry would cancel the transition. Windows fullscreen deliberately
	// remains a bottom child HWND of the React host and must continue accepting
	// the host's live geometry so overlay controls stay aligned with the video.
	if fullscreenOwnsPresentation && !listenEmbeddedVideoFullscreenAllowsHostGeometry() {
		return true, nil
	}
	owner := listenClaimEmbeddedVideoOwner(window)
	shown, err := player.showEmbeddedVideoWindow(window, rect)
	if err != nil {
		listenReleaseEmbeddedVideoOwner(owner)
		return false, err
	}
	waitForResize, unregisterResizeWaiter := player.registerEmbeddedVideoResizeWaiter(rect.Sequence)
	defer unregisterResizeWaiter()
	execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(rect))
	execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
	if !wasEmbeddedVisible {
		player.scheduleEmbeddedVideoModeRefresh(window)
	}
	if !shown {
		listenReleaseEmbeddedVideoOwner(owner)
		return false, nil
	}
	resizeReady := player.waitForEmbeddedVideoResize(waitForResize)
	if playbackProvider == listenplayback.PlaybackProviderYouTube && rect.Sequence == 0 {
		resizeReady = false
	}
	return listenLiveEmbeddedVideoRevealReady(
		playbackProvider,
		shown,
		resizeReady,
		listenEmbeddedVideoOwnerActive(owner),
	), nil
}

func (player *ListenYouTubeLivePlayer) HideEmbeddedVideo() error {
	_, err := player.HideEmbeddedVideoForSequence(0)
	return err
}

func (player *ListenYouTubeLivePlayer) HideEmbeddedVideoForSequence(sequence uint64) (bool, error) {
	if player == nil {
		return false, nil
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if sequence > 0 && sequence < player.embeddedSequence {
		player.mu.Unlock()
		return false, nil
	}
	if sequence > player.embeddedSequence {
		player.embeddedSequence = sequence
	}
	player.videoVisible = false
	player.embeddedVisible = false
	player.embeddedRefreshToken += 1
	window := player.window
	nativeWindowFullscreen := player.embeddedNativeWindowFullscreen &&
		listenEmbeddedVideoUsesNativeWindowFullscreen()
	generation := player.embeddedFullscreenMonitor
	elementFullscreen := !nativeWindowFullscreen && (player.embeddedFullscreenActive ||
		player.embeddedFullscreenTransition ||
		player.embeddedFullscreenNativeSeen)
	if nativeWindowFullscreen || elementFullscreen {
		// Keep presentation ownership until the platform exit completes. The
		// The native exit hook will hide without re-embedding because visibility was
		// cleared above.
		player.embeddedFullscreenTransition = true
	}
	player.mu.Unlock()
	if window == nil {
		player.mu.Lock()
		waiter := player.clearEmbeddedFullscreenStateLocked()
		player.mu.Unlock()
		completeListenNativeFullscreenWaiter(waiter, false)
		return false, nil
	}
	if nativeWindowFullscreen {
		if window.IsFullscreen() {
			window.UnFullscreen()
		} else {
			player.restoreEmbeddedAfterNativeWindowFullscreenLocked(window, generation)
		}
		return true, nil
	}
	if elementFullscreen {
		_ = requestListenEmbeddedVideoFullscreen(
			window,
			listenLivePlayerSource,
			&player.embeddedFullscreen,
			false,
		)
	}
	if window.IsFullscreen() {
		window.UnFullscreen()
	}
	player.mu.Lock()
	waiter := player.clearEmbeddedFullscreenStateLocked()
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(waiter, false)
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	hideListenYouTubeMediaWindow(window)
	return true, nil
}

func (player *ListenYouTubeLivePlayer) RequestEmbeddedVideoFullscreen(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if player.playbackProvider != provider || player.playbackSessionID != strings.TrimSpace(sessionID) {
		player.mu.Unlock()
		return fmt.Errorf("stale youtube playback session")
	}
	window := player.window
	visible := player.videoVisible && player.embeddedVisible
	if window != nil && visible && player.embeddedFullscreenActive {
		player.mu.Unlock()
		return nil
	}
	player.mu.Unlock()
	if window == nil || !visible {
		return fmt.Errorf("embedded youtube video unavailable")
	}
	if listenEmbeddedVideoUsesNativeWindowFullscreen() {
		return player.requestEmbeddedVideoNativeWindowFullscreenLocked(window)
	}
	player.mu.Lock()
	player.embeddedFullscreenVersion += 1
	version := player.embeddedFullscreenVersion
	player.embeddedFullscreenMonitor += 1
	monitor := player.embeddedFullscreenMonitor
	player.embeddedFullscreenNativeSeen = false
	player.embeddedFullscreenTransition = window != nil && visible
	player.mu.Unlock()
	go player.monitorNativeEmbeddedVideoFullscreen(window, monitor)
	err := requestListenEmbeddedVideoFullscreen(
		window,
		listenLivePlayerSource,
		&player.embeddedFullscreen,
		true,
	)
	nativeOwnsPresentation, nativeFullscreenKnown :=
		listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow())
	if err != nil && nativeFullscreenKnown && nativeOwnsPresentation {
		err = nil
	}
	player.mu.Lock()
	nativeChanged := false
	if player.window == window {
		if nativeFullscreenKnown && nativeOwnsPresentation {
			player.embeddedFullscreenNativeSeen = true
			if !player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
				player.embeddedFullscreenVersion += 1
				player.embeddedFullscreenActive = true
				player.embeddedFullscreenTransition = false
				nativeChanged = true
			}
		} else if player.embeddedFullscreenVersion == version {
			player.embeddedFullscreenTransition = false
			if err == nil {
				player.embeddedFullscreenActive = true
			}
		}
	}
	currentProvider := player.playbackProvider
	currentSessionID := player.playbackSessionID
	player.mu.Unlock()
	if nativeChanged {
		player.dispatchEmbeddedVideoFullscreenChange(
			currentProvider,
			currentSessionID,
			true,
			"native-fullscreen-request",
		)
	}
	return err
}

func (player *ListenYouTubeLivePlayer) requestEmbeddedVideoNativeWindowFullscreenLocked(
	window *application.WebviewWindow,
) error {
	if player == nil || window == nil {
		return fmt.Errorf("embedded youtube video unavailable")
	}
	waiter := make(chan bool, 1)
	player.mu.Lock()
	if player.window != window || !player.videoVisible || !player.embeddedVisible {
		player.mu.Unlock()
		return fmt.Errorf("embedded youtube video unavailable")
	}
	player.embeddedFullscreenVersion += 1
	player.embeddedFullscreenMonitor += 1
	generation := player.embeddedFullscreenMonitor
	player.embeddedRefreshToken += 1
	player.embeddedFullscreenNativeSeen = false
	player.embeddedNativeWindowFullscreen = true
	player.embeddedNativeFullscreenWaiter = waiter
	player.embeddedFullscreenActive = false
	player.embeddedFullscreenTransition = true
	player.mu.Unlock()

	// Native window fullscreen does not depend on HTML transient activation.
	// Restore the singleton WebView to its owning player window first, let that
	// window participate in one UI turn, then ask the OS to fullscreen it.
	execListenYouTubeMusicJS(window, listenYouTubeLiveNativeWindowFullscreenModeScript(true))
	if !detachListenNativeEmbeddedWebViewForFullscreen(window.NativeWindow()) {
		player.mu.Lock()
		if player.window == window && player.embeddedFullscreenMonitor == generation {
			pendingWaiter := player.clearEmbeddedFullscreenStateLocked()
			player.mu.Unlock()
			completeListenNativeFullscreenWaiter(pendingWaiter, false)
		} else {
			player.mu.Unlock()
		}
		return fmt.Errorf("embedded youtube video could not detach for native fullscreen")
	}
	window.SetTitle("YouTube")
	var hostWindow *application.WebviewWindow
	if player.windows != nil {
		hostWindow = player.windows.mainWindow
	}
	prepareListenNativeFullscreenWindow(window, hostWindow, 720, 405)
	window.Show()
	window.Focus()
	if delay := listenNativeWindowFullscreenPreparationDelay(); delay > 0 {
		time.Sleep(delay)
	}

	player.mu.Lock()
	valid := player.window == window &&
		player.embeddedFullscreenMonitor == generation &&
		player.embeddedNativeWindowFullscreen
	player.mu.Unlock()
	if !valid {
		return fmt.Errorf("stale youtube fullscreen request")
	}
	window.Fullscreen()

	entered := waitForListenNativeWindowFullscreenState(
		waiter,
		true,
		listenEmbeddedVideoFullscreenTimeout,
		nil,
	)
	if !entered {
		entered = window.IsFullscreen()
	}
	player.mu.Lock()
	if player.embeddedNativeFullscreenWaiter == waiter {
		player.embeddedNativeFullscreenWaiter = nil
	}
	valid = player.window == window &&
		player.embeddedFullscreenMonitor == generation &&
		player.embeddedNativeWindowFullscreen
	if entered && valid {
		changed := !player.embeddedFullscreenActive || player.embeddedFullscreenTransition
		player.embeddedFullscreenActive = true
		player.embeddedFullscreenTransition = false
		provider := player.playbackProvider
		sessionID := player.playbackSessionID
		player.mu.Unlock()
		if changed {
			player.dispatchEmbeddedVideoFullscreenChange(provider, sessionID, true, "native-window-fullscreen")
		}
		return nil
	}
	player.mu.Unlock()
	if valid {
		player.restoreEmbeddedAfterNativeWindowFullscreenLocked(window, generation)
	}
	return fmt.Errorf("the native video window did not enter fullscreen")
}

func (player *ListenYouTubeLivePlayer) handleNativeWindowFullscreenEvent(
	window *application.WebviewWindow,
	active bool,
) {
	if player == nil || window == nil || !listenEmbeddedVideoUsesNativeWindowFullscreen() {
		return
	}
	player.mu.Lock()
	if player.window != window || !player.embeddedNativeWindowFullscreen {
		player.mu.Unlock()
		return
	}
	waiter := player.embeddedNativeFullscreenWaiter
	generation := player.embeddedFullscreenMonitor
	provider := player.playbackProvider
	sessionID := player.playbackSessionID
	changed := false
	if active {
		changed = !player.embeddedFullscreenActive || player.embeddedFullscreenTransition
		player.embeddedFullscreenActive = true
		player.embeddedFullscreenTransition = false
	} else {
		// Keep inline geometry suspended until the WebView is back under the main
		// window. The restore goroutine publishes the authoritative false event.
		player.embeddedFullscreenTransition = true
	}
	player.mu.Unlock()
	if waiter != nil {
		select {
		case waiter <- active:
		default:
		}
	}
	if active {
		if changed {
			player.dispatchEmbeddedVideoFullscreenChange(provider, sessionID, true, "native-window-fullscreen")
		}
		return
	}
	go player.restoreEmbeddedAfterNativeWindowFullscreen(window, generation)
}

func (player *ListenYouTubeLivePlayer) restoreEmbeddedAfterNativeWindowFullscreen(
	window *application.WebviewWindow,
	generation uint64,
) {
	if player == nil || window == nil {
		return
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.restoreEmbeddedAfterNativeWindowFullscreenLocked(window, generation)
}

func (player *ListenYouTubeLivePlayer) handlePlayerWindowClose(
	window *application.WebviewWindow,
) {
	if player == nil || window == nil {
		return
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if player.window != window {
		player.mu.Unlock()
		return
	}
	wasVideoVisible := player.videoVisible
	wasEmbedded := player.embeddedVisible
	nativeWindowFullscreen := player.embeddedNativeWindowFullscreen &&
		listenEmbeddedVideoUsesNativeWindowFullscreen()
	nativeGeneration := player.embeddedFullscreenMonitor
	elementFullscreen := !nativeWindowFullscreen && (player.embeddedFullscreenActive ||
		player.embeddedFullscreenTransition ||
		player.embeddedFullscreenNativeSeen)
	player.videoVisible = false
	player.embeddedVisible = false
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedRefreshToken += 1
	player.embeddedFullscreenTransition = nativeWindowFullscreen || elementFullscreen
	player.mu.Unlock()

	if nativeWindowFullscreen {
		if wasVideoVisible {
			execListenYouTubeMusicJS(window, listenYouTubeLiveExitVideoModeScript())
			player.dispatchVideoClosed()
		}
		if window.IsFullscreen() {
			window.UnFullscreen()
		} else {
			player.restoreEmbeddedAfterNativeWindowFullscreenLocked(window, nativeGeneration)
		}
		return
	}
	if elementFullscreen {
		_ = requestListenEmbeddedVideoFullscreen(
			window,
			listenLivePlayerSource,
			&player.embeddedFullscreen,
			false,
		)
	}
	if window.IsFullscreen() {
		window.UnFullscreen()
	}
	player.mu.Lock()
	waiter := player.clearEmbeddedFullscreenStateLocked()
	player.mu.Unlock()
	completeListenNativeFullscreenWaiter(waiter, false)
	if wasEmbedded {
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
	}
	if wasVideoVisible {
		execListenYouTubeMusicJS(window, listenYouTubeLiveExitVideoModeScript())
		player.dispatchVideoClosed()
	}
	hideListenYouTubeMediaWindow(window)
}

func (player *ListenYouTubeLivePlayer) dispatchVideoClosed() {
	provider, sessionID := player.CurrentPlaybackIdentity()
	player.dispatch(map[string]any{
		"source":    listenLivePlayerSource,
		"type":      "video-closed",
		"provider":  provider,
		"sessionId": sessionID,
	})
}

// clearEmbeddedFullscreenStateLocked invalidates both the DOM/native watcher
// and the native player-window path. The caller must hold player.mu and signal
// the returned waiter only after releasing it.
func (player *ListenYouTubeLivePlayer) clearEmbeddedFullscreenStateLocked() chan bool {
	player.embeddedFullscreenVersion += 1
	player.embeddedFullscreenMonitor += 1
	player.embeddedFullscreenNativeSeen = false
	player.embeddedNativeWindowFullscreen = false
	waiter := player.embeddedNativeFullscreenWaiter
	player.embeddedNativeFullscreenWaiter = nil
	player.embeddedFullscreenActive = false
	player.embeddedFullscreenTransition = false
	return waiter
}

func completeListenNativeFullscreenWaiter(waiter chan bool, active bool) {
	if waiter == nil {
		return
	}
	select {
	case waiter <- active:
	default:
	}
}

// restoreEmbeddedAfterNativeWindowFullscreenLocked returns the WebView to the
// latest inline rect before telling the frontend that fullscreen has ended.
// The caller must hold commandMu so delayed geometry refreshes cannot race the
// native reparent.
func (player *ListenYouTubeLivePlayer) restoreEmbeddedAfterNativeWindowFullscreenLocked(
	window *application.WebviewWindow,
	generation uint64,
) {
	player.mu.Lock()
	if player.window != window ||
		player.embeddedFullscreenMonitor != generation ||
		!player.embeddedNativeWindowFullscreen {
		player.mu.Unlock()
		return
	}
	shouldEmbed := player.videoVisible && player.embeddedVisible
	rect := player.embeddedRect
	volume := player.targetVolume
	muted := player.targetMuted
	provider := player.playbackProvider
	sessionID := player.playbackSessionID
	player.mu.Unlock()

	execListenYouTubeMusicJS(window, listenYouTubeLiveNativeWindowFullscreenModeScript(false))
	hideListenYouTubeMediaWindow(window)
	shown := false
	if shouldEmbed {
		listenClaimEmbeddedVideoOwner(window)
		shown, _ = player.showEmbeddedVideoWindow(window, rect)
		if shown {
			execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(rect))
			execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
		}
	} else {
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	}

	player.mu.Lock()
	if player.window != window || player.embeddedFullscreenMonitor != generation {
		player.mu.Unlock()
		return
	}
	player.embeddedFullscreenVersion += 1
	player.embeddedFullscreenNativeSeen = false
	player.embeddedNativeWindowFullscreen = false
	player.embeddedNativeFullscreenWaiter = nil
	player.embeddedFullscreenActive = false
	player.embeddedFullscreenTransition = false
	player.mu.Unlock()
	if shown {
		player.scheduleEmbeddedVideoModeRefresh(window)
	}
	player.dispatchEmbeddedVideoFullscreenChange(provider, sessionID, false, "native-window-unfullscreen")
}

func (player *ListenYouTubeLivePlayer) dispatchEmbeddedVideoFullscreenChange(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	active bool,
	reason string,
) {
	player.dispatch(map[string]any{
		"source":    listenLivePlayerSource,
		"type":      listenEmbeddedVideoFullscreenChangeType,
		"provider":  provider,
		"sessionId": sessionID,
		"active":    active,
		"reason":    reason,
	})
}

func (player *ListenYouTubeLivePlayer) monitorNativeEmbeddedVideoFullscreen(
	window *application.WebviewWindow,
	monitor uint64,
) {
	if player == nil || window == nil || monitor == 0 {
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(5 * time.Second)
	for {
		player.mu.Lock()
		if player.window != window || player.embeddedFullscreenMonitor != monitor {
			player.mu.Unlock()
			return
		}
		nativeSeen := player.embeddedFullscreenNativeSeen
		player.mu.Unlock()
		ownsPresentation, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow())
		if !known {
			if !nativeSeen && time.Now().After(deadline) {
				return
			}
			<-ticker.C
			continue
		}
		changed, finished, provider, sessionID, valid :=
			player.applyNativeEmbeddedVideoFullscreenState(window, monitor, ownsPresentation)
		if !valid {
			return
		}
		if changed {
			player.dispatchEmbeddedVideoFullscreenChange(
				provider,
				sessionID,
				ownsPresentation,
				"native-fullscreen-state",
			)
		}
		if finished {
			return
		}
		if !ownsPresentation && time.Now().After(deadline) {
			return
		}
		<-ticker.C
	}
}

func (player *ListenYouTubeLivePlayer) applyNativeEmbeddedVideoFullscreenState(
	window *application.WebviewWindow,
	monitor uint64,
	ownsPresentation bool,
) (changed bool, finished bool, provider listenplayback.PlaybackProvider, sessionID string, valid bool) {
	if player == nil || window == nil || monitor == 0 {
		return false, false, "", "", false
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.window != window || player.embeddedFullscreenMonitor != monitor {
		return false, false, "", "", false
	}
	wasNativeSeen := player.embeddedFullscreenNativeSeen
	if ownsPresentation {
		player.embeddedFullscreenNativeSeen = true
		if !player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
			player.embeddedFullscreenVersion += 1
			player.embeddedFullscreenActive = true
			player.embeddedFullscreenTransition = false
			changed = true
		}
	} else if wasNativeSeen {
		player.embeddedFullscreenNativeSeen = false
		finished = true
		if player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
			player.embeddedFullscreenVersion += 1
			player.embeddedFullscreenActive = false
			player.embeddedFullscreenTransition = false
			changed = true
		}
	}
	return changed, finished, player.playbackProvider, player.playbackSessionID, true
}

func (player *ListenYouTubeLivePlayer) ExitEmbeddedVideoFullscreen(
	provider listenplayback.PlaybackProvider,
	sessionID string,
) error {
	if player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	player.commandMu.Lock()
	defer player.commandMu.Unlock()
	player.mu.Lock()
	if player.playbackProvider != provider || player.playbackSessionID != strings.TrimSpace(sessionID) {
		player.mu.Unlock()
		return fmt.Errorf("stale youtube playback session")
	}
	window := player.window
	if player.embeddedNativeWindowFullscreen && listenEmbeddedVideoUsesNativeWindowFullscreen() {
		waiter := make(chan bool, 1)
		player.embeddedNativeFullscreenWaiter = waiter
		player.embeddedFullscreenTransition = true
		generation := player.embeddedFullscreenMonitor
		player.mu.Unlock()
		if window == nil {
			return fmt.Errorf("embedded youtube video unavailable")
		}
		window.UnFullscreen()
		exited := waitForListenNativeWindowFullscreenState(
			waiter,
			false,
			2*listenEmbeddedVideoFullscreenTimeout,
			func(active bool) {
				// An exit can be requested while the native window is still entering. The
				// first UnFullscreen call is then a no-op; retry as soon as the
				// delayed did-enter event arrives instead of mistaking true for
				// exit completion.
				if active {
					window.UnFullscreen()
				}
			},
		)
		if exited {
			player.mu.Lock()
			if player.embeddedNativeFullscreenWaiter == waiter {
				player.embeddedNativeFullscreenWaiter = nil
			}
			player.mu.Unlock()
			return nil
		}
		player.mu.Lock()
		if player.embeddedNativeFullscreenWaiter == waiter {
			player.embeddedNativeFullscreenWaiter = nil
		}
		stillCurrent := player.window == window &&
			player.embeddedFullscreenMonitor == generation &&
			player.embeddedNativeWindowFullscreen
		player.mu.Unlock()
		if stillCurrent && !window.IsFullscreen() {
			player.restoreEmbeddedAfterNativeWindowFullscreenLocked(window, generation)
			return nil
		}
		return fmt.Errorf("the native video window did not exit fullscreen")
	}
	player.embeddedFullscreenVersion += 1
	version := player.embeddedFullscreenVersion
	player.embeddedFullscreenTransition = window != nil
	player.mu.Unlock()
	if window == nil {
		return fmt.Errorf("embedded youtube video unavailable")
	}
	err := requestListenEmbeddedVideoFullscreen(
		window,
		listenLivePlayerSource,
		&player.embeddedFullscreen,
		false,
	)
	player.mu.Lock()
	if player.window == window && player.embeddedFullscreenVersion == version {
		player.embeddedFullscreenTransition = false
		if err == nil {
			player.embeddedFullscreenActive = false
		}
	}
	player.mu.Unlock()
	return err
}

func waitForListenNativeWindowFullscreenState(
	waiter <-chan bool,
	want bool,
	timeout time.Duration,
	onUnexpected func(bool),
) bool {
	if waiter == nil || timeout <= 0 {
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case active := <-waiter:
			if active == want {
				return true
			}
			if onUnexpected != nil {
				onUnexpected(active)
			}
		case <-timer.C:
			return false
		}
	}
}

func (player *ListenYouTubeLivePlayer) ShowAirPlayPicker(anchor ListenAirPlayAnchor) error {
	if player != nil && player.windows != nil && player.windows.mainWindow != nil {
		if showListenNativeAirPlayPicker(player.windows.mainWindow.NativeWindow(), anchor) {
			return nil
		}
	}
	return nil
}

func (player *ListenYouTubeLivePlayer) showEmbeddedVideoWindow(window *application.WebviewWindow, rect ListenEmbeddedVideoRect) (bool, error) {
	if player == nil || window == nil || player.windows == nil || player.windows.mainWindow == nil {
		return false, nil
	}
	owner := listenEmbeddedVideoOwnerID(window)
	shown := listenShowNativeEmbeddedWebViewForOwner(
		owner,
		window,
		player.windows.mainWindow,
		rect,
	)
	if !shown {
		return false, fmt.Errorf("embedded listen live video unavailable")
	}
	return true, nil
}

func (player *ListenYouTubeLivePlayer) scheduleEmbeddedVideoModeRefresh(window *application.WebviewWindow) {
	if player == nil || window == nil {
		return
	}
	windowID := window.ID()
	owner := listenEmbeddedVideoOwnerID(window)
	player.mu.Lock()
	player.embeddedRefreshToken += 1
	token := player.embeddedRefreshToken
	player.mu.Unlock()
	go func() {
		for _, delay := range []time.Duration{
			250 * time.Millisecond,
			750 * time.Millisecond,
			1500 * time.Millisecond,
			3 * time.Second,
		} {
			time.Sleep(delay)
			keepRefreshing := func() bool {
				// Serialize refresh geometry with the fullscreen command. A refresh
				// that reparents or resizes the embedded WebView while WebKit owns
				// its fullscreen presentation immediately tears fullscreen down.
				player.commandMu.Lock()
				defer player.commandMu.Unlock()

				player.mu.Lock()
				active := player.videoVisible &&
					player.embeddedVisible &&
					player.window != nil &&
					player.window.ID() == windowID &&
					player.embeddedRefreshToken == token
				fullscreenOwnsPresentation := listenEmbeddedVideoFullscreenOwnsPresentation(
					player.embeddedFullscreenActive,
					player.embeddedFullscreenTransition,
				)
				rect := player.embeddedRect
				volume := player.targetVolume
				muted := player.targetMuted
				player.mu.Unlock()
				if nativeOwnsPresentation, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow()); known && nativeOwnsPresentation {
					fullscreenOwnsPresentation = true
				}
				if !active {
					return false
				}
				if fullscreenOwnsPresentation && !listenEmbeddedVideoFullscreenAllowsHostGeometry() {
					// The WebView remains in its fullscreen owner, but a newly
					// navigated document still needs the video-only CSS and volume
					// bridge. Do not pass the stale inline resize target here.
					execListenYouTubeMusicJS(window, listenYouTubeLiveNativeWindowFullscreenModeScript(true))
					execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript())
					execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
					return true
				}
				if !listenEmbeddedVideoOwnerActive(owner) {
					return false
				}
				_, _ = player.showEmbeddedVideoWindow(window, rect)
				execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(rect))
				execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
				return true
			}()
			if !keepRefreshing {
				return
			}
		}
	}()
}

func listenEmbeddedVideoFullscreenOwnsPresentation(active bool, transitioning bool) bool {
	return active || transitioning
}

// A regular YouTube /watch document contains the complete site chrome. Unlike
// the lightweight media/embed surfaces, mounting its native WebView is not
// sufficient proof that it is safe to reveal. The document bridge must first
// confirm that the selected video, its player root, and every clipping ancestor
// have been converted to the viewport-sized video-only presentation.
func listenYouTubeEmbeddedVideoRevealReady(nativeShown bool, resizeReady bool, ownerActive bool) bool {
	return nativeShown && resizeReady && ownerActive
}

func listenLiveEmbeddedVideoRevealReady(
	provider listenplayback.PlaybackProvider,
	nativeShown bool,
	resizeReady bool,
	ownerActive bool,
) bool {
	if provider == listenplayback.PlaybackProviderYouTube {
		return listenYouTubeEmbeddedVideoRevealReady(nativeShown, resizeReady, ownerActive)
	}
	return listenEmbeddedVideoRevealReady(nativeShown, resizeReady, ownerActive)
}

func listenEmbeddedVideoCanApplyInlineGeometry(
	embeddedVisible bool,
	fullscreenActive bool,
	fullscreenTransitioning bool,
	nativeWindowFullscreen bool,
) bool {
	return embeddedVisible &&
		!nativeWindowFullscreen &&
		(!listenEmbeddedVideoFullscreenOwnsPresentation(fullscreenActive, fullscreenTransitioning) ||
			listenEmbeddedVideoFullscreenAllowsHostGeometry())
}

func (player *ListenYouTubeLivePlayer) registerEmbeddedVideoResizeWaiter(sequence uint64) (<-chan bool, func()) {
	if player == nil || sequence == 0 {
		return nil, func() {}
	}
	waiter := make(chan bool, 1)
	player.mu.Lock()
	if player.embeddedResizeWaiters == nil {
		player.embeddedResizeWaiters = make(map[uint64]chan bool)
	}
	player.embeddedResizeWaiters[sequence] = waiter
	player.mu.Unlock()
	return waiter, func() {
		player.mu.Lock()
		if player.embeddedResizeWaiters != nil {
			delete(player.embeddedResizeWaiters, sequence)
		}
		player.mu.Unlock()
	}
}

func (player *ListenYouTubeLivePlayer) waitForEmbeddedVideoResize(waiter <-chan bool) bool {
	if waiter == nil {
		return true
	}
	select {
	case ready := <-waiter:
		return ready
	case <-time.After(listenEmbeddedVideoResizeTimeout):
		return false
	}
}

func (player *ListenYouTubeLivePlayer) completeEmbeddedVideoResize(sequence uint64, ready bool) bool {
	if player == nil || sequence == 0 {
		return false
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.embeddedResizeWaiters == nil {
		return false
	}
	waiter, ok := player.embeddedResizeWaiters[sequence]
	if !ok {
		return false
	}
	delete(player.embeddedResizeWaiters, sequence)
	select {
	case waiter <- ready:
	default:
	}
	return true
}

func (player *ListenYouTubeLivePlayer) Status() ListenLivePlayerStatus {
	player.mu.Lock()
	defer player.mu.Unlock()
	title := player.observedTitle
	if title == "" {
		title = player.requestTitle
	}
	artist := player.observedArtist
	if artist == "" {
		artist = player.requestArtist
	}
	return ListenLivePlayerStatus{
		ListenPlayerStatus: ListenPlayerStatus{
			Provider:        player.playbackProvider,
			SessionID:       player.playbackSessionID,
			Available:       player.window != nil,
			VideoID:         player.currentVideo,
			ObservedVideoID: player.currentVideo,
			State:           player.currentState,
			Title:           title,
			Artist:          artist,
			ThumbnailURL:    player.observedThumb,
			LikeStatus:      player.selections.Rating,
			Advertising:     player.advertising,
			AdLabel:         player.adLabel,
			ErrorCode:       player.errorCode,
			ErrorMessage:    player.errorMessage,
			CurrentTime:     player.currentTime,
			Duration:        player.duration,
			BufferedTime:    player.bufferedTime,
			Volume:          player.targetVolume,
			Muted:           player.targetMuted,
		},
		Controls:          player.controls,
		CaptionOptions:    append([]ListenLivePlayerOption(nil), player.captionOptions...),
		AudioTrackOptions: append([]ListenLivePlayerOption(nil), player.audioTrackOptions...),
		QualityOptions:    append([]ListenLivePlayerOption(nil), player.qualityOptions...),
		PlaybackRateOptions: append(
			[]ListenLivePlayerOption(nil),
			player.playbackRateOptions...,
		),
		Selections: player.selections,
	}
}

func (player *ListenYouTubeLivePlayer) HandleRawMessage(window application.Window, message string, _ *application.OriginInfo) bool {
	if player == nil || window == nil || window.Name() != listenLivePlayerWindowName {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	if source, _ := payload["source"].(string); source != listenLivePlayerSource {
		return false
	}

	player.mu.Lock()
	activeWindow := player.window
	player.mu.Unlock()
	if activeWindow == nil || window.ID() != activeWindow.ID() {
		return true
	}
	return player.handlePlaybackPayload(payload)
}

func (player *ListenYouTubeLivePlayer) handlePlaybackPayload(payload map[string]any) bool {
	eventType := listenPayloadString(payload, "type")
	if eventType == listenEmbeddedVideoResizeReadyType {
		sequence, _ := listenPayloadUint64(payload, "sequence")
		ready := listenPayloadBool(payload, "ready")
		player.completeEmbeddedVideoResize(sequence, ready)
		return true
	}
	if eventType == listenEmbeddedVideoFullscreenResultType {
		requestID, _ := listenPayloadUint64(payload, "requestId")
		player.embeddedFullscreen.complete(requestID, listenEmbeddedVideoFullscreenResult{
			succeeded: listenPayloadBool(payload, "succeeded"),
			message:   listenPayloadString(payload, "message"),
		})
		return true
	}
	if eventType == listenEmbeddedVideoFullscreenChangeType {
		active := listenPayloadBool(payload, "active")
		reason := listenPayloadString(payload, "reason")
		player.mu.Lock()
		wasActive := player.embeddedFullscreenActive
		transitioning := player.embeddedFullscreenTransition
		nativeSeen := player.embeddedFullscreenNativeSeen
		// Some WebKit builds emit webkitbeginfullscreen before
		// HTMLVideoElement.webkitDisplayingFullscreen becomes true. A caption
		// track can make that native hand-off long enough for a stale inactive
		// snapshot to arrive first. Do not give inline geometry back while an
		// enter request is still pending; the request result will clear the
		// transition if fullscreen is genuinely rejected.
		if !active && transitioning && !wasActive {
			player.mu.Unlock()
			return true
		}
		// Polling may discover entry, but it is not authoritative for exit.
		// WKWebView can clear the DOM fullscreen snapshot while its native
		// presentation is still active; accepting that stale false causes the
		// delayed embedded refresh to resize the view and tear fullscreen down.
		if !active && reason == "poll" && (wasActive || transitioning) {
			player.mu.Unlock()
			return true
		}
		// Once the native watcher has observed WebKit ownership, JavaScript exit
		// signals are hints only. The watcher publishes the definitive false when
		// WKFullscreenState reaches NotInFullscreen, after WebKit has restored the
		// view hierarchy.
		if !active && nativeSeen {
			player.mu.Unlock()
			return true
		}
		if active == wasActive && !transitioning {
			player.mu.Unlock()
			return true
		}
		player.embeddedFullscreenVersion += 1
		player.embeddedFullscreenActive = active
		player.embeddedFullscreenTransition = false
		provider := player.playbackProvider
		sessionID := player.playbackSessionID
		player.mu.Unlock()
		// This is deliberately a dedicated event rather than a partial player
		// status. YouTubeWorkspacePage can suspend native geometry without
		// replacing its title, timing, controls, or other playback state.
		player.dispatchEmbeddedVideoFullscreenChange(provider, sessionID, active, reason)
		return true
	}
	state := listenPayloadString(payload, "state")
	observedVideoID := listenPayloadString(payload, "observedVideoId")
	requestedVideoID := listenPayloadString(payload, "requestedVideoId")
	videoID := observedVideoID
	if videoID == "" {
		videoID = listenPayloadString(payload, "videoId")
	}
	title := listenPayloadString(payload, "title")
	artist := listenPayloadString(payload, "artist")
	thumbnailURL := listenPayloadString(payload, "thumbnailUrl")
	advertising := listenPayloadBool(payload, "advertising") || listenPayloadBool(payload, "ad")
	adLabel := listenPayloadString(payload, "adLabel")
	errorCode := listenPayloadDisplayString(payload, "errorCode")
	if errorCode == "" {
		errorCode = listenPayloadDisplayString(payload, "code")
	}
	errorMessage := listenPayloadString(payload, "errorMessage")
	if errorMessage == "" {
		errorMessage = listenPayloadString(payload, "message")
	}
	currentTime, hasCurrentTime := listenPayloadFloat(payload, "currentTime")
	duration, hasDuration := listenPayloadFloat(payload, "duration")
	bufferedTime, hasBufferedTime := listenPayloadFloat(payload, "bufferedTime")
	_, hasObservedVolume := listenPayloadFloat(payload, "volume")
	_, hasObservedMuted := listenPayloadBoolValue(payload, "muted")
	controls, hasControls := listenLiveControlsFromPayload(payload["controls"])
	captionOptions, hasCaptionOptions := listenLiveOptionsFromPayload(payload["captionOptions"])
	audioTrackOptions, hasAudioTrackOptions := listenLiveOptionsFromPayload(payload["audioTrackOptions"])
	qualityOptions, hasQualityOptions := listenLiveOptionsFromPayload(payload["qualityOptions"])
	playbackRateOptions, hasPlaybackRateOptions := listenLiveOptionsFromPayload(payload["playbackRateOptions"])
	selections, hasSelections := listenLiveSelectionsFromPayload(payload["selections"])

	player.mu.Lock()
	currentVideo := player.currentVideo
	requestTitle := player.requestTitle
	requestArtist := player.requestArtist
	videoVisible := player.videoVisible
	hideAfterActivation := false
	windowToHide := player.window
	if videoID != "" && currentVideo != "" && videoID != currentVideo && !advertising {
		player.mu.Unlock()
		return true
	}
	if requestedVideoID == "" {
		requestedVideoID = currentVideo
	}
	if observedVideoID == "" {
		observedVideoID = videoID
	}
	if videoID == "" || videoID != currentVideo {
		videoID = currentVideo
	}
	if observedVideoID == "" {
		observedVideoID = videoID
	}
	if requestedVideoID == "" {
		requestedVideoID = videoID
	}
	if title == "" {
		title = requestTitle
	}
	if artist == "" {
		artist = requestArtist
	}
	if eventType == "track-ended" && state == "" {
		state = "ended"
	}
	if state != "" {
		player.currentState = state
		if state == "playing" && !player.activated && !videoVisible {
			player.activated = true
			hideAfterActivation = true
		}
	}
	if title != "" {
		player.observedTitle = title
	}
	if artist != "" {
		player.observedArtist = artist
	}
	if thumbnailURL != "" {
		player.observedThumb = thumbnailURL
	}
	player.advertising = advertising
	player.adLabel = adLabel
	if state == "error" {
		player.errorCode = errorCode
		player.errorMessage = errorMessage
	} else if state != "" {
		player.errorCode = ""
		player.errorMessage = ""
	}
	if hasCurrentTime {
		player.currentTime = currentTime
	}
	if hasDuration {
		player.duration = duration
	}
	if hasBufferedTime {
		player.bufferedTime = bufferedTime
	}
	// WebView status is observed state, not command authority. A delayed
	// volumechange from YouTube must not roll the app's latest desired value
	// backward while a slider command is in flight.
	if hasControls {
		player.controls = controls
	}
	if hasCaptionOptions {
		player.captionOptions = captionOptions
	}
	if hasAudioTrackOptions {
		player.audioTrackOptions = audioTrackOptions
	}
	if hasQualityOptions {
		player.qualityOptions = qualityOptions
	}
	if hasPlaybackRateOptions {
		player.playbackRateOptions = playbackRateOptions
	}
	if hasSelections {
		player.selections = selections
	}
	playbackState := listenLivePlaybackState(player.currentState)
	if state == "" && !hasCurrentTime && !hasDuration && errorMessage == "" {
		playbackState = ""
	}
	playbackEvent := listenplayback.PlaybackBackendEvent{
		Provider:  player.playbackProvider,
		SessionID: player.playbackSessionID,
		State:     playbackState,
		Position:  player.currentTime,
		Duration:  player.duration,
		Volume:    player.targetVolume,
		Muted:     player.targetMuted,
		Error:     player.errorMessage,
		HasTiming: hasCurrentTime || hasDuration,
		HasVolume: hasObservedVolume || hasObservedMuted || state != "",
	}
	playbackListeners := player.playbackListenersLocked()
	playbackProvider := player.playbackProvider
	playbackSessionID := player.playbackSessionID
	player.mu.Unlock()

	if hideAfterActivation && windowToHide != nil {
		hideListenYouTubeMediaWindow(windowToHide)
	}

	if state != "" {
		payload["state"] = state
	}
	if videoID != "" {
		payload["videoId"] = videoID
		payload["observedVideoId"] = observedVideoID
		payload["requestedVideoId"] = requestedVideoID
	}
	if title != "" {
		payload["title"] = title
	}
	if artist != "" {
		payload["artist"] = artist
	}
	payload["volume"] = playbackEvent.Volume
	payload["muted"] = playbackEvent.Muted
	if playbackProvider != "" {
		payload["provider"] = playbackProvider
	}
	if playbackSessionID != "" {
		payload["sessionId"] = playbackSessionID
	}
	if thumbnailURL != "" {
		payload["thumbnailUrl"] = thumbnailURL
	}
	payload["advertising"] = advertising
	if adLabel != "" {
		payload["adLabel"] = adLabel
	}
	if state == "error" {
		if errorCode != "" {
			payload["errorCode"] = errorCode
			payload["code"] = errorCode
		}
		if errorMessage != "" {
			payload["errorMessage"] = errorMessage
			payload["message"] = errorMessage
		}
	}
	player.dispatch(payload)
	player.notifyPlaybackListeners(playbackEvent, playbackListeners)
	return true
}

func (player *ListenYouTubeLivePlayer) currentWindow() *application.WebviewWindow {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.window
}

func (player *ListenYouTubeLivePlayer) createWindowLocked(request ListenPlayerPlayRequest) *application.WebviewWindow {
	if player.app == nil {
		return nil
	}

	bridgeScript := listenYouTubeLiveBridgeScript(request)
	window := player.app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{
		Name:        listenLivePlayerWindowName,
		Title:       "Listen Live",
		Width:       720,
		Height:      405,
		MinWidth:    320,
		MinHeight:   180,
		URL:         listenYouTubeMusicBlankURL,
		JS:          bridgeScript,
		Hidden:      true,
		AlwaysOnTop: false,
		Windows: application.WindowsWindow{
			Permissions: map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState{
				remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow,
			},
		},
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				FullscreenEnabled: application.Enabled,
			},
		},
	}))
	registerWebViewRemoteCapabilityPolicy(window)
	configureListenYouTubeMusicNativeWindow(window.NativeWindow(), listenYouTubeMusicUserAgent())
	bridgeHook, bridgeInstalled := attachListenYouTubeMusicBridge(window, bridgeScript)
	if !bridgeInstalled {
		window.Close()
		return nil
	}
	player.closeHook = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		go player.handlePlayerWindowClose(window)
	})
	player.fullscreenHook = window.RegisterHook(events.Common.WindowFullscreen, func(_ *application.WindowEvent) {
		player.handleNativeWindowFullscreenEvent(window, true)
	})
	player.unfullscreenHook = window.RegisterHook(events.Common.WindowUnFullscreen, func(_ *application.WindowEvent) {
		player.handleNativeWindowFullscreenEvent(window, false)
	})
	installListenNativeWindowFullscreenEscape(window)
	player.bridgeHook = bridgeHook
	player.window = window
	return window
}

func (player *ListenYouTubeLivePlayer) playbackCookies(ctx context.Context) []appcookies.Record {
	if player == nil || player.cookies == nil {
		return nil
	}
	records, err := player.cookies.RecordsForSiteKey(ctx, "youtube")
	if err != nil {
		return nil
	}
	return filterListenPlaybackCookies(appcookies.MatchURL(records, listenYouTubeOrigin+"/"), time.Now())
}

func (player *ListenYouTubeLivePlayer) dispatch(payload map[string]any) {
	if player == nil || player.windows == nil {
		return
	}
	player.windows.dispatchWindowEvent(listenLivePlayerEventName, payload)
}

func (player *ListenYouTubeLivePlayer) dispatchPlaybackState(state string, reason string) {
	if player == nil {
		return
	}
	player.mu.Lock()
	player.currentState = state
	videoID := player.currentVideo
	title := player.requestTitle
	artist := player.requestArtist
	advertising := player.advertising
	adLabel := player.adLabel
	errorCode := player.errorCode
	errorMessage := player.errorMessage
	currentTime := player.currentTime
	duration := player.duration
	volume := player.targetVolume
	muted := player.targetMuted
	playbackEvent := listenplayback.PlaybackBackendEvent{
		Provider:  player.playbackProvider,
		SessionID: player.playbackSessionID,
		State:     listenLivePlaybackState(state),
		Position:  currentTime,
		Duration:  duration,
		Volume:    volume,
		Muted:     muted,
		Error:     errorMessage,
		HasTiming: true,
		HasVolume: true,
	}
	playbackListeners := player.playbackListenersLocked()
	playbackProvider := player.playbackProvider
	playbackSessionID := player.playbackSessionID
	player.mu.Unlock()
	player.dispatch(map[string]any{
		"source":           listenLivePlayerSource,
		"type":             "state",
		"state":            state,
		"reason":           reason,
		"videoId":          videoID,
		"observedVideoId":  videoID,
		"requestedVideoId": videoID,
		"title":            title,
		"artist":           artist,
		"advertising":      advertising,
		"adLabel":          adLabel,
		"errorCode":        errorCode,
		"errorMessage":     errorMessage,
		"currentTime":      currentTime,
		"duration":         duration,
		"volume":           volume,
		"muted":            muted,
		"provider":         playbackProvider,
		"sessionId":        playbackSessionID,
	})
	player.notifyPlaybackListeners(playbackEvent, playbackListeners)
}

func (player *ListenYouTubeLivePlayer) playbackListenersLocked() []listenplayback.PlaybackBackendEventListener {
	listeners := make([]listenplayback.PlaybackBackendEventListener, 0, len(player.playbackListeners))
	for _, listener := range player.playbackListeners {
		listeners = append(listeners, listener)
	}
	return listeners
}

func (player *ListenYouTubeLivePlayer) notifyPlaybackListeners(
	event listenplayback.PlaybackBackendEvent,
	listeners []listenplayback.PlaybackBackendEventListener,
) {
	if event.Provider == "" || event.SessionID == "" {
		return
	}
	if event.State == "" && !event.HasTiming && event.Error == "" {
		return
	}
	for _, listener := range listeners {
		listener(event)
	}
}

func listenLivePlaybackState(value string) listenplayback.PlaybackState {
	switch listenplayback.PlaybackState(strings.TrimSpace(value)) {
	case listenplayback.PlaybackStateIdle:
		return listenplayback.PlaybackStateIdle
	case listenplayback.PlaybackStateLoading:
		return listenplayback.PlaybackStateLoading
	case listenplayback.PlaybackStatePlaying:
		return listenplayback.PlaybackStatePlaying
	case listenplayback.PlaybackStatePaused:
		return listenplayback.PlaybackStatePaused
	case listenplayback.PlaybackStateBuffering:
		return listenplayback.PlaybackStateBuffering
	case listenplayback.PlaybackStateEnded:
		return listenplayback.PlaybackStateEnded
	case listenplayback.PlaybackStateError:
		return listenplayback.PlaybackStateError
	default:
		return ""
	}
}

func listenLiveControlsFromPayload(value any) (ListenLivePlayerControls, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return ListenLivePlayerControls{}, false
	}
	return ListenLivePlayerControls{
		Like:         listenPayloadBool(object, "like"),
		Dislike:      listenPayloadBool(object, "dislike"),
		Captions:     listenPayloadBool(object, "captions"),
		AudioTrack:   listenPayloadBool(object, "audioTrack"),
		Quality:      listenPayloadBool(object, "quality"),
		Volume:       listenPayloadBool(object, "volume"),
		PlaybackRate: listenPayloadBool(object, "playbackRate"),
	}, true
}

func listenLiveOptionsFromPayload(value any) ([]ListenLivePlayerOption, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]ListenLivePlayerOption, 0, min(len(items), 64))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(listenPayloadString(object, "id"))
		label := strings.TrimSpace(listenPayloadString(object, "label"))
		if id == "" || label == "" || len(id) > 128 || len(label) > 256 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, ListenLivePlayerOption{ID: id, Label: label})
		if len(result) == 64 {
			break
		}
	}
	return result, true
}

func listenLiveSelectionsFromPayload(value any) (ListenLivePlayerSelections, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return ListenLivePlayerSelections{}, false
	}
	selection := ListenLivePlayerSelections{
		Rating:         strings.TrimSpace(listenPayloadString(object, "rating")),
		CaptionID:      strings.TrimSpace(listenPayloadString(object, "captionId")),
		AudioTrackID:   strings.TrimSpace(listenPayloadString(object, "audioTrackId")),
		QualityID:      strings.TrimSpace(listenPayloadString(object, "qualityId")),
		PlaybackRateID: strings.TrimSpace(listenPayloadString(object, "playbackRateId")),
	}
	if selection.Rating != "like" && selection.Rating != "dislike" {
		selection.Rating = "none"
	}
	return selection, true
}

func normalizeListenEmbeddedVideoRect(rect ListenEmbeddedVideoRect) ListenEmbeddedVideoRect {
	if rect.X < 0 {
		rect.X = 0
	}
	if rect.Y < 0 {
		rect.Y = 0
	}
	if rect.Width < 1 {
		rect.Width = 1
	}
	if rect.Height < 1 {
		rect.Height = 1
	}
	if rect.ViewportWidth < 1 || rect.ViewportHeight < 1 {
		rect.CenterX = 0
		rect.CenterY = 0
		rect.ViewportWidth = 0
		rect.ViewportHeight = 0
	}
	if rect.Radius < 0 {
		rect.Radius = 0
	}
	maxRadius := min(rect.Width, rect.Height) / 2
	if rect.Radius > maxRadius {
		rect.Radius = maxRadius
	}
	return rect
}

func listenYouTubeLiveEmbedURL(videoID string, language string) string {
	values := url.Values{}
	values.Set("autoplay", "1")
	values.Set("controls", "1")
	values.Set("enablejsapi", "1")
	values.Set("fs", "1")
	if normalized := normalizeListenPlayerLanguage(language); normalized != "" {
		values.Set("hl", normalized)
	}
	values.Set("iv_load_policy", "3")
	values.Set("modestbranding", "1")
	values.Set("origin", listenYouTubeClientOrigin())
	values.Set("playsinline", "1")
	values.Set("rel", "0")
	return listenYouTubeOrigin + "/embed/" + url.PathEscape(strings.TrimSpace(videoID)) + "?" + values.Encode()
}

// Regular YouTube playback uses the full watch page so the browser remains
// responsible for signed media URLs, DRM, account entitlements, ads, captions,
// and quality selection. Hush/radio keeps the lightweight embed path because it
// shares this player but is a distinct stream provider.
func listenYouTubePlaybackURL(
	provider listenplayback.PlaybackProvider,
	videoID string,
	language string,
) string {
	if provider == listenplayback.PlaybackProviderYouTube {
		return listenYouTubeWatchURL(videoID, language)
	}
	return listenYouTubeLiveEmbedURL(videoID, language)
}

func listenYouTubeLiveCanReuseDocument(
	currentVideo string,
	loadedProvider listenplayback.PlaybackProvider,
	loadedLanguage string,
	provider listenplayback.PlaybackProvider,
	request ListenPlayerPlayRequest,
) bool {
	request = normalizeListenPlayerPlayRequest(request)
	return !request.ForceReload &&
		strings.TrimSpace(currentVideo) == request.VideoID &&
		loadedProvider == provider &&
		normalizeListenPlayerLanguage(loadedLanguage) == request.Language
}

func listenYouTubeWatchURL(videoID string, language string) string {
	values := url.Values{}
	normalizedVideoID := strings.TrimSpace(videoID)
	values.Set("v", normalizedVideoID)
	values.Set("autoplay", "1")
	if normalized := normalizeListenPlayerLanguage(language); normalized != "" {
		values.Set("hl", normalized)
		values.Set("persist_hl", "1")
	}
	marker := url.Values{}
	marker.Set("xiadown-request", normalizedVideoID)
	return listenYouTubeOrigin + "/watch?" + values.Encode() + "#" + marker.Encode()
}

func listenYouTubeClientOrigin() string {
	return "https://" + listenYouTubeClientID
}

// Keep the document-start bridge and the later geometry refresh on the same
// video-only contract. The watch page is a full YouTube document, so every
// ancestor that can establish a clipping or transformed containing block must
// be neutralised before the React host is allowed to reveal its native hole.
const listenYouTubeLiveVideoModeCSS = `
html, body { width: 100% !important; height: 100% !important; margin: 0 !important; padding: 0 !important; overflow: hidden !important; background: #000 !important; }
html body * { visibility: hidden !important; }
.listen-live-video-visible, .listen-live-video-root { visibility: visible !important; }
.listen-live-video-visible:not(.listen-live-video-root) { opacity: 1 !important; overflow: visible !important; transform: none !important; translate: none !important; scale: none !important; rotate: none !important; filter: none !important; perspective: none !important; clip: auto !important; clip-path: none !important; contain: none !important; }
#player, #movie_player, .html5-video-player { background: #000 !important; }
.listen-live-video-root { position: fixed !important; inset: 0 !important; width: 100vw !important; height: 100vh !important; min-width: 100vw !important; min-height: 100vh !important; max-width: none !important; max-height: none !important; margin: 0 !important; padding: 0 !important; border: 0 !important; overflow: hidden !important; transform: none !important; translate: none !important; scale: none !important; rotate: none !important; filter: none !important; clip: auto !important; clip-path: none !important; contain: none !important; z-index: 2147483647 !important; }
.listen-live-video-surface:not(.listen-live-video-root) { position: absolute !important; inset: 0 !important; width: 100% !important; height: 100% !important; min-width: 0 !important; min-height: 0 !important; max-width: none !important; max-height: none !important; margin: 0 !important; padding: 0 !important; border: 0 !important; overflow: visible !important; transform: none !important; translate: none !important; scale: none !important; rotate: none !important; clip: auto !important; clip-path: none !important; opacity: 1 !important; }
video.listen-live-video-surface, .video-stream.listen-live-video-surface { object-fit: contain !important; object-position: center center !important; background: #000 !important; }
.listen-live-video-root .ytp-caption-window-container, .listen-live-video-root .ytp-caption-window-container *, .listen-live-video-root .caption-window, .listen-live-video-root .caption-window * { visibility: visible !important; pointer-events: none !important; }
.listen-live-video-root .ytp-gradient-top, .listen-live-video-root .ytp-gradient-bottom, .listen-live-video-root .ytp-chrome-top, .listen-live-video-root .ytp-chrome-bottom, .listen-live-video-root .ytp-pause-overlay, .listen-live-video-root .ytp-cards-teaser, .listen-live-video-root .ytp-ce-element, .listen-live-video-root .ytp-watermark { opacity: 0 !important; pointer-events: none !important; }
html.listen-live-native-window-fullscreen .listen-live-video-root .ytp-gradient-bottom, html.listen-live-native-window-fullscreen .listen-live-video-root .ytp-gradient-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: none !important; }
html.listen-live-native-window-fullscreen .listen-live-video-root .ytp-chrome-bottom, html.listen-live-native-window-fullscreen .listen-live-video-root .ytp-chrome-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; }
html.listen-live-native-window-fullscreen .listen-live-video-root .ytp-fullscreen-button { display: none !important; }
html:fullscreen .listen-live-video-root .ytp-gradient-bottom, html:fullscreen .listen-live-video-root .ytp-gradient-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: none !important; }
html:fullscreen .listen-live-video-root .ytp-chrome-bottom, html:fullscreen .listen-live-video-root .ytp-chrome-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; }
html:fullscreen .listen-live-video-root .ytp-fullscreen-button { display: none !important; }
html:-webkit-full-screen .listen-live-video-root .ytp-gradient-bottom, html:-webkit-full-screen .listen-live-video-root .ytp-gradient-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: none !important; }
html:-webkit-full-screen .listen-live-video-root .ytp-chrome-bottom, html:-webkit-full-screen .listen-live-video-root .ytp-chrome-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; }
html:-webkit-full-screen .listen-live-video-root .ytp-fullscreen-button { display: none !important; }
.listen-live-video-root:fullscreen .ytp-gradient-bottom, .listen-live-video-root:fullscreen .ytp-gradient-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: none !important; }
.listen-live-video-root:fullscreen .ytp-chrome-bottom, .listen-live-video-root:fullscreen .ytp-chrome-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; }
.listen-live-video-root:-webkit-full-screen .ytp-gradient-bottom, .listen-live-video-root:-webkit-full-screen .ytp-gradient-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: none !important; }
.listen-live-video-root:-webkit-full-screen .ytp-chrome-bottom, .listen-live-video-root:-webkit-full-screen .ytp-chrome-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; }
`

func listenYouTubeLiveBridgeScript(request ListenPlayerPlayRequest) string {
	initial, _ := json.Marshal(normalizeListenPlayerPlayRequest(request))
	videoModeCSS, _ := json.Marshal(listenYouTubeLiveVideoModeCSS)
	return fmt.Sprintf(`
(function() {
  "use strict";
  if (window.top !== window || window.__listenLiveBridgeInstalled) return;
  window.__listenLiveBridgeInstalled = true;

  const SOURCE = %q;
  const INITIAL_REQUEST = %s;
  const VIDEO_MODE_CSS = %s;
  const REQUEST_STORAGE_KEY = "__listenLivePlaybackRequest";
  const DOCUMENT_VOLUME_BOOT_KEY = "__xiadownListenLiveVolumeBooted";
	try {
	  if (window.sessionStorage.getItem(DOCUMENT_VOLUME_BOOT_KEY) !== "true") {
		const stored = JSON.parse(window.localStorage.getItem(REQUEST_STORAGE_KEY) || "null");
		window.localStorage.setItem(
		  REQUEST_STORAGE_KEY,
		  JSON.stringify(Object.assign({}, stored && typeof stored === "object" ? stored : {}, INITIAL_REQUEST))
		);
		window.sessionStorage.setItem(DOCUMENT_VOLUME_BOOT_KEY, "true");
	  }
	} catch (error) {}
  const UPDATE_THROTTLE_MS = 500;
  const POLL_INTERVAL_MS = 1000;
  const AUTOPLAY_ATTEMPTS = 48;
  const AUTOPLAY_INTERVAL_MS = 500;
  let lastUpdateAt = 0;
  let pollTimer = null;
  let autoplayTimer = null;
  let autoplayCount = 0;
  let mediaSessionOverrideFrame = null;
  let listenersAttachedTo = new WeakSet();
  let lastRequestedAction = "";
	let volumeEnforcing = false;
	let volumeApplyFrame = null;
	let lastAdvertising = false;
	let lastStrongAdAt = 0;
	let autonavBlockInProgress = false;
	let lastAutonavToggleClickAt = 0;
	let failedControls = new Set();
	let captionTrackCache = [];
	let captionTrackCacheVideoId = "";
	let pendingRequestedVideoId = "";
	let videoModeDocumentRootObserver = null;
	const VIDEO_MODE_STYLE_ID = "listen-live-video-mode-style";

	function liveVideoModeRequested() {
		let active = window.__listenLiveVideoModeActive === true;
		try {
			active = active || window.localStorage.getItem("__listenLiveVideoModeActive") === "true";
		} catch (error) {}
		return active;
	}

	function installLiveVideoModeStyle() {
		if (!liveVideoModeRequested()) return false;
		window.__listenLiveVideoModeActive = true;
		if (document.getElementById(VIDEO_MODE_STYLE_ID)) return true;
		const root = document.head || document.documentElement;
		if (!root) return false;
		const style = document.createElement("style");
		style.id = VIDEO_MODE_STYLE_ID;
		style.textContent = VIDEO_MODE_CSS;
		root.appendChild(style);
		return true;
	}

	function installLiveVideoModeAtDocumentStart() {
		if (!liveVideoModeRequested()) return;
		if (installLiveVideoModeStyle() || videoModeDocumentRootObserver || !window.MutationObserver) return;
		videoModeDocumentRootObserver = new MutationObserver(() => {
			if (!liveVideoModeRequested()) {
				videoModeDocumentRootObserver.disconnect();
				videoModeDocumentRootObserver = null;
				return;
			}
			if (!installLiveVideoModeStyle()) return;
			videoModeDocumentRootObserver.disconnect();
			videoModeDocumentRootObserver = null;
			markLiveVideoModeTree();
		});
		// The WebView bridge executes before <html>/<head> exists. Observing the
		// Document prevents a full /watch page from flashing before DOMContentLoaded.
		videoModeDocumentRootObserver.observe(document, { childList: true, subtree: true });
	}

	function liveVideoModeRoot(video) {
		const closestPlayer = video?.closest("#movie_player, .html5-video-player");
		if (closestPlayer) return closestPlayer;
		const api = document.getElementById("movie_player");
		if (api && (!video || api.contains(video))) return api;
		return video?.parentElement || null;
	}

	function markLiveVideoModeTree() {
		if (!liveVideoModeRequested()) return false;
		const video = videoElement();
		const root = liveVideoModeRoot(video);
		if (!video || !root) return false;
		document.querySelectorAll(".listen-live-video-visible, .listen-live-video-root, .listen-live-video-surface").forEach((element) => {
			element.classList.remove("listen-live-video-visible", "listen-live-video-root", "listen-live-video-surface");
		});
		let current = video;
		let reachedRoot = false;
		while (current && current !== document.documentElement) {
			current.classList.add("listen-live-video-visible");
			if (!reachedRoot) current.classList.add("listen-live-video-surface");
			if (current === root) reachedRoot = true;
			current = current.parentElement;
		}
		if (!reachedRoot || current !== document.documentElement) {
			document.querySelectorAll(".listen-live-video-visible, .listen-live-video-surface").forEach((element) => {
				element.classList.remove("listen-live-video-visible", "listen-live-video-surface");
			});
			return false;
		}
		root.classList.add("listen-live-video-root");
		return true;
	}

	function restoreLiveVideoMode() {
		if (!liveVideoModeRequested()) return false;
		installLiveVideoModeAtDocumentStart();
		if (!installLiveVideoModeStyle()) return false;
		return markLiveVideoModeTree();
	}
	// The fragment is written by the native load request. When it matches the
	// current watch URL it distinguishes an intentional A -> B load from
	// YouTube autonav even if localStorage still contains the valid old A.
	const navigationRequestedVideoId = (() => {
		try {
			const hash = String(window.location.hash || "").replace(/^#/, "");
			const candidate = new URLSearchParams(hash).get("xiadown-request") || "";
			return /^[A-Za-z0-9_-]{11}$/.test(candidate) ? candidate : "";
		} catch (error) {
			return "";
		}
	})();

  function post(payload) {
    const message = JSON.stringify(Object.assign({ source: SOURCE }, payload));
    try {
      if (window._wails && typeof window._wails.invoke === "function") {
        window._wails.invoke(message);
        return;
      }
      if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.external) {
        window.webkit.messageHandlers.external.postMessage(message);
        return;
      }
      if (window.chrome && window.chrome.webview && typeof window.chrome.webview.postMessage === "function") {
        window.chrome.webview.postMessage(message);
        return;
      }
      if (window.wails && typeof window.wails.invoke === "function") {
        window.wails.invoke(message);
      }
    } catch (error) {}
  }

  function readRequest() {
    let stored = null;
    try {
      stored = JSON.parse(window.localStorage.getItem("__listenLivePlaybackRequest") || "null");
    } catch (error) {}
    const urlVideoId = videoIdFromURL();
    const initialVideoId = String(INITIAL_REQUEST.videoId || "");
    const storedVideoId = String((stored && stored.videoId) || "");
	function requestForNavigationVideo(videoId) {
		const request = Object.assign({}, INITIAL_REQUEST, stored || {}, { videoId });
		if (storedVideoId !== videoId) {
			// Do not persist A's video-scoped metadata as B merely because
			// storage lost a race with the native navigation.
			request.title = "";
			request.artist = "";
			request.startSeconds = 0;
			request.forceReload = false;
		}
		return request;
	}
	// api.request(B) runs while the old A watch URL is still visible. Preserve
	// that explicit request instead of letting INITIAL_REQUEST(A) overwrite the
	// just-written storage value during the synchronous state callback.
	if (pendingRequestedVideoId && storedVideoId === pendingRequestedVideoId) {
		return Object.assign({}, INITIAL_REQUEST, stored, { videoId: pendingRequestedVideoId });
	}
	if (urlVideoId && navigationRequestedVideoId === urlVideoId) {
		return requestForNavigationVideo(urlVideoId);
	}
    if (urlVideoId && urlVideoId === initialVideoId) {
      if (!stored || (storedVideoId && storedVideoId !== initialVideoId)) {
        return Object.assign({}, INITIAL_REQUEST);
      }
      return Object.assign({}, INITIAL_REQUEST, stored, { videoId: initialVideoId });
    }
	// A navigation creates a fresh document whose baked INITIAL_REQUEST still
	// describes the first video loaded into this singleton window. If WebKit's
	// localStorage is temporarily unavailable, the current /watch?v= URL is the
	// only valid identity for the new document. Falling back to the stale initial
	// ID misclassifies the intentional switch as autonav and pauses at 0:00.
	if (urlVideoId && !validYouTubeVideoId(storedVideoId)) {
		return requestForNavigationVideo(urlVideoId);
	}
    return Object.assign({}, INITIAL_REQUEST, stored || {});
  }

  function writeRequest(next) {
    const request = Object.assign({}, readRequest(), next || {});
    try {
      window.localStorage.setItem("__listenLivePlaybackRequest", JSON.stringify(request));
    } catch (error) {}
    return request;
  }

  function finiteNumber(value, fallback) {
    return Number.isFinite(value) ? Math.max(0, value) : fallback;
  }

  function videoIdFromURL() {
    try {
      const fromQuery = new URL(window.location.href).searchParams.get("v") || "";
      if (fromQuery) return fromQuery;
    } catch (error) {}
    const match = window.location.pathname.match(/\/embed\/([A-Za-z0-9_-]{11})/);
    return match ? match[1] : "";
  }

  function currentRequestVideoId() {
    const request = readRequest();
    return String(request.videoId || INITIAL_REQUEST.videoId || "");
  }

  function playerApi() {
    return document.getElementById("movie_player") || null;
  }

  function isRegularYouTubeWatchPage() {
    try {
      const locationURL = new URL(window.location.href);
      return locationURL.hostname === "www.youtube.com" && locationURL.pathname === "/watch";
    } catch (error) {
      return false;
    }
  }

  function playerVideoData() {
    const api = playerApi();
    if (!api || typeof api.getVideoData !== "function") return null;
    try {
      const data = api.getVideoData();
      return data && typeof data === "object" ? data : null;
    } catch (error) {
      return null;
    }
  }

  function validYouTubeVideoId(value) {
    const candidate = String(value || "").trim();
    return /^[A-Za-z0-9_-]{11}$/.test(candidate) ? candidate : "";
  }

  function disableWatchAutonav() {
    if (!isRegularYouTubeWatchPage()) return;
    const api = playerApi();
    if (api && typeof api.setOption === "function") {
      try { api.setOption("autonav", "autoplay", false); } catch (error) {}
      try { api.setOption("autonav", "autonav_disabled", true); } catch (error) {}
    }
    const toggles = Array.from(document.querySelectorAll(".ytp-autonav-toggle-button"));
    const now = Date.now();
    toggles.some((toggle) => {
      const checked = toggle.getAttribute("aria-checked") === "true";
      const pressed = toggle.getAttribute("aria-pressed") === "true";
      if ((checked || pressed) && now - lastAutonavToggleClickAt >= 1500) {
        lastAutonavToggleClickAt = now;
        try { toggle.click(); } catch (error) {}
        return true;
      }
      return false;
    });
  }

  function isElementVisible(element) {
    if (!element || !element.isConnected) return false;
    const rect = element.getBoundingClientRect();
    if (!rect || rect.width <= 1 || rect.height <= 1) return false;
    const style = window.getComputedStyle(element);
    if (!style || style.display === "none" || style.visibility === "hidden") return false;
    const opacity = Number(style.opacity);
    return !Number.isFinite(opacity) || opacity > 0.01;
  }

  function hasActiveAdPlayerClass() {
    const api = playerApi();
    return Boolean(
      api &&
      api.classList &&
      (api.classList.contains("ad-showing") ||
        api.classList.contains("ad-interrupting"))
    );
  }

  function adControlSelectors() {
    return [
      ".ytp-ad-skip-button-modern",
      ".ytp-ad-skip-button",
      ".ytp-ad-skip-button-container button",
      ".ytp-ad-skip-button-container",
      ".ytp-skip-ad-button",
      ".ytp-ad-skip-button-slot button",
      ".ytp-ad-skip-button-slot",
      "button[aria-label*='skip' i]",
      "button[aria-label*='跳过']",
      "button[aria-label*='略過']",
      "button[aria-label*='スキップ']",
      "button[aria-label*='건너뛰기']"
    ].join(",");
  }

  function visibleAdElements() {
    const adSelector = [
      ".ytp-ad-preview-container",
      ".ytp-ad-text",
      ".ytp-ad-preview-text",
      ".ytp-ad-simple-ad-badge",
      ".ytp-ad-duration-remaining",
      adControlSelectors()
    ].join(",");
    return Array.from(document.querySelectorAll(adSelector)).filter(isElementVisible);
  }

  function isAdControlElement(element) {
    const selector = adControlSelectors();
    return Boolean(
      element &&
      (element.matches?.(selector) || element.closest?.(selector))
    );
  }

  function isControlDisabled(element) {
    if (!element) return true;
    return Boolean(
      element.disabled ||
      element.hasAttribute?.("disabled") ||
      element.getAttribute?.("aria-disabled") === "true" ||
      element.classList?.contains("disabled")
    );
  }

  function normalizeAdLabel(value) {
    return String(value || "").replace(/\s+/g, " ").trim().slice(0, 48);
  }

  function adElementText(element) {
    return normalizeAdLabel(element ? (element.textContent || element.innerText || "") : "");
  }

  function isMeaningfulAdElement(element) {
    if (!element) return false;
    if (isAdControlElement(element)) return true;
    if (element.matches?.(".ytp-ad-duration-remaining")) {
      return /\d/.test(adElementText(element));
    }
    const text = adElementText(element);
    if (!text || /skip|跳过|略過|スキップ|건너뛰기/i.test(text)) {
      return false;
    }
    return true;
  }

  function adLabelFromElements(elements) {
    const labelSelectors = [
      ".ytp-ad-text",
      ".ytp-ad-preview-text",
      ".ytp-ad-preview-container",
      ".ytp-ad-simple-ad-badge"
    ];
    for (const selector of labelSelectors) {
      const element = Array.from(document.querySelectorAll(selector)).find(isElementVisible);
      const text = adElementText(element);
      if (text && !/skip|跳过|略過|スキップ|건너뛰기/i.test(text)) return text;
    }
    for (const element of elements) {
      if (isAdControlElement(element)) continue;
      const text = adElementText(element);
      if (text && !/skip|跳过|略過|スキップ|건너뛰기/i.test(text)) return text;
    }
    return "";
  }

  function adSnapshot() {
    const now = Date.now();
    const hasClass = hasActiveAdPlayerClass();
    const elements = visibleAdElements().filter(isMeaningfulAdElement);
    const hasStrongSignal = elements.length > 0;

    if (hasStrongSignal) {
      lastStrongAdAt = now;
    }

    const advertising = hasStrongSignal || (hasClass && lastStrongAdAt > 0 && now - lastStrongAdAt < 1500);
    const label = advertising ? adLabelFromElements(elements) : "";
    lastAdvertising = advertising;
    return { advertising, label };
  }

  function normalizeErrorLine(value) {
    return String(value || "").replace(/[ \t\f\v]+/g, " ").trim();
  }

  function normalizedErrorElementLines(element) {
    return String(element?.innerText || element?.textContent || "")
      .split(/\r?\n+/)
      .map(normalizeErrorLine)
      .filter(Boolean);
  }

  function visibleErrorElements() {
    const primarySelector = [
      ".ytp-error-content-wrap-reason",
      ".ytp-error-content-wrap-subreason"
    ].join(",");
    const primaryElements = Array.from(document.querySelectorAll(primarySelector)).filter(isElementVisible);
    if (primaryElements.length > 0) {
      return primaryElements;
    }
    const fallbackSelector = [
      ".ytp-error-content-wrap",
      ".ytp-error-content",
      ".ytp-error"
    ].join(",");
    return Array.from(document.querySelectorAll(fallbackSelector)).filter(isElementVisible);
  }

  function uniqueErrorMessages(elements) {
    const messages = [];
    elements.forEach((element) => {
      normalizedErrorElementLines(element).forEach((line) => {
        if (messages.some((message) => message === line || message.includes(line))) {
          return;
        }
        const parentIndex = messages.findIndex((message) => line.includes(message));
        if (parentIndex >= 0) {
          messages[parentIndex] = line;
          return;
        }
        messages.push(line);
      });
    });
    return messages;
  }

  function normalizeErrorMessage(value) {
    return String(value || "")
      .split(/\r?\n+/)
      .map(normalizeErrorLine)
      .filter(Boolean)
      .join("\n")
      .slice(0, 180);
  }

  function errorCodeFromText(text) {
    const normalized = String(text || "");
    const labelled = normalized.match(/(?:error\s*code|code)\s*[:#]?\s*([0-9]{2,3}(?:-[0-9]+)?)/i);
    if (labelled && labelled[1]) return labelled[1];
    const fallback = normalized.match(/\b([0-9]{2,3}-[0-9]+|15[023])\b/);
    return fallback && fallback[1] ? fallback[1] : "";
  }

  function errorSnapshot(video) {
    const elements = visibleErrorElements();
    const text = normalizeErrorMessage(uniqueErrorMessages(elements).join("\n"));
    const videoError = video && video.error ? video.error : null;
    if (elements.length === 0 && !videoError) {
      return { errored: false, code: "", message: "" };
    }
    const code = errorCodeFromText(text) || String(videoError?.code || "");
    const message = text || normalizeErrorMessage(videoError?.message || "");
    return { errored: true, code, message };
  }

  function wasAdvertisingRecently() {
    return lastAdvertising || (lastStrongAdAt > 0 && Date.now() - lastStrongAdAt < 2500);
  }

  function playerStateCode() {
    const api = playerApi();
    if (api && typeof api.getPlayerState === "function") {
      try {
        const state = Number(api.getPlayerState());
        if (Number.isFinite(state)) return state;
      } catch (error) {}
    }
    return null;
  }

  function videoElements() {
    return Array.from(document.querySelectorAll("video"));
  }

  function videoElement() {
    const videos = videoElements();
    return videos.find((video) => !video.paused && !video.ended) ||
      videos.find((video) => video.readyState > 0) ||
      videos[0] ||
      null;
  }

  function bufferedEnd(video) {
    if (!video || !video.buffered || video.buffered.length === 0) return 0;
    try {
      return finiteNumber(video.buffered.end(video.buffered.length - 1), 0);
    } catch (error) {
      return 0;
    }
  }

  function stateFromVideo(video, reason) {
    if (lastRequestedAction === "pause") return "paused";
    const apiState = playerStateCode();
    if (apiState === 1) return "playing";
    if (apiState === 3) return "buffering";
    if (!video) return "loading";
    if (video.error) return "error";
    if (lastRequestedAction === "play") return video.readyState < 2 ? "loading" : "buffering";
    if (apiState === 2 || apiState === 5) return "paused";
    if (apiState === 0 || reason === "ended") return "ended";
    if (video.ended) return "ended";
    if (video.seeking || reason === "waiting" || reason === "stalled" || reason === "seeking") return "buffering";
    if (!video.paused) return video.readyState < 2 ? "buffering" : "playing";
    return "paused";
  }

  function currentVolumeState() {
    const request = readRequest();
    const muted = Boolean(request.muted);
    const volume = Math.max(0, Math.min(1, Number(request.volume ?? 1)));
    return { muted, volume, ytVolume: Math.round(volume * 100) };
  }

  function applyVolumeToMediaElement(video, state) {
    if (!video) return;
    const next = state || currentVolumeState();
    try {
      if (Math.abs(Number(video.volume || 0) - next.volume) > 0.01) {
        video.volume = next.volume;
      }
      if (video.muted !== next.muted) {
        video.muted = next.muted;
      }
    } catch (error) {}
  }

  function scheduleVolumeApply() {
    if (volumeApplyFrame !== null) return;
    const run = () => {
      volumeApplyFrame = null;
      applyVolume();
    };
    if (typeof window.requestAnimationFrame === "function") {
      volumeApplyFrame = window.requestAnimationFrame(run);
    } else {
      volumeApplyFrame = window.setTimeout(run, 0);
    }
  }

  function patchMediaElementVolumeProperty(propertyName) {
    try {
      const proto = window.HTMLMediaElement && window.HTMLMediaElement.prototype;
      if (!proto) return;
      const marker = "__listenLiveNativePatched" + propertyName;
      if (proto[marker]) return;
      const descriptor = Object.getOwnPropertyDescriptor(proto, propertyName);
      if (!descriptor || !descriptor.configurable || typeof descriptor.get !== "function" || typeof descriptor.set !== "function") return;
      Object.defineProperty(proto, marker, { value: true });
      Object.defineProperty(proto, propertyName, {
        configurable: true,
        enumerable: descriptor.enumerable,
        get: function() {
          return descriptor.get.call(this);
        },
        set: function(value) {
          const next = currentVolumeState();
          const enforced = propertyName === "volume" ? next.volume : next.muted;
          return descriptor.set.call(this, volumeEnforcing ? value : enforced);
        }
      });
    } catch (error) {}
  }

  function patchMediaElementPlay() {
    try {
      const proto = window.HTMLMediaElement && window.HTMLMediaElement.prototype;
      if (!proto || typeof proto.play !== "function" || proto.play.__listenLiveNativeVolumePatched) return;
      const nativePlay = proto.play;
      const patchedPlay = function(...args) {
        try { applyVolumeToMediaElement(this); } catch (error) {}
        return nativePlay.apply(this, args);
      };
      patchedPlay.__listenLiveNativeVolumePatched = true;
      Object.defineProperty(proto, "play", {
        configurable: true,
        writable: true,
        value: patchedPlay
      });
    } catch (error) {}
  }

  function installVolumeGuards() {
    patchMediaElementVolumeProperty("volume");
    patchMediaElementVolumeProperty("muted");
    patchMediaElementPlay();
    ["DOMContentLoaded", "readystatechange", "load"].forEach((name) => {
      try { document.addEventListener(name, scheduleVolumeApply, true); } catch (error) {}
    });
    try {
      const root = document.documentElement || document.body;
      if (root && window.MutationObserver) {
        const observer = new MutationObserver(scheduleVolumeApply);
        observer.observe(root, { childList: true, subtree: true });
      }
    } catch (error) {}
  }

  function applyVolume() {
    if (volumeEnforcing) return;
    const volumeState = currentVolumeState();
    volumeEnforcing = true;
    try {
    const videos = videoElements();
    videos.forEach((video) => {
        applyVolumeToMediaElement(video, volumeState);
    });
      // The Windows mixer applies an independent process/session gain. Keep
      // XiaDown's gain in one WebView endpoint and do not mirror it into both
      // the media element and YouTube API, which creates a feedback loop.
      if (videos.length === 0) {
        const api = playerApi();
        if (api && typeof api.setVolume === "function") {
          try { api.setVolume(volumeState.ytVolume); } catch (error) {}
        }
        if (api && typeof api.mute === "function" && typeof api.unMute === "function") {
          try {
            if (volumeState.muted) api.mute();
            else api.unMute();
          } catch (error) {}
        }
      }
    } finally {
      volumeEnforcing = false;
    }
  }

  function optionText(value, fallback) {
    if (typeof value === "string") return value.trim() || fallback;
    if (!value || typeof value !== "object") return fallback;
    const candidates = [value.displayName, value.name, value.label, value.languageName, value.qualityLabel];
    for (const candidate of candidates) {
      if (typeof candidate === "string" && candidate.trim()) return candidate.trim();
      if (candidate && typeof candidate.simpleText === "string" && candidate.simpleText.trim()) {
        return candidate.simpleText.trim();
      }
      if (candidate && Array.isArray(candidate.runs)) {
        const text = candidate.runs.map((run) => String(run && run.text || "")).join("").trim();
        if (text) return text;
      }
    }
    return fallback;
  }

  function captionTrackId(track, index) {
    if (!track || typeof track !== "object") return "";
    const explicit = track.vssId || track.id || track.languageCode || track.lang;
    if (explicit) return String(explicit);
    return Number.isInteger(index) && index >= 0 ? "caption-" + index : "";
  }

  function audioTrackId(track, index) {
    if (!track || typeof track !== "object") return "";
    const base = String(track.id || track.audioTrackId || track.languageCode || track.lang || "audio");
    return base + "-" + String(index);
  }

  function captionSnapshot() {
    const api = playerApi();
    let rawOptions = [];
    let current = null;
	const data = playerVideoData();
	const videoID = validYouTubeVideoId(
	  data && (data.video_id || data.videoId)
	) || validYouTubeVideoId(videoIdFromURL()) ||
	  validYouTubeVideoId(currentRequestVideoId());
	if (videoID && captionTrackCacheVideoId !== videoID) {
	  captionTrackCacheVideoId = videoID;
	  captionTrackCache = [];
	  failedControls.delete("captions");
	}
    if (api && typeof api.getOption === "function") {
      try {
        const value = api.getOption("captions", "tracklist");
        if (Array.isArray(value)) rawOptions = value;
      } catch (error) {}
      try { current = api.getOption("captions", "track"); } catch (error) {}
    }
	// YouTube can report an empty tracklist and mark its native CC button
	// unavailable immediately after captions are switched Off. The tracks are
	// still selectable; keep the last non-empty list for this video so Off only
	// clears the selection instead of erasing the captions capability.
	if (rawOptions.length > 0) {
	  captionTrackCache = rawOptions.slice();
	} else if (captionTrackCache.length > 0) {
	  rawOptions = captionTrackCache.slice();
	}
    const options = rawOptions.map((track, index) => ({
      id: captionTrackId(track, index),
      label: optionText(track, String(track && track.languageCode || (index + 1)))
    })).filter((option) => option.id && option.label);
    const currentID = captionTrackId(current, rawOptions.indexOf(current));
    const button = document.querySelector(".ytp-subtitles-button");
    const selectable = Boolean(
      api && typeof api.getOption === "function" && typeof api.setOption === "function" && options.length > 0
    );
    const buttonPressed = Boolean(button && button.getAttribute("aria-pressed") === "true");
    return {
      available: selectable,
      selectable,
      // A selected track is not proof that YouTube is rendering it. When its
      // native CC button exists, aria-pressed is the actual on-screen state.
      enabled: button ? buttonPressed : Boolean(currentID),
      currentID,
      options,
      rawOptions,
      button
    };
  }

  function audioSnapshot() {
    const api = playerApi();
    let rawOptions = [];
    let current = null;
    if (api && typeof api.getAvailableAudioTracks === "function") {
      try {
        const value = api.getAvailableAudioTracks();
        if (Array.isArray(value)) rawOptions = value;
      } catch (error) {}
    }
    if (api && typeof api.getAudioTrack === "function") {
      try { current = api.getAudioTrack(); } catch (error) {}
    }
    const options = rawOptions.map((track, index) => ({
      id: audioTrackId(track, index),
      label: optionText(track, String(track && track.languageCode || (index + 1)))
    })).filter((option) => option.id && option.label);
    let currentID = "";
    rawOptions.some((track, index) => {
      if (track === current || (
        track && current &&
        String(track.id || track.audioTrackId || track.languageCode || "") ===
          String(current.id || current.audioTrackId || current.languageCode || "")
      )) {
        currentID = audioTrackId(track, index);
        return true;
      }
      return false;
    });
    return {
      available: Boolean(
        api && typeof api.getAvailableAudioTracks === "function" &&
        typeof api.getAudioTrack === "function" && typeof api.setAudioTrack === "function" && rawOptions.length > 1
      ),
      currentID,
      options,
      rawOptions
    };
  }

  function qualitySnapshot() {
    const api = playerApi();
    let rawOptions = [];
    let currentID = "";
    if (api && typeof api.getAvailableQualityLevels === "function") {
      try {
        const value = api.getAvailableQualityLevels();
        if (Array.isArray(value)) rawOptions = value.map((item) => String(item || "")).filter(Boolean);
      } catch (error) {}
    }
    if (api && typeof api.getPlaybackQuality === "function") {
      try { currentID = String(api.getPlaybackQuality() || ""); } catch (error) {}
    }
    return {
      available: Boolean(
        api && typeof api.getAvailableQualityLevels === "function" &&
        typeof api.getPlaybackQuality === "function" &&
        typeof api.setPlaybackQualityRange === "function" && rawOptions.length > 1
      ),
      currentID,
      options: rawOptions.map((id) => ({ id, label: id })),
      rawOptions
    };
  }

  function playbackRateID(value) {
    const rate = Number(value);
    if (!Number.isFinite(rate) || rate < 0.25 || rate > 2) return "";
    return String(Number(rate.toFixed(2)));
  }

  function playbackRateSnapshot(video) {
    const api = playerApi();
    const commonRates = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];
    const apiAvailable = Boolean(
      api && typeof api.getAvailablePlaybackRates === "function" &&
      typeof api.getPlaybackRate === "function" &&
      typeof api.setPlaybackRate === "function"
    );
    const videoAvailable = Boolean(video && typeof video.playbackRate === "number");
    let rawRates = [];
    if (apiAvailable) {
      try {
        const value = api.getAvailablePlaybackRates();
        if (Array.isArray(value)) rawRates = value;
      } catch (error) {}
    }
    if (rawRates.length === 0 && videoAvailable) rawRates = commonRates;
    const ids = Array.from(new Set(
      rawRates.map(playbackRateID).filter(Boolean)
    )).sort((left, right) => Number(left) - Number(right));
    let currentID = "1";
    if (apiAvailable) {
      try { currentID = playbackRateID(api.getPlaybackRate()) || "1"; } catch (error) {}
    } else if (videoAvailable) {
      currentID = playbackRateID(video.playbackRate) || "1";
    }
    return {
      available: Boolean((apiAvailable || videoAvailable) && ids.length > 1),
      currentID,
      options: ids.map((id) => ({ id, label: id + "x" })),
      rawIDs: ids,
      apiAvailable,
      videoAvailable
    };
  }

  function ratingButton(kind) {
    const root = playerApi() || document;
    const selector = kind === "like"
      ? ".ytp-like-button, button.ytp-like-button"
      : ".ytp-dislike-button, button.ytp-dislike-button";
    return Array.from(root.querySelectorAll ? root.querySelectorAll(selector) : [])
      .find((element) => isElementVisible(element) && !isControlDisabled(element)) || null;
  }

  function normalizedRating(value) {
    const rating = String(value || "").toLowerCase();
    if (rating.includes("dislike")) return "dislike";
    if (rating.includes("like")) return "like";
    return "none";
  }

  function ratingSnapshot() {
    const api = playerApi();
    const likeButton = ratingButton("like");
    const dislikeButton = ratingButton("dislike");
    const hasGetter = Boolean(api && typeof api.getLikeStatus === "function");
    let rating = "none";
    if (hasGetter) {
      try { rating = normalizedRating(api.getLikeStatus()); } catch (error) {}
    } else if (likeButton && likeButton.getAttribute("aria-pressed") === "true") {
      rating = "like";
    } else if (dislikeButton && dislikeButton.getAttribute("aria-pressed") === "true") {
      rating = "dislike";
    }
    return {
      like: Boolean(likeButton || (hasGetter && api && typeof api.likeVideo === "function")),
      dislike: Boolean(dislikeButton || (hasGetter && api && typeof api.dislikeVideo === "function")),
      rating,
      likeButton,
      dislikeButton
    };
  }

  function volumeSnapshot(video) {
    const api = playerApi();
    const request = readRequest();
    const bridgeAvailable = Boolean(validYouTubeVideoId(request.videoId || currentRequestVideoId()));
    const domAvailable = Boolean(video && typeof video.volume === "number");
    const apiAvailable = Boolean(
      api && typeof api.getVolume === "function" && typeof api.setVolume === "function" &&
      typeof api.isMuted === "function" && typeof api.mute === "function" && typeof api.unMute === "function"
    );
    let volume = domAvailable ? Number(video.volume) : Number(request.volume ?? 1);
    let muted = domAvailable ? Boolean(video.muted) : Boolean(request.muted);
    if (!domAvailable && apiAvailable) {
      try { volume = Number(api.getVolume()) / 100; } catch (error) {}
      try { muted = Boolean(api.isMuted()); } catch (error) {}
    }
    return {
      // This injected bridge can persist and later apply volume even during
      // YouTube's transient player/video replacement states. Capability is
      // therefore tied to the requested video, not optional page API getters.
      available: bridgeAvailable,
      volume: Math.max(0, Math.min(1, Number.isFinite(volume) ? volume : 1)),
      muted
    };
  }

  function controlSnapshot(video) {
    const captions = captionSnapshot();
    const audio = audioSnapshot();
    const quality = qualitySnapshot();
    const playbackRate = playbackRateSnapshot(video);
    const rating = ratingSnapshot();
    const volume = volumeSnapshot(video);
    return {
      controls: {
		like: rating.like && !failedControls.has("like"),
		dislike: rating.dislike && !failedControls.has("dislike"),
		captions: captions.available && !failedControls.has("captions"),
		audioTrack: audio.available && !failedControls.has("audioTrack"),
		quality: quality.available && !failedControls.has("quality"),
		volume: volume.available,
		playbackRate: playbackRate.available && !failedControls.has("playbackRate")
      },
      captionOptions: captions.options,
      audioTrackOptions: audio.options,
      qualityOptions: quality.options,
      playbackRateOptions: playbackRate.options,
      selections: {
        rating: rating.rating,
        captionId: captions.enabled ? captions.currentID : "",
        audioTrackId: audio.currentID,
        qualityId: quality.currentID,
        playbackRateId: playbackRate.currentID
      },
      volume: volume.volume,
      muted: volume.muted
    };
  }

  function metadataSnapshot(advertising) {
    const request = readRequest();
    const data = playerVideoData();
    const requestedVideoId = validYouTubeVideoId(request.videoId || currentRequestVideoId());
    const urlVideoId = validYouTubeVideoId(videoIdFromURL());
    const observedVideoId = validYouTubeVideoId(
      data && (data.video_id || data.videoId)
    ) || urlVideoId || requestedVideoId;
    if (pendingRequestedVideoId && urlVideoId === pendingRequestedVideoId) {
      pendingRequestedVideoId = "";
    }
    const trackChanged = Boolean(
      requestedVideoId && observedVideoId && requestedVideoId !== observedVideoId
    );
    const observedTitle = String(data && data.title || "").trim();
    const observedArtist = String(
      data && (data.author || data.channelName || data.ownerName) || ""
    ).trim();
    const useRequestedMetadata = Boolean(advertising) || !trackChanged;
    const title = useRequestedMetadata
      ? String(request.title || observedTitle || requestedVideoId || observedVideoId)
      : String(observedTitle || observedVideoId);
    const artist = useRequestedMetadata
      ? String(request.artist || observedArtist || "YouTube Live")
      : observedArtist;
    const thumbnailVideoId = useRequestedMetadata
      ? (requestedVideoId || observedVideoId)
      : observedVideoId;
    return {
      videoId: requestedVideoId || observedVideoId,
      observedVideoId,
      requestedVideoId,
      title,
      artist,
      thumbnailUrl: thumbnailVideoId ? "https://i.ytimg.com/vi/" + encodeURIComponent(thumbnailVideoId) + "/hqdefault.jpg" : "",
      trackChanged,
      metadataSource: trackChanged && !advertising ? "player" : (data ? "request+player" : "request")
    };
  }

  function blockUnexpectedWatchPlayback(metadata, advertising) {
    if (
      !isRegularYouTubeWatchPage() ||
      advertising ||
      !metadata ||
      !metadata.trackChanged ||
      !metadata.observedVideoId ||
      !metadata.requestedVideoId
    ) {
      return false;
    }
    const urlVideoId = validYouTubeVideoId(videoIdFromURL());
    // During an intentional A -> B switch, the watch URL changes to B before
    // movie_player.getVideoData() stops reporting A. That transient mismatch
    // is the requested page loading, not YouTube autonav. Pausing it here
    // cancels the bridge's autoplay recovery and leaves B stuck at 0:00.
    if (
      (urlVideoId && urlVideoId === metadata.requestedVideoId) ||
      (pendingRequestedVideoId && pendingRequestedVideoId === metadata.requestedVideoId)
    ) {
      return false;
    }
    disableWatchAutonav();
    lastRequestedAction = "pause";
    cancelAutoplay();
    if (pollTimer) {
      window.clearInterval(pollTimer);
      pollTimer = null;
    }
    if (autonavBlockInProgress) return true;
    autonavBlockInProgress = true;
    const api = playerApi();
    if (api && typeof api.pauseVideo === "function") {
      try { api.pauseVideo(); } catch (error) {}
    }
    videoElements().forEach((video) => {
      try { if (!video.paused) video.pause(); } catch (error) {}
    });
    window.setTimeout(() => { autonavBlockInProgress = false; }, 0);
    return true;
  }

  function installMediaSessionHandlers() {
    try {
      if (!navigator.mediaSession || typeof navigator.mediaSession.setActionHandler !== "function") return;
      try {
        navigator.mediaSession.setActionHandler("nexttrack", () => post({ type: "remote-next" }));
      } catch (error) {}
      try {
        navigator.mediaSession.setActionHandler("previoustrack", () => post({ type: "remote-previous" }));
      } catch (error) {}
    } catch (error) {}
  }

  function scheduleMediaSessionOverrideLoop() {
    if (mediaSessionOverrideFrame !== null) return;
    mediaSessionOverrideFrame = window.requestAnimationFrame(() => {
      mediaSessionOverrideFrame = null;
      installMediaSessionHandlers();
      scheduleMediaSessionOverrideLoop();
    });
  }

  function sendState(reason, force) {
	restoreLiveVideoMode();
    const now = Date.now();
    if (!force && now - lastUpdateAt < UPDATE_THROTTLE_MS) return;
    lastUpdateAt = now;
    installMediaSessionHandlers();
    const video = videoElement();
    const error = errorSnapshot(video);
    const ad = adSnapshot();
    const metadata = metadataSnapshot(ad.advertising);
    disableWatchAutonav();
    const autonavBlocked = blockUnexpectedWatchPlayback(metadata, ad.advertising);
    const state = error.errored ? "error" : (autonavBlocked ? "paused" : stateFromVideo(video, reason));
    const duration = video ? finiteNumber(video.duration, 0) : 0;
    const currentTime = video ? finiteNumber(video.currentTime, 0) : 0;
	const control = controlSnapshot(video);
    const payload = {
      type: "state",
      state,
      reason: reason || "",
      videoId: metadata.videoId,
      observedVideoId: metadata.observedVideoId,
      requestedVideoId: metadata.requestedVideoId,
      title: metadata.title,
      artist: metadata.artist,
      thumbnailUrl: metadata.thumbnailUrl,
      trackChanged: metadata.trackChanged,
      autonavBlocked,
      metadataSource: metadata.metadataSource,
      currentTime,
      duration,
      bufferedTime: bufferedEnd(video),
	  volume: control.volume,
	  muted: control.muted,
	  controls: control.controls,
	  captionOptions: control.captionOptions,
	  audioTrackOptions: control.audioTrackOptions,
	  qualityOptions: control.qualityOptions,
	  playbackRateOptions: control.playbackRateOptions,
	  selections: control.selections,
      advertising: ad.advertising,
      adLabel: ad.label,
      errorCode: error.code,
      errorMessage: error.message,
      readyState: video ? video.readyState : 0,
      networkState: video ? video.networkState : 0,
      url: window.location.href
    };
    if (error.errored) {
      payload.code = error.code || (video && video.error ? video.error.code || 0 : 0);
      payload.message = error.message;
    }
    post(payload);
  }

  function sendTrackEnded(video, reason) {
    const ad = adSnapshot();
    const metadata = metadataSnapshot(ad.advertising);
    disableWatchAutonav();
    const error = errorSnapshot(video);
	const control = controlSnapshot(video);
    post({
      type: "track-ended",
      state: error.errored ? "error" : "ended",
      reason: reason || "ended",
      videoId: metadata.videoId,
      observedVideoId: metadata.observedVideoId,
      requestedVideoId: metadata.requestedVideoId,
      title: metadata.title,
      artist: metadata.artist,
      thumbnailUrl: metadata.thumbnailUrl,
      trackChanged: metadata.trackChanged,
      metadataSource: metadata.metadataSource,
      currentTime: video ? finiteNumber(video.currentTime, 0) : 0,
      duration: video ? finiteNumber(video.duration, 0) : 0,
      bufferedTime: bufferedEnd(video),
	  volume: control.volume,
	  muted: control.muted,
	  controls: control.controls,
	  captionOptions: control.captionOptions,
	  audioTrackOptions: control.audioTrackOptions,
	  qualityOptions: control.qualityOptions,
	  playbackRateOptions: control.playbackRateOptions,
	  selections: control.selections,
      advertising: ad.advertising,
      adLabel: ad.label,
      errorCode: error.code,
      errorMessage: error.message,
      code: error.errored ? error.code || (video && video.error ? video.error.code || 0 : 0) : 0,
      message: error.errored ? error.message : "",
      readyState: video ? video.readyState : 0,
      networkState: video ? video.networkState : 0,
      url: window.location.href
    });
  }

  function startPolling() {
    if (pollTimer) return;
    sendState("poll-start", true);
    pollTimer = window.setInterval(() => sendState("poll", false), POLL_INTERVAL_MS);
  }

  function stopPolling() {
    if (pollTimer) {
      window.clearInterval(pollTimer);
      pollTimer = null;
    }
    sendState("poll-stop", true);
  }

  function cancelAutoplay() {
    if (!autoplayTimer) return;
    window.clearInterval(autoplayTimer);
    autoplayTimer = null;
    autoplayCount = 0;
  }

  function invokePlay(reason) {
    lastRequestedAction = "play";
    sendState(reason || "play-requested", true);
    applyVolume();
    const api = playerApi();
    if (api && typeof api.playVideo === "function") {
      try { api.playVideo(); } catch (error) {}
    }
    const video = videoElement();
    if (video) {
      const result = video.play();
      if (result && typeof result.catch === "function") {
        result.catch(() => sendState("play-rejected", true));
      }
    }
    sendState(reason || "play", true);
  }

  function invokePause(reason) {
    lastRequestedAction = "pause";
    cancelAutoplay();
    stopPolling();
    sendState(reason || "pause-requested", true);
    const api = playerApi();
    if (api && typeof api.pauseVideo === "function") {
      try { api.pauseVideo(); } catch (error) {}
    }
    videoElements().forEach((video) => {
      try { if (!video.paused) video.pause(); } catch (error) {}
    });
    sendState(reason || "pause", true);
  }

  function attachVideoListeners() {
    const videos = videoElements();
    if (videos.length === 0) return;
    videos.forEach((video) => {
      if (listenersAttachedTo.has(video)) return;
      listenersAttachedTo.add(video);
      ["loadstart", "loadedmetadata", "loadeddata", "canplay", "canplaythrough", "durationchange", "progress"].forEach((name) => {
        video.addEventListener(name, () => {
          applyVolume();
          sendState(name, true);
        });
      });
      video.addEventListener("play", () => {
        if (lastRequestedAction === "pause") {
          try { video.pause(); } catch (error) {}
          sendState("play-blocked-after-pause", true);
          return;
        }
        applyVolume();
        startPolling();
        sendState("play", true);
      });
      video.addEventListener("playing", () => {
        if (lastRequestedAction === "pause") {
          try { video.pause(); } catch (error) {}
          stopPolling();
          sendState("playing-blocked-after-pause", true);
          return;
        }
        lastRequestedAction = "";
        applyVolume();
        cancelAutoplay();
        startPolling();
        sendState("playing", true);
      });
      video.addEventListener("pause", () => {
        if (video.ended) return;
        if (lastRequestedAction !== "pause") lastRequestedAction = "";
        stopPolling();
        sendState("pause", true);
      });
      video.addEventListener("waiting", () => sendState("waiting", true));
      video.addEventListener("stalled", () => sendState("stalled", true));
      video.addEventListener("seeking", () => sendState("seeking", true));
      video.addEventListener("volumechange", () => {
        if (!volumeEnforcing) scheduleVolumeApply();
      });
      video.addEventListener("seeked", () => sendState("seeked", true));
      video.addEventListener("ended", () => {
        lastRequestedAction = "";
        const ad = adSnapshot();
        if (ad.advertising || wasAdvertisingRecently()) {
          sendState("ad-ended", true);
          startPolling();
          return;
        }
        stopPolling();
        sendTrackEnded(video, "ended");
      });
      video.addEventListener("error", () => sendState("error", true));
    });
  }

  function scheduleAutoplay() {
    if (autoplayTimer) window.clearInterval(autoplayTimer);
    autoplayCount = 0;
    autoplayTimer = window.setInterval(() => {
      if (lastRequestedAction === "pause") {
        cancelAutoplay();
        sendState("autoplay-cancelled", true);
        return;
      }
      autoplayCount += 1;
      attachVideoListeners();
      if (playerStateCode() === 1) {
        cancelAutoplay();
        lastRequestedAction = "";
        startPolling();
        sendState("autoplay-confirmed", true);
        return;
      }
      invokePlay("autoplay");
      if (autoplayCount >= AUTOPLAY_ATTEMPTS) {
        cancelAutoplay();
        sendState("autoplay-timeout", true);
      }
    }, AUTOPLAY_INTERVAL_MS);
  }

  function controlCapabilityKey(command) {
    if (command === "toggle-captions" || command === "select-caption") return "captions";
    if (command === "select-audio-track") return "audioTrack";
    if (command === "select-quality") return "quality";
    if (command === "select-playback-rate") return "playbackRate";
    if (command === "set-volume") return "volume";
    return command;
  }

  function sendControlResult(command, value, success) {
    const key = controlCapabilityKey(command);
    if (success) failedControls.delete(key);
	// Caption and volume verification can race YouTube's transient media
	// replacement states. Keep those bridge-backed controls retryable; their
	// snapshots remain the source of truth for actual availability.
	else if (key === "volume") failedControls.delete(key);
	else if (key !== "captions") failedControls.add(key);
    post({
      type: "control-result",
      command,
      value: String(value || ""),
      success: Boolean(success)
    });
    sendState("control-" + command, true);
  }

  function verifyControlLater(command, value, verify, delay) {
    window.setTimeout(() => {
      let success = false;
      try { success = Boolean(verify()); } catch (error) {}
      sendControlResult(command, value, success);
    }, Math.max(0, Number(delay || 80)));
  }

  function invokeRatingControl(command) {
    const kind = command === "dislike" ? "dislike" : "like";
    const before = ratingSnapshot();
    const button = kind === "like" ? before.likeButton : before.dislikeButton;
    if (button) {
      try { button.click(); } catch (error) {
        sendControlResult(command, kind, false);
        return;
      }
      verifyControlLater(command, kind, () => ratingSnapshot().rating !== before.rating, 100);
      return;
    }
    const api = playerApi();
    const method = kind === "like" ? "likeVideo" : "dislikeVideo";
    if (!api || typeof api.getLikeStatus !== "function" || typeof api[method] !== "function") {
      sendControlResult(command, kind, false);
      return;
    }
    try { api[method](); } catch (error) {
      sendControlResult(command, kind, false);
      return;
    }
    verifyControlLater(command, kind, () => ratingSnapshot().rating === kind, 160);
  }

  function invokeCaptionControl(command, value) {
    const before = captionSnapshot();
    const api = playerApi();
    if (command === "toggle-captions" && before.button) {
      try { before.button.click(); } catch (error) {
        sendControlResult(command, value, false);
        return;
      }
      verifyControlLater(command, value, () => captionSnapshot().enabled !== before.enabled, 120);
      return;
    }
    if (!api || typeof api.getOption !== "function" || typeof api.setOption !== "function") {
      sendControlResult(command, value, false);
      return;
    }
    let target = null;
    if (command === "toggle-captions") {
      target = before.enabled ? {} : before.rawOptions[0];
    } else if (value) {
      target = before.rawOptions.find((track, index) => captionTrackId(track, index) === value) || null;
    } else {
      target = {};
    }
    if (!target) {
      sendControlResult(command, value, false);
      return;
    }
    const shouldEnable = command === "toggle-captions" ? !before.enabled : Boolean(value);
    const targetID = shouldEnable
      ? captionTrackId(target, before.rawOptions.indexOf(target))
      : "";
    try { api.setOption("captions", "track", target); } catch (error) {
      sendControlResult(command, value, false);
      return;
    }
    // setOption can switch YouTube's remembered track without turning the CC
    // renderer on. Reconcile the native button, then re-apply the requested
    // track because enabling CC may restore YouTube's previous/default track.
    window.setTimeout(() => {
      const selected = captionSnapshot();
      const buttonPressed = Boolean(
        selected.button && selected.button.getAttribute("aria-pressed") === "true"
      );
      if (selected.button && buttonPressed !== shouldEnable) {
        try { selected.button.click(); } catch (error) {
          sendControlResult(command, value, false);
          return;
        }
      }
      if (shouldEnable) {
        try { api.setOption("captions", "track", target); } catch (error) {
          sendControlResult(command, value, false);
          return;
        }
      }
      verifyControlLater(command, value, () => {
        const after = captionSnapshot();
        const stateMatches = shouldEnable ? after.enabled : !after.enabled;
        const trackMatches = !shouldEnable || !targetID || after.currentID === targetID;
        return stateMatches && trackMatches;
      }, 180);
    }, 80);
  }

  function invokeAudioControl(command, value) {
    const before = audioSnapshot();
    const api = playerApi();
    const target = before.rawOptions.find((track, index) => audioTrackId(track, index) === value);
    if (!target || !api || typeof api.setAudioTrack !== "function" || typeof api.getAudioTrack !== "function") {
      sendControlResult(command, value, false);
      return;
    }
    try { api.setAudioTrack(target); } catch (error) {
      sendControlResult(command, value, false);
      return;
    }
    verifyControlLater(command, value, () => audioSnapshot().currentID === value, 180);
  }

  function invokeQualityControl(command, value) {
    const before = qualitySnapshot();
    const api = playerApi();
    if (!before.rawOptions.includes(value) || !api ||
        typeof api.setPlaybackQualityRange !== "function" || typeof api.getPlaybackQuality !== "function") {
      sendControlResult(command, value, false);
      return;
    }
    try { api.setPlaybackQualityRange(value); } catch (error) {
      sendControlResult(command, value, false);
      return;
    }
    verifyControlLater(command, value, () => qualitySnapshot().currentID === value, 300);
  }

  function invokePlaybackRateControl(command, value) {
    const targetID = playbackRateID(value);
    const video = videoElement();
    const before = playbackRateSnapshot(video);
    if (!targetID || !before.rawIDs.includes(targetID)) {
      sendControlResult(command, value, false);
      return;
    }
    const api = playerApi();
    let applied = false;
    if (before.apiAvailable && api && typeof api.setPlaybackRate === "function") {
      try {
        api.setPlaybackRate(Number(targetID));
        applied = true;
      } catch (error) {}
    } else if (before.videoAvailable && video) {
      try {
        video.playbackRate = Number(targetID);
        applied = true;
      } catch (error) {}
    }
    if (!applied) {
      sendControlResult(command, value, false);
      return;
    }
    verifyControlLater(
      command,
      targetID,
      () => playbackRateSnapshot(videoElement()).currentID === targetID,
      120
    );
  }

  function invokeVolumeControl(command, volume, muted) {
    const nextVolume = Math.max(0, Math.min(1, Number(volume)));
    if (!Number.isFinite(nextVolume)) {
      sendControlResult(command, "", false);
      return;
    }
    writeRequest({ volume: nextVolume, muted: Boolean(muted) });
    applyVolume();
    verifyControlLater(command, String(nextVolume), () => {
      const after = volumeSnapshot(videoElement());
      return after.available && Math.abs(after.volume - nextVolume) < 0.011 && after.muted === Boolean(muted);
    }, 40);
  }

  function invokeControl(request) {
    const command = String(request && request.command || "");
    const value = String(request && request.value || "");
    if (command === "like" || command === "dislike") {
      invokeRatingControl(command);
      return;
    }
    if (command === "toggle-captions" || command === "select-caption") {
      invokeCaptionControl(command, value);
      return;
    }
    if (command === "select-audio-track") {
      invokeAudioControl(command, value);
      return;
    }
    if (command === "select-quality") {
      invokeQualityControl(command, value);
      return;
    }
    if (command === "select-playback-rate") {
      invokePlaybackRateControl(command, value);
      return;
    }
    if (command === "set-volume") {
      invokeVolumeControl(command, request.volume, request.muted);
      return;
    }
    sendControlResult(command, value, false);
  }

  function installFullscreenEscape() {
    window.addEventListener("keydown", (event) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      const video = videoElement();
      const fullscreenElement = document.fullscreenElement || document.webkitFullscreenElement;
      if (!fullscreenElement && !(video && video.webkitDisplayingFullscreen)) return;
      event.preventDefault();
      event.stopPropagation();
      try {
        if (typeof document.exitFullscreen === "function") {
          void document.exitFullscreen();
        } else if (typeof document.webkitExitFullscreen === "function") {
          document.webkitExitFullscreen();
        } else if (video && typeof video.webkitExitFullscreen === "function") {
          video.webkitExitFullscreen();
        }
      } catch (error) {}
    }, true);
  }

  function boot() {
	restoreLiveVideoMode();
    try {
      window.localStorage.setItem("__listenLivePlaybackRequest", JSON.stringify(Object.assign({}, INITIAL_REQUEST, readRequest())));
    } catch (error) {}
    applyVolume();
    disableWatchAutonav();
    post(Object.assign({
      type: "ready",
      state: "loading",
      url: window.location.href
    }, metadataSnapshot()));
    attachVideoListeners();
    const bodyObserver = new MutationObserver(() => {
      attachVideoListeners();
      disableWatchAutonav();
	  restoreLiveVideoMode();
    });
    bodyObserver.observe(document.documentElement || document.body, { childList: true, subtree: true });
    installMediaSessionHandlers();
    scheduleMediaSessionOverrideLoop();
    scheduleAutoplay();
    sendState("boot", true);
  }

  window.__listenLivePlayer = {
    play: () => {
      invokePlay("api-play");
      scheduleAutoplay();
    },
    pause: () => invokePause("api-pause"),
    replay: (videoId) => {
      const request = readRequest();
      const nextVideoId = String(videoId || request.videoId || "");
      if (nextVideoId) {
        writeRequest(Object.assign({}, request, { videoId: nextVideoId }));
        const api = playerApi();
        if (api && typeof api.loadVideoById === "function") {
          try { api.loadVideoById(nextVideoId); } catch (error) {}
        }
      }
      invokePlay("api-replay");
      scheduleAutoplay();
    },
    seek: (seconds) => {
      const api = playerApi();
      const next = finiteNumber(Number(seconds || 0), 0);
      if (api && typeof api.seekTo === "function") {
        try { api.seekTo(next, true); } catch (error) {}
      }
      sendState("api-seek", true);
    },
    volume: (volume, muted) => {
      writeRequest({ volume, muted });
      applyVolume();
      sendState("api-volume", true);
    },
    request: (next) => {
      const request = writeRequest(next || {});
      pendingRequestedVideoId = validYouTubeVideoId(request.videoId);
      lastRequestedAction = "play";
      sendState("api-request", true);
      const api = playerApi();
      if (api && typeof api.loadVideoById === "function" && request.videoId) {
        try { api.loadVideoById(String(request.videoId)); } catch (error) {}
      }
      applyVolume();
      scheduleAutoplay();
    },
	snapshot: () => sendState("api-snapshot", true),
	control: (request) => invokeControl(request || {})
  };

  // Restore the persisted embedded presentation before waiting for the page
  // DOM. A navigation must never expose YouTube's watch chrome through the
  // React hole while the later geometry refresh is still in flight.
  installLiveVideoModeAtDocumentStart();
  restoreLiveVideoMode();
  installVolumeGuards();
  installFullscreenEscape();
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot, { once: true });
  } else {
    boot();
  }
})();
`, listenLivePlayerSource, string(initial), string(videoModeCSS))
}

func listenYouTubeLiveVideoModeScript(rects ...ListenEmbeddedVideoRect) string {
	requestJSON := listenEmbeddedVideoResizeRequestJSON(rects...)
	videoModeCSS, _ := json.Marshal(listenYouTubeLiveVideoModeCSS)
	script := `
(function() {
  "use strict";

  const SOURCE = "listen-youtube-live-player";
  const EMBEDDED_RESIZE_REQUEST = __LISTEN_EMBEDDED_RESIZE_REQUEST__;
  const VIDEO_MODE_CSS = __LISTEN_LIVE_VIDEO_MODE_CSS__;

  try { window.localStorage.setItem("__listenLiveVideoModeActive", "true"); } catch (error) {}
  window.__listenLiveVideoModeActive = true;

  function post(payload) {
    const message = JSON.stringify(Object.assign({ source: SOURCE }, payload));
    try {
      if (window._wails && typeof window._wails.invoke === "function") {
        window._wails.invoke(message);
        return;
      }
      if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.external) {
        window.webkit.messageHandlers.external.postMessage(message);
        return;
      }
      if (window.chrome && window.chrome.webview && typeof window.chrome.webview.postMessage === "function") {
        window.chrome.webview.postMessage(message);
        return;
      }
      if (window.wails && typeof window.wails.invoke === "function") {
        window.wails.invoke(message);
      }
    } catch (error) {}
  }

  function finiteDimension(value) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(0, number) : 0;
  }

  function removeLegacyBlackout() {
    document.getElementById("listen-live-video-blackout")?.remove();
  }

  function syncNativeWindowFullscreenClass() {
    let active = false;
    try {
      active = window.sessionStorage.getItem("__listenNativeWindowFullscreenActive") === "true";
    } catch (error) {}
    document.documentElement.classList.toggle("listen-live-native-window-fullscreen", active);
  }

  function installVideoStyles() {
		syncNativeWindowFullscreenClass();
    const styleId = "listen-live-video-mode-style";
    let style = document.getElementById(styleId);
    if (!style) {
	  const styleRoot = document.head || document.documentElement;
	  if (!styleRoot) return false;
      style = document.createElement("style");
      style.id = styleId;
	  styleRoot.appendChild(style);
    }
	style.textContent = VIDEO_MODE_CSS;
	return true;
  }

  function videoElement() {
    const videos = Array.from(document.querySelectorAll("video"));
    const hasFrame = (video) => video.videoWidth > 0 && video.videoHeight > 0;
    return videos.find((video) => !video.paused && !video.ended && hasFrame(video)) ||
      videos.find((video) => !video.paused && !video.ended) ||
      videos.find((video) => video.readyState > 0 && hasFrame(video)) ||
      videos.find((video) => video.readyState > 0) ||
      videos[0] ||
      null;
  }

  function rootElement(video) {
	return video?.closest("#movie_player, .html5-video-player") ||
	  document.getElementById("movie_player") ||
      video?.parentElement ||
      null;
  }

  function markVideoTree() {
    const video = videoElement();
    const root = rootElement(video);
    if (!video || !root) return false;
    document.querySelectorAll(".listen-live-video-visible, .listen-live-video-root, .listen-live-video-surface").forEach((element) => {
      element.classList.remove("listen-live-video-visible", "listen-live-video-root", "listen-live-video-surface");
    });
	let current = video;
	let reachedRoot = false;
    while (current && current !== document.documentElement) {
      current.classList.add("listen-live-video-visible");
	  if (!reachedRoot) current.classList.add("listen-live-video-surface");
	  if (current === root) reachedRoot = true;
      current = current.parentElement;
    }
	if (!reachedRoot || current !== document.documentElement) {
	  document.querySelectorAll(".listen-live-video-visible, .listen-live-video-surface").forEach((element) => {
		element.classList.remove("listen-live-video-visible", "listen-live-video-surface");
	  });
	  return false;
	}
	root.classList.add("listen-live-video-root");
	return true;
  }

  function dimensionsMatch(actual, expected) {
    const actualSize = finiteDimension(actual);
    const expectedSize = finiteDimension(expected);
    if (expectedSize <= 1) return actualSize > 1;
	const tolerance = Math.max(4, expectedSize * 0.02);
    return Math.abs(actualSize - expectedSize) <= tolerance;
  }

  function originMatches(value) {
	const coordinate = Number(value);
	return Number.isFinite(coordinate) && Math.abs(coordinate) <= 3;
  }

  function videoTreeIsIsolated(video, root, expectedWidth, expectedHeight) {
	if (!video || !root || !root.classList.contains("listen-live-video-root") ||
		!video.classList.contains("listen-live-video-visible")) {
	  return false;
	}
	const rootStyle = window.getComputedStyle(root);
	if (!rootStyle || rootStyle.position !== "fixed" || rootStyle.visibility !== "visible" ||
		rootStyle.transform !== "none" || rootStyle.clipPath !== "none" ||
		rootStyle.overflowX !== "hidden" || rootStyle.overflowY !== "hidden") {
	  return false;
	}
	let current = video;
	let reachedRoot = false;
	while (current && current !== document.documentElement) {
	  if (!current.classList.contains("listen-live-video-visible")) return false;
	  if (!reachedRoot) {
		if (!current.classList.contains("listen-live-video-surface")) return false;
		const rect = current.getBoundingClientRect();
		if (!originMatches(rect.left) || !originMatches(rect.top) ||
			!dimensionsMatch(rect.width, expectedWidth) ||
			!dimensionsMatch(rect.height, expectedHeight)) {
		  return false;
		}
	  }
	  if (current === root) reachedRoot = true;
	  const style = window.getComputedStyle(current);
	  const opacity = Number(style.opacity);
	  if (style.visibility !== "visible" || style.transform !== "none" ||
		  style.clipPath !== "none" || (Number.isFinite(opacity) && opacity <= 0.01)) {
		return false;
	  }
	  current = current.parentElement;
	}
	return reachedRoot && current === document.documentElement;
  }

  function embeddedResizeSnapshot(request) {
    const video = videoElement();
    const root = rootElement(video);
    const videoRect = video ? video.getBoundingClientRect() : null;
    const rootRect = root ? root.getBoundingClientRect() : null;
    const viewportWidth = finiteDimension(window.innerWidth || document.documentElement.clientWidth || 0);
    const viewportHeight = finiteDimension(window.innerHeight || document.documentElement.clientHeight || 0);
    const videoRectWidth = finiteDimension(videoRect ? videoRect.width : 0);
    const videoRectHeight = finiteDimension(videoRect ? videoRect.height : 0);
    const rootRectWidth = finiteDimension(rootRect ? rootRect.width : 0);
    const rootRectHeight = finiteDimension(rootRect ? rootRect.height : 0);
    const expectedWidth = finiteDimension(request && request.width);
    const expectedHeight = finiteDimension(request && request.height);
    const viewportMatches =
      dimensionsMatch(viewportWidth, expectedWidth) &&
      dimensionsMatch(viewportHeight, expectedHeight);
	const videoMatches = Boolean(videoRect) &&
	  originMatches(videoRect.left) && originMatches(videoRect.top) &&
	  dimensionsMatch(videoRectWidth, expectedWidth) &&
	  dimensionsMatch(videoRectHeight, expectedHeight);
	const rootMatches = Boolean(rootRect) &&
	  originMatches(rootRect.left) && originMatches(rootRect.top) &&
	  dimensionsMatch(rootRectWidth, expectedWidth) &&
	  dimensionsMatch(rootRectHeight, expectedHeight);
	const hasVideoFrame = Boolean(video) &&
	  finiteDimension(video.videoWidth) > 1 &&
	  finiteDimension(video.videoHeight) > 1;
	const treeIsolated = videoTreeIsIsolated(video, root, expectedWidth, expectedHeight);
    return {
	  ready: viewportMatches && videoMatches && rootMatches && hasVideoFrame && treeIsolated,
      viewportWidth,
      viewportHeight,
      videoRectWidth,
      videoRectHeight,
      rootRectWidth,
      rootRectHeight,
      expectedWidth,
      expectedHeight,
      hasVideo: Boolean(video),
	  hasRoot: Boolean(root),
	  hasVideoFrame,
	  videoMatches,
	  rootMatches,
	  treeIsolated
    };
  }

  function postEmbeddedResizeReady(request, ready, reason, snapshot) {
    if (!request || !request.sequence) return;
    const sequence = String(request.sequence);
    if (window.__listenEmbeddedResizeSequence === sequence) {
      window.__listenEmbeddedResizeSequence = "";
    }
    post({
      type: "embedded-video-resize-ready",
      sequence,
      ready: ready === true,
      reason: reason || "",
      viewportWidth: snapshot ? snapshot.viewportWidth : 0,
      viewportHeight: snapshot ? snapshot.viewportHeight : 0,
      videoRectWidth: snapshot ? snapshot.videoRectWidth : 0,
      videoRectHeight: snapshot ? snapshot.videoRectHeight : 0,
      rootRectWidth: snapshot ? snapshot.rootRectWidth : 0,
      rootRectHeight: snapshot ? snapshot.rootRectHeight : 0,
      expectedWidth: snapshot ? snapshot.expectedWidth : 0,
      expectedHeight: snapshot ? snapshot.expectedHeight : 0,
      hasVideo: snapshot ? snapshot.hasVideo === true : false,
	  hasRoot: snapshot ? snapshot.hasRoot === true : false,
	  hasVideoFrame: snapshot ? snapshot.hasVideoFrame === true : false,
	  videoMatches: snapshot ? snapshot.videoMatches === true : false,
	  rootMatches: snapshot ? snapshot.rootMatches === true : false,
	  treeIsolated: snapshot ? snapshot.treeIsolated === true : false
    });
  }

  function waitForEmbeddedResize(request) {
    if (!request || !request.sequence) return;
    const sequence = String(request.sequence);
    window.__listenEmbeddedResizeSequence = sequence;
    let attempts = 0;
    let stableCount = 0;
    let lastSignature = "";
    const tick = () => {
      if (window.__listenEmbeddedResizeSequence !== sequence) return;
      removeLegacyBlackout();
      installVideoStyles();
      markVideoTree();
      const snapshot = embeddedResizeSnapshot(request);
      const signature = [
        Math.round(snapshot.viewportWidth * 2) / 2,
        Math.round(snapshot.viewportHeight * 2) / 2,
        Math.round(snapshot.videoRectWidth * 2) / 2,
        Math.round(snapshot.videoRectHeight * 2) / 2,
        Math.round(snapshot.rootRectWidth * 2) / 2,
        Math.round(snapshot.rootRectHeight * 2) / 2,
        snapshot.hasVideo ? "1" : "0",
		snapshot.hasRoot ? "1" : "0",
		snapshot.hasVideoFrame ? "1" : "0",
		snapshot.videoMatches ? "1" : "0",
		snapshot.rootMatches ? "1" : "0",
		snapshot.treeIsolated ? "1" : "0"
      ].join(":");
      stableCount = signature === lastSignature ? stableCount + 1 : 0;
      lastSignature = signature;
      if (snapshot.ready && stableCount >= 1) {
        postEmbeddedResizeReady(request, true, "ready", snapshot);
        return;
      }
      attempts += 1;
      if (attempts >= 48) {
        postEmbeddedResizeReady(request, false, "timeout", snapshot);
        return;
      }
      window.requestAnimationFrame(tick);
    };
    window.requestAnimationFrame(tick);
  }

  function enforce() {
    let active = window.__listenLiveVideoModeActive;
    try {
      active = active && window.localStorage.getItem("__listenLiveVideoModeActive") === "true";
    } catch (error) {}
    if (!active) return;
    removeLegacyBlackout();
    installVideoStyles();
    if (markVideoTree()) {
      try {
        window.__listenLiveVideoModeLastMarkedAt = Date.now();
      } catch (error) {}
    }
    window.requestAnimationFrame(enforce);
  }

  removeLegacyBlackout();
  installVideoStyles();
  try { window.scrollTo(0, 0); } catch (error) {}
  markVideoTree();
  waitForEmbeddedResize(EMBEDDED_RESIZE_REQUEST);
  window.requestAnimationFrame(enforce);
})();
	`
	return strings.NewReplacer(
		"__LISTEN_EMBEDDED_RESIZE_REQUEST__", requestJSON,
		"__LISTEN_LIVE_VIDEO_MODE_CSS__", string(videoModeCSS),
	).Replace(script)
}

func listenYouTubeLiveNativeWindowFullscreenModeScript(active bool) string {
	return fmt.Sprintf(`
(function() {
  const active = %t;
  try {
    if (active) window.sessionStorage.setItem("__listenNativeWindowFullscreenActive", "true");
    else window.sessionStorage.removeItem("__listenNativeWindowFullscreenActive");
  } catch (error) {}
  document.documentElement.classList.toggle("listen-live-native-window-fullscreen", active);
})();
`, active)
}

func listenYouTubeLiveExitVideoModeScript() string {
	return `
(function() {
  window.__listenLiveVideoModeActive = false;
  try { window.localStorage.setItem("__listenLiveVideoModeActive", "false"); } catch (error) {}
	try { window.sessionStorage.removeItem("__listenNativeWindowFullscreenActive"); } catch (error) {}
	document.documentElement.classList.remove("listen-live-native-window-fullscreen");
  document.getElementById("listen-live-video-blackout")?.remove();
  document.getElementById("listen-live-video-mode-style")?.remove();
  document.querySelectorAll(".listen-live-video-visible, .listen-live-video-root, .listen-live-video-surface").forEach((element) => {
    element.classList.remove("listen-live-video-visible", "listen-live-video-root", "listen-live-video-surface");
  });
  document.body.style.overflow = "";
  document.body.style.background = "";
})();
`
}

func listenYouTubeLivePrepareLoadScript(request ListenPlayerPlayRequest) string {
	requestJSON, _ := json.Marshal(normalizeListenPlayerPlayRequest(request))
	return fmt.Sprintf(`
(function() {
  try {
    const request = %s;
    const api = window.__listenLivePlayer;
    // Pause belongs to the old session. Apply the new request last so its play
    // intent and autoplay recovery cannot be overwritten by a sticky pause.
    if (api && typeof api.pause === "function") api.pause();
    else document.querySelector("video")?.pause();
    if (api && typeof api.request === "function") api.request(request);
    else window.localStorage.setItem("__listenLivePlaybackRequest", JSON.stringify(request));
  } catch (error) {}
})();
`, string(requestJSON))
}

func listenYouTubeLivePauseScript() string {
	return `
(function() {
  const api = window.__listenLivePlayer;
  if (api && typeof api.pause === "function") {
    try {
      api.pause();
      return;
    } catch (error) {}
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.pauseVideo === "function") {
    try { moviePlayer.pauseVideo(); } catch (error) {}
  }
  document.querySelectorAll("video").forEach((video) => {
    try { if (!video.paused) video.pause(); } catch (error) {}
  });
})();
`
}

func listenYouTubeLiveResumeScript() string {
	return `
(function() {
  const api = window.__listenLivePlayer;
  if (api && typeof api.play === "function") {
    api.play();
    return;
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.playVideo === "function") {
    try { moviePlayer.playVideo(); } catch (error) {}
  }
  document.querySelector("video")?.play();
})();
`
}

func listenYouTubeLiveReplayScript(videoID string, volume float64, muted bool) string {
	request := ListenPlayerPlayRequest{
		VideoID: videoID,
		Volume:  clampListenVolume(volume),
		Muted:   muted,
	}
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const request = %s;
  const api = window.__listenLivePlayer;
  if (api && typeof api.replay === "function") {
    if (request.videoId) api.replay(request.videoId);
    else api.play();
    return;
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.playVideo === "function") {
    try { moviePlayer.playVideo(); } catch (error) {}
  }
  document.querySelector("video")?.play();
})();
`, string(requestJSON))
}

func listenYouTubeLiveSameVideoPlayScript(request ListenPlayerPlayRequest) string {
	request = normalizeListenPlayerPlayRequest(request)
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const request = %s;
  const api = window.__listenLivePlayer;
  try { window.localStorage.setItem("__listenLivePlaybackRequest", JSON.stringify(request)); } catch (error) {}
  if (api && typeof api.volume === "function") {
    api.volume(request.volume, request.muted);
  }
  if (api && typeof api.play === "function") {
    api.play();
    return;
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.playVideo === "function") {
    try { moviePlayer.playVideo(); } catch (error) {}
  }
  document.querySelector("video")?.play();
})();
`, string(requestJSON))
}

func listenYouTubeLiveSeekScript(seconds float64) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__listenLivePlayer;
  if (api && typeof api.seek === "function") {
    api.seek(%f);
    return;
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.seekTo === "function") {
    try { moviePlayer.seekTo(%f, true); } catch (error) {}
  }
})();
`, clampListenSeconds(seconds), clampListenSeconds(seconds))
}

func listenYouTubeLiveVolumeScript(volume float64, muted bool) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__listenLivePlayer;
  if (api && typeof api.volume === "function") {
    api.volume(%f, %t);
    return;
  }
  const volume = %f;
  document.querySelectorAll("video").forEach((video) => {
    try {
      video.volume = volume;
      video.muted = %t;
    } catch (error) {}
  });
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.setVolume === "function") {
    try { moviePlayer.setVolume(Math.round(volume * 100)); } catch (error) {}
  }
  if (moviePlayer && typeof moviePlayer.mute === "function" && typeof moviePlayer.unMute === "function") {
    try {
      if (%t) moviePlayer.mute();
      else moviePlayer.unMute();
    } catch (error) {}
  }
})();
`, clampListenVolume(volume), muted, clampListenVolume(volume), muted, muted)
}

func listenYouTubeLiveControlScript(request ListenLivePlaybackControlRequest) string {
	request.Volume = clampListenVolume(request.Volume)
	payload, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const player = window.__listenLivePlayer;
  if (!player || typeof player.control !== "function") return;
  player.control(%s);
})();
`, string(payload))
}
