package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/listenplayback"
	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	listenPlayerWindowName = "listen-youtube-music-player"
	listenPlayerEventName  = "listen:youtube-music-player"
	listenPlayerSource     = "listen-youtube-music-player"

	listenYouTubeMusicOrigin   = "https://music.youtube.com"
	listenYouTubeMusicBlankURL = "about:blank"

	listenEmbeddedVideoResizeReadyType = "embedded-video-resize-ready"
	listenEmbeddedVideoResizeTimeout   = 1400 * time.Millisecond
)

var listenYouTubeVideoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

type ListenPlayerPlayRequest struct {
	VideoID              string  `json:"videoId"`
	Title                string  `json:"title"`
	Artist               string  `json:"artist"`
	Language             string  `json:"language,omitempty"`
	StartSeconds         float64 `json:"startSeconds"`
	RestartFromStart     bool    `json:"restartFromStart,omitempty"`
	ForceReload          bool    `json:"forceReload,omitempty"`
	Volume               float64 `json:"volume"`
	Muted                bool    `json:"muted"`
	PlaybackAudioQuality string  `json:"playbackAudioQuality,omitempty"`
}

type ListenPlayerVolumeRequest struct {
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted"`
}

type ListenPlayerSeekRequest struct {
	Seconds float64 `json:"seconds"`
}

type ListenAirPlayAnchor struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ListenEmbeddedVideoRect struct {
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	CenterX        float64 `json:"centerX,omitempty"`
	CenterY        float64 `json:"centerY,omitempty"`
	ViewportWidth  float64 `json:"viewportWidth,omitempty"`
	ViewportHeight float64 `json:"viewportHeight,omitempty"`
	Radius         float64 `json:"radius,omitempty"`
	Interactive    bool    `json:"interactive,omitempty"`
	Sequence       uint64  `json:"sequence,omitempty"`
}

type ListenEmbeddedVideoHideRequest struct {
	Sequence uint64 `json:"sequence,omitempty"`
}

type ListenPlayerStatus struct {
	Available       bool    `json:"available"`
	VideoID         string  `json:"videoId"`
	ObservedVideoID string  `json:"observedVideoId,omitempty"`
	State           string  `json:"state"`
	Title           string  `json:"title,omitempty"`
	Artist          string  `json:"artist,omitempty"`
	ThumbnailURL    string  `json:"thumbnailUrl,omitempty"`
	LikeStatus      string  `json:"likeStatus,omitempty"`
	VideoAvailable  bool    `json:"videoAvailable,omitempty"`
	VideoKnown      bool    `json:"videoAvailabilityKnown,omitempty"`
	Advertising     bool    `json:"advertising,omitempty"`
	AdLabel         string  `json:"adLabel,omitempty"`
	ErrorCode       string  `json:"errorCode,omitempty"`
	ErrorMessage    string  `json:"errorMessage,omitempty"`
	CurrentTime     float64 `json:"currentTime,omitempty"`
	Duration        float64 `json:"duration,omitempty"`
	BufferedTime    float64 `json:"bufferedTime,omitempty"`
}

type ListenPlayerHandler struct {
	player  *ListenYouTubeMusicPlayer
	service *listenplayback.PlayerService
}

type listenPlayerCookieProvider interface {
	RecordsForSiteKey(ctx context.Context, siteKey string) ([]appcookies.Record, error)
}

func NewListenPlayerHandler(player *ListenYouTubeMusicPlayer, service ...*listenplayback.PlayerService) *ListenPlayerHandler {
	handler := &ListenPlayerHandler{player: player}
	if len(service) > 0 {
		handler.service = service[0]
	}
	return handler
}

func (handler *ListenPlayerHandler) ServiceName() string {
	return "ListenPlayerHandler"
}

func (handler *ListenPlayerHandler) Play(ctx context.Context, request ListenPlayerPlayRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		handler.service.RecordPlaybackIntent()
		return handler.service.PlayTrack(ctx, listenPlaybackTrackFromPlayRequest(request), listenplayback.PlayOptions{
			StartSeconds:     request.StartSeconds,
			RestartFromStart: request.RestartFromStart,
			ForceReload:      request.ForceReload,
		})
	}
	return handler.player.Play(request)
}

func (handler *ListenPlayerHandler) Pause(ctx context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		return handler.service.Pause(ctx)
	}
	return handler.player.Pause()
}

func (handler *ListenPlayerHandler) Resume(ctx context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		handler.service.RecordPlaybackIntent()
		return handler.service.Resume(ctx)
	}
	return handler.player.Resume()
}

func (handler *ListenPlayerHandler) Replay(ctx context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		handler.service.RecordPlaybackIntent()
		if err := handler.service.Seek(ctx, 0); err != nil {
			return err
		}
		return handler.service.Resume(ctx)
	}
	return handler.player.Replay()
}

func (handler *ListenPlayerHandler) Seek(ctx context.Context, request ListenPlayerSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		return handler.service.Seek(ctx, request.Seconds)
	}
	return handler.player.Seek(request.Seconds)
}

func (handler *ListenPlayerHandler) SetVolume(ctx context.Context, request ListenPlayerVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		return handler.service.SetVolume(ctx, request.Volume, request.Muted)
	}
	return handler.player.SetVolume(request.Volume, request.Muted)
}

func (handler *ListenPlayerHandler) Reset(ctx context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	if handler.service != nil {
		if err := handler.service.Stop(ctx); err != nil {
			return err
		}
	}
	return handler.player.Reset()
}

func (handler *ListenPlayerHandler) ShowWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.ShowWindow()
}

func (handler *ListenPlayerHandler) HideWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.HideWindow()
}

func (handler *ListenPlayerHandler) ShowVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.ShowVideoWindow()
}

func (handler *ListenPlayerHandler) HideVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.HideVideoWindow()
}

func (handler *ListenPlayerHandler) HideVideoWindowForSequence(_ context.Context, request ListenEmbeddedVideoHideRequest) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("listen player unavailable")
	}
	return handler.player.HideVideoWindowForSequence(request.Sequence)
}

func (handler *ListenPlayerHandler) ShowEmbeddedVideo(_ context.Context, rect ListenEmbeddedVideoRect) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("listen player unavailable")
	}
	return handler.player.ShowEmbeddedVideo(rect)
}

func (handler *ListenPlayerHandler) HideEmbeddedVideo(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.HideEmbeddedVideo()
}

func (handler *ListenPlayerHandler) HideEmbeddedVideoForSequence(_ context.Context, request ListenEmbeddedVideoHideRequest) (bool, error) {
	if handler == nil || handler.player == nil {
		return false, fmt.Errorf("listen player unavailable")
	}
	return handler.player.HideEmbeddedVideoForSequence(request.Sequence)
}

func (handler *ListenPlayerHandler) ShowAirPlayPicker(_ context.Context, anchor ListenAirPlayAnchor) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.ShowAirPlayPicker(anchor)
}

func (handler *ListenPlayerHandler) StartLyricsPoll(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.StartLyricsPoll()
}

func (handler *ListenPlayerHandler) StopLyricsPoll(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen player unavailable")
	}
	return handler.player.StopLyricsPoll()
}

func (handler *ListenPlayerHandler) Status(_ context.Context) (ListenPlayerStatus, error) {
	if handler == nil || handler.player == nil {
		return ListenPlayerStatus{}, fmt.Errorf("listen player unavailable")
	}
	return handler.player.Status(), nil
}

type ListenYouTubeMusicPlayer struct {
	app     *application.App
	windows *WindowManager
	cookies listenPlayerCookieProvider

	mu                    sync.Mutex
	window                *application.WebviewWindow
	closeHook             func()
	bridgeHook            func()
	currentVideo          string
	currentState          string
	activated             bool
	targetVolume          float64
	targetMuted           bool
	playbackAudioQuality  string
	requestTitle          string
	requestArtist         string
	observedVideo         string
	observedTitle         string
	observedArtist        string
	observedThumb         string
	observedLike          string
	advertising           bool
	adLabel               string
	errorCode             string
	errorMessage          string
	currentTime           float64
	duration              float64
	bufferedTime          float64
	lastPlayAt            time.Time
	videoVisible          bool
	embeddedVisible       bool
	embeddedRect          ListenEmbeddedVideoRect
	embeddedRefreshToken  uint64
	embeddedSequence      uint64
	embeddedResizeWaiters map[uint64]chan bool
	playbackService       *listenplayback.PlayerService
}

func NewListenYouTubeMusicPlayer(app *application.App, windows *WindowManager, cookies listenPlayerCookieProvider) *ListenYouTubeMusicPlayer {
	return &ListenYouTubeMusicPlayer{
		app:                  app,
		windows:              windows,
		cookies:              cookies,
		currentState:         "idle",
		targetVolume:         1,
		playbackAudioQuality: settings.DefaultPlaybackAudioQuality.String(),
	}
}

func (player *ListenYouTubeMusicPlayer) SetPlaybackService(service *listenplayback.PlayerService) {
	if player == nil {
		return
	}
	player.mu.Lock()
	player.playbackService = service
	player.mu.Unlock()
}

func (player *ListenYouTubeMusicPlayer) SetPlaybackAudioQuality(value string) error {
	if player == nil {
		return nil
	}
	quality, err := settings.ParsePlaybackAudioQuality(value)
	if err != nil {
		return err
	}
	normalized := quality.String()
	player.mu.Lock()
	player.playbackAudioQuality = normalized
	window := player.window
	player.mu.Unlock()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicPlaybackAudioQualityScript(normalized))
	return nil
}

func (player *ListenYouTubeMusicPlayer) Play(request ListenPlayerPlayRequest) error {
	if player == nil || player.app == nil {
		return fmt.Errorf("listen player unavailable")
	}
	request = normalizeListenPlayerPlayRequest(request)
	if !listenYouTubeVideoIDPattern.MatchString(request.VideoID) {
		return fmt.Errorf("invalid youtube video id")
	}
	cookies := player.playbackCookies(context.Background())

	player.mu.Lock()
	if request.PlaybackAudioQuality == "" {
		request.PlaybackAudioQuality = player.playbackAudioQuality
	}
	if request.PlaybackAudioQuality == "" {
		request.PlaybackAudioQuality = settings.DefaultPlaybackAudioQuality.String()
	}
	player.targetVolume = request.Volume
	player.targetMuted = request.Muted
	player.requestTitle = request.Title
	player.requestArtist = request.Artist
	window := player.window
	videoVisible := player.videoVisible
	embeddedVisible := player.embeddedVisible
	embeddedRect := player.embeddedRect
	createdWindow := window == nil
	sameVideo := !request.ForceReload && !createdWindow && (player.currentVideo == request.VideoID || player.observedVideo == request.VideoID)
	if sameVideo {
		player.currentState = "buffering"
		player.lastPlayAt = time.Now()
		player.currentVideo = request.VideoID
		player.mu.Unlock()
		player.dispatchPlaybackState("buffering", "same-video-play-requested")
		if window == nil {
			return fmt.Errorf("listen player window unavailable")
		}
		if embeddedVisible {
			listenClaimEmbeddedVideoOwner(window)
			_, _ = player.showEmbeddedVideoWindow(window, embeddedRect)
			player.scheduleEmbeddedVideoModeRefresh(window)
		}
		if videoVisible {
			if embeddedVisible {
				execListenYouTubeMusicJS(window, listenYouTubeMusicVideoModeScript(embeddedRect))
			} else {
				player.scheduleVideoModeRefresh(window)
			}
		}
		if request.RestartFromStart {
			execListenYouTubeMusicJS(window, listenYouTubeMusicReplayScript(0, request.Volume, request.Muted))
		} else {
			execListenYouTubeMusicJS(window, listenYouTubeMusicSameVideoResumeScript(request))
		}
		return nil
	}

	player.currentState = "loading"
	player.observedVideo = ""
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.observedLike = ""
	player.advertising = false
	player.adLabel = ""
	player.errorCode = ""
	player.errorMessage = ""
	player.currentTime = 0
	player.duration = 0
	player.bufferedTime = 0
	player.lastPlayAt = time.Now()
	if window == nil {
		window = player.createWindowLocked(request)
	}
	player.currentVideo = request.VideoID
	player.mu.Unlock()

	player.dispatch(map[string]any{
		"source":           listenPlayerSource,
		"type":             "state",
		"state":            "loading",
		"videoId":          request.VideoID,
		"observedVideoId":  request.VideoID,
		"requestedVideoId": request.VideoID,
		"title":            request.Title,
		"artist":           request.Artist,
	})

	if window == nil {
		return fmt.Errorf("listen player window unavailable")
	}
	if embeddedVisible {
		listenClaimEmbeddedVideoOwner(window)
		_, _ = player.showEmbeddedVideoWindow(window, embeddedRect)
	}
	if videoVisible {
		if embeddedVisible {
			execListenYouTubeMusicJS(window, listenYouTubeMusicVideoModeScript(embeddedRect))
		} else {
			player.scheduleVideoModeRefresh(window)
		}
	}

	if createdWindow {
		loadListenYouTubeMusicURL(window, listenYouTubeMusicWatchURL(request.VideoID, request.Language), cookies)
		if embeddedVisible {
			player.scheduleEmbeddedVideoModeRefresh(window)
		}
	}

	if createdWindow {
		return nil
	}

	execListenYouTubeMusicJS(window, listenYouTubeMusicPrepareLoadScript(request))
	loadListenYouTubeMusicURL(window, listenYouTubeMusicWatchURL(request.VideoID, request.Language), cookies)
	if embeddedVisible {
		player.scheduleEmbeddedVideoModeRefresh(window)
	}
	return nil
}

func (player *ListenYouTubeMusicPlayer) Pause() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("paused", "pause-requested")
	execListenYouTubeMusicJS(window, listenYouTubeMusicPauseScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) Resume() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("buffering", "resume-requested")
	execListenYouTubeMusicJS(window, listenYouTubeMusicResumeScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) Next() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicNextScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) Previous() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicPreviousScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) Replay() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	volume := player.targetVolume
	muted := player.targetMuted
	player.mu.Unlock()
	player.dispatchPlaybackState("buffering", "replay-requested")
	execListenYouTubeMusicJS(window, listenYouTubeMusicReplayScript(0, volume, muted))
	return nil
}

func (player *ListenYouTubeMusicPlayer) Seek(seconds float64) error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("buffering", "seek-requested")
	execListenYouTubeMusicJS(window, listenYouTubeMusicSeekScript(seconds))
	return nil
}

func (player *ListenYouTubeMusicPlayer) SetVolume(volume float64, muted bool) error {
	volume = clampListenVolume(volume)

	player.mu.Lock()
	player.targetVolume = volume
	player.targetMuted = muted
	window := player.window
	player.mu.Unlock()

	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicVolumeScript(volume, muted))
	return nil
}

func (player *ListenYouTubeMusicPlayer) StartLyricsPoll() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicStartLyricsPollScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) StopLyricsPoll() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicStopLyricsPollScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) Reset() error {
	if player == nil {
		return nil
	}
	player.mu.Lock()
	window := player.window
	closeHook := player.closeHook
	bridgeHook := player.bridgeHook
	player.window = nil
	player.closeHook = nil
	player.bridgeHook = nil
	player.currentVideo = ""
	player.currentState = "idle"
	player.activated = false
	player.videoVisible = false
	player.embeddedVisible = false
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedRefreshToken += 1
	player.requestTitle = ""
	player.requestArtist = ""
	player.observedVideo = ""
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.observedLike = ""
	player.advertising = false
	player.adLabel = ""
	player.currentTime = 0
	player.duration = 0
	player.bufferedTime = 0
	player.lastPlayAt = time.Time{}
	player.mu.Unlock()

	if closeHook != nil {
		closeHook()
	}
	if bridgeHook != nil {
		bridgeHook()
	}
	if window != nil {
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
		execListenYouTubeMusicJS(window, listenYouTubeMusicPauseScript())
		window.SetURL(listenYouTubeMusicBlankURL)
		window.Close()
	}
	player.dispatchPlaybackState("idle", "reset")
	return nil
}

func (player *ListenYouTubeMusicPlayer) ShowWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	window.Show()
	window.Focus()
	return nil
}

func (player *ListenYouTubeMusicPlayer) HideWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	window.Hide()
	return nil
}

func (player *ListenYouTubeMusicPlayer) ShowVideoWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	wasEmbeddedVisible := player.embeddedVisible
	player.videoVisible = true
	player.embeddedVisible = false
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedRefreshToken += 1
	player.mu.Unlock()
	if wasEmbeddedVisible {
		listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
		hideListenNativeEmbeddedWebView(window.NativeWindow())
	}
	window.SetTitle("Listen Video")
	window.SetMinSize(320, 180)
	window.SetSize(720, 405)
	window.Show()
	execListenYouTubeMusicJS(window, listenYouTubeMusicVideoModeScript())
	player.scheduleVideoModeRefresh(window)
	return nil
}

func (player *ListenYouTubeMusicPlayer) HideVideoWindow() error {
	_, err := player.HideVideoWindowForSequence(0)
	return err
}

func (player *ListenYouTubeMusicPlayer) HideVideoWindowForSequence(sequence uint64) (bool, error) {
	window := player.currentWindow()
	if window == nil {
		return false, nil
	}
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
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedRefreshToken += 1
	player.mu.Unlock()
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	execListenYouTubeMusicJS(window, listenYouTubeMusicExitVideoModeScript())
	window.Hide()
	return true, nil
}

func (player *ListenYouTubeMusicPlayer) ShowEmbeddedVideo(rect ListenEmbeddedVideoRect) (bool, error) {
	if player == nil {
		return false, nil
	}
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
	window := player.window
	player.mu.Unlock()
	if window == nil {
		return false, nil
	}
	owner := listenClaimEmbeddedVideoOwner(window)
	shown, err := player.showEmbeddedVideoWindow(window, rect)
	if err != nil {
		listenReleaseEmbeddedVideoOwner(owner)
		return false, err
	}
	waitForResize, unregisterResizeWaiter := player.registerEmbeddedVideoResizeWaiter(rect.Sequence)
	defer unregisterResizeWaiter()
	execListenYouTubeMusicJS(window, listenYouTubeMusicVideoModeScript(rect))
	execListenYouTubeMusicJS(window, listenYouTubeMusicVolumeScript(volume, muted))
	if !wasEmbeddedVisible {
		player.scheduleEmbeddedVideoModeRefresh(window)
	}
	if !shown {
		listenReleaseEmbeddedVideoOwner(owner)
		return false, nil
	}
	ready := player.waitForEmbeddedVideoResize(waitForResize)
	return ready && listenEmbeddedVideoOwnerActive(owner), nil
}

func (player *ListenYouTubeMusicPlayer) HideEmbeddedVideo() error {
	_, err := player.HideEmbeddedVideoForSequence(0)
	return err
}

func (player *ListenYouTubeMusicPlayer) HideEmbeddedVideoForSequence(sequence uint64) (bool, error) {
	if player == nil {
		return false, nil
	}
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
	player.embeddedRect = ListenEmbeddedVideoRect{}
	player.embeddedRefreshToken += 1
	window := player.window
	player.mu.Unlock()
	if window == nil {
		return false, nil
	}
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	execListenYouTubeMusicJS(window, listenYouTubeMusicExitVideoModeScript())
	window.Hide()
	return true, nil
}

func (player *ListenYouTubeMusicPlayer) ShowAirPlayPicker(anchor ListenAirPlayAnchor) error {
	if player != nil && player.windows != nil && player.windows.mainWindow != nil {
		if showListenNativeAirPlayPicker(player.windows.mainWindow.NativeWindow(), anchor) {
			return nil
		}
	}
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execListenYouTubeMusicJS(window, listenYouTubeMusicAirPlayScript())
	return nil
}

func (player *ListenYouTubeMusicPlayer) Status() ListenPlayerStatus {
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
	thumbnailURL := player.observedThumb
	if thumbnailURL == "" {
		thumbnailURL = listenYouTubePosterURL(player.observedVideo)
	}
	if thumbnailURL == "" {
		thumbnailURL = listenYouTubePosterURL(player.currentVideo)
	}
	return ListenPlayerStatus{
		Available:       player.window != nil,
		VideoID:         player.currentVideo,
		ObservedVideoID: player.observedVideo,
		State:           player.currentState,
		Title:           title,
		Artist:          artist,
		ThumbnailURL:    thumbnailURL,
		LikeStatus:      player.observedLike,
		Advertising:     player.advertising,
		AdLabel:         player.adLabel,
		ErrorCode:       player.errorCode,
		ErrorMessage:    player.errorMessage,
		CurrentTime:     player.currentTime,
		Duration:        player.duration,
		BufferedTime:    player.bufferedTime,
	}
}

func (player *ListenYouTubeMusicPlayer) HandleRawMessage(window application.Window, message string, _ *application.OriginInfo) bool {
	if player == nil || window == nil || window.Name() != listenPlayerWindowName {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	eventType := listenPayloadString(payload, "type")
	if source, _ := payload["source"].(string); source != listenPlayerSource {
		return false
	}

	player.mu.Lock()
	activeWindow := player.window
	player.mu.Unlock()
	if activeWindow == nil || window.ID() != activeWindow.ID() {
		return true
	}

	if eventType == listenEmbeddedVideoResizeReadyType {
		sequence, _ := listenPayloadUint64(payload, "sequence")
		ready := listenPayloadBool(payload, "ready")
		player.completeEmbeddedVideoResize(sequence, ready)
		return true
	}
	if eventType == "PLAYBACK_AUDIO_QUALITY_OBSERVED" {
		player.handleListenPlayerObservedAudioQuality(payload)
		return true
	}
	state := listenPayloadString(payload, "state")
	videoID := listenPayloadString(payload, "observedVideoId")
	if videoID == "" {
		videoID = listenPayloadString(payload, "videoId")
	}
	title := listenPayloadString(payload, "title")
	artist := listenPayloadString(payload, "artist")
	thumbnailURL := listenPayloadString(payload, "thumbnailUrl")
	likeStatus := listenPayloadString(payload, "likeStatus")
	advertising := listenPayloadBool(payload, "advertising") || listenPayloadBool(payload, "ad")
	adLabel := listenPayloadString(payload, "adLabel")
	trackChanged := listenPayloadBool(payload, "trackChanged")
	errorCode := listenPayloadDisplayString(payload, "errorCode")
	if errorCode == "" {
		errorCode = listenPayloadDisplayString(payload, "code")
	}
	errorMessage := listenPayloadString(payload, "errorMessage")
	if errorMessage == "" {
		errorMessage = listenPayloadString(payload, "message")
	}
	player.mu.Lock()
	currentVideo := player.currentVideo
	requestTitle := player.requestTitle
	requestArtist := player.requestArtist
	recentRequestedSwitch := !player.lastPlayAt.IsZero() && time.Since(player.lastPlayAt) < 2*time.Second
	if eventType == "track-ended" &&
		listenYouTubeVideoIDPattern.MatchString(videoID) &&
		currentVideo != "" &&
		videoID != currentVideo {
		player.mu.Unlock()
		return true
	}
	if eventType != "track-ended" &&
		listenYouTubeVideoIDPattern.MatchString(videoID) &&
		currentVideo != "" &&
		videoID != currentVideo &&
		(recentRequestedSwitch || isListenStalePlaybackState(state)) {
		player.mu.Unlock()
		return true
	}
	hideAfterActivation := false
	windowToHide := player.window
	videoVisible := player.videoVisible
	if advertising && currentVideo != "" && videoID != currentVideo {
		videoID = currentVideo
	}
	currentEvent := videoID == "" || videoID == currentVideo
	if currentEvent {
		if title == "" {
			title = requestTitle
		}
		if artist == "" {
			artist = requestArtist
		}
	} else if videoID != "" && videoID == player.observedVideo {
		if title == "" {
			title = player.observedTitle
		}
		if artist == "" {
			artist = player.observedArtist
		}
		if thumbnailURL == "" {
			thumbnailURL = player.observedThumb
		}
		if likeStatus == "" {
			likeStatus = player.observedLike
		}
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
	if listenYouTubeVideoIDPattern.MatchString(videoID) {
		player.observedVideo = videoID
	} else if currentEvent && currentVideo != "" {
		videoID = currentVideo
		player.observedVideo = currentVideo
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
	if likeStatus != "" {
		player.observedLike = likeStatus
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
	if currentTime, ok := listenPayloadFloat(payload, "currentTime"); ok {
		player.currentTime = currentTime
	}
	if duration, ok := listenPayloadFloat(payload, "duration"); ok {
		player.duration = duration
	}
	if bufferedTime, ok := listenPayloadFloat(payload, "bufferedTime"); ok {
		player.bufferedTime = bufferedTime
	}
	playbackService := player.playbackService
	player.mu.Unlock()

	if hideAfterActivation && windowToHide != nil {
		windowToHide.Hide()
	}

	if state != "" {
		payload["state"] = state
	}
	if videoID != "" {
		payload["videoId"] = videoID
		payload["observedVideoId"] = videoID
	}
	if currentVideo != "" {
		payload["requestedVideoId"] = currentVideo
	}
	if title != "" {
		payload["title"] = title
	}
	if artist != "" {
		payload["artist"] = artist
	}
	if thumbnailURL != "" {
		payload["thumbnailUrl"] = thumbnailURL
	}
	if likeStatus != "" {
		payload["likeStatus"] = likeStatus
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
	player.syncPlaybackServiceFromNativeEvent(
		playbackService,
		eventType,
		state,
		videoID,
		title,
		artist,
		thumbnailURL,
		likeStatus,
		trackChanged,
		payload,
	)
	player.dispatch(payload)
	return true
}

func (player *ListenYouTubeMusicPlayer) handleListenPlayerObservedAudioQuality(payload map[string]any) {
	observed, videoID, ok := listenObservedPlaybackAudioQuality(payload)
	if !ok {
		return
	}
	player.mu.Lock()
	currentVideo := player.currentVideo
	if videoID != "" && currentVideo != "" && videoID != currentVideo {
		player.mu.Unlock()
		return
	}
	playbackService := player.playbackService
	player.mu.Unlock()
	if playbackService != nil {
		playbackService.UpdatePlaybackAudioQuality(context.Background(), observed)
	}
}

func listenObservedPlaybackAudioQuality(payload map[string]any) (string, string, bool) {
	if listenPayloadString(payload, "type") != "PLAYBACK_AUDIO_QUALITY_OBSERVED" {
		return "", "", false
	}
	observed := listenplayback.NormalizeObservedPlaybackAudioQuality(listenPayloadString(payload, "observed"))
	if observed == "" {
		return "", "", false
	}
	return observed, listenPayloadString(payload, "videoId"), true
}

func (player *ListenYouTubeMusicPlayer) syncPlaybackServiceFromNativeEvent(
	service *listenplayback.PlayerService,
	eventType string,
	state string,
	videoID string,
	title string,
	artist string,
	thumbnailURL string,
	likeStatus string,
	trackChanged bool,
	payload map[string]any,
) {
	if service == nil || advertisingPayload(payload) {
		return
	}
	ctx := context.Background()
	switch eventType {
	case "remote-next":
		_ = service.Next(ctx)
		return
	case "remote-previous":
		_ = service.Previous(ctx)
		return
	case "track-ended":
		_ = service.HandleTrackEnded(ctx, videoID)
		return
	case "lyrics-time":
		if currentTime, ok := listenPayloadFloat(payload, "currentTime"); ok {
			service.UpdateLyricsTime(ctx, currentTime)
		}
		return
	}
	currentTime, hasCurrentTime := listenPayloadFloat(payload, "currentTime")
	duration, hasDuration := listenPayloadFloat(payload, "duration")
	if state != "" && (hasCurrentTime || hasDuration) {
		_ = service.UpdatePlaybackState(ctx, listenPlaybackPayloadIsPlaying(state, payload), currentTime, duration)
	}
	if videoID != "" || title != "" {
		_ = service.UpdateTrackMetadata(ctx, listenplayback.ObservedTrack{
			ObservedVideoID: videoID,
			Title:           title,
			Artist:          artist,
			ThumbnailURL:    thumbnailURL,
			LikeStatus:      likeStatus,
			TrackChanged:    trackChanged,
			MetadataSource:  listenPayloadString(payload, "metadataSource"),
		})
	}
}

func advertisingPayload(payload map[string]any) bool {
	return listenPayloadBool(payload, "advertising") || listenPayloadBool(payload, "ad")
}

func listenPlaybackPayloadIsPlaying(state string, payload map[string]any) bool {
	if state != "playing" && state != "buffering" {
		return false
	}
	if paused, ok := listenPayloadBoolValue(payload, "paused"); ok && paused {
		return false
	}
	if ended, ok := listenPayloadBoolValue(payload, "ended"); ok && ended {
		return false
	}
	return true
}

func (player *ListenYouTubeMusicPlayer) currentWindow() *application.WebviewWindow {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.window
}

func (player *ListenYouTubeMusicPlayer) scheduleVideoModeRefresh(window *application.WebviewWindow) {
	if player == nil || window == nil {
		return
	}
	windowID := window.ID()
	go func() {
		for _, delay := range []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond, 3 * time.Second} {
			time.Sleep(delay)
			player.mu.Lock()
			active := player.videoVisible &&
				!player.embeddedVisible &&
				player.window != nil &&
				player.window.ID() == windowID
			player.mu.Unlock()
			if !active {
				return
			}
			execListenYouTubeMusicJS(window, listenYouTubeMusicVideoModeScript())
		}
	}()
}

func (player *ListenYouTubeMusicPlayer) showEmbeddedVideoWindow(window *application.WebviewWindow, rect ListenEmbeddedVideoRect) (bool, error) {
	if player == nil || window == nil || player.windows == nil || player.windows.mainWindow == nil {
		return false, nil
	}
	owner := listenEmbeddedVideoOwnerID(window)
	shown := listenShowNativeEmbeddedWebViewForOwner(
		owner,
		window.NativeWindow(),
		player.windows.mainWindow.NativeWindow(),
		rect,
	)
	if !shown {
		return false, fmt.Errorf("embedded listen video unavailable")
	}
	return true, nil
}

func (player *ListenYouTubeMusicPlayer) scheduleEmbeddedVideoModeRefresh(window *application.WebviewWindow) {
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
			player.mu.Lock()
			active := player.videoVisible &&
				player.embeddedVisible &&
				player.window != nil &&
				player.window.ID() == windowID &&
				player.embeddedRefreshToken == token
			rect := player.embeddedRect
			volume := player.targetVolume
			muted := player.targetMuted
			player.mu.Unlock()
			if !active {
				return
			}
			if !listenEmbeddedVideoOwnerActive(owner) {
				return
			}
			_, _ = player.showEmbeddedVideoWindow(window, rect)
			execListenYouTubeMusicJS(window, listenYouTubeMusicVideoModeScript(rect))
			execListenYouTubeMusicJS(window, listenYouTubeMusicVolumeScript(volume, muted))
		}
	}()
}

func (player *ListenYouTubeMusicPlayer) registerEmbeddedVideoResizeWaiter(sequence uint64) (<-chan bool, func()) {
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

func (player *ListenYouTubeMusicPlayer) waitForEmbeddedVideoResize(waiter <-chan bool) bool {
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

func (player *ListenYouTubeMusicPlayer) completeEmbeddedVideoResize(sequence uint64, ready bool) bool {
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

func (player *ListenYouTubeMusicPlayer) createWindowLocked(request ListenPlayerPlayRequest) *application.WebviewWindow {
	if player.app == nil {
		return nil
	}

	bridgeScript := listenYouTubeMusicBridgeScript(request)
	window := player.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:        listenPlayerWindowName,
		Title:       "Listen YouTube Music",
		Width:       420,
		Height:      180,
		MinWidth:    320,
		MinHeight:   120,
		URL:         listenYouTubeMusicBlankURL,
		JS:          bridgeScript,
		Hidden:      true,
		AlwaysOnTop: false,
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				FullscreenEnabled: application.Enabled,
			},
		},
	})
	configureListenYouTubeMusicNativeWindow(window.NativeWindow(), listenYouTubeMusicUserAgent())
	player.closeHook = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		player.mu.Lock()
		wasVideoVisible := player.videoVisible
		wasEmbedded := player.embeddedVisible
		if wasVideoVisible {
			player.videoVisible = false
		}
		if wasEmbedded {
			player.embeddedVisible = false
			player.embeddedRect = ListenEmbeddedVideoRect{}
			player.embeddedRefreshToken += 1
		}
		player.mu.Unlock()
		if wasVideoVisible {
			if wasEmbedded {
				listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
				hideListenNativeEmbeddedWebView(window.NativeWindow())
			}
			execListenYouTubeMusicJS(window, listenYouTubeMusicExitVideoModeScript())
			player.dispatch(map[string]any{
				"source": listenPlayerSource,
				"type":   "video-closed",
			})
		}
		window.Hide()
	})
	player.bridgeHook = attachListenYouTubeMusicBridge(window, bridgeScript)
	player.window = window
	return window
}

func (player *ListenYouTubeMusicPlayer) playbackCookies(ctx context.Context) []appcookies.Record {
	if player == nil || player.cookies == nil {
		return nil
	}
	records, err := player.cookies.RecordsForSiteKey(ctx, "youtube")
	if err != nil {
		return nil
	}
	return filterListenPlaybackCookies(appcookies.MatchURL(records, listenYouTubeMusicOrigin+"/"), time.Now())
}

func filterListenPlaybackCookies(records []appcookies.Record, now time.Time) []appcookies.Record {
	if len(records) == 0 {
		return nil
	}
	result := make([]appcookies.Record, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		record.Name = strings.TrimSpace(record.Name)
		record.Domain = strings.TrimSpace(record.Domain)
		record.Path = strings.TrimSpace(record.Path)
		if record.Name == "" || record.Value == "" || record.Domain == "" {
			continue
		}
		if record.Path == "" {
			record.Path = "/"
		}
		if record.Expires > 0 && !time.Unix(record.Expires, 0).After(now) {
			continue
		}
		key := strings.ToLower(record.Name) + "\x00" + strings.ToLower(record.Domain) + "\x00" + record.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, record)
	}
	return result
}

func (player *ListenYouTubeMusicPlayer) dispatch(payload map[string]any) {
	if player == nil || player.windows == nil {
		return
	}
	player.windows.dispatchWindowEvent(listenPlayerEventName, payload)
}

func (player *ListenYouTubeMusicPlayer) dispatchPlaybackState(state string, reason string) {
	if player == nil {
		return
	}
	player.mu.Lock()
	player.currentState = state
	videoID := player.currentVideo
	title := player.requestTitle
	artist := player.requestArtist
	player.mu.Unlock()
	player.dispatch(map[string]any{
		"source":           listenPlayerSource,
		"type":             "state",
		"state":            state,
		"reason":           reason,
		"videoId":          videoID,
		"observedVideoId":  videoID,
		"requestedVideoId": videoID,
		"title":            title,
		"artist":           artist,
	})
}

func listenPayloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func listenPayloadDisplayString(payload map[string]any, key string) string {
	switch value := payload[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ""
		}
		if value == math.Trunc(value) {
			return fmt.Sprintf("%.0f", value)
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
	case float32:
		number := float64(value)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return ""
		}
		if number == math.Trunc(number) {
			return fmt.Sprintf("%.0f", number)
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", number), "0"), ".")
	case int:
		return fmt.Sprintf("%d", value)
	case int64:
		return fmt.Sprintf("%d", value)
	case json.Number:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func listenPayloadBool(payload map[string]any, key string) bool {
	switch value := payload[key].(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	case float64:
		return value != 0
	case int:
		return value != 0
	case int64:
		return value != 0
	case json.Number:
		number, err := value.Float64()
		return err == nil && number != 0
	default:
		return false
	}
}

func listenPayloadBoolValue(payload map[string]any, key string) (bool, bool) {
	if _, exists := payload[key]; !exists {
		return false, false
	}
	return listenPayloadBool(payload, key), true
}

func listenPayloadFloat(payload map[string]any, key string) (float64, bool) {
	switch value := payload[key].(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		return math.Max(0, value), true
	case float32:
		number := float64(value)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return math.Max(0, number), true
	case int:
		if value < 0 {
			return 0, true
		}
		return float64(value), true
	case int64:
		if value < 0 {
			return 0, true
		}
		return float64(value), true
	case json.Number:
		number, err := value.Float64()
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return math.Max(0, number), true
	default:
		return 0, false
	}
}

func listenPayloadUint64(payload map[string]any, key string) (uint64, bool) {
	switch value := payload[key].(type) {
	case float64:
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		return uint64(value), true
	case float32:
		number := float64(value)
		if number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return uint64(number), true
	case int:
		if value <= 0 {
			return 0, false
		}
		return uint64(value), true
	case int64:
		if value <= 0 {
			return 0, false
		}
		return uint64(value), true
	case uint64:
		return value, value > 0
	case json.Number:
		number, err := strconv.ParseUint(strings.TrimSpace(value.String()), 10, 64)
		return number, err == nil && number > 0
	case string:
		number, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		return number, err == nil && number > 0
	default:
		return 0, false
	}
}

func listenEmbeddedVideoResizeRequestJSON(rects ...ListenEmbeddedVideoRect) string {
	if len(rects) == 0 || rects[0].Sequence == 0 {
		return "null"
	}
	rect := normalizeListenEmbeddedVideoRect(rects[0])
	payload, err := json.Marshal(map[string]any{
		"sequence": rect.Sequence,
		"width":    rect.Width,
		"height":   rect.Height,
	})
	if err != nil {
		return "null"
	}
	return string(payload)
}

func isListenStalePlaybackState(state string) bool {
	switch state {
	case "", "idle", "paused", "ended", "error":
		return true
	default:
		return false
	}
}

func normalizeListenPlayerPlayRequest(request ListenPlayerPlayRequest) ListenPlayerPlayRequest {
	request.VideoID = strings.TrimSpace(request.VideoID)
	request.Title = strings.TrimSpace(request.Title)
	request.Artist = strings.TrimSpace(request.Artist)
	request.Language = normalizeListenPlayerLanguage(request.Language)
	request.StartSeconds = clampListenSeconds(request.StartSeconds)
	request.Volume = clampListenVolume(request.Volume)
	request.PlaybackAudioQuality = normalizeListenPlaybackAudioQualityPreference(request.PlaybackAudioQuality)
	return request
}

func normalizeListenPlaybackAudioQualityPreference(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	quality, err := settings.ParsePlaybackAudioQuality(trimmed)
	if err != nil {
		return settings.DefaultPlaybackAudioQuality.String()
	}
	return quality.String()
}

func normalizeListenPlayerLanguage(language string) string {
	return youtubemusic.NormalizeLocale(language)
}

func clampListenVolume(volume float64) float64 {
	if math.IsNaN(volume) || math.IsInf(volume, 0) {
		return 1
	}
	if volume < 0 {
		return 0
	}
	if volume > 1 {
		return 1
	}
	return volume
}

func clampListenSeconds(seconds float64) float64 {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0
	}
	return seconds
}

func listenYouTubeMusicWatchURL(videoID string, language string) string {
	values := url.Values{}
	values.Set("v", strings.TrimSpace(videoID))
	if normalized := normalizeListenPlayerLanguage(language); normalized != "" {
		values.Set("hl", normalized)
		values.Set("persist_hl", "1")
	}
	return listenYouTubeMusicOrigin + "/watch?" + values.Encode()
}

func listenYouTubePosterURL(videoID string) string {
	trimmed := strings.TrimSpace(videoID)
	if !listenYouTubeVideoIDPattern.MatchString(trimmed) {
		return ""
	}
	return "https://i.ytimg.com/vi/" + url.PathEscape(trimmed) + "/hqdefault.jpg"
}

func listenYouTubeMusicBridgeScript(request ListenPlayerPlayRequest) string {
	initial, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  "use strict";
  const SOURCE = %q;
  const INITIAL_REQUEST = %s;
  const REQUEST_STORAGE_KEY = "__listenPlaybackRequest";
  const PLAYBACK_AUDIO_QUALITY_STORAGE_KEY = "xiadownPlaybackAudioQuality";
	  const UPDATE_THROTTLE_MS = 120;
	  const POLL_INTERVAL_MS = 250;
	  const LYRICS_POLL_INTERVAL_MS = 100;
	  const AUTOPLAY_ATTEMPTS = 48;
  const AUTOPLAY_INTERVAL_MS = 500;
  const START_POSITION_TOLERANCE_SECONDS = 1.25;
  const START_POSITION_MAX_ATTEMPTS = 24;
	  let lastUpdateAt = 0;
	  let pollTimer = null;
	  let lyricsPollTimer = null;
	  let autoplayTimer = null;
  let autoplayCount = 0;
  let autoplayRecoveryPending = true;
  let mediaSessionOverrideFrame = null;
  let listenersAttachedTo = new WeakSet();
  let startAppliedForVideo = "";
  let startApplyAttemptKey = "";
  let startApplyAttemptCount = 0;
  let lastRequestedAction = "";
  let lastObservedVideoId = "";
  let lastObservedTitle = "";
  let lastObservedArtist = "";
  let lastEffectiveMediaVideoId = "";
  let lastEffectiveMediaTime = 0;
  let lastEffectiveMediaAdvancedAt = 0;
  let lastStrongAdAt = 0;
  let lastAdvertising = false;
  const AD_FILTER_FALLBACK_STUCK_MS = 12000;
  const AD_FILTER_FALLBACK_DISABLE_MS = 10 * 60 * 1000;
  const AD_FILTER_FALLBACK_DISABLE_KEY = "__xiadownYouTubeAdBlockDisabledUntil";
  let adFallbackObservedAt = 0;
  let adFallbackProgressAt = 0;
  let adFallbackLastTime = 0;
  let adFallbackReloadedFor = "";
  let lastStartSkipLogKey = "";
  let volumeEnforcing = false;
  let volumeApplyFrame = null;
  let volumeBurstTimer = null;
  let audioQualityApplyScheduled = false;
  let lastObservedAudioQualityKey = "";
  let booted = false;

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

  function postDebug(action, details) {
    post({
      type: "debug",
      category: "storage",
      action,
      details: details || {}
    });
  }

  function postObservedPlaybackAudioQuality(observation) {
    try {
      const observed = normalizeObservedPlaybackAudioQuality(observation && observation.observed);
      if (!observed) return;
      const videoId = String((observation && observation.videoId) || "");
      const key = observed + "|" + videoId;
      if (key === lastObservedAudioQualityKey) return;
      lastObservedAudioQualityKey = key;
      post({
        type: "PLAYBACK_AUDIO_QUALITY_OBSERVED",
        observed,
        videoId
      });
    } catch (error) {}
  }

  function readRequest() {
    let stored = null;
    try {
      stored = JSON.parse(window.localStorage.getItem(REQUEST_STORAGE_KEY) || "null");
    } catch (error) {}
    const storedRequest = stored && typeof stored === "object" ? stored : {};
    let request = Object.assign({}, INITIAL_REQUEST, storedRequest);
    const activeVideoId = currentVideoId();
    if (
      activeVideoId &&
      String(INITIAL_REQUEST.videoId || "") === activeVideoId &&
      String(storedRequest.videoId || "") !== activeVideoId
    ) {
      request = Object.assign({}, storedRequest, INITIAL_REQUEST);
    }
    return normalizeRequest(request);
  }

  function normalizeRequest(request) {
    const next = Object.assign({}, request || {});
    next.videoId = String(next.videoId || "");
    next.title = String(next.title || "");
    next.artist = String(next.artist || "");
    next.language = String(next.language || "");
    next.startSeconds = finiteNumber(Number(next.startSeconds || 0), 0);
    next.restartFromStart = next.restartFromStart === true;
    next.forceReload = next.forceReload === true;
    next.volume = Math.max(0, Math.min(1, Number(next.volume ?? 1)));
    next.muted = Boolean(next.muted);
    delete next.playbackAudioQuality;
    return next;
  }

  function persistRequest(request) {
    const normalized = normalizeRequest(request);
    try {
      window.localStorage.setItem(REQUEST_STORAGE_KEY, JSON.stringify(normalized));
    } catch (error) {}
    return normalized;
  }

  function writeRequest(next) {
    const previous = readRequest();
    const incoming = Object.assign({}, next || {});
    if (Object.prototype.hasOwnProperty.call(incoming, "videoId")) {
      if (!Object.prototype.hasOwnProperty.call(incoming, "restartFromStart")) {
        incoming.restartFromStart = false;
      }
      if (!Object.prototype.hasOwnProperty.call(incoming, "startSeconds")) {
        incoming.startSeconds = 0;
      }
    }
    const request = persistRequest(
      Object.prototype.hasOwnProperty.call(incoming, "videoId")
        ? Object.assign({}, INITIAL_REQUEST, { volume: previous.volume, muted: previous.muted }, incoming)
        : Object.assign({}, previous, incoming)
    );
    if (Object.prototype.hasOwnProperty.call(incoming, "videoId")) {
      lastObservedAudioQualityKey = "";
      scheduleVolumeBurst();
      autoplayRecoveryPending = true;
    }
    if (
      Object.prototype.hasOwnProperty.call(incoming, "videoId") &&
      (
        String(incoming.videoId || "") !== String(previous.videoId || "") ||
        request.restartFromStart === true ||
        finiteNumber(Number(request.startSeconds || 0), 0) > 0.5
      )
    ) {
      startAppliedForVideo = "";
      startApplyAttemptKey = "";
      startApplyAttemptCount = 0;
    }
    return request;
  }

  function finiteNumber(value, fallback) {
    return Number.isFinite(value) ? Math.max(0, value) : fallback;
  }

  function playerData() {
    const player = document.querySelector("ytmusic-player");
    if (player && player.playerApi && typeof player.playerApi.getVideoData === "function") {
      const data = player.playerApi.getVideoData();
      if (data && typeof data === "object") return data;
    }
    const moviePlayer = document.getElementById("movie_player");
    if (moviePlayer && typeof moviePlayer.getVideoData === "function") {
      const data = moviePlayer.getVideoData();
      if (data && typeof data === "object") return data;
    }
    return null;
  }

  function playerApi() {
    const player = document.querySelector("ytmusic-player");
    if (player && player.playerApi) return player.playerApi;
    const moviePlayer = document.getElementById("movie_player");
    if (moviePlayer) return moviePlayer;
    return null;
  }

  function normalizePlaybackAudioQualityPreference(value) {
    switch (String(value || "").trim()) {
      case "AUDIO_QUALITY_LOW":
      case "AUDIO_QUALITY_MEDIUM":
      case "AUDIO_QUALITY_HIGH":
      case "AUDIO_QUALITY_AUTO":
        return String(value || "").trim();
      default:
        return "AUDIO_QUALITY_AUTO";
    }
  }

  function writePlaybackAudioQualityPreference(value) {
    const quality = normalizePlaybackAudioQualityPreference(value);
    window.__xiadownPlaybackAudioQuality = quality;
    try {
      window.localStorage.setItem(PLAYBACK_AUDIO_QUALITY_STORAGE_KEY, quality);
    } catch (error) {}
    return quality;
  }

  writePlaybackAudioQualityPreference(INITIAL_REQUEST.playbackAudioQuality);

  function currentPlaybackAudioQualityPreference() {
    if (typeof window.__xiadownPlaybackAudioQuality === "string") {
      return normalizePlaybackAudioQualityPreference(window.__xiadownPlaybackAudioQuality);
    }
    try {
      return normalizePlaybackAudioQualityPreference(window.localStorage.getItem(PLAYBACK_AUDIO_QUALITY_STORAGE_KEY));
    } catch (error) {
      return "AUDIO_QUALITY_AUTO";
    }
  }

  function youtubeAudioQualityValue(quality) {
    switch (normalizePlaybackAudioQualityPreference(quality)) {
      case "AUDIO_QUALITY_LOW":
      case "AUDIO_QUALITY_MEDIUM":
      case "AUDIO_QUALITY_HIGH":
        return normalizePlaybackAudioQualityPreference(quality);
      case "AUDIO_QUALITY_AUTO":
      default:
        return "AUDIO_QUALITY_AUTO";
    }
  }

  function callAudioQualityFunction(target, name, args) {
    try {
      if (target && typeof target[name] === "function") {
        target[name].apply(target, args);
        return true;
      }
    } catch (error) {}
    return false;
  }

  function applyAudioQualityToPlayer(target, quality) {
    const desired = youtubeAudioQualityValue(quality);
    let applied = callAudioQualityFunction(target, "setAudioQuality", [desired]);
    try {
      if (target && typeof target.setOption === "function") {
        [
          ["audio", "quality", desired],
          ["audio", "audioQuality", desired],
          ["player", "audioQuality", desired],
          ["player", "audio_quality", desired],
          ["playback", "audioQuality", desired],
          ["playback", "audio_quality", desired]
        ].forEach((args) => {
          try {
            target.setOption(args[0], args[1], args[2]);
            applied = true;
          } catch (error) {}
        });
      }
    } catch (error) {}
    return applied;
  }

  function candidateAudioQualityPlayers() {
    const players = [];
    const add = (target, source) => {
      if (target) players.push({ target, source });
    };
    try {
      const ytmusicPlayer = document.querySelector("ytmusic-player");
      if (ytmusicPlayer) {
        add(ytmusicPlayer, "ytmusic-player");
        if (ytmusicPlayer.playerApi) add(ytmusicPlayer.playerApi, "ytmusic-player.playerApi");
      }
    } catch (error) {}
    try {
      add(document.getElementById("movie_player"), "movie_player");
    } catch (error) {}
    try {
      if (window.yt && window.yt.player) add(window.yt.player, "window.yt.player");
    } catch (error) {}
    return players;
  }

  function readAudioQualityFunctionValue(target, names) {
    for (let index = 0; index < names.length; index += 1) {
      const name = names[index];
      try {
        if (target && typeof target[name] === "function") {
          const value = target[name]();
          if (value !== null && typeof value !== "undefined") return { name, value };
        }
      } catch (error) {}
    }
    return null;
  }

  function readAudioQualityProperty(target, names) {
    for (let index = 0; index < names.length; index += 1) {
      const name = names[index];
      try {
        if (target && target[name] !== null && typeof target[name] !== "undefined") {
          return { name, value: target[name] };
        }
      } catch (error) {}
    }
    return null;
  }

  function safeAudioQualityPrimitive(value) {
    if (value === null || typeof value === "undefined") return null;
    const type = typeof value;
    if (type === "string") return value.length > 160 ? value.substring(0, 160) : value;
    if (type === "number") return Number.isFinite(value) ? value : null;
    if (type === "boolean") return value;
    return null;
  }

  function videoIdFromAudioQualityPlayers(players) {
    for (let index = 0; index < players.length; index += 1) {
      const entry = players[index];
      const dataResult = readAudioQualityFunctionValue(entry.target, ["getVideoData"]);
      if (!dataResult || !dataResult.value || typeof dataResult.value !== "object") continue;
      const videoId = safeAudioQualityPrimitive(
        dataResult.value.video_id ||
        dataResult.value.videoId ||
        dataResult.value.videoID
      );
      if (videoId) return String(videoId);
    }
    try {
      if (window.location && window.location.href && typeof URL === "function") {
        const url = new URL(window.location.href);
        return String(safeAudioQualityPrimitive(url.searchParams.get("v")) || "");
      }
    } catch (error) {}
    return "";
  }

  function audioQualityFromItag(itag) {
    switch (String(itag)) {
      case "139":
      case "249":
      case "250":
        return "AUDIO_QUALITY_LOW";
      case "140":
      case "251":
        return "AUDIO_QUALITY_MEDIUM";
      case "141":
        return "AUDIO_QUALITY_HIGH";
      default:
        return null;
    }
  }

  function normalizeObservedPlaybackAudioQuality(value) {
    switch (String(value || "").trim()) {
      case "AUDIO_QUALITY_LOW":
        return "AUDIO_QUALITY_LOW";
      case "AUDIO_QUALITY_MEDIUM":
        return "AUDIO_QUALITY_MEDIUM";
      case "AUDIO_QUALITY_HIGH":
        return "AUDIO_QUALITY_HIGH";
      default:
        return "";
    }
  }

  function inferredAudioQualityFromText(value) {
    const text = String(value || "");
    let token = "";
    function observedQualityFromToken(candidate) {
      if (candidate.length < 2 || candidate.length > 3) return null;
      const quality = audioQualityFromItag(candidate);
      if (!quality) return null;
      return { quality, itag: candidate };
    }
    for (let index = 0; index <= text.length; index += 1) {
      const character = index < text.length ? text.charAt(index) : "";
      if (character >= "0" && character <= "9") {
        token += character;
        continue;
      }
      if (token.length > 0) {
        const inferred = observedQualityFromToken(token);
        if (inferred) return inferred;
        token = "";
      }
    }
    return null;
  }

  function inferredAudioQualityFromValue(value) {
    if (value === null || typeof value === "undefined") return null;
    if (Array.isArray(value)) {
      for (let index = 0; index < value.length; index += 1) {
        const inferred = inferredAudioQualityFromValue(value[index]);
        if (inferred) return inferred;
      }
      return null;
    }
    const type = typeof value;
    if (type === "string" || type === "number" || type === "boolean") {
      return inferredAudioQualityFromText(value);
    }
    return null;
  }

  function readAudioQualityStatsProperty(stats, names) {
    if (!stats || typeof stats !== "object") return null;
    const candidates = [];
    names.forEach((name) => {
      candidates.push(name);
      candidates.push("debug_" + name);
    });
    return readAudioQualityProperty(stats, candidates);
  }

  function inferredAudioQualityFromStats(stats) {
    const keys = [
      "itag",
      "audioItag",
      "afmt",
      "audioFormat",
      "audio_format",
      "codec",
      "codecs",
      "audioCodec",
      "audioCodecs"
    ];
    if (!stats || typeof stats !== "object") return null;
    for (let index = 0; index < keys.length; index += 1) {
      const key = keys[index];
      const entry = readAudioQualityStatsProperty(stats, [key]);
      if (!entry) continue;
      const inferred = inferredAudioQualityFromValue(entry.value);
      if (inferred) return inferred;
    }
    return null;
  }

  function observedAudioQualityFromStats(stats) {
    const direct = readAudioQualityStatsProperty(stats, [
      "audioQuality",
      "quality",
      "playbackQuality"
    ]);
    if (direct) {
      const normalized = normalizeObservedPlaybackAudioQuality(direct.value);
      if (normalized) return normalized;
      const inferred = inferredAudioQualityFromValue(direct.value);
      if (inferred) return inferred.quality;
    }
    const inferred = inferredAudioQualityFromStats(stats);
    return inferred ? inferred.quality : "";
  }

  function observedPlaybackAudioQuality(players) {
    const candidates = players || candidateAudioQualityPlayers();
    const observation = {
      observed: "",
      videoId: videoIdFromAudioQualityPlayers(candidates)
    };
    for (let index = 0; index < candidates.length; index += 1) {
      const entry = candidates[index];
      const observed = readAudioQualityFunctionValue(entry.target, [
        "getAudioQuality",
        "getPlaybackAudioQuality",
        "getPreferredAudioQuality"
      ]) || readAudioQualityProperty(entry.target, [
        "audioQuality",
        "playbackAudioQuality",
        "preferredAudioQuality"
      ]);
      if (!observed) continue;
      const normalized = normalizeObservedPlaybackAudioQuality(observed.value);
      if (normalized) {
        observation.observed = normalized;
        return observation;
      }
      const inferred = inferredAudioQualityFromValue(observed.value);
      if (inferred) {
        observation.observed = inferred.quality;
        return observation;
      }
    }
    for (let index = 0; index < candidates.length; index += 1) {
      const entry = candidates[index];
      const statsResult = readAudioQualityFunctionValue(entry.target, ["getStatsForNerds"]);
      const stats = statsResult && statsResult.value && typeof statsResult.value === "object" ? statsResult.value : null;
      const observed = observedAudioQualityFromStats(stats);
      if (!observed) continue;
      observation.observed = observed;
      return observation;
    }
    return observation;
  }

  function applyPlaybackAudioQuality() {
    const quality = currentPlaybackAudioQualityPreference();
    window.__xiadownPlaybackAudioQuality = quality;
    const players = candidateAudioQualityPlayers();
    let applied = false;
    players.forEach((entry) => {
      applied = applyAudioQualityToPlayer(entry.target, quality) || applied;
    });
    postObservedPlaybackAudioQuality(observedPlaybackAudioQuality(players));
    return applied;
  }

  window.__xiadownApplyPlaybackAudioQuality = applyPlaybackAudioQuality;

  function schedulePlaybackAudioQualityApply() {
    if (audioQualityApplyScheduled) return;
    audioQualityApplyScheduled = true;
    const run = () => {
      audioQualityApplyScheduled = false;
      applyPlaybackAudioQuality();
    };
    try {
      if (typeof window.requestAnimationFrame === "function") {
        window.requestAnimationFrame(run);
      } else {
        window.setTimeout(run, 0);
      }
    } catch (error) {
      run();
    }
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

  function adFilteringFallbackDisabled() {
    try {
      const disabledUntil = Number(window.localStorage && window.localStorage.getItem(AD_FILTER_FALLBACK_DISABLE_KEY));
      return Number.isFinite(disabledUntil) && disabledUntil > Date.now();
    } catch (error) {
      return false;
    }
  }

  function resetAdFilteringFallbackWatch() {
    adFallbackObservedAt = 0;
    adFallbackProgressAt = 0;
    adFallbackLastTime = 0;
  }

  function disableAdFilteringForFallback() {
    try {
      if (typeof window.__xiadownDisableYouTubeAdBlock === "function") {
        window.__xiadownDisableYouTubeAdBlock(AD_FILTER_FALLBACK_DISABLE_MS);
        return;
      }
      window.localStorage.setItem(
        AD_FILTER_FALLBACK_DISABLE_KEY,
        String(Date.now() + AD_FILTER_FALLBACK_DISABLE_MS)
      );
    } catch (error) {}
  }

  function maybeReloadWithUnfilteredAds(ad, media, videoId, state) {
    if (!ad || !ad.advertising || adFilteringFallbackDisabled()) {
      resetAdFilteringFallbackWatch();
      return false;
    }
    const now = Date.now();
    const currentTime = finiteNumber(Number(media && media.currentTime || 0), 0);
    if (adFallbackObservedAt <= 0) {
      adFallbackObservedAt = now;
      adFallbackProgressAt = now;
      adFallbackLastTime = currentTime;
      return false;
    }
    if (currentTime > adFallbackLastTime + 0.08) {
      adFallbackProgressAt = now;
      adFallbackLastTime = currentTime;
      return false;
    }
    if (
      now - adFallbackObservedAt < AD_FILTER_FALLBACK_STUCK_MS ||
      now - adFallbackProgressAt < AD_FILTER_FALLBACK_STUCK_MS
    ) {
      return false;
    }
    const request = readRequest();
    const key = String(request.videoId || videoId || currentVideoId() || window.location.href || "");
    if (!key || adFallbackReloadedFor === key) {
      return false;
    }
    adFallbackReloadedFor = key;
    disableAdFilteringForFallback();
    postDebug("ad-filter-fallback-reload", {
      videoId: key,
      currentTime,
      duration: finiteNumber(Number(media && media.duration || 0), 0),
      state: state || ""
    });
    window.setTimeout(() => {
      try { window.location.reload(); } catch (error) {}
    }, 0);
    return true;
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

  function apiNumber(api, names) {
    if (!api) return 0;
    for (const name of names) {
      try {
        if (typeof api[name] === "function") {
          const value = finiteNumber(api[name](), NaN);
          if (Number.isFinite(value) && value >= 0) return value;
        }
        const value = finiteNumber(api[name], NaN);
        if (Number.isFinite(value) && value >= 0) return value;
      } catch (error) {}
    }
    return 0;
  }

  function progressStateSnapshot(api) {
    if (!api || typeof api.getProgressState !== "function") {
      return { currentTime: 0, duration: 0, bufferedTime: 0 };
    }
    try {
      const progress = api.getProgressState();
      if (!progress || typeof progress !== "object") {
        return { currentTime: 0, duration: 0, bufferedTime: 0 };
      }
      return {
        currentTime: finiteNumber(progress.current ?? progress.currentTime ?? progress.elapsed ?? 0, 0),
        duration: finiteNumber(progress.duration ?? progress.total ?? 0, 0),
        bufferedTime: finiteNumber(progress.loaded ?? progress.buffered ?? progress.bufferedTime ?? 0, 0)
      };
    } catch (error) {
      return { currentTime: 0, duration: 0, bufferedTime: 0 };
    }
  }

  function playerApiMediaSnapshot() {
    const api = playerApi();
    const progress = progressStateSnapshot(api);
    const moviePlayer = document.getElementById("movie_player");
    const apiCurrent = Math.max(
      progress.currentTime,
      apiNumber(api, ["getCurrentTime", "currentTime"]),
      apiNumber(moviePlayer, ["getCurrentTime", "currentTime"])
    );
    const apiDuration = Math.max(
      progress.duration,
      apiNumber(api, ["getDuration", "duration"]),
      apiNumber(moviePlayer, ["getDuration", "duration"])
    );
    return {
      currentTime: apiCurrent,
      duration: apiDuration,
      bufferedTime: progress.bufferedTime > 0 && progress.bufferedTime <= 1 && apiDuration > 0
        ? progress.bufferedTime * apiDuration
        : progress.bufferedTime
    };
  }

  function effectiveMediaSnapshot(video, videoId) {
    const apiState = playerStateCode();
    const api = playerApiMediaSnapshot();
    const videoCurrent = video ? finiteNumber(video.currentTime, 0) : 0;
    const videoDuration = video ? finiteNumber(video.duration, 0) : 0;
    const bufferedTime = Math.max(bufferedEnd(video), api.bufferedTime);
    const currentTime = Math.max(videoCurrent, api.currentTime);
    const duration = Math.max(videoDuration, api.duration);
    const now = Date.now();
    const key = videoId || currentVideoId();
    if (key && key !== lastEffectiveMediaVideoId) {
      lastEffectiveMediaVideoId = key;
      lastEffectiveMediaTime = 0;
      lastEffectiveMediaAdvancedAt = 0;
    }
    const previousMediaTime = lastEffectiveMediaTime;
    if (previousMediaTime > 0 && currentTime + 1 < previousMediaTime) {
      lastEffectiveMediaAdvancedAt = 0;
    }
    if (previousMediaTime > 0 && currentTime > previousMediaTime + 0.05) {
      lastEffectiveMediaAdvancedAt = now;
    }
    if (currentTime >= 0) {
      lastEffectiveMediaTime = currentTime;
    }
    const elementPlaying = Boolean(video && !video.paused && !video.ended);
    const apiProgressAdvancing =
      apiState === 1 &&
      currentTime > 0.05 &&
      lastEffectiveMediaAdvancedAt > 0 &&
      now - lastEffectiveMediaAdvancedAt < 1800;
    const playing = elementPlaying || apiProgressAdvancing;
    return {
      apiState,
      currentTime,
      duration,
      bufferedTime,
      playing,
      paused: !playing,
      ended: video ? video.ended : apiState === 0,
      readyState: video ? video.readyState : (duration > 0 || currentTime > 0 ? 4 : 0),
      networkState: video ? video.networkState : 0,
      videoWidth: video ? finiteNumber(video.videoWidth, 0) : 0,
      videoHeight: video ? finiteNumber(video.videoHeight, 0) : 0
    };
  }

  function advertisingMediaSnapshot(video, fallback) {
    if (!video) return fallback;
    return Object.assign({}, fallback || {}, {
      currentTime: finiteNumber(video.currentTime, 0),
      duration: finiteNumber(video.duration, 0),
      bufferedTime: bufferedEnd(video),
      paused: video.paused,
      ended: video.ended,
      readyState: video.readyState,
      networkState: video.networkState,
      videoWidth: finiteNumber(video.videoWidth, 0),
      videoHeight: finiteNumber(video.videoHeight, 0)
    });
  }

  function currentVideoId() {
    const data = playerData();
    const fromAPI = data && (data.video_id || data.videoId);
    if (fromAPI) return String(fromAPI);
    try {
      return new URL(window.location.href).searchParams.get("v") || "";
    } catch (error) {
      return "";
    }
  }

  function currentTitle() {
    const data = playerData();
    if (data && typeof data.title === "string" && data.title.trim()) {
      return data.title.trim();
    }
    const element = document.querySelector(".ytmusic-player-bar.title");
    return element ? (element.textContent || "").trim() : "";
  }

  function currentArtist() {
    const data = playerData();
    if (data && typeof data.author === "string" && data.author.trim()) {
      return data.author.trim();
    }
    const element = document.querySelector(".ytmusic-player-bar.byline");
    return element ? (element.textContent || "").trim() : "";
  }

  function currentThumbnail() {
    const element = document.querySelector(".ytmusic-player-bar .thumbnail img, ytmusic-player-bar .image");
    return element ? (element.src || element.getAttribute("src") || "") : "";
  }

  function currentLikeStatus() {
    const renderer = document.querySelector("ytmusic-like-button-renderer");
    if (!renderer) return "";
    const status = String(renderer.getAttribute("like-status") || "").toUpperCase();
    if (status === "LIKE" || status === "DISLIKE" || status === "INDIFFERENT") return status;
    return "";
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

  function metadataSnapshot() {
    const data = playerData();
    let videoId = "";
    let title = "";
    let artist = "";
    let metadataSource = "";
    if (data && (data.video_id || data.videoId)) {
      videoId = String(data.video_id || data.videoId || "").trim();
    }
    if (data && typeof data.title === "string" && data.title.trim()) {
      title = data.title.trim();
      metadataSource = "api";
    }
    if (data && typeof data.author === "string" && data.author.trim()) {
      artist = data.author.trim();
      metadataSource = metadataSource || "api";
    }
    if (!videoId) {
      try {
        videoId = new URL(window.location.href).searchParams.get("v") || "";
      } catch (error) {}
    }
    if (!title) {
      const element = document.querySelector(".ytmusic-player-bar.title");
      title = element ? (element.textContent || "").trim() : "";
      if (title) metadataSource = metadataSource || "dom";
    }
    if (!artist) {
      const element = document.querySelector(".ytmusic-player-bar.byline");
      artist = element ? (element.textContent || "").trim() : "";
      if (artist) metadataSource = metadataSource || "dom";
    }
    return {
      videoId,
      title,
      artist,
      thumbnailUrl: currentThumbnail(),
      metadataSource,
      likeStatus: currentLikeStatus()
    };
  }

  function videoElement() {
    const videos = videoElements();
    return videos.find((video) => !video.paused && !video.ended) ||
      videos.find((video) => video.readyState > 0 && finiteNumber(video.duration, 0) > 0) ||
      videos[0] ||
      null;
  }

  function videoElements() {
    return Array.from(document.querySelectorAll("video"));
  }

  function pauseVideos() {
    const api = playerApi();
    if (api && typeof api.pauseVideo === "function") {
      try { api.pauseVideo(); } catch (error) {}
    }
    const videos = videoElements();
    videos.forEach((video) => {
      try {
        if (!video.paused) video.pause();
      } catch (error) {}
    });
    return videos.length > 0;
  }

  function anyVideoPlaying() {
    return videoElements().some((video) => !video.paused && !video.ended);
  }

  function otherVideoPlaying(endedVideo) {
    return videoElements().some((video) => video !== endedVideo && !video.paused && !video.ended);
  }

  function bufferedEnd(video) {
    if (!video || !video.buffered || video.buffered.length === 0) return 0;
    try {
      return finiteNumber(video.buffered.end(video.buffered.length - 1), 0);
    } catch (error) {
      return 0;
    }
  }

  function stateFromVideo(video, reason, media) {
    if (lastRequestedAction === "pause") return "paused";
    const apiState = media ? media.apiState : playerStateCode();
    if (video && video.error) return "error";
    if (media && media.ended) return "ended";
    if (media && media.playing) {
      if (
        apiState === 3 ||
        media.readyState < 3 ||
        Boolean(video && video.seeking) ||
        reason === "waiting" ||
        reason === "stalled" ||
        reason === "seeking"
      ) {
        return "buffering";
      }
      return "playing";
    }
    if (!video) return "loading";
    if (lastRequestedAction === "play") {
      return media && (media.currentTime > 0.15 || media.bufferedTime > 0.15) ? "buffering" : "loading";
    }
    if (apiState === 2) return "paused";
    if (apiState === 0 || reason === "ended") return "ended";
    if (reason === "loadstart" || reason === "emptied") return "loading";
    if (video.seeking || reason === "waiting" || reason === "stalled" || reason === "seeking") {
      return "loading";
    }
    return "paused";
  }

  function currentVolumeState() {
    const request = readRequest();
    const muted = Boolean(request.muted);
    const volume = Math.max(0, Math.min(1, Number(request.volume ?? 1)));
    const effectiveVolume = muted ? 0 : volume;
    return {
      muted,
      volume,
      effectiveVolume,
      ytVolume: Math.round(effectiveVolume * 100)
    };
  }

  function applyVolumeToMediaElement(video, state) {
    if (!video) return;
    const next = state || currentVolumeState();
    try {
      if (Math.abs(Number(video.volume || 0) - next.effectiveVolume) > 0.01) {
        video.volume = next.effectiveVolume;
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

  function scheduleVolumeBurst() {
    if (volumeBurstTimer) {
      window.clearInterval(volumeBurstTimer);
      volumeBurstTimer = null;
    }
    let count = 0;
    volumeBurstTimer = window.setInterval(() => {
      applyVolume();
      count += 1;
      if (count >= 15) {
        window.clearInterval(volumeBurstTimer);
        volumeBurstTimer = null;
      }
    }, 200);
  }

  function patchMediaElementVolumeProperty(propertyName) {
    try {
      const proto = window.HTMLMediaElement && window.HTMLMediaElement.prototype;
      if (!proto) return;
      const marker = "__listenNativePatched" + propertyName;
      if (proto[marker]) return;
      const descriptor = Object.getOwnPropertyDescriptor(proto, propertyName);
      if (!descriptor || !descriptor.configurable || typeof descriptor.get !== "function" || typeof descriptor.set !== "function") {
        return;
      }
      Object.defineProperty(proto, marker, { value: true });
      Object.defineProperty(proto, propertyName, {
        configurable: true,
        enumerable: descriptor.enumerable,
        get: function() {
          return descriptor.get.call(this);
        },
        set: function(value) {
          const next = currentVolumeState();
          const enforced = propertyName === "volume" ? next.effectiveVolume : next.muted;
          return descriptor.set.call(this, volumeEnforcing ? value : enforced);
        }
      });
    } catch (error) {}
  }

  function patchMediaElementPlay() {
    try {
      const proto = window.HTMLMediaElement && window.HTMLMediaElement.prototype;
      if (!proto || typeof proto.play !== "function" || proto.play.__listenNativeVolumePatched) return;
      const nativePlay = proto.play;
      const patchedPlay = function(...args) {
        try {
          applyVolumeToMediaElement(this);
          scheduleVolumeBurst();
        } catch (error) {}
        return nativePlay.apply(this, args);
      };
      patchedPlay.__listenNativeVolumePatched = true;
      Object.defineProperty(proto, "play", {
        configurable: true,
        writable: true,
        value: patchedPlay
      });
    } catch (error) {}
  }

  function installVolumeGuards() {
    if (window.__listenNativeVolumeGuardsInstalled) return;
    window.__listenNativeVolumeGuardsInstalled = true;
    patchMediaElementVolumeProperty("volume");
    patchMediaElementVolumeProperty("muted");
    patchMediaElementPlay();
    ["DOMContentLoaded", "readystatechange", "load"].forEach((name) => {
      try { document.addEventListener(name, scheduleVolumeApply, true); } catch (error) {}
    });
    try {
      const root = document.documentElement || document.body;
      if (root && window.MutationObserver) {
        const volumeObserver = new MutationObserver(scheduleVolumeApply);
        volumeObserver.observe(root, { childList: true, subtree: true });
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
      const player = document.querySelector("ytmusic-player");
      if (player && player.playerApi && typeof player.playerApi.setVolume === "function") {
        player.playerApi.setVolume(volumeState.ytVolume);
      }
      const moviePlayer = document.getElementById("movie_player");
      if (moviePlayer && typeof moviePlayer.setVolume === "function") {
        moviePlayer.setVolume(volumeState.ytVolume);
      }
    } finally {
      window.setTimeout(() => {
        volumeEnforcing = false;
      }, 50);
    }
  }

  function applyStartPosition(video) {
    if (!video) return;
    const request = readRequest();
    const activeVideoId = currentVideoId();
    const requestedVideoId = String(request.videoId || "");
    if (activeVideoId && requestedVideoId && activeVideoId !== requestedVideoId) {
      const skipKey = activeVideoId + "::" + requestedVideoId;
      if (skipKey !== lastStartSkipLogKey) {
        lastStartSkipLogKey = skipKey;
        postDebug("skip-start-position-video-mismatch", { activeVideoId, requestedVideoId });
      }
      return;
    }
    const applyKey = activeVideoId || requestedVideoId;
    if (applyKey && startAppliedForVideo === applyKey) return;
    if (request.restartFromStart === true) {
      try {
        if (finiteNumber(video.currentTime, 0) > 0.15) {
          postDebug("apply-restart-from-start", {
            videoId: applyKey,
            currentTime: finiteNumber(video.currentTime, 0)
          });
          video.currentTime = 0;
        }
        if (applyKey) startAppliedForVideo = applyKey;
      } catch (error) {}
      return;
    }
    const start = finiteNumber(Number(request.startSeconds || 0), 0);
    if (start <= 0.5) return;
    const duration = finiteNumber(video.duration, 0);
    if (duration > 0 && start >= duration - 1) return;
    const current = finiteNumber(video.currentTime, 0);
    if (Math.abs(current - start) <= START_POSITION_TOLERANCE_SECONDS || current > start) {
      if (applyKey) startAppliedForVideo = applyKey;
      return;
    }
    if (startApplyAttemptKey !== applyKey) {
      startApplyAttemptKey = applyKey;
      startApplyAttemptCount = 0;
    }
    startApplyAttemptCount += 1;
    if (startApplyAttemptCount > START_POSITION_MAX_ATTEMPTS) {
      postDebug("apply-start-seconds-timeout", {
        videoId: applyKey,
        startSeconds: start,
        currentTime: current,
        duration
      });
      if (applyKey) startAppliedForVideo = applyKey;
      return;
    }
    try {
      postDebug("apply-start-seconds", {
        videoId: applyKey,
        startSeconds: start,
        currentTime: current,
        duration
      });
      video.currentTime = start;
    } catch (error) {}
  }

  function shouldDelayPlaybackForStartPosition(video) {
    if (!video) return false;
    const request = readRequest();
    const start = finiteNumber(Number(request.startSeconds || 0), 0);
    if (start <= 0.5) return false;
    const activeVideoId = currentVideoId();
    const requestedVideoId = String(request.videoId || "");
    if (activeVideoId && requestedVideoId && activeVideoId !== requestedVideoId) {
      return false;
    }
    const applyKey = activeVideoId || requestedVideoId;
    if (applyKey && startAppliedForVideo === applyKey) return false;
    const duration = finiteNumber(video.duration, 0);
    if (duration > 0 && start >= duration - 1) return false;
    const current = finiteNumber(video.currentTime, 0);
    if (Math.abs(current - start) <= START_POSITION_TOLERANCE_SECONDS || current > start) {
      if (applyKey) startAppliedForVideo = applyKey;
      return false;
    }
    return true;
  }

  function installMediaSessionHandlers() {
    try {
      if (!navigator.mediaSession || typeof navigator.mediaSession.setActionHandler !== "function") return;
      installMediaSessionActionHandlerGuard();
      try {
        navigator.mediaSession.setActionHandler("seekforward", null);
      } catch (error) {}
      try {
        navigator.mediaSession.setActionHandler("seekbackward", null);
      } catch (error) {}
      try {
        navigator.mediaSession.setActionHandler("nexttrack", () => post({ type: "remote-next" }));
      } catch (error) {}
      try {
        navigator.mediaSession.setActionHandler("previoustrack", () => post({ type: "remote-previous" }));
      } catch (error) {}
    } catch (error) {}
  }

  function installMediaSessionActionHandlerGuard() {
    try {
      const mediaSession = navigator.mediaSession;
      if (
        !mediaSession ||
        typeof mediaSession.setActionHandler !== "function" ||
        mediaSession.__listenSetActionHandlerWrapped
      ) {
        return;
      }
      const originalSetActionHandler = mediaSession.setActionHandler.bind(mediaSession);
      mediaSession.setActionHandler = function(type, handler) {
        if (type === "seekforward" || type === "seekbackward") {
          return originalSetActionHandler(type, null);
        }
        return originalSetActionHandler(type, handler);
      };
      Object.defineProperty(mediaSession, "__listenSetActionHandlerWrapped", {
        configurable: false,
        enumerable: false,
        value: true
      });
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
    const now = Date.now();
    if (!force && now - lastUpdateAt < UPDATE_THROTTLE_MS) return;
    lastUpdateAt = now;
    installMediaSessionHandlers();
    const video = videoElement();
    const error = errorSnapshot(video);
    const metadata = metadataSnapshot();
    const videoId = metadata.videoId;
    const media = effectiveMediaSnapshot(video, videoId);
    const state = error.errored ? "error" : stateFromVideo(video, reason, media);
    const videoIdChanged = Boolean(videoId && videoId !== lastObservedVideoId);
    const metadataChanged = Boolean(
      (metadata.title && metadata.title !== lastObservedTitle) ||
      (metadata.artist && metadata.artist !== lastObservedArtist)
    );
    const trackChanged = videoIdChanged || metadataChanged;
    if (videoId) lastObservedVideoId = videoId;
    if (metadata.title) lastObservedTitle = metadata.title;
    if (metadata.artist) lastObservedArtist = metadata.artist;
    const ad = adSnapshot();
    const payloadMedia = ad.advertising ? advertisingMediaSnapshot(video, media) : media;
    maybeReloadWithUnfilteredAds(ad, payloadMedia, videoId, state);
    const payload = {
      type: "state",
      state,
      reason: reason || "",
      videoId,
      observedVideoId: videoId,
      title: metadata.title,
      artist: metadata.artist,
      thumbnailUrl: metadata.thumbnailUrl,
      likeStatus: metadata.likeStatus,
      trackChanged,
      metadataSource: metadata.metadataSource,
      currentTime: payloadMedia.currentTime,
      duration: payloadMedia.duration,
      bufferedTime: payloadMedia.bufferedTime,
      paused: payloadMedia.paused,
      ended: payloadMedia.ended,
      videoWidth: payloadMedia.videoWidth,
      videoHeight: payloadMedia.videoHeight,
      advertising: ad.advertising,
      adLabel: ad.label,
      errorCode: error.code,
      errorMessage: error.message,
      readyState: payloadMedia.readyState,
      networkState: payloadMedia.networkState,
      url: window.location.href
    };
    if (error.errored) {
      payload.code = error.code || (video && video.error ? video.error.code || 0 : 0);
      payload.message = error.message;
    }
    post(payload);
  }

  function sendTrackEnded(video, reason) {
    const metadata = metadataSnapshot();
    const videoId = metadata.videoId || lastObservedVideoId || currentVideoId();
    const ad = adSnapshot();
    const error = errorSnapshot(video);
    if (videoId) lastObservedVideoId = videoId;
    if (metadata.title) lastObservedTitle = metadata.title;
    if (metadata.artist) lastObservedArtist = metadata.artist;
    post({
      type: "track-ended",
      state: error.errored ? "error" : "ended",
      reason: reason || "ended",
      videoId,
      observedVideoId: videoId,
      title: metadata.title || lastObservedTitle,
      artist: metadata.artist || lastObservedArtist,
      thumbnailUrl: metadata.thumbnailUrl,
      likeStatus: metadata.likeStatus,
      trackChanged: false,
      metadataSource: metadata.metadataSource,
      currentTime: video ? finiteNumber(video.currentTime, 0) : 0,
      duration: video ? finiteNumber(video.duration, 0) : 0,
      bufferedTime: bufferedEnd(video),
      paused: video ? video.paused : true,
      ended: video ? video.ended : true,
      videoWidth: video ? finiteNumber(video.videoWidth, 0) : 0,
      videoHeight: video ? finiteNumber(video.videoHeight, 0) : 0,
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

	  function sendLyricsTime(reason) {
	    const video = videoElement();
	    if (!video) return;
	    const metadata = metadataSnapshot();
	    const videoId = metadata.videoId || lastObservedVideoId || currentVideoId();
	    post({
	      type: "lyrics-time",
	      reason: reason || "lyrics-poll",
	      videoId,
	      observedVideoId: videoId,
	      currentTime: finiteNumber(video.currentTime, 0),
	      duration: finiteNumber(video.duration, 0)
	    });
	  }

	  function startLyricsPoll() {
	    if (lyricsPollTimer) return;
	    sendLyricsTime("lyrics-poll-start");
	    lyricsPollTimer = window.setInterval(() => sendLyricsTime("lyrics-poll"), LYRICS_POLL_INTERVAL_MS);
	  }

	  function stopLyricsPoll() {
	    if (!lyricsPollTimer) return;
	    window.clearInterval(lyricsPollTimer);
	    lyricsPollTimer = null;
	    sendLyricsTime("lyrics-poll-stop");
	  }

  function cancelAutoplay() {
    if (!autoplayTimer) return;
    window.clearInterval(autoplayTimer);
    autoplayTimer = null;
    autoplayCount = 0;
  }

  function attemptAutoplayRecovery(video, reason) {
    if (!autoplayRecoveryPending || lastRequestedAction === "pause") return "noop";
    if (!video || !video.paused) {
      autoplayRecoveryPending = false;
      return "noop";
    }
    if (video.readyState < 2) return "not-ready";
    if (shouldDelayPlaybackForStartPosition(video)) {
      applyStartPosition(video);
      sendState(reason || "autoplay-waiting-for-start-position", true);
      return "not-ready";
    }
    autoplayRecoveryPending = false;
    lastRequestedAction = "play";
    scheduleVolumeBurst();
    applyVolume();
    applyStartPosition(video);
    const button = document.querySelector(".play-pause-button.ytmusic-player-bar, ytmusic-player-bar .play-pause-button");
    if (button && !isControlDisabled(button)) {
      try {
        button.click();
        sendState(reason || "autoplay-recovery-click", true);
        return "clicked";
      } catch (error) {}
    }
    try {
      const result = video.play();
      if (result && typeof result.catch === "function") {
        result.catch(() => sendState("autoplay-recovery-rejected", true));
      }
    } catch (error) {}
    const player = document.querySelector("ytmusic-player");
    if (player && player.playerApi && typeof player.playerApi.playVideo === "function") {
      try { player.playerApi.playVideo(); } catch (error) {}
    }
    const moviePlayer = document.getElementById("movie_player");
    if (moviePlayer && typeof moviePlayer.playVideo === "function") {
      try { moviePlayer.playVideo(); } catch (error) {}
    }
    sendState(reason || "autoplay-recovery", true);
    return "played";
  }

  function invokePlay(reason) {
    autoplayRecoveryPending = true;
    lastRequestedAction = "play";
    const video = videoElement();
    sendState(reason || "play-requested", true);
    scheduleVolumeBurst();
    applyVolume();
    if (video) {
      applyStartPosition(video);
      if (shouldDelayPlaybackForStartPosition(video)) {
        scheduleAutoplay();
        sendState(reason || "play-waiting-for-start-position", true);
        return;
      }
      const result = video.play();
      if (result && typeof result.catch === "function") {
        result.catch(() => sendState("play-rejected", true));
      }
    }
    const player = document.querySelector("ytmusic-player");
    if (player && player.playerApi && typeof player.playerApi.playVideo === "function") {
      try { player.playerApi.playVideo(); } catch (error) {}
    }
    const moviePlayer = document.getElementById("movie_player");
    if (moviePlayer && typeof moviePlayer.playVideo === "function") {
      try { moviePlayer.playVideo(); } catch (error) {}
    }
    sendState(reason || "play", true);
  }

  function invokePause(reason) {
    lastRequestedAction = "pause";
    autoplayRecoveryPending = false;
    cancelAutoplay();
    sendState(reason || "pause-requested", true);
    pauseVideos();
    [120, 450].forEach((delay) => {
      window.setTimeout(() => {
        if (lastRequestedAction !== "pause") return;
        if (anyVideoPlaying()) pauseVideos();
        if (delay >= 450 && anyVideoPlaying()) {
          lastRequestedAction = "";
          sendState("pause-failed", true);
          return;
        }
        sendState("pause-confirm", true);
      }, delay);
    });
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
          if (name === "loadedmetadata" || name === "loadeddata" || name === "canplay") {
            schedulePlaybackAudioQualityApply();
          }
          applyStartPosition(video);
          if (name === "loadeddata" || name === "canplay" || name === "canplaythrough") {
            attemptAutoplayRecovery(video, "autoplay-recovery-" + name);
          }
          sendState(name, true);
        });
      });
      video.addEventListener("play", () => {
        if (lastRequestedAction === "pause") {
          pauseVideos();
          sendState("play-blocked-after-pause", true);
          return;
        }
        applyVolume();
        startPolling();
        sendState("play", true);
      });
      video.addEventListener("playing", () => {
        if (lastRequestedAction === "pause") {
          pauseVideos();
          stopPolling();
          sendState("playing-blocked-after-pause", true);
          return;
        }
        lastRequestedAction = "";
        autoplayRecoveryPending = false;
        applyVolume();
        schedulePlaybackAudioQualityApply();
        cancelAutoplay();
        startPolling();
        sendState("playing", true);
      });
      video.addEventListener("emptied", () => {
        schedulePlaybackAudioQualityApply();
        sendState("emptied", true);
      });
      video.addEventListener("pause", () => {
        if (video.ended) return;
        if (!video.ended && lastRequestedAction !== "pause") lastRequestedAction = "";
        if (!anyVideoPlaying()) stopPolling();
        sendState("pause", true);
      });
      video.addEventListener("waiting", () => sendState("waiting", true));
      video.addEventListener("stalled", () => sendState("stalled", true));
      video.addEventListener("seeking", () => sendState("seeking", true));
      video.addEventListener("volumechange", () => {
        if (!volumeEnforcing) applyVolume();
      });
      video.addEventListener("seeked", () => {
        applyStartPosition(video);
        sendState("seeked", true);
      });
      video.addEventListener("ended", () => {
        lastRequestedAction = "";
        const ad = adSnapshot();
        if (ad.advertising || wasAdvertisingRecently()) {
          sendState("ad-ended", true);
          startPolling();
          return;
        }
        if (otherVideoPlaying(video)) {
          sendState("stale-video-ended", true);
          return;
        }
        if (!anyVideoPlaying() && pollTimer) {
          window.clearInterval(pollTimer);
          pollTimer = null;
        }
        sendTrackEnded(video, "ended");
      });
      video.addEventListener("error", () => sendState("error", true));
      let volumeBurstCount = 0;
      const volumeBurst = window.setInterval(() => {
        applyVolume();
        volumeBurstCount += 1;
        if (volumeBurstCount >= 15) {
          window.clearInterval(volumeBurst);
        }
      }, 200);
      if (video.readyState >= 3) {
        attemptAutoplayRecovery(video, "autoplay-recovery-ready");
      }
    });
    if (anyVideoPlaying()) startPolling();
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
      const video = videoElement();
      if (video && shouldDelayPlaybackForStartPosition(video)) {
        applyStartPosition(video);
        sendState("autoplay-waiting-for-start-position", true);
        if (autoplayCount >= AUTOPLAY_ATTEMPTS) {
          cancelAutoplay();
          sendState("autoplay-timeout", true);
        }
        return;
      }
      const media = effectiveMediaSnapshot(video, currentVideoId());
      if (media.playing && media.readyState >= 2) {
        cancelAutoplay();
        lastRequestedAction = "";
        startPolling();
        sendState("autoplay-confirmed", true);
        return;
      }
      const recovery = attemptAutoplayRecovery(video, "autoplay-recovery-timer");
      if (recovery === "clicked" || recovery === "played") {
        return;
      }
      invokePlay("autoplay");
      if (autoplayCount >= AUTOPLAY_ATTEMPTS) {
        cancelAutoplay();
        sendState("autoplay-timeout", true);
      }
    }, AUTOPLAY_INTERVAL_MS);
  }

  function boot() {
    if (booted) return;
    booted = true;
    installVolumeGuards();
    try {
      if (!window.localStorage.getItem(REQUEST_STORAGE_KEY)) {
        persistRequest(INITIAL_REQUEST);
      }
    } catch (error) {}
    applyVolume();
    scheduleVolumeBurst();
    applyPlaybackAudioQuality();
    const bootMetadata = metadataSnapshot();
    post({
      type: "ready",
      state: "loading",
      videoId: bootMetadata.videoId,
      observedVideoId: bootMetadata.videoId,
      title: bootMetadata.title,
      artist: bootMetadata.artist,
      thumbnailUrl: bootMetadata.thumbnailUrl,
      likeStatus: bootMetadata.likeStatus,
      metadataSource: bootMetadata.metadataSource,
      url: window.location.href
    });
    attachVideoListeners();
    const playerBar = document.querySelector("ytmusic-player-bar");
    if (playerBar) {
      const observer = new MutationObserver(() => sendState("mutation", false));
      observer.observe(playerBar, { attributes: true, characterData: true, childList: true, subtree: true });
    }
    const bodyObserver = new MutationObserver(() => {
      attachVideoListeners();
      schedulePlaybackAudioQualityApply();
      sendState("dom-mutation", false);
    });
    bodyObserver.observe(document.documentElement || document.body, { childList: true, subtree: true, attributes: true });
    installMediaSessionHandlers();
    scheduleMediaSessionOverrideLoop();
    scheduleAutoplay();
    sendState("boot", true);
  }

  window.__listenNativePlayer = {
    play: () => {
      invokePlay("api-play");
      scheduleAutoplay();
    },
    pause: () => {
      invokePause("api-pause");
    },
    replay: (seconds) => {
      lastRequestedAction = "play";
      const video = videoElement();
      if (video) {
        video.currentTime = finiteNumber(Number(seconds || 0), 0);
      }
      invokePlay("api-replay");
      scheduleAutoplay();
    },
    seek: (seconds) => {
      const video = videoElement();
      if (video) {
        video.currentTime = finiteNumber(Number(seconds || 0), 0);
      }
      sendState("api-seek", true);
    },
    volume: (volume, muted) => {
      writeRequest({ volume, muted });
      applyVolume();
      sendState("api-volume", true);
    },
    next: () => {
      applyVolume();
      scheduleVolumeBurst();
      const button = document.querySelector(".next-button.ytmusic-player-bar");
      if (button && !isControlDisabled(button)) {
        button.click();
        window.setTimeout(applyVolume, 0);
        window.setTimeout(applyVolume, 120);
        sendState("api-next", true);
        return "clicked";
      }
      sendState("api-next-unavailable", true);
      return "no-button";
    },
    previous: () => {
      applyVolume();
      scheduleVolumeBurst();
      const button = document.querySelector(".previous-button.ytmusic-player-bar");
      if (button && !isControlDisabled(button)) {
        button.click();
        window.setTimeout(applyVolume, 0);
        window.setTimeout(applyVolume, 120);
        sendState("api-previous", true);
        return "clicked";
      }
      sendState("api-previous-unavailable", true);
      return "no-button";
    },
	    startLyricsPoll: () => {
	      startLyricsPoll();
	      return "started";
	    },
	    stopLyricsPoll: () => {
	      stopLyricsPoll();
	      return "stopped";
	    },
	    showAirPlayPicker: () => {
      const video = videoElement();
      if (video && typeof video.webkitShowPlaybackTargetPicker === "function") {
        video.webkitShowPlaybackTargetPicker();
        sendState("api-airplay", true);
        return "picker-shown";
      }
      sendState("api-airplay-unsupported", true);
      return "unsupported";
    },
    request: (next) => {
      const incoming = next || {};
      if (Object.prototype.hasOwnProperty.call(incoming, "playbackAudioQuality")) {
        writePlaybackAudioQualityPreference(incoming.playbackAudioQuality);
      }
      const requestPayload = Object.assign({}, incoming);
      delete requestPayload.playbackAudioQuality;
      const request = writeRequest(requestPayload);
      applyVolume();
      schedulePlaybackAudioQualityApply();
      sendState("api-request", true);
      return request;
    },
    snapshot: () => sendState("api-snapshot", true)
  };

  installVolumeGuards();
  if (document.readyState !== "loading") {
    boot();
  } else {
    document.addEventListener("DOMContentLoaded", boot, { once: true });
  }
})();
`, listenPlayerSource, string(initial))
}

func listenYouTubeMusicPrepareLoadScript(request ListenPlayerPlayRequest) string {
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  try {
    const request = %s;
    const api = window.__listenNativePlayer;
    if (api && typeof api.request === "function") api.request(request);
    else {
      if (request.playbackAudioQuality) {
        window.__xiadownPlaybackAudioQuality = request.playbackAudioQuality;
        window.localStorage.setItem("xiadownPlaybackAudioQuality", request.playbackAudioQuality);
      }
      const storedRequest = Object.assign({}, request);
      delete storedRequest.playbackAudioQuality;
      window.localStorage.setItem("__listenPlaybackRequest", JSON.stringify(storedRequest));
    }
    if (api && typeof api.pause === "function") api.pause();
    else document.querySelector("video")?.pause();
  } catch (error) {}
})();
`, string(requestJSON))
}

func listenYouTubeMusicPlaybackAudioQualityScript(value string) string {
	quality := normalizeListenPlaybackAudioQualityPreference(value)
	if quality == "" {
		quality = settings.DefaultPlaybackAudioQuality.String()
	}
	qualityJSON, _ := json.Marshal(quality)
	return fmt.Sprintf(`
(function() {
  try {
    const quality = %s;
    window.__xiadownPlaybackAudioQuality = quality;
    try {
      window.localStorage.setItem("xiadownPlaybackAudioQuality", quality);
    } catch (error) {}
    if (typeof window.__xiadownApplyPlaybackAudioQuality === "function") {
      window.__xiadownApplyPlaybackAudioQuality();
    }
  } catch (error) {}
})();
`, string(qualityJSON))
}

func listenYouTubeMusicPauseScript() string {
	return `
(function() {
  "use strict";
  const api = window.__listenNativePlayer;
  if (api && typeof api.pause === "function") {
    try {
      api.pause();
      return;
    } catch (error) {}
  }
  const videos = Array.from(document.querySelectorAll("video"));
  videos.forEach((video) => {
    try {
      if (!video.paused) video.pause();
    } catch (error) {}
  });
})();
`
}

func listenYouTubeMusicResumeScript() string {
	return `
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.play === "function") {
    api.play();
    return;
  }
  try { document.querySelector("video")?.play(); } catch (error) {}
  const player = document.querySelector("ytmusic-player");
  if (player && player.playerApi && typeof player.playerApi.playVideo === "function") {
    try { player.playerApi.playVideo(); } catch (error) {}
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.playVideo === "function") {
    try { moviePlayer.playVideo(); } catch (error) {}
  }
})();
	`
}

func listenYouTubeMusicStartLyricsPollScript() string {
	return `
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.startLyricsPoll === "function") {
    api.startLyricsPoll();
  }
})();
`
}

func listenYouTubeMusicStopLyricsPollScript() string {
	return `
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.stopLyricsPoll === "function") {
    api.stopLyricsPoll();
  }
})();
`
}

func listenYouTubeMusicNextScript() string {
	return `
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.next === "function") {
    api.next();
    return;
  }
  const button = document.querySelector(".next-button.ytmusic-player-bar");
  if (button && !button.disabled && button.getAttribute("aria-disabled") !== "true") {
    button.click();
  }
})();
`
}

func listenYouTubeMusicPreviousScript() string {
	return `
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.previous === "function") {
    api.previous();
    return;
  }
  const button = document.querySelector(".previous-button.ytmusic-player-bar");
  if (button && !button.disabled && button.getAttribute("aria-disabled") !== "true") {
    button.click();
  }
})();
`
}

func listenYouTubeMusicReplayScript(seconds float64, volume float64, muted bool) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.volume === "function") api.volume(%f, %t);
  if (api && typeof api.replay === "function") {
    api.replay(%f);
    return;
  }
  const video = document.querySelector("video");
  if (video) {
    video.currentTime = %f;
    video.play();
  }
})();
`, clampListenVolume(volume), muted, clampListenSeconds(seconds), clampListenSeconds(seconds))
}

func listenYouTubeMusicSameVideoResumeScript(request ListenPlayerPlayRequest) string {
	request = normalizeListenPlayerPlayRequest(request)
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const request = %s;
  const api = window.__listenNativePlayer;
  if (api && typeof api.request === "function") api.request(request);
  else {
    if (request.playbackAudioQuality) {
      window.__xiadownPlaybackAudioQuality = request.playbackAudioQuality;
      window.localStorage.setItem("xiadownPlaybackAudioQuality", request.playbackAudioQuality);
    }
    const storedRequest = Object.assign({}, request);
    delete storedRequest.playbackAudioQuality;
    window.localStorage.setItem("__listenPlaybackRequest", JSON.stringify(storedRequest));
    if (typeof window.__xiadownApplyPlaybackAudioQuality === "function") {
      window.__xiadownApplyPlaybackAudioQuality();
    }
  }
  if (api && typeof api.volume === "function") api.volume(request.volume, request.muted);
  const video = document.querySelector("video");
  const start = Math.max(0, Number(request.startSeconds || 0));
  if (video && start > 0.5 && (!Number.isFinite(video.currentTime) || video.currentTime < 0.5)) {
    video.currentTime = start;
  }
  if (api && typeof api.play === "function") {
    api.play();
    return;
  }
  if (video) video.play();
})();
`, string(requestJSON))
}

func listenYouTubeMusicSeekScript(seconds float64) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.seek === "function") {
    api.seek(%f);
    return;
  }
  const video = document.querySelector("video");
  if (video) video.currentTime = %f;
})();
`, clampListenSeconds(seconds), clampListenSeconds(seconds))
}

func listenYouTubeMusicVolumeScript(volume float64, muted bool) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.volume === "function") {
    api.volume(%f, %t);
    return;
  }
  try {
    const stored = JSON.parse(window.localStorage.getItem("__listenPlaybackRequest") || "{}");
    stored.volume = %f;
    stored.muted = %t;
    window.localStorage.setItem("__listenPlaybackRequest", JSON.stringify(stored));
  } catch (error) {}
  const effectiveVolume = %t ? 0 : %f;
  const video = document.querySelector("video");
  if (video) {
    video.volume = effectiveVolume;
    video.muted = %t;
  }
  const ytVolume = Math.round(effectiveVolume * 100);
  const player = document.querySelector("ytmusic-player");
  if (player && player.playerApi && typeof player.playerApi.setVolume === "function") player.playerApi.setVolume(ytVolume);
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.setVolume === "function") moviePlayer.setVolume(ytVolume);
})();
`, clampListenVolume(volume), muted, clampListenVolume(volume), muted, muted, clampListenVolume(volume), muted)
}

func listenYouTubeMusicAirPlayScript() string {
	return `
(function() {
  const api = window.__listenNativePlayer;
  if (api && typeof api.showAirPlayPicker === "function") {
    api.showAirPlayPicker();
    return;
  }
  const videos = Array.from(document.querySelectorAll("video"));
  const video = videos.find((item) => !item.paused && !item.ended) || videos[0];
  if (video && typeof video.webkitShowPlaybackTargetPicker === "function") {
    video.webkitShowPlaybackTargetPicker();
  }
})();
`
}

func listenYouTubeMusicVideoModeScript(rects ...ListenEmbeddedVideoRect) string {
	requestJSON := listenEmbeddedVideoResizeRequestJSON(rects...)
	script := `
(function() {
  "use strict";

  const SOURCE = "listen-youtube-music-player";
  const EMBEDDED_RESIZE_REQUEST = __LISTEN_EMBEDDED_RESIZE_REQUEST__;
  const ENFORCE_BURST_MS = 1600;
  const ENFORCE_HEARTBEAT_MS = 1200;
  const VIDEO_MODE_STYLE_TEXT = [
    "html, body, * { visibility: hidden !important; }",
    "html, body { background: #000 !important; overflow: hidden !important; visibility: visible !important; }",
    ".listen-video-visible { visibility: visible !important; display: block !important; opacity: 1 !important; padding: 0 !important; margin: 0 !important; background: #000 !important; z-index: 2147483640 !important; }",
    ".listen-video-visible { position: fixed !important; inset: 0 !important; width: 100vw !important; height: 100vh !important; overflow: visible !important; }",
    "video.listen-video-visible, .video-stream.listen-video-visible { z-index: 2147483647 !important; object-fit: contain !important; }"
  ].join("\n");

  try { window.localStorage.setItem("__listenVideoModeActive", "true"); } catch (error) {}
  window.__listenVideoModeActive = true;
  if (typeof window.__listenVideoModeCleanup === "function") {
    try { window.__listenVideoModeCleanup(); } catch (error) {}
  }

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

  function ensureBlackout() {
    if (document.getElementById("listen-video-blackout")) return;
    const blackout = document.createElement("div");
    blackout.id = "listen-video-blackout";
    blackout.style.cssText = [
      "position:fixed!important",
      "inset:0!important",
      "background:#000!important",
      "z-index:2147483646!important"
    ].join(";");
    document.body.appendChild(blackout);
  }

  function removeBlackout() {
    document.getElementById("listen-video-blackout")?.remove();
  }

  function activateYouTubeMusicVideoMode() {
    const playerPage = document.querySelector("ytmusic-player-page");
    if (playerPage && typeof playerPage.videoMode !== "undefined" && playerPage.videoMode !== true) {
      playerPage.videoMode = true;
      if (typeof playerPage.onVideoModeChanged === "function") {
        playerPage.onVideoModeChanged();
      }
      return true;
    }

    const switcher = document.querySelector("ytmusic-av-switcher");
    const videoButton = switcher?.querySelector("#video-button");
    if (videoButton && !videoButton.hasAttribute("active")) {
      videoButton.click();
      return true;
    }

    const buttons = Array.from(document.querySelectorAll("tp-yt-paper-button, button, [role='button']"));
    const fallback = buttons.find((button) => (button.textContent || button.innerText || "").trim().toLowerCase() === "video");
    if (fallback) {
      const active = fallback.hasAttribute("active") ||
        fallback.classList.contains("active") ||
        fallback.getAttribute("aria-pressed") === "true";
      if (!active) fallback.click();
      return true;
    }

    return false;
  }

  function installVideoStyles() {
    const styleId = "listen-video-mode-style";
    let style = document.getElementById(styleId);
    if (!style) {
      style = document.createElement("style");
      style.id = styleId;
      document.head.appendChild(style);
    }
    if (style.textContent !== VIDEO_MODE_STYLE_TEXT) {
      style.textContent = VIDEO_MODE_STYLE_TEXT;
    }
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

  function markVideoTree() {
    const video = videoElement();
    if (!video) return false;
    document.querySelectorAll(".listen-video-visible, .listen-video-root").forEach((element) => {
      element.classList.remove("listen-video-visible", "listen-video-root");
    });
    let current = video;
    while (current && current !== document.documentElement) {
      current.classList.add("listen-video-visible");
      current = current.parentElement;
    }
    return true;
  }

  function dimensionsMatch(actual, expected) {
    const actualSize = finiteDimension(actual);
    const expectedSize = finiteDimension(expected);
    if (expectedSize <= 1) return actualSize > 1;
    const tolerance = Math.max(6, expectedSize * 0.04);
    return Math.abs(actualSize - expectedSize) <= tolerance;
  }

  function embeddedResizeSnapshot(request) {
    const video = videoElement();
    const videoRect = video ? video.getBoundingClientRect() : null;
    const viewportWidth = finiteDimension(window.innerWidth || document.documentElement.clientWidth || 0);
    const viewportHeight = finiteDimension(window.innerHeight || document.documentElement.clientHeight || 0);
    const videoRectWidth = finiteDimension(videoRect ? videoRect.width : 0);
    const videoRectHeight = finiteDimension(videoRect ? videoRect.height : 0);
    const expectedWidth = finiteDimension(request && request.width);
    const expectedHeight = finiteDimension(request && request.height);
    const viewportMatches =
      dimensionsMatch(viewportWidth, expectedWidth) &&
      dimensionsMatch(viewportHeight, expectedHeight);
    const videoMatches =
      Boolean(video) &&
      dimensionsMatch(videoRectWidth, viewportWidth) &&
      dimensionsMatch(videoRectHeight, viewportHeight);
    return {
      ready: viewportMatches && videoMatches,
      viewportWidth,
      viewportHeight,
      videoRectWidth,
      videoRectHeight,
      expectedWidth,
      expectedHeight,
      hasVideo: Boolean(video)
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
      expectedWidth: snapshot ? snapshot.expectedWidth : 0,
      expectedHeight: snapshot ? snapshot.expectedHeight : 0,
      hasVideo: snapshot ? snapshot.hasVideo === true : false
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
      activateYouTubeMusicVideoMode();
      const marked = markVideoTree();
      if (marked) {
        installVideoStyles();
        removeBlackout();
      }
      const snapshot = embeddedResizeSnapshot(request);
      const signature = [
        Math.round(snapshot.viewportWidth * 2) / 2,
        Math.round(snapshot.viewportHeight * 2) / 2,
        Math.round(snapshot.videoRectWidth * 2) / 2,
        Math.round(snapshot.videoRectHeight * 2) / 2,
        snapshot.hasVideo ? "1" : "0"
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

  function videoModeActive() {
    let active = window.__listenVideoModeActive;
    try {
      active = active && window.localStorage.getItem("__listenVideoModeActive") === "true";
    } catch (error) {}
    return Boolean(active);
  }

  function enforce() {
    if (!videoModeActive()) return false;
    activateYouTubeMusicVideoMode();
    if (markVideoTree()) {
      installVideoStyles();
      removeBlackout();
    }
    return true;
  }

  let enforceFrame = 0;
  let burstUntil = Date.now() + ENFORCE_BURST_MS;
  let heartbeatTimer = 0;
  let mutationTimer = 0;
  let delayedTimers = [];
  let observer = null;

  function scheduleEnforceBurst() {
    burstUntil = Math.max(burstUntil, Date.now() + ENFORCE_BURST_MS);
    scheduleEnforce();
  }

  function scheduleEnforce() {
    if (enforceFrame || !videoModeActive()) return;
    enforceFrame = window.requestAnimationFrame(() => {
      enforceFrame = 0;
      const active = enforce();
      if (active && Date.now() < burstUntil) {
        scheduleEnforce();
      }
    });
  }

  function scheduleDeferredEnforce() {
    if (mutationTimer || !videoModeActive()) return;
    mutationTimer = window.setTimeout(() => {
      mutationTimer = 0;
      scheduleEnforce();
    }, 120);
  }

  function scheduleDelay(callback, delay) {
    const timer = window.setTimeout(() => {
      delayedTimers = delayedTimers.filter((item) => item !== timer);
      callback();
    }, delay);
    delayedTimers.push(timer);
  }

  function installEnforceScheduler() {
    scheduleEnforceBurst();
    heartbeatTimer = window.setInterval(() => {
      if (!videoModeActive()) {
        window.clearInterval(heartbeatTimer);
        heartbeatTimer = 0;
        return;
      }
      scheduleEnforce();
    }, ENFORCE_HEARTBEAT_MS);
    if (window.MutationObserver) {
      try {
        observer = new MutationObserver(scheduleDeferredEnforce);
        observer.observe(document.body || document.documentElement, { childList: true, subtree: true });
      } catch (error) {}
    }
    window.addEventListener("resize", scheduleEnforceBurst);
    window.addEventListener("orientationchange", scheduleEnforceBurst);
    window.__listenVideoModeCleanup = () => {
      if (enforceFrame) {
        window.cancelAnimationFrame(enforceFrame);
        enforceFrame = 0;
      }
      if (heartbeatTimer) {
        window.clearInterval(heartbeatTimer);
        heartbeatTimer = 0;
      }
      if (mutationTimer) {
        window.clearTimeout(mutationTimer);
        mutationTimer = 0;
      }
      delayedTimers.forEach((timer) => window.clearTimeout(timer));
      delayedTimers = [];
      if (observer) {
        observer.disconnect();
        observer = null;
      }
      window.removeEventListener("resize", scheduleEnforceBurst);
      window.removeEventListener("orientationchange", scheduleEnforceBurst);
    };
  }

  function revealIfReady() {
    activateYouTubeMusicVideoMode();
    if (markVideoTree()) {
      installVideoStyles();
      removeBlackout();
      return true;
    }
    return false;
  }

  ensureBlackout();
  activateYouTubeMusicVideoMode();
  try { window.scrollTo(0, 0); } catch (error) {}
  revealIfReady();
  waitForEmbeddedResize(EMBEDDED_RESIZE_REQUEST);
  installEnforceScheduler();
  [80, 180, 360, 720].forEach((delay) => {
    scheduleDelay(() => {
      revealIfReady();
      scheduleEnforceBurst();
    }, delay);
  });
  scheduleDelay(removeBlackout, 2500);
})();
	`
	return strings.Replace(script, "__LISTEN_EMBEDDED_RESIZE_REQUEST__", requestJSON, 1)
}

func listenYouTubeMusicExitVideoModeScript() string {
	return `
(function() {
  window.__listenVideoModeActive = false;
  try { window.localStorage.setItem("__listenVideoModeActive", "false"); } catch (error) {}
  if (typeof window.__listenVideoModeCleanup === "function") {
    try { window.__listenVideoModeCleanup(); } catch (error) {}
    window.__listenVideoModeCleanup = null;
  }
  document.getElementById("listen-video-blackout")?.remove();
  document.getElementById("listen-video-mode-style")?.remove();
  document.querySelectorAll(".listen-video-visible, .listen-video-root").forEach((element) => {
    element.classList.remove("listen-video-visible", "listen-video-root");
  });
  document.body.style.overflow = "";
  document.body.style.background = "";
})();
`
}
