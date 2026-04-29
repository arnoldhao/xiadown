package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	dreamFMPlayerWindowName = "dreamfm-youtube-music-player"
	dreamFMPlayerEventName  = "dreamfm:youtube-music-player"
	dreamFMPlayerSource     = "dreamfm-youtube-music-player"

	dreamFMYouTubeMusicOrigin   = "https://music.youtube.com"
	dreamFMYouTubeMusicBlankURL = "about:blank"
)

var dreamFMYouTubeVideoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

type DreamFMPlayerPlayRequest struct {
	VideoID      string  `json:"videoId"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	StartSeconds float64 `json:"startSeconds"`
	Volume       float64 `json:"volume"`
	Muted        bool    `json:"muted"`
}

type DreamFMPlayerVolumeRequest struct {
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted"`
}

type DreamFMPlayerSeekRequest struct {
	Seconds float64 `json:"seconds"`
}

type DreamFMAirPlayAnchor struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type DreamFMPlayerStatus struct {
	Available       bool    `json:"available"`
	VideoID         string  `json:"videoId"`
	ObservedVideoID string  `json:"observedVideoId,omitempty"`
	State           string  `json:"state"`
	Title           string  `json:"title,omitempty"`
	Artist          string  `json:"artist,omitempty"`
	ThumbnailURL    string  `json:"thumbnailUrl,omitempty"`
	LikeStatus      string  `json:"likeStatus,omitempty"`
	VideoAvailable  bool    `json:"videoAvailable"`
	VideoKnown      bool    `json:"videoAvailabilityKnown,omitempty"`
	Advertising     bool    `json:"advertising,omitempty"`
	AdLabel         string  `json:"adLabel,omitempty"`
	AdSkippable     bool    `json:"adSkippable,omitempty"`
	AdSkipLabel     string  `json:"adSkipLabel,omitempty"`
	ErrorCode       string  `json:"errorCode,omitempty"`
	ErrorMessage    string  `json:"errorMessage,omitempty"`
	CurrentTime     float64 `json:"currentTime,omitempty"`
	Duration        float64 `json:"duration,omitempty"`
	BufferedTime    float64 `json:"bufferedTime,omitempty"`
}

type DreamFMPlayerHandler struct {
	player *DreamFMYouTubeMusicPlayer
}

type dreamFMPlayerCookieProvider interface {
	CookiesForConnectorType(ctx context.Context, connectorType connectors.ConnectorType) ([]appcookies.Record, error)
}

func NewDreamFMPlayerHandler(player *DreamFMYouTubeMusicPlayer) *DreamFMPlayerHandler {
	return &DreamFMPlayerHandler{player: player}
}

func (handler *DreamFMPlayerHandler) ServiceName() string {
	return "DreamFMPlayerHandler"
}

func (handler *DreamFMPlayerHandler) Play(_ context.Context, request DreamFMPlayerPlayRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Play(request)
}

func (handler *DreamFMPlayerHandler) Pause(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Pause()
}

func (handler *DreamFMPlayerHandler) Resume(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Resume()
}

func (handler *DreamFMPlayerHandler) Replay(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Replay()
}

func (handler *DreamFMPlayerHandler) Seek(_ context.Context, request DreamFMPlayerSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Seek(request.Seconds)
}

func (handler *DreamFMPlayerHandler) SkipAd(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.SkipAd()
}

func (handler *DreamFMPlayerHandler) SetVolume(_ context.Context, request DreamFMPlayerVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.SetVolume(request.Volume, request.Muted)
}

func (handler *DreamFMPlayerHandler) Reset(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Reset()
}

func (handler *DreamFMPlayerHandler) ShowWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.ShowWindow()
}

func (handler *DreamFMPlayerHandler) HideWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.HideWindow()
}

func (handler *DreamFMPlayerHandler) ShowVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.ShowVideoWindow()
}

func (handler *DreamFMPlayerHandler) HideVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.HideVideoWindow()
}

func (handler *DreamFMPlayerHandler) ShowAirPlayPicker(_ context.Context, anchor DreamFMAirPlayAnchor) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.ShowAirPlayPicker(anchor)
}

func (handler *DreamFMPlayerHandler) Status(_ context.Context) (DreamFMPlayerStatus, error) {
	if handler == nil || handler.player == nil {
		return DreamFMPlayerStatus{}, fmt.Errorf("dreamfm player unavailable")
	}
	return handler.player.Status(), nil
}

type DreamFMYouTubeMusicPlayer struct {
	app     *application.App
	windows *WindowManager
	cookies dreamFMPlayerCookieProvider

	mu             sync.Mutex
	window         *application.WebviewWindow
	closeHook      func()
	bridgeHook     func()
	currentVideo   string
	currentState   string
	activated      bool
	targetVolume   float64
	targetMuted    bool
	requestTitle   string
	requestArtist  string
	observedVideo  string
	observedTitle  string
	observedArtist string
	observedThumb  string
	observedLike   string
	videoAvailable bool
	videoKnown     bool
	advertising    bool
	adLabel        string
	adSkippable    bool
	adSkipLabel    string
	errorCode      string
	errorMessage   string
	currentTime    float64
	duration       float64
	bufferedTime   float64
	lastPlayAt     time.Time
	videoVisible   bool
}

func NewDreamFMYouTubeMusicPlayer(app *application.App, windows *WindowManager, cookies dreamFMPlayerCookieProvider) *DreamFMYouTubeMusicPlayer {
	return &DreamFMYouTubeMusicPlayer{
		app:          app,
		windows:      windows,
		cookies:      cookies,
		currentState: "idle",
		targetVolume: 1,
	}
}

func (player *DreamFMYouTubeMusicPlayer) Play(request DreamFMPlayerPlayRequest) error {
	if player == nil || player.app == nil {
		return fmt.Errorf("dreamfm player unavailable")
	}
	request = normalizeDreamFMPlayerPlayRequest(request)
	if !dreamFMYouTubeVideoIDPattern.MatchString(request.VideoID) {
		return fmt.Errorf("invalid youtube video id")
	}
	cookies := player.playbackCookies(context.Background())

	player.mu.Lock()
	player.targetVolume = request.Volume
	player.targetMuted = request.Muted
	player.currentState = "loading"
	player.requestTitle = request.Title
	player.requestArtist = request.Artist
	player.observedVideo = ""
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.observedLike = ""
	player.videoAvailable = false
	player.videoKnown = false
	player.advertising = false
	player.adLabel = ""
	player.adSkippable = false
	player.adSkipLabel = ""
	player.errorCode = ""
	player.errorMessage = ""
	player.currentTime = 0
	player.duration = 0
	player.bufferedTime = 0
	player.lastPlayAt = time.Now()
	window := player.window
	videoVisible := player.videoVisible
	createdWindow := window == nil
	sameVideo := player.currentVideo == request.VideoID || player.observedVideo == request.VideoID
	if window == nil {
		window = player.createWindowLocked(request)
	}
	player.currentVideo = request.VideoID
	player.mu.Unlock()

	player.dispatch(map[string]any{
		"source":           dreamFMPlayerSource,
		"type":             "state",
		"state":            "loading",
		"videoId":          request.VideoID,
		"observedVideoId":  request.VideoID,
		"requestedVideoId": request.VideoID,
		"title":            request.Title,
		"artist":           request.Artist,
	})

	if window == nil {
		return fmt.Errorf("dreamfm player window unavailable")
	}
	if videoVisible {
		player.scheduleVideoModeRefresh(window)
	}

	if createdWindow {
		loadDreamFMYouTubeMusicURL(window, dreamFMYouTubeMusicWatchURL(request.VideoID), cookies)
	}

	if createdWindow {
		return nil
	}

	if sameVideo {
		execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicSameVideoPlayScript(request))
		return nil
	}

	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicPrepareLoadScript(request))
	loadDreamFMYouTubeMusicURL(window, dreamFMYouTubeMusicWatchURL(request.VideoID), cookies)
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) Pause() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("paused", "pause-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicPauseScript())
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) Resume() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("buffering", "resume-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicResumeScript())
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) Replay() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	volume := player.targetVolume
	muted := player.targetMuted
	player.mu.Unlock()
	player.dispatchPlaybackState("buffering", "replay-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicReplayScript(0, volume, muted))
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) Seek(seconds float64) error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("buffering", "seek-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicSeekScript(seconds))
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) SkipAd() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicSkipAdScript())
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) SetVolume(volume float64, muted bool) error {
	volume = clampDreamFMVolume(volume)

	player.mu.Lock()
	player.targetVolume = volume
	player.targetMuted = muted
	window := player.window
	player.mu.Unlock()

	if window == nil {
		return nil
	}
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicVolumeScript(volume, muted))
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) Reset() error {
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
	player.requestTitle = ""
	player.requestArtist = ""
	player.observedVideo = ""
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.videoAvailable = false
	player.videoKnown = false
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
		execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicPauseScript())
		window.SetURL(dreamFMYouTubeMusicBlankURL)
		window.Close()
	}
	player.dispatchPlaybackState("idle", "reset")
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) ShowWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	window.Show()
	window.Focus()
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) HideWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	window.Hide()
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) ShowVideoWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	player.videoVisible = true
	player.mu.Unlock()
	window.SetTitle("Dream.FM Video")
	window.SetMinSize(320, 180)
	window.SetSize(720, 405)
	window.Show()
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicVideoModeScript())
	player.scheduleVideoModeRefresh(window)
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) HideVideoWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	player.videoVisible = false
	player.mu.Unlock()
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicExitVideoModeScript())
	window.Hide()
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) ShowAirPlayPicker(anchor DreamFMAirPlayAnchor) error {
	if player != nil && player.windows != nil && player.windows.mainWindow != nil {
		if showDreamFMNativeAirPlayPicker(player.windows.mainWindow.NativeWindow(), anchor) {
			return nil
		}
	}
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicAirPlayScript())
	return nil
}

func (player *DreamFMYouTubeMusicPlayer) Status() DreamFMPlayerStatus {
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
	return DreamFMPlayerStatus{
		Available:       player.window != nil,
		VideoID:         player.currentVideo,
		ObservedVideoID: player.observedVideo,
		State:           player.currentState,
		Title:           title,
		Artist:          artist,
		ThumbnailURL:    player.observedThumb,
		LikeStatus:      player.observedLike,
		VideoAvailable:  player.videoAvailable,
		VideoKnown:      player.videoKnown,
		Advertising:     player.advertising,
		AdLabel:         player.adLabel,
		AdSkippable:     player.adSkippable,
		AdSkipLabel:     player.adSkipLabel,
		ErrorCode:       player.errorCode,
		ErrorMessage:    player.errorMessage,
		CurrentTime:     player.currentTime,
		Duration:        player.duration,
		BufferedTime:    player.bufferedTime,
	}
}

func (player *DreamFMYouTubeMusicPlayer) HandleRawMessage(window application.Window, message string, _ *application.OriginInfo) bool {
	if player == nil || window == nil || window.Name() != dreamFMPlayerWindowName {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	if source, _ := payload["source"].(string); source != dreamFMPlayerSource {
		return false
	}

	player.mu.Lock()
	activeWindow := player.window
	player.mu.Unlock()
	if activeWindow == nil || window.ID() != activeWindow.ID() {
		return true
	}

	eventType := dreamFMPayloadString(payload, "type")
	state := dreamFMPayloadString(payload, "state")
	videoID := dreamFMPayloadString(payload, "observedVideoId")
	if videoID == "" {
		videoID = dreamFMPayloadString(payload, "videoId")
	}
	title := dreamFMPayloadString(payload, "title")
	artist := dreamFMPayloadString(payload, "artist")
	thumbnailURL := dreamFMPayloadString(payload, "thumbnailUrl")
	likeStatus := dreamFMPayloadString(payload, "likeStatus")
	videoAvailable, hasVideoAvailable := dreamFMPayloadBoolValue(payload, "videoAvailable")
	videoKnown, hasVideoKnown := dreamFMPayloadBoolValue(payload, "videoAvailabilityKnown")
	videoAvailabilityKnown := videoKnown || (!hasVideoKnown && hasVideoAvailable && videoAvailable)
	advertising := dreamFMPayloadBool(payload, "advertising") || dreamFMPayloadBool(payload, "ad")
	adLabel := dreamFMPayloadString(payload, "adLabel")
	adSkippable := advertising && dreamFMPayloadBool(payload, "adSkippable")
	adSkipLabel := dreamFMPayloadString(payload, "adSkipLabel")
	errorCode := dreamFMPayloadDisplayString(payload, "errorCode")
	if errorCode == "" {
		errorCode = dreamFMPayloadDisplayString(payload, "code")
	}
	errorMessage := dreamFMPayloadString(payload, "errorMessage")
	if errorMessage == "" {
		errorMessage = dreamFMPayloadString(payload, "message")
	}
	player.mu.Lock()
	currentVideo := player.currentVideo
	requestTitle := player.requestTitle
	requestArtist := player.requestArtist
	recentRequestedSwitch := !player.lastPlayAt.IsZero() && time.Since(player.lastPlayAt) < 2*time.Second
	if eventType == "track-ended" &&
		dreamFMYouTubeVideoIDPattern.MatchString(videoID) &&
		currentVideo != "" &&
		videoID != currentVideo {
		player.mu.Unlock()
		return true
	}
	if eventType != "track-ended" &&
		dreamFMYouTubeVideoIDPattern.MatchString(videoID) &&
		currentVideo != "" &&
		videoID != currentVideo &&
		(recentRequestedSwitch || isDreamFMStalePlaybackState(state)) {
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
	if dreamFMYouTubeVideoIDPattern.MatchString(videoID) {
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
	if videoAvailabilityKnown {
		player.videoKnown = true
		player.videoAvailable = videoAvailable
	}
	player.advertising = advertising
	player.adLabel = adLabel
	player.adSkippable = adSkippable
	player.adSkipLabel = adSkipLabel
	if state == "error" {
		player.errorCode = errorCode
		player.errorMessage = errorMessage
	} else if state != "" {
		player.errorCode = ""
		player.errorMessage = ""
	}
	if currentTime, ok := dreamFMPayloadFloat(payload, "currentTime"); ok {
		player.currentTime = currentTime
	}
	if duration, ok := dreamFMPayloadFloat(payload, "duration"); ok {
		player.duration = duration
	}
	if bufferedTime, ok := dreamFMPayloadFloat(payload, "bufferedTime"); ok {
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
	if hasVideoKnown || hasVideoAvailable {
		payload["videoAvailable"] = videoAvailable
		payload["videoAvailabilityKnown"] = videoAvailabilityKnown
	}
	payload["advertising"] = advertising
	if adLabel != "" {
		payload["adLabel"] = adLabel
	}
	payload["adSkippable"] = adSkippable
	if adSkipLabel != "" {
		payload["adSkipLabel"] = adSkipLabel
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

func (player *DreamFMYouTubeMusicPlayer) currentWindow() *application.WebviewWindow {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.window
}

func (player *DreamFMYouTubeMusicPlayer) scheduleVideoModeRefresh(window *application.WebviewWindow) {
	if player == nil || window == nil {
		return
	}
	windowID := window.ID()
	go func() {
		for _, delay := range []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond, 3 * time.Second} {
			time.Sleep(delay)
			player.mu.Lock()
			active := player.videoVisible && player.window != nil && player.window.ID() == windowID
			player.mu.Unlock()
			if !active {
				return
			}
			execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicVideoModeScript())
		}
	}()
}

func (player *DreamFMYouTubeMusicPlayer) createWindowLocked(request DreamFMPlayerPlayRequest) *application.WebviewWindow {
	if player.app == nil {
		return nil
	}

	bridgeScript := dreamFMYouTubeMusicBridgeScript(request)
	window := player.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:        dreamFMPlayerWindowName,
		Title:       "DreamFM YouTube Music",
		Width:       420,
		Height:      180,
		MinWidth:    320,
		MinHeight:   120,
		URL:         dreamFMYouTubeMusicBlankURL,
		JS:          bridgeScript,
		Hidden:      true,
		AlwaysOnTop: false,
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				FullscreenEnabled: application.Enabled,
			},
		},
	})
	configureDreamFMYouTubeMusicNativeWindow(window.NativeWindow(), dreamFMYouTubeMusicUserAgent())
	player.closeHook = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		player.mu.Lock()
		wasVideoVisible := player.videoVisible
		if wasVideoVisible {
			player.videoVisible = false
		}
		player.mu.Unlock()
		if wasVideoVisible {
			execDreamFMYouTubeMusicJS(window, dreamFMYouTubeMusicExitVideoModeScript())
			player.dispatch(map[string]any{
				"source": dreamFMPlayerSource,
				"type":   "video-closed",
			})
		}
		window.Hide()
	})
	player.bridgeHook = attachDreamFMYouTubeMusicBridge(window, bridgeScript)
	player.window = window
	return window
}

func (player *DreamFMYouTubeMusicPlayer) playbackCookies(ctx context.Context) []appcookies.Record {
	if player == nil || player.cookies == nil {
		return nil
	}
	records, err := player.cookies.CookiesForConnectorType(ctx, connectors.ConnectorYouTube)
	if err != nil {
		return nil
	}
	return filterDreamFMPlaybackCookies(appcookies.MatchURL(records, dreamFMYouTubeMusicOrigin+"/"), time.Now())
}

func filterDreamFMPlaybackCookies(records []appcookies.Record, now time.Time) []appcookies.Record {
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

func (player *DreamFMYouTubeMusicPlayer) dispatch(payload map[string]any) {
	if player == nil || player.windows == nil {
		return
	}
	player.windows.dispatchWindowEvent(dreamFMPlayerEventName, payload)
}

func (player *DreamFMYouTubeMusicPlayer) dispatchPlaybackState(state string, reason string) {
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
		"source":           dreamFMPlayerSource,
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

func dreamFMPayloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func dreamFMPayloadDisplayString(payload map[string]any, key string) string {
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

func dreamFMPayloadBool(payload map[string]any, key string) bool {
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

func dreamFMPayloadBoolValue(payload map[string]any, key string) (bool, bool) {
	if _, exists := payload[key]; !exists {
		return false, false
	}
	return dreamFMPayloadBool(payload, key), true
}

func dreamFMPayloadFloat(payload map[string]any, key string) (float64, bool) {
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

func isDreamFMStalePlaybackState(state string) bool {
	switch state {
	case "", "idle", "paused", "ended", "error":
		return true
	default:
		return false
	}
}

func normalizeDreamFMPlayerPlayRequest(request DreamFMPlayerPlayRequest) DreamFMPlayerPlayRequest {
	request.VideoID = strings.TrimSpace(request.VideoID)
	request.Title = strings.TrimSpace(request.Title)
	request.Artist = strings.TrimSpace(request.Artist)
	request.StartSeconds = clampDreamFMSeconds(request.StartSeconds)
	request.Volume = clampDreamFMVolume(request.Volume)
	return request
}

func clampDreamFMVolume(volume float64) float64 {
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

func clampDreamFMSeconds(seconds float64) float64 {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0
	}
	return seconds
}

func dreamFMYouTubeMusicWatchURL(videoID string) string {
	values := url.Values{}
	values.Set("v", strings.TrimSpace(videoID))
	return dreamFMYouTubeMusicOrigin + "/watch?" + values.Encode()
}

func dreamFMYouTubeMusicBridgeScript(request DreamFMPlayerPlayRequest) string {
	initial, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  "use strict";
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
  let listenersAttachedTo = new WeakSet();
  let startAppliedForVideo = "";
  let lastRequestedAction = "";
  let lastObservedVideoId = "";
  let lastObservedTitle = "";
  let lastObservedArtist = "";
  let lastStrongAdAt = 0;
  let lastAdvertising = false;

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
      stored = JSON.parse(window.localStorage.getItem("__dreamfmPlaybackRequest") || "null");
    } catch (error) {}
    return Object.assign({}, INITIAL_REQUEST, stored || {});
  }

  function writeRequest(next) {
    const request = Object.assign({}, readRequest(), next || {});
    try {
      window.localStorage.setItem("__dreamfmPlaybackRequest", JSON.stringify(request));
    } catch (error) {}
    if (next && Object.prototype.hasOwnProperty.call(next, "videoId")) {
      startAppliedForVideo = "";
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

  function visibleAdElements() {
    const adSelector = [
      ".ytp-ad-preview-container",
      ".ytp-ad-text",
      ".ytp-ad-preview-text",
      ".ytp-ad-simple-ad-badge",
      ".ytp-ad-duration-remaining",
      ".ytp-ad-skip-button",
      ".ytp-ad-skip-button-modern",
      ".ytp-ad-skip-button-container button"
    ].join(",");
    return Array.from(document.querySelectorAll(adSelector)).filter(isElementVisible);
  }

  function isSkipElement(element) {
    return Boolean(
      element &&
      (element.matches?.(".ytp-ad-skip-button-modern, .ytp-ad-skip-button, .ytp-ad-skip-button-container button") ||
        element.closest?.(".ytp-ad-skip-button-modern, .ytp-ad-skip-button, .ytp-ad-skip-button-container"))
    );
  }

  function skipButton() {
    const buttons = Array.from(document.querySelectorAll([
      ".ytp-ad-skip-button-modern",
      ".ytp-ad-skip-button",
      ".ytp-ad-skip-button-container button",
      "button.ytp-ad-skip-button-modern",
      "button.ytp-ad-skip-button"
    ].join(",")));
    return buttons.find((button) => {
      if (!isElementVisible(button)) return false;
      if (button.disabled || button.getAttribute("aria-disabled") === "true") return false;
      return true;
    }) || null;
  }

  function normalizeAdLabel(value) {
    return String(value || "").replace(/\s+/g, " ").trim().slice(0, 48);
  }

  function adElementText(element) {
    return normalizeAdLabel(element ? (element.textContent || element.innerText || "") : "");
  }

  function isMeaningfulAdElement(element) {
    if (!element) return false;
    if (isSkipElement(element)) return true;
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
      if (isSkipElement(element)) continue;
      const text = adElementText(element);
      if (text && !/skip|跳过|略過|スキップ|건너뛰기/i.test(text)) return text;
    }
    return "";
  }

  function adSnapshot() {
    const now = Date.now();
    const hasClass = hasActiveAdPlayerClass();
    const elements = visibleAdElements().filter(isMeaningfulAdElement);
    const skip = skipButton();
    const hasStrongSignal = elements.length > 0 || Boolean(skip);
    if (hasStrongSignal) {
      lastStrongAdAt = now;
    }
    const advertising = hasStrongSignal || (hasClass && lastStrongAdAt > 0 && now - lastStrongAdAt < 1500);
    const label = advertising ? adLabelFromElements(elements) : "";
    const activeSkip = advertising ? skip : null;
    const skipLabel = normalizeAdLabel(activeSkip ? (activeSkip.getAttribute("aria-label") || activeSkip.textContent || activeSkip.innerText || "") : "");
    lastAdvertising = advertising;
    return { advertising, label, skippable: Boolean(activeSkip), skipLabel };
  }

  function invokeSkipAd(reason) {
    const button = skipButton();
    if (!button) {
      sendState(reason || "skip-ad-unavailable", true);
      return false;
    }
    try {
      button.click();
    } catch (error) {
      try {
        button.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, view: window }));
      } catch (ignored) {}
    }
    sendState(reason || "skip-ad", true);
    return true;
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

  function videoAvailabilitySnapshot() {
    const switcher = document.querySelector("ytmusic-av-switcher");
    const videoButton = switcher?.querySelector("#video-button") ||
      document.querySelector("ytmusic-av-switcher #video-button");
    if (videoButton && isElementVisible(videoButton) && !isControlDisabled(videoButton)) {
      return { known: true, available: true };
    }
    const buttons = Array.from(document.querySelectorAll("tp-yt-paper-button, button, [role='button']"));
    const hasVideoToggle = buttons.some((button) => {
      const text = (button.textContent || button.innerText || "").trim().toLowerCase();
      return (text === "video" || text === "song") && isElementVisible(button) && !isControlDisabled(button);
    });
    if (hasVideoToggle) return { known: true, available: true };
    return { known: false, available: false };
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
    if (playerStateCode() === 1) return true;
    return videoElements().some((video) => !video.paused && !video.ended);
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
    if (apiState === 2) return "paused";
    if (apiState === 0 || reason === "ended") return "ended";
    if (!video) return "loading";
    if (video.error) return "error";
    if (video.ended) return "ended";
    if (reason === "loadstart" || reason === "emptied") return "loading";
    if (video.seeking || reason === "waiting" || reason === "stalled" || reason === "seeking") {
      return "buffering";
    }
    if (!video.paused) {
      return video.readyState < 3 ? "buffering" : "playing";
    }
    if (lastRequestedAction === "play" && video.readyState < 3) return "loading";
    return "paused";
  }

  function applyVolume() {
    const request = readRequest();
    const muted = Boolean(request.muted);
    const volume = Math.max(0, Math.min(1, Number(request.volume ?? 1)));
    const effectiveVolume = muted ? 0 : volume;
    const videos = videoElements();
    videos.forEach((video) => {
      video.volume = effectiveVolume;
      video.muted = muted;
    });
    const ytVolume = Math.round(effectiveVolume * 100);
    const player = document.querySelector("ytmusic-player");
    if (player && player.playerApi && typeof player.playerApi.setVolume === "function") {
      player.playerApi.setVolume(ytVolume);
    }
    const moviePlayer = document.getElementById("movie_player");
    if (moviePlayer && typeof moviePlayer.setVolume === "function") {
      moviePlayer.setVolume(ytVolume);
    }
  }

  function applyStartPosition(video) {
    if (!video) return;
    const request = readRequest();
    const videoId = currentVideoId();
    const start = finiteNumber(Number(request.startSeconds || 0), 0);
    if (start <= 0.5 || startAppliedForVideo === videoId) return;
    const duration = finiteNumber(video.duration, 0);
    if (duration > 0 && start >= duration - 1) return;
    try {
      video.currentTime = start;
      startAppliedForVideo = videoId;
    } catch (error) {}
  }

  function sendState(reason, force) {
    const now = Date.now();
    if (!force && now - lastUpdateAt < UPDATE_THROTTLE_MS) return;
    lastUpdateAt = now;
    const video = videoElement();
    const error = errorSnapshot(video);
    const state = error.errored ? "error" : stateFromVideo(video, reason);
    const metadata = metadataSnapshot();
    const videoId = metadata.videoId;
    const videoIdChanged = Boolean(videoId && videoId !== lastObservedVideoId);
    const metadataChanged = Boolean(
      (metadata.title && metadata.title !== lastObservedTitle) ||
      (metadata.artist && metadata.artist !== lastObservedArtist)
    );
    const trackChanged = videoIdChanged || metadataChanged;
    if (videoId) lastObservedVideoId = videoId;
    if (metadata.title) lastObservedTitle = metadata.title;
    if (metadata.artist) lastObservedArtist = metadata.artist;
    const duration = video ? finiteNumber(video.duration, 0) : 0;
    const currentTime = video ? finiteNumber(video.currentTime, 0) : 0;
    const ad = adSnapshot();
    const videoAvailability = videoAvailabilitySnapshot();
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
      videoAvailable: videoAvailability.available,
      videoAvailabilityKnown: videoAvailability.known,
      trackChanged,
      metadataSource: metadata.metadataSource,
      currentTime,
      duration,
      bufferedTime: bufferedEnd(video),
      advertising: ad.advertising,
      adLabel: ad.label,
      adSkippable: ad.skippable,
      adSkipLabel: ad.skipLabel,
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
    const videoId = metadata.videoId || lastObservedVideoId || currentVideoId();
    const ad = adSnapshot();
    const error = errorSnapshot(video);
    const videoAvailability = videoAvailabilitySnapshot();
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
      videoAvailable: videoAvailability.available,
      videoAvailabilityKnown: videoAvailability.known,
      trackChanged: false,
      metadataSource: metadata.metadataSource,
      currentTime: video ? finiteNumber(video.currentTime, 0) : 0,
      duration: video ? finiteNumber(video.duration, 0) : 0,
      bufferedTime: bufferedEnd(video),
      advertising: ad.advertising,
      adLabel: ad.label,
      adSkippable: ad.skippable,
      adSkipLabel: ad.skipLabel,
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
    const video = videoElement();
    sendState(reason || "play-requested", true);
    applyVolume();
    if (video) {
      applyStartPosition(video);
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
          applyStartPosition(video);
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
        applyVolume();
        cancelAutoplay();
        startPolling();
        sendState("playing", true);
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
      video.addEventListener("seeked", () => sendState("seeked", true));
      video.addEventListener("ended", () => {
        lastRequestedAction = "";
        const ad = adSnapshot();
        if (ad.advertising || wasAdvertisingRecently()) {
          sendState("ad-ended", true);
          startPolling();
          return;
        }
        if (!anyVideoPlaying() && pollTimer) {
          window.clearInterval(pollTimer);
          pollTimer = null;
        }
        sendTrackEnded(video, "ended");
      });
      video.addEventListener("error", () => sendState("error", true));
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
      if (playerStateCode() === 1 || (video && !video.paused && video.readyState >= 2)) {
        cancelAutoplay();
        lastRequestedAction = "";
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
      if (!window.localStorage.getItem("__dreamfmPlaybackRequest")) {
        window.localStorage.setItem("__dreamfmPlaybackRequest", JSON.stringify(INITIAL_REQUEST));
      }
    } catch (error) {}
    const bootMetadata = metadataSnapshot();
    const bootVideoAvailability = videoAvailabilitySnapshot();
    post({
      type: "ready",
      state: "loading",
      videoId: bootMetadata.videoId,
      observedVideoId: bootMetadata.videoId,
      title: bootMetadata.title,
      artist: bootMetadata.artist,
      thumbnailUrl: bootMetadata.thumbnailUrl,
      likeStatus: bootMetadata.likeStatus,
      videoAvailable: bootVideoAvailability.available,
      videoAvailabilityKnown: bootVideoAvailability.known,
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
      sendState("dom-mutation", false);
    });
    bodyObserver.observe(document.documentElement || document.body, { childList: true, subtree: true, attributes: true });
    scheduleAutoplay();
    sendState("boot", true);
  }

  window.__dreamfmNativePlayer = {
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
    skipAd: () => invokeSkipAd("api-skip-ad"),
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
    request: writeRequest,
    snapshot: () => sendState("api-snapshot", true)
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot, { once: true });
  } else {
    boot();
  }
})();
`, dreamFMPlayerSource, string(initial))
}

func dreamFMYouTubeMusicPrepareLoadScript(request DreamFMPlayerPlayRequest) string {
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  try {
    const request = %s;
    const api = window.__dreamfmNativePlayer;
    if (api && typeof api.request === "function") api.request(request);
    else window.localStorage.setItem("__dreamfmPlaybackRequest", JSON.stringify(request));
    if (api && typeof api.pause === "function") api.pause();
    else document.querySelector("video")?.pause();
  } catch (error) {}
})();
`, string(requestJSON))
}

func dreamFMYouTubeMusicPauseScript() string {
	return `
(function() {
  "use strict";
  const api = window.__dreamfmNativePlayer;
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

func dreamFMYouTubeMusicResumeScript() string {
	return `
(function() {
  const api = window.__dreamfmNativePlayer;
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

func dreamFMYouTubeMusicReplayScript(seconds float64, volume float64, muted bool) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__dreamfmNativePlayer;
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
`, clampDreamFMVolume(volume), muted, clampDreamFMSeconds(seconds), clampDreamFMSeconds(seconds))
}

func dreamFMYouTubeMusicSameVideoPlayScript(request DreamFMPlayerPlayRequest) string {
	request = normalizeDreamFMPlayerPlayRequest(request)
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const request = %s;
  const api = window.__dreamfmNativePlayer;
  if (api && typeof api.request === "function") {
    api.request(request);
  } else {
    try { window.localStorage.setItem("__dreamfmPlaybackRequest", JSON.stringify(request)); } catch (error) {}
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

func dreamFMYouTubeMusicSeekScript(seconds float64) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__dreamfmNativePlayer;
  if (api && typeof api.seek === "function") {
    api.seek(%f);
    return;
  }
  const video = document.querySelector("video");
  if (video) video.currentTime = %f;
})();
`, clampDreamFMSeconds(seconds), clampDreamFMSeconds(seconds))
}

func dreamFMYouTubeMusicSkipAdScript() string {
	return `
(function() {
  const api = window.__dreamfmNativePlayer;
  if (api && typeof api.skipAd === "function") {
    api.skipAd();
    return;
  }
  const buttons = Array.from(document.querySelectorAll([
    ".ytp-ad-skip-button-modern",
    ".ytp-ad-skip-button",
    ".ytp-ad-skip-button-container button",
    "button.ytp-ad-skip-button-modern",
    "button.ytp-ad-skip-button"
  ].join(",")));
  const button = buttons.find((element) => {
    const rect = element.getBoundingClientRect();
    const style = window.getComputedStyle(element);
    return rect && rect.width > 1 && rect.height > 1 &&
      style.display !== "none" &&
      style.visibility !== "hidden" &&
      !element.disabled &&
      element.getAttribute("aria-disabled") !== "true";
  });
  if (button) {
    try { button.click(); } catch (error) {}
  }
})();
`
}

func dreamFMYouTubeMusicVolumeScript(volume float64, muted bool) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__dreamfmNativePlayer;
  if (api && typeof api.volume === "function") {
    api.volume(%f, %t);
    return;
  }
  try {
    const stored = JSON.parse(window.localStorage.getItem("__dreamfmPlaybackRequest") || "{}");
    stored.volume = %f;
    stored.muted = %t;
    window.localStorage.setItem("__dreamfmPlaybackRequest", JSON.stringify(stored));
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
`, clampDreamFMVolume(volume), muted, clampDreamFMVolume(volume), muted, muted, clampDreamFMVolume(volume), muted)
}

func dreamFMYouTubeMusicAirPlayScript() string {
	return `
(function() {
  const api = window.__dreamfmNativePlayer;
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

func dreamFMYouTubeMusicVideoModeScript() string {
	return `
(function() {
  "use strict";

  try { window.localStorage.setItem("__dreamfmVideoModeActive", "true"); } catch (error) {}
  window.__dreamfmVideoModeActive = true;

  function ensureBlackout() {
    if (document.getElementById("dreamfm-video-blackout")) return;
    const blackout = document.createElement("div");
    blackout.id = "dreamfm-video-blackout";
    blackout.style.cssText = [
      "position:fixed!important",
      "inset:0!important",
      "background:#000!important",
      "z-index:2147483646!important"
    ].join(";");
    document.body.appendChild(blackout);
  }

  function removeBlackout() {
    document.getElementById("dreamfm-video-blackout")?.remove();
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
    const styleId = "dreamfm-video-mode-style";
    let style = document.getElementById(styleId);
    if (!style) {
      style = document.createElement("style");
      style.id = styleId;
      document.head.appendChild(style);
    }
    style.textContent = [
      "html, body, * { visibility: hidden !important; }",
      "html, body { background: #000 !important; overflow: hidden !important; visibility: visible !important; }",
      ".dreamfm-video-visible { visibility: visible !important; display: block !important; opacity: 1 !important; padding: 0 !important; margin: 0 !important; background: #000 !important; z-index: 2147483640 !important; }",
      ".dreamfm-video-visible { position: fixed !important; inset: 0 !important; width: 100vw !important; height: 100vh !important; overflow: visible !important; }",
      "video.dreamfm-video-visible, .video-stream.dreamfm-video-visible { z-index: 2147483647 !important; object-fit: contain !important; }"
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

  function markVideoTree() {
    const video = videoElement();
    if (!video) return false;
    document.querySelectorAll(".dreamfm-video-visible, .dreamfm-video-root").forEach((element) => {
      element.classList.remove("dreamfm-video-visible", "dreamfm-video-root");
    });
    let current = video;
    while (current && current !== document.documentElement) {
      current.classList.add("dreamfm-video-visible");
      current = current.parentElement;
    }
    return true;
  }

  function enforce() {
    let active = window.__dreamfmVideoModeActive;
    try {
      active = active && window.localStorage.getItem("__dreamfmVideoModeActive") === "true";
    } catch (error) {}
    if (!active) return;
    activateYouTubeMusicVideoMode();
    if (markVideoTree()) {
      installVideoStyles();
      removeBlackout();
    }
    window.requestAnimationFrame(enforce);
  }

  ensureBlackout();
  activateYouTubeMusicVideoMode();
  try { window.scrollTo(0, 0); } catch (error) {}
  window.setTimeout(() => {
    if (markVideoTree()) {
      installVideoStyles();
      removeBlackout();
    }
    window.requestAnimationFrame(enforce);
  }, 500);
  window.setTimeout(removeBlackout, 2500);
})();
`
}

func dreamFMYouTubeMusicExitVideoModeScript() string {
	return `
(function() {
  window.__dreamfmVideoModeActive = false;
  try { window.localStorage.setItem("__dreamfmVideoModeActive", "false"); } catch (error) {}
  document.getElementById("dreamfm-video-blackout")?.remove();
  document.getElementById("dreamfm-video-mode-style")?.remove();
  document.querySelectorAll(".dreamfm-video-visible, .dreamfm-video-root").forEach((element) => {
    element.classList.remove("dreamfm-video-visible", "dreamfm-video-root");
  });
  document.body.style.overflow = "";
  document.body.style.background = "";
})();
`
}
