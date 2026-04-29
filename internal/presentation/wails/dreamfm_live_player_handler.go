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
	"xiadown/internal/domain/connectors"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	dreamFMLivePlayerWindowName = "dreamfm-youtube-live-player"
	dreamFMLivePlayerEventName  = "dreamfm:youtube-live-player"
	dreamFMLivePlayerSource     = "dreamfm-youtube-live-player"

	dreamFMYouTubeOrigin   = "https://www.youtube.com"
	dreamFMYouTubeClientID = "com.dreamapp.xiadown"
)

type DreamFMLivePlayerHandler struct {
	player *DreamFMYouTubeLivePlayer
}

func NewDreamFMLivePlayerHandler(player *DreamFMYouTubeLivePlayer) *DreamFMLivePlayerHandler {
	return &DreamFMLivePlayerHandler{player: player}
}

func (handler *DreamFMLivePlayerHandler) ServiceName() string {
	return "DreamFMLivePlayerHandler"
}

func (handler *DreamFMLivePlayerHandler) Play(_ context.Context, request DreamFMPlayerPlayRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Play(request)
}

func (handler *DreamFMLivePlayerHandler) Pause(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Pause()
}

func (handler *DreamFMLivePlayerHandler) Resume(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Resume()
}

func (handler *DreamFMLivePlayerHandler) Replay(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Replay()
}

func (handler *DreamFMLivePlayerHandler) Seek(_ context.Context, request DreamFMPlayerSeekRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Seek(request.Seconds)
}

func (handler *DreamFMLivePlayerHandler) SkipAd(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.SkipAd()
}

func (handler *DreamFMLivePlayerHandler) SetVolume(_ context.Context, request DreamFMPlayerVolumeRequest) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.SetVolume(request.Volume, request.Muted)
}

func (handler *DreamFMLivePlayerHandler) Reset(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Reset()
}

func (handler *DreamFMLivePlayerHandler) ShowWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.ShowWindow()
}

func (handler *DreamFMLivePlayerHandler) HideWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.HideWindow()
}

func (handler *DreamFMLivePlayerHandler) ShowVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.ShowVideoWindow()
}

func (handler *DreamFMLivePlayerHandler) HideVideoWindow(_ context.Context) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.HideVideoWindow()
}

func (handler *DreamFMLivePlayerHandler) ShowAirPlayPicker(_ context.Context, anchor DreamFMAirPlayAnchor) error {
	if handler == nil || handler.player == nil {
		return fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.ShowAirPlayPicker(anchor)
}

func (handler *DreamFMLivePlayerHandler) Status(_ context.Context) (DreamFMPlayerStatus, error) {
	if handler == nil || handler.player == nil {
		return DreamFMPlayerStatus{}, fmt.Errorf("dreamfm live player unavailable")
	}
	return handler.player.Status(), nil
}

type DreamFMYouTubeLivePlayer struct {
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
	observedTitle  string
	observedArtist string
	observedThumb  string
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

func NewDreamFMYouTubeLivePlayer(app *application.App, windows *WindowManager, cookies dreamFMPlayerCookieProvider) *DreamFMYouTubeLivePlayer {
	return &DreamFMYouTubeLivePlayer{
		app:          app,
		windows:      windows,
		cookies:      cookies,
		currentState: "idle",
		targetVolume: 1,
	}
}

func (player *DreamFMYouTubeLivePlayer) Play(request DreamFMPlayerPlayRequest) error {
	if player == nil || player.app == nil {
		return fmt.Errorf("dreamfm live player unavailable")
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
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
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
	sameVideo := player.currentVideo == request.VideoID
	if window == nil {
		window = player.createWindowLocked(request)
	}
	player.currentVideo = request.VideoID
	player.mu.Unlock()

	player.dispatch(map[string]any{
		"source":           dreamFMLivePlayerSource,
		"type":             "state",
		"state":            "loading",
		"videoId":          request.VideoID,
		"observedVideoId":  request.VideoID,
		"requestedVideoId": request.VideoID,
		"title":            request.Title,
		"artist":           request.Artist,
	})

	if window == nil {
		return fmt.Errorf("dreamfm live player window unavailable")
	}
	if videoVisible {
		window.Show()
	}
	if createdWindow {
		loadDreamFMYouTubeMusicURL(window, dreamFMYouTubeLiveEmbedURL(request.VideoID), cookies)
		return nil
	}
	if sameVideo {
		execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveSameVideoPlayScript(request))
		return nil
	}

	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLivePrepareLoadScript(request))
	loadDreamFMYouTubeMusicURL(window, dreamFMYouTubeLiveEmbedURL(request.VideoID), cookies)
	return nil
}

func (player *DreamFMYouTubeLivePlayer) Pause() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("paused", "pause-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLivePauseScript())
	return nil
}

func (player *DreamFMYouTubeLivePlayer) Resume() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("buffering", "resume-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveResumeScript())
	return nil
}

func (player *DreamFMYouTubeLivePlayer) Replay() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	videoID := player.currentVideo
	volume := player.targetVolume
	muted := player.targetMuted
	player.mu.Unlock()
	player.dispatchPlaybackState("buffering", "replay-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveReplayScript(videoID, volume, muted))
	return nil
}

func (player *DreamFMYouTubeLivePlayer) Seek(seconds float64) error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.dispatchPlaybackState("buffering", "seek-requested")
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveSeekScript(seconds))
	return nil
}

func (player *DreamFMYouTubeLivePlayer) SkipAd() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveSkipAdScript())
	return nil
}

func (player *DreamFMYouTubeLivePlayer) SetVolume(volume float64, muted bool) error {
	volume = clampDreamFMVolume(volume)

	player.mu.Lock()
	player.targetVolume = volume
	player.targetMuted = muted
	window := player.window
	player.mu.Unlock()

	if window == nil {
		return nil
	}
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveVolumeScript(volume, muted))
	return nil
}

func (player *DreamFMYouTubeLivePlayer) Reset() error {
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
	player.observedTitle = ""
	player.observedArtist = ""
	player.observedThumb = ""
	player.advertising = false
	player.adLabel = ""
	player.adSkippable = false
	player.adSkipLabel = ""
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
		execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLivePauseScript())
		window.SetURL(dreamFMYouTubeMusicBlankURL)
		window.Close()
	}
	player.dispatchPlaybackState("idle", "reset")
	return nil
}

func (player *DreamFMYouTubeLivePlayer) ShowWindow() error {
	return player.ShowVideoWindow()
}

func (player *DreamFMYouTubeLivePlayer) HideWindow() error {
	return player.HideVideoWindow()
}

func (player *DreamFMYouTubeLivePlayer) ShowVideoWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	player.videoVisible = true
	volume := player.targetVolume
	muted := player.targetMuted
	player.mu.Unlock()
	window.SetTitle("Dream.FM Live")
	window.SetMinSize(320, 180)
	window.SetSize(720, 405)
	window.Show()
	window.Focus()
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveVolumeScript(volume, muted))
	return nil
}

func (player *DreamFMYouTubeLivePlayer) HideVideoWindow() error {
	window := player.currentWindow()
	if window == nil {
		return nil
	}
	player.mu.Lock()
	player.videoVisible = false
	player.mu.Unlock()
	execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveExitVideoModeScript())
	window.Hide()
	return nil
}

func (player *DreamFMYouTubeLivePlayer) ShowAirPlayPicker(anchor DreamFMAirPlayAnchor) error {
	if player != nil && player.windows != nil && player.windows.mainWindow != nil {
		if showDreamFMNativeAirPlayPicker(player.windows.mainWindow.NativeWindow(), anchor) {
			return nil
		}
	}
	return nil
}

func (player *DreamFMYouTubeLivePlayer) Status() DreamFMPlayerStatus {
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
		ObservedVideoID: player.currentVideo,
		State:           player.currentState,
		Title:           title,
		Artist:          artist,
		ThumbnailURL:    player.observedThumb,
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

func (player *DreamFMYouTubeLivePlayer) HandleRawMessage(window application.Window, message string, _ *application.OriginInfo) bool {
	if player == nil || window == nil || window.Name() != dreamFMLivePlayerWindowName {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	if source, _ := payload["source"].(string); source != dreamFMLivePlayerSource {
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

func (player *DreamFMYouTubeLivePlayer) currentWindow() *application.WebviewWindow {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.window
}

func (player *DreamFMYouTubeLivePlayer) createWindowLocked(request DreamFMPlayerPlayRequest) *application.WebviewWindow {
	if player.app == nil {
		return nil
	}

	bridgeScript := dreamFMYouTubeLiveBridgeScript(request)
	window := player.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:        dreamFMLivePlayerWindowName,
		Title:       "DreamFM Live",
		Width:       720,
		Height:      405,
		MinWidth:    320,
		MinHeight:   180,
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
			execDreamFMYouTubeMusicJS(window, dreamFMYouTubeLiveExitVideoModeScript())
			player.dispatch(map[string]any{
				"source": dreamFMLivePlayerSource,
				"type":   "video-closed",
			})
		}
		window.Hide()
	})
	player.bridgeHook = attachDreamFMYouTubeMusicBridge(window, bridgeScript)
	player.window = window
	return window
}

func (player *DreamFMYouTubeLivePlayer) playbackCookies(ctx context.Context) []appcookies.Record {
	if player == nil || player.cookies == nil {
		return nil
	}
	records, err := player.cookies.CookiesForConnectorType(ctx, connectors.ConnectorYouTube)
	if err != nil {
		return nil
	}
	return filterDreamFMPlaybackCookies(appcookies.MatchURL(records, dreamFMYouTubeOrigin+"/"), time.Now())
}

func (player *DreamFMYouTubeLivePlayer) dispatch(payload map[string]any) {
	if player == nil || player.windows == nil {
		return
	}
	player.windows.dispatchWindowEvent(dreamFMLivePlayerEventName, payload)
}

func (player *DreamFMYouTubeLivePlayer) dispatchPlaybackState(state string, reason string) {
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
	adSkippable := player.adSkippable
	adSkipLabel := player.adSkipLabel
	errorCode := player.errorCode
	errorMessage := player.errorMessage
	player.mu.Unlock()
	player.dispatch(map[string]any{
		"source":           dreamFMLivePlayerSource,
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
		"adSkippable":      adSkippable,
		"adSkipLabel":      adSkipLabel,
		"errorCode":        errorCode,
		"errorMessage":     errorMessage,
	})
}

func dreamFMYouTubeLiveEmbedURL(videoID string) string {
	values := url.Values{}
	values.Set("autoplay", "1")
	values.Set("controls", "1")
	values.Set("enablejsapi", "1")
	values.Set("fs", "1")
	values.Set("iv_load_policy", "3")
	values.Set("modestbranding", "1")
	values.Set("origin", dreamFMYouTubeClientOrigin())
	values.Set("playsinline", "1")
	values.Set("rel", "0")
	return dreamFMYouTubeOrigin + "/embed/" + url.PathEscape(strings.TrimSpace(videoID)) + "?" + values.Encode()
}

func dreamFMYouTubeClientOrigin() string {
	return "https://" + dreamFMYouTubeClientID
}

func dreamFMYouTubeLiveBridgeScript(request DreamFMPlayerPlayRequest) string {
	initial, _ := json.Marshal(normalizeDreamFMPlayerPlayRequest(request))
	return fmt.Sprintf(`
(function() {
  "use strict";
  if (window.__dreamfmLiveBridgeInstalled) return;
  window.__dreamfmLiveBridgeInstalled = true;

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
      stored = JSON.parse(window.localStorage.getItem("__dreamfmLivePlaybackRequest") || "null");
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
      window.localStorage.setItem("__dreamfmLivePlaybackRequest", JSON.stringify(request));
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
    if (apiState === 2 || apiState === 5) return "paused";
    if (apiState === 0 || reason === "ended") return "ended";
    if (!video) return "loading";
    if (video.error) return "error";
    if (video.ended) return "ended";
    if (video.seeking || reason === "waiting" || reason === "stalled" || reason === "seeking") return "buffering";
    if (!video.paused) return video.readyState < 2 ? "buffering" : "playing";
    if (lastRequestedAction === "play") return "loading";
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

  function sendState(reason, force) {
    const now = Date.now();
    if (!force && now - lastUpdateAt < UPDATE_THROTTLE_MS) return;
    lastUpdateAt = now;
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
      window.localStorage.setItem("__dreamfmLivePlaybackRequest", JSON.stringify(Object.assign({}, INITIAL_REQUEST, readRequest())));
    } catch (error) {}
    post(Object.assign({
      type: "ready",
      state: "loading",
      url: window.location.href
    }, metadataSnapshot()));
    attachVideoListeners();
    const bodyObserver = new MutationObserver(() => attachVideoListeners());
    bodyObserver.observe(document.documentElement || document.body, { childList: true, subtree: true });
    scheduleAutoplay();
    sendState("boot", true);
  }

  window.__dreamfmLivePlayer = {
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
    skipAd: () => invokeSkipAd("api-skip-ad"),
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
`, dreamFMLivePlayerSource, string(initial))
}

func dreamFMYouTubeLiveVideoModeScript() string {
	return `
(function() {
  "use strict";

  try { window.localStorage.setItem("__dreamfmLiveVideoModeActive", "true"); } catch (error) {}
  window.__dreamfmLiveVideoModeActive = true;

  function ensureBlackout() {
    if (document.getElementById("dreamfm-live-video-blackout")) return;
    const blackout = document.createElement("div");
    blackout.id = "dreamfm-live-video-blackout";
    blackout.style.cssText = [
      "position:fixed!important",
      "inset:0!important",
      "background:#000!important",
      "z-index:2147483646!important"
    ].join(";");
    document.body.appendChild(blackout);
  }

  function removeBlackout() {
    document.getElementById("dreamfm-live-video-blackout")?.remove();
  }

  function installVideoStyles() {
    const styleId = "dreamfm-live-video-mode-style";
    let style = document.getElementById(styleId);
    if (!style) {
      style = document.createElement("style");
      style.id = styleId;
      document.head.appendChild(style);
    }
    style.textContent = [
      "html, body, * { visibility: hidden !important; }",
      "html, body { background: #000 !important; overflow: hidden !important; visibility: visible !important; }",
      ".dreamfm-live-video-visible, .dreamfm-live-video-visible * { visibility: visible !important; }",
      ".dreamfm-live-video-root { position: fixed !important; inset: 0 !important; width: 100vw !important; height: 100vh !important; min-width: 100vw !important; min-height: 100vh !important; margin: 0 !important; padding: 0 !important; background: #000 !important; z-index: 2147483647 !important; }",
      ".dreamfm-live-video-root video, .dreamfm-live-video-root .video-stream { width: 100% !important; height: 100% !important; object-fit: contain !important; }"
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
    document.querySelectorAll(".dreamfm-live-video-visible, .dreamfm-live-video-root").forEach((element) => {
      element.classList.remove("dreamfm-live-video-visible", "dreamfm-live-video-root");
    });
    root.classList.add("dreamfm-live-video-root");
    let current = root;
    while (current && current !== document.documentElement) {
      current.classList.add("dreamfm-live-video-visible");
      current = current.parentElement;
    }
    video.classList.add("dreamfm-live-video-visible");
    return true;
  }

  function enforce() {
    let active = window.__dreamfmLiveVideoModeActive;
    try {
      active = active && window.localStorage.getItem("__dreamfmLiveVideoModeActive") === "true";
    } catch (error) {}
    if (!active) return;
    if (markVideoTree()) {
      installVideoStyles();
      removeBlackout();
    }
    window.requestAnimationFrame(enforce);
  }

  ensureBlackout();
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

func dreamFMYouTubeLiveExitVideoModeScript() string {
	return `
(function() {
  window.__dreamfmLiveVideoModeActive = false;
  try { window.localStorage.setItem("__dreamfmLiveVideoModeActive", "false"); } catch (error) {}
  document.getElementById("dreamfm-live-video-blackout")?.remove();
  document.getElementById("dreamfm-live-video-mode-style")?.remove();
  document.querySelectorAll(".dreamfm-live-video-visible, .dreamfm-live-video-root").forEach((element) => {
    element.classList.remove("dreamfm-live-video-visible", "dreamfm-live-video-root");
  });
  document.body.style.overflow = "";
  document.body.style.background = "";
})();
`
}

func dreamFMYouTubeLivePrepareLoadScript(request DreamFMPlayerPlayRequest) string {
	requestJSON, _ := json.Marshal(normalizeDreamFMPlayerPlayRequest(request))
	return fmt.Sprintf(`
(function() {
  try {
    const request = %s;
    const api = window.__dreamfmLivePlayer;
    if (api && typeof api.request === "function") api.request(request);
    else window.localStorage.setItem("__dreamfmLivePlaybackRequest", JSON.stringify(request));
    if (api && typeof api.pause === "function") api.pause();
    else document.querySelector("video")?.pause();
  } catch (error) {}
})();
`, string(requestJSON))
}

func dreamFMYouTubeLivePauseScript() string {
	return `
(function() {
  const api = window.__dreamfmLivePlayer;
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

func dreamFMYouTubeLiveResumeScript() string {
	return `
(function() {
  const api = window.__dreamfmLivePlayer;
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

func dreamFMYouTubeLiveReplayScript(videoID string, volume float64, muted bool) string {
	request := DreamFMPlayerPlayRequest{
		VideoID: videoID,
		Volume:  clampDreamFMVolume(volume),
		Muted:   muted,
	}
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const request = %s;
  const api = window.__dreamfmLivePlayer;
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

func dreamFMYouTubeLiveSameVideoPlayScript(request DreamFMPlayerPlayRequest) string {
	request = normalizeDreamFMPlayerPlayRequest(request)
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`
(function() {
  const request = %s;
  const api = window.__dreamfmLivePlayer;
  try { window.localStorage.setItem("__dreamfmLivePlaybackRequest", JSON.stringify(request)); } catch (error) {}
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

func dreamFMYouTubeLiveSeekScript(seconds float64) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__dreamfmLivePlayer;
  if (api && typeof api.seek === "function") {
    api.seek(%f);
    return;
  }
  const moviePlayer = document.getElementById("movie_player");
  if (moviePlayer && typeof moviePlayer.seekTo === "function") {
    try { moviePlayer.seekTo(%f, true); } catch (error) {}
  }
})();
`, clampDreamFMSeconds(seconds), clampDreamFMSeconds(seconds))
}

func dreamFMYouTubeLiveSkipAdScript() string {
	return `
(function() {
  const api = window.__dreamfmLivePlayer;
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

func dreamFMYouTubeLiveVolumeScript(volume float64, muted bool) string {
	return fmt.Sprintf(`
(function() {
  const api = window.__dreamfmLivePlayer;
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
`, clampDreamFMVolume(volume), muted, clampDreamFMVolume(volume), muted, muted)
}
