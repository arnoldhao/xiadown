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
	player *ListenYouTubeLivePlayer
}

func NewListenLivePlayerHandler(player *ListenYouTubeLivePlayer) *ListenLivePlayerHandler {
	return &ListenLivePlayerHandler{player: player}
}

func (handler *ListenLivePlayerHandler) ServiceName() string {
	return "ListenLivePlayerHandler"
}

func (handler *ListenLivePlayerHandler) Play(_ context.Context, request ListenPlayerPlayRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Play(request)
}

func (handler *ListenLivePlayerHandler) Pause(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Pause()
}

func (handler *ListenLivePlayerHandler) Resume(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Resume()
}

func (handler *ListenLivePlayerHandler) Replay(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Replay()
}

func (handler *ListenLivePlayerHandler) Seek(_ context.Context, request ListenPlayerSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Seek(request.Seconds)
}

func (handler *ListenLivePlayerHandler) SetVolume(_ context.Context, request ListenPlayerVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.SetVolume(request.Volume, request.Muted)
}

func (handler *ListenLivePlayerHandler) Reset(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Reset()
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

func (handler *ListenLivePlayerHandler) Status(_ context.Context) (ListenPlayerStatus, error) {
	if handler == nil || handler.player == nil {
		return ListenPlayerStatus{}, fmt.Errorf("listen live player unavailable")
	}
	return handler.player.Status(), nil
}

type ListenYouTubeLivePlayer struct {
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
	requestTitle          string
	requestArtist         string
	observedTitle         string
	observedArtist        string
	observedThumb         string
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
}

func NewListenYouTubeLivePlayer(app *application.App, windows *WindowManager, cookies listenPlayerCookieProvider) *ListenYouTubeLivePlayer {
	return &ListenYouTubeLivePlayer{
		app:          app,
		windows:      windows,
		cookies:      cookies,
		currentState: "idle",
		targetVolume: 1,
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
	player.currentTime = 0
	player.duration = 0
	player.bufferedTime = 0
	player.lastPlayAt = time.Now()
	window := player.window
	videoVisible := player.videoVisible
	embeddedVisible := player.embeddedVisible
	embeddedRect := player.embeddedRect
	sameVideo := !request.ForceReload && player.currentVideo == request.VideoID
	createdWindow := window == nil
	if window == nil {
		window = player.createWindowLocked(request)
	}
	player.currentVideo = request.VideoID
	player.mu.Unlock()

	player.dispatch(map[string]any{
		"source":           listenLivePlayerSource,
		"type":             "state",
		"state":            "loading",
		"videoId":          request.VideoID,
		"observedVideoId":  request.VideoID,
		"requestedVideoId": request.VideoID,
		"title":            request.Title,
		"artist":           request.Artist,
	})

	if window == nil {
		return fmt.Errorf("listen live player window unavailable")
	}
	if embeddedVisible {
		listenClaimEmbeddedVideoOwner(window)
		_, _ = player.showEmbeddedVideoWindow(window, embeddedRect)
	}
	if videoVisible {
		if embeddedVisible {
			execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(embeddedRect))
		} else {
			window.Show()
		}
	}
	if createdWindow {
		loadListenYouTubeMusicURL(window, listenYouTubeLiveEmbedURL(request.VideoID, request.Language), cookies)
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
	loadListenYouTubeMusicURL(window, listenYouTubeLiveEmbedURL(request.VideoID, request.Language), cookies)
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
	player.mu.Unlock()

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
		execListenYouTubeMusicJS(window, listenYouTubeLivePauseScript())
		window.SetURL(listenYouTubeMusicBlankURL)
		window.Close()
	}
	player.dispatchPlaybackState("idle", "reset")
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
	player.mu.Unlock()
	if window == nil {
		return false, nil
	}
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	execListenYouTubeMusicJS(window, listenYouTubeLiveExitVideoModeScript())
	window.Hide()
	return true, nil
}

func (player *ListenYouTubeLivePlayer) ShowEmbeddedVideo(rect ListenEmbeddedVideoRect) (bool, error) {
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
	execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(rect))
	execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
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

func (player *ListenYouTubeLivePlayer) HideEmbeddedVideo() error {
	_, err := player.HideEmbeddedVideoForSequence(0)
	return err
}

func (player *ListenYouTubeLivePlayer) HideEmbeddedVideoForSequence(sequence uint64) (bool, error) {
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
	player.embeddedRefreshToken += 1
	window := player.window
	player.mu.Unlock()
	if window == nil {
		return false, nil
	}
	listenReleaseEmbeddedVideoOwner(listenEmbeddedVideoOwnerID(window))
	hideListenNativeEmbeddedWebView(window.NativeWindow())
	window.Hide()
	return true, nil
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
		window.NativeWindow(),
		player.windows.mainWindow.NativeWindow(),
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
			execListenYouTubeMusicJS(window, listenYouTubeLiveVideoModeScript(rect))
			execListenYouTubeMusicJS(window, listenYouTubeLiveVolumeScript(volume, muted))
		}
	}()
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

func (player *ListenYouTubeLivePlayer) Status() ListenPlayerStatus {
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
	return ListenPlayerStatus{
		Available:       player.window != nil,
		VideoID:         player.currentVideo,
		ObservedVideoID: player.currentVideo,
		State:           player.currentState,
		Title:           title,
		Artist:          artist,
		ThumbnailURL:    player.observedThumb,
		Advertising:     player.advertising,
		AdLabel:         player.adLabel,
		ErrorCode:       player.errorCode,
		ErrorMessage:    player.errorMessage,
		CurrentTime:     player.currentTime,
		Duration:        player.duration,
		BufferedTime:    player.bufferedTime,
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
	state := listenPayloadString(payload, "state")
	videoID := listenPayloadString(payload, "observedVideoId")
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

	player.mu.Lock()
	currentVideo := player.currentVideo
	requestTitle := player.requestTitle
	requestArtist := player.requestArtist
	videoVisible := player.videoVisible
	hideAfterActivation := false
	windowToHide := player.window
	if videoID == "" || videoID != currentVideo {
		videoID = currentVideo
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
	if currentTime, ok := listenPayloadFloat(payload, "currentTime"); ok {
		player.currentTime = currentTime
	}
	if duration, ok := listenPayloadFloat(payload, "duration"); ok {
		player.duration = duration
	}
	if bufferedTime, ok := listenPayloadFloat(payload, "bufferedTime"); ok {
		player.bufferedTime = bufferedTime
	}
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
		payload["requestedVideoId"] = videoID
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
	window := player.app.Window.NewWithOptions(application.WebviewWindowOptions{
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
			execListenYouTubeMusicJS(window, listenYouTubeLiveExitVideoModeScript())
			player.dispatch(map[string]any{
				"source": listenLivePlayerSource,
				"type":   "video-closed",
			})
		}
		window.Hide()
	})
	player.bridgeHook = attachListenYouTubeMusicBridge(window, bridgeScript)
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
	})
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

func listenYouTubeClientOrigin() string {
	return "https://" + listenYouTubeClientID
}

func listenYouTubeLiveBridgeScript(request ListenPlayerPlayRequest) string {
	initial, _ := json.Marshal(normalizeListenPlayerPlayRequest(request))
	return fmt.Sprintf(`
(function() {
  "use strict";
  if (window.__listenLiveBridgeInstalled) return;
  window.__listenLiveBridgeInstalled = true;

  const SOURCE = %q;
  const INITIAL_REQUEST = %s;
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
  let lastAdvertising = false;
  let lastStrongAdAt = 0;

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
    if (urlVideoId && urlVideoId === initialVideoId) {
      if (!stored || (storedVideoId && storedVideoId !== initialVideoId)) {
        return Object.assign({}, INITIAL_REQUEST);
      }
      return Object.assign({}, INITIAL_REQUEST, stored, { videoId: initialVideoId });
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

  function applyVolume() {
    const request = readRequest();
    const muted = Boolean(request.muted);
    const volume = Math.max(0, Math.min(1, Number(request.volume ?? 1)));
    const videos = videoElements();
    videos.forEach((video) => {
      try {
        video.volume = volume;
        video.muted = muted;
      } catch (error) {}
    });
    const api = playerApi();
    if (api && typeof api.setVolume === "function") {
      try { api.setVolume(Math.round(volume * 100)); } catch (error) {}
    }
    if (api && typeof api.mute === "function" && typeof api.unMute === "function") {
      try {
        if (muted) api.mute();
        else api.unMute();
      } catch (error) {}
    }
  }

  function metadataSnapshot() {
    const request = readRequest();
    const videoId = String(request.videoId || currentRequestVideoId());
    const title = String(request.title || videoId);
    const artist = String(request.artist || "YouTube Live");
    return {
      videoId,
      title,
      artist,
      thumbnailUrl: videoId ? "https://i.ytimg.com/vi/" + encodeURIComponent(videoId) + "/hqdefault.jpg" : "",
      metadataSource: "request"
    };
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
    const now = Date.now();
    if (!force && now - lastUpdateAt < UPDATE_THROTTLE_MS) return;
    lastUpdateAt = now;
    installMediaSessionHandlers();
    const video = videoElement();
    const error = errorSnapshot(video);
    const state = error.errored ? "error" : stateFromVideo(video, reason);
    const metadata = metadataSnapshot();
    const duration = video ? finiteNumber(video.duration, 0) : 0;
    const currentTime = video ? finiteNumber(video.currentTime, 0) : 0;
    const ad = adSnapshot();
    const payload = {
      type: "state",
      state,
      reason: reason || "",
      videoId: metadata.videoId,
      observedVideoId: metadata.videoId,
      requestedVideoId: metadata.videoId,
      title: metadata.title,
      artist: metadata.artist,
      thumbnailUrl: metadata.thumbnailUrl,
      trackChanged: false,
      metadataSource: metadata.metadataSource,
      currentTime,
      duration,
      bufferedTime: bufferedEnd(video),
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
    const metadata = metadataSnapshot();
    const ad = adSnapshot();
    const error = errorSnapshot(video);
    post({
      type: "track-ended",
      state: error.errored ? "error" : "ended",
      reason: reason || "ended",
      videoId: metadata.videoId,
      observedVideoId: metadata.videoId,
      requestedVideoId: metadata.videoId,
      title: metadata.title,
      artist: metadata.artist,
      thumbnailUrl: metadata.thumbnailUrl,
      trackChanged: false,
      metadataSource: metadata.metadataSource,
      currentTime: video ? finiteNumber(video.currentTime, 0) : 0,
      duration: video ? finiteNumber(video.duration, 0) : 0,
      bufferedTime: bufferedEnd(video),
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

  function boot() {
    try {
      window.localStorage.setItem("__listenLivePlaybackRequest", JSON.stringify(Object.assign({}, INITIAL_REQUEST, readRequest())));
    } catch (error) {}
    applyVolume();
    post(Object.assign({
      type: "ready",
      state: "loading",
      url: window.location.href
    }, metadataSnapshot()));
    attachVideoListeners();
    const bodyObserver = new MutationObserver(() => attachVideoListeners());
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
      lastRequestedAction = "play";
      sendState("api-request", true);
      const api = playerApi();
      if (api && typeof api.loadVideoById === "function" && request.videoId) {
        try { api.loadVideoById(String(request.videoId)); } catch (error) {}
      }
      applyVolume();
      scheduleAutoplay();
    },
    snapshot: () => sendState("api-snapshot", true)
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot, { once: true });
  } else {
    boot();
  }
})();
`, listenLivePlayerSource, string(initial))
}

func listenYouTubeLiveVideoModeScript(rects ...ListenEmbeddedVideoRect) string {
	requestJSON := listenEmbeddedVideoResizeRequestJSON(rects...)
	script := `
(function() {
  "use strict";

  const SOURCE = "listen-youtube-live-player";
  const EMBEDDED_RESIZE_REQUEST = __LISTEN_EMBEDDED_RESIZE_REQUEST__;

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

  function installVideoStyles() {
    const styleId = "listen-live-video-mode-style";
    let style = document.getElementById(styleId);
    if (!style) {
      style = document.createElement("style");
      style.id = styleId;
      document.head.appendChild(style);
    }
    style.textContent = [
      "html, body { width: 100% !important; height: 100% !important; margin: 0 !important; padding: 0 !important; overflow: hidden !important; background: #000 !important; }",
      "#player, #movie_player, .html5-video-player { background: #000 !important; }",
      ".listen-live-video-root { position: fixed !important; inset: 0 !important; width: 100vw !important; height: 100vh !important; min-width: 100vw !important; min-height: 100vh !important; margin: 0 !important; padding: 0 !important; overflow: hidden !important; z-index: 2147483647 !important; }",
      ".listen-live-video-root .html5-video-container, .listen-live-video-root .html5-main-video, .listen-live-video-root video, .listen-live-video-root .video-stream { position: absolute !important; inset: 0 !important; width: 100% !important; height: 100% !important; max-width: none !important; max-height: none !important; opacity: 1 !important; visibility: visible !important; }",
      ".listen-live-video-root video, .listen-live-video-root .video-stream { object-fit: contain !important; background: #000 !important; }",
      ".listen-live-video-root .ytp-gradient-top, .listen-live-video-root .ytp-gradient-bottom, .listen-live-video-root .ytp-chrome-top, .listen-live-video-root .ytp-chrome-bottom, .listen-live-video-root .ytp-pause-overlay, .listen-live-video-root .ytp-cards-teaser, .listen-live-video-root .ytp-ce-element, .listen-live-video-root .ytp-watermark { opacity: 0 !important; pointer-events: none !important; }"
    ].join("\n");
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
    return document.getElementById("movie_player") ||
      video?.closest(".html5-video-player") ||
      video?.parentElement ||
      null;
  }

  function markVideoTree() {
    const video = videoElement();
    const root = rootElement(video);
    if (!video || !root) return false;
    document.querySelectorAll(".listen-live-video-visible, .listen-live-video-root").forEach((element) => {
      element.classList.remove("listen-live-video-visible", "listen-live-video-root");
    });
    root.classList.add("listen-live-video-root");
    let current = root;
    while (current && current !== document.documentElement) {
      current.classList.add("listen-live-video-visible");
      current = current.parentElement;
    }
    video.classList.add("listen-live-video-visible");
    return true;
  }

  function dimensionsMatch(actual, expected) {
    const actualSize = finiteDimension(actual);
    const expectedSize = finiteDimension(expected);
    if (expectedSize <= 1) return actualSize > 1;
    const tolerance = Math.max(10, expectedSize * 0.08);
    return Math.abs(actualSize - expectedSize) <= tolerance;
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
    const hasPlayerSurface = Boolean(root || video);
    return {
      ready: viewportMatches && hasPlayerSurface,
      viewportWidth,
      viewportHeight,
      videoRectWidth,
      videoRectHeight,
      rootRectWidth,
      rootRectHeight,
      expectedWidth,
      expectedHeight,
      hasVideo: Boolean(video),
      hasRoot: Boolean(root)
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
      hasRoot: snapshot ? snapshot.hasRoot === true : false
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
        snapshot.hasRoot ? "1" : "0"
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
	return strings.Replace(script, "__LISTEN_EMBEDDED_RESIZE_REQUEST__", requestJSON, 1)
}

func listenYouTubeLiveExitVideoModeScript() string {
	return `
(function() {
  window.__listenLiveVideoModeActive = false;
  try { window.localStorage.setItem("__listenLiveVideoModeActive", "false"); } catch (error) {}
  document.getElementById("listen-live-video-blackout")?.remove();
  document.getElementById("listen-live-video-mode-style")?.remove();
  document.querySelectorAll(".listen-live-video-visible, .listen-live-video-root").forEach((element) => {
    element.classList.remove("listen-live-video-visible", "listen-live-video-root");
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
    if (api && typeof api.request === "function") api.request(request);
    else window.localStorage.setItem("__listenLivePlaybackRequest", JSON.stringify(request));
    if (api && typeof api.pause === "function") api.pause();
    else document.querySelector("video")?.pause();
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
