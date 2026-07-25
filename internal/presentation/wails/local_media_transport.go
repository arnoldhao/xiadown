package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"runtime"
	"strings"
	"sync"

	"xiadown/internal/application/listenplayback"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	localMediaWindowName = "native-local-media-player"
	localMediaSource     = "native-local-media-player"
)

// NativeLocalMediaWebviewTransport owns a hidden HTMLMediaElement window in
// Go. The window is independent of the main React tree, so local audio keeps
// playing while workspaces and companion views are mounted or replaced.
type NativeLocalMediaWebviewTransport struct {
	app     *application.App
	windows *WindowManager
	baseURL string

	mu                sync.Mutex
	window            *application.WebviewWindow
	closeHook         func()
	bridgeHook        func()
	ready             bool
	closed            bool
	current           listenplayback.NativeLocalMediaRequest
	desiredPlaying    bool
	dispatchedSession string
	nextSubscriberID  uint64
	subscribers       map[uint64]listenplayback.PlaybackBackendEventListener
}

var _ listenplayback.NativeLocalMediaTransport = (*NativeLocalMediaWebviewTransport)(nil)

type localMediaStartCommand struct {
	SessionID    string  `json:"sessionId"`
	URI          string  `json:"uri"`
	Kind         string  `json:"kind"`
	StartSeconds float64 `json:"startSeconds"`
	Volume       float64 `json:"volume"`
	Muted        bool    `json:"muted"`
	Autoplay     bool    `json:"autoplay"`
}

type localMediaReadyAction uint8

const (
	localMediaReadyNoop localMediaReadyAction = iota
	localMediaReadyStart
	localMediaReadyStop
)

func NewNativeLocalMediaWebviewTransport(
	app *application.App,
	baseURL string,
	windows ...*WindowManager,
) *NativeLocalMediaWebviewTransport {
	transport := &NativeLocalMediaWebviewTransport{
		app:         app,
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		subscribers: make(map[uint64]listenplayback.PlaybackBackendEventListener),
	}
	if len(windows) > 0 {
		transport.windows = windows[0]
	}
	return transport
}

func (transport *NativeLocalMediaWebviewTransport) Availability() (bool, string) {
	if transport == nil || transport.app == nil {
		return false, "desktop local media transport is unavailable"
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return true, ""
	default:
		return false, fmt.Sprintf("local media playback is not supported on %s", runtime.GOOS)
	}
}

func (transport *NativeLocalMediaWebviewTransport) Start(ctx context.Context, request listenplayback.NativeLocalMediaRequest) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if available, reason := transport.Availability(); !available {
		return &listenplayback.PlaybackUnsupportedError{Provider: listenplayback.PlaybackProviderLocal, Reason: reason}
	}
	resolvedURI, err := resolveNativeLocalMediaURI(transport.baseURL, request.URI)
	if err != nil {
		return err
	}
	request.URI = resolvedURI
	request.SessionID = strings.TrimSpace(request.SessionID)
	if request.SessionID == "" {
		return fmt.Errorf("local media session id is required")
	}

	transport.mu.Lock()
	if transport.closed {
		transport.mu.Unlock()
		return fmt.Errorf("local media transport is closed")
	}
	window, err := transport.ensureWindowLocked()
	if err != nil {
		transport.mu.Unlock()
		return err
	}
	transport.current = request
	transport.desiredPlaying = true
	ready := transport.ready
	command := transport.startCommandLocked()
	if ready {
		transport.dispatchedSession = request.SessionID
	} else {
		transport.dispatchedSession = ""
	}
	transport.mu.Unlock()
	if ready {
		execListenYouTubeMusicJS(window, localMediaStartScript(command))
	}
	return nil
}

func (transport *NativeLocalMediaWebviewTransport) Play(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	transport.mu.Lock()
	transport.desiredPlaying = true
	window := transport.window
	ready := transport.ready
	hasMedia := transport.current.SessionID != ""
	transport.mu.Unlock()
	if ready && window != nil && hasMedia {
		execListenYouTubeMusicJS(window, localMediaSimpleCommandScript("play"))
	}
	return nil
}

func (transport *NativeLocalMediaWebviewTransport) Pause(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	transport.mu.Lock()
	transport.desiredPlaying = false
	window := transport.window
	ready := transport.ready
	transport.mu.Unlock()
	if ready && window != nil {
		execListenYouTubeMusicJS(window, localMediaSimpleCommandScript("pause"))
	}
	return nil
}

func (transport *NativeLocalMediaWebviewTransport) Stop(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	transport.mu.Lock()
	transport.desiredPlaying = false
	transport.current = listenplayback.NativeLocalMediaRequest{}
	transport.dispatchedSession = ""
	window := transport.window
	ready := transport.ready
	transport.mu.Unlock()
	if ready && window != nil {
		execListenYouTubeMusicJS(window, localMediaSimpleCommandScript("stop"))
	}
	return nil
}

func (transport *NativeLocalMediaWebviewTransport) Seek(ctx context.Context, seconds float64) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if seconds < 0 {
		seconds = 0
	}
	transport.mu.Lock()
	transport.current.StartSeconds = seconds
	window := transport.window
	ready := transport.ready
	transport.mu.Unlock()
	if ready && window != nil {
		execListenYouTubeMusicJS(window, localMediaNumberCommandScript("seek", seconds))
	}
	return nil
}

func (transport *NativeLocalMediaWebviewTransport) SetVolume(ctx context.Context, volume float64, muted bool) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if volume < 0 {
		volume = 0
	} else if volume > 1 {
		volume = 1
	}
	muted = muted || volume <= 0
	transport.mu.Lock()
	transport.current.Volume = volume
	transport.current.Muted = muted
	window := transport.window
	ready := transport.ready
	transport.mu.Unlock()
	if ready && window != nil {
		execListenYouTubeMusicJS(window, localMediaVolumeScript(volume, muted))
	}
	return nil
}

func (transport *NativeLocalMediaWebviewTransport) Subscribe(listener listenplayback.PlaybackBackendEventListener) func() {
	if transport == nil || listener == nil {
		return func() {}
	}
	transport.mu.Lock()
	transport.nextSubscriberID++
	id := transport.nextSubscriberID
	transport.subscribers[id] = listener
	transport.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			transport.mu.Lock()
			delete(transport.subscribers, id)
			transport.mu.Unlock()
		})
	}
}

func (transport *NativeLocalMediaWebviewTransport) Close() error {
	if transport == nil {
		return nil
	}
	transport.mu.Lock()
	if transport.closed {
		transport.mu.Unlock()
		return nil
	}
	transport.closed = true
	window := transport.window
	closeHook := transport.closeHook
	bridgeHook := transport.bridgeHook
	transport.window = nil
	transport.closeHook = nil
	transport.bridgeHook = nil
	transport.ready = false
	transport.current = listenplayback.NativeLocalMediaRequest{}
	transport.desiredPlaying = false
	transport.dispatchedSession = ""
	transport.mu.Unlock()
	if closeHook != nil {
		closeHook()
	}
	if bridgeHook != nil {
		bridgeHook()
	}
	if window != nil {
		execListenYouTubeMusicJS(window, localMediaSimpleCommandScript("stop"))
		releaseListenMediaWebViewParking(window)
		releaseWebViewRemoteCapabilityPolicy(window)
		window.Close()
	}
	return nil
}

// HandleRawMessage consumes HTMLMediaElement events sent through the native
// WebView bridge. It is called by the application's single raw-message router.
func (transport *NativeLocalMediaWebviewTransport) HandleRawMessage(window application.Window, message string, _ *application.OriginInfo) bool {
	if transport == nil || window == nil || window.Name() != localMediaWindowName {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	if listenPayloadString(payload, "source") != localMediaSource {
		return false
	}

	eventType := listenPayloadString(payload, "type")
	transport.mu.Lock()
	activeWindow := transport.window
	if activeWindow == nil || window.ID() != activeWindow.ID() {
		transport.mu.Unlock()
		return true
	}
	if eventType == "ready" {
		transport.ready = true
		action, command := transport.readyActionLocked()
		transport.mu.Unlock()
		switch action {
		case localMediaReadyStart:
			execListenYouTubeMusicJS(activeWindow, localMediaStartScript(command))
		case localMediaReadyStop:
			execListenYouTubeMusicJS(activeWindow, localMediaSimpleCommandScript("stop"))
		}
		return true
	}

	sessionID := listenPayloadString(payload, "sessionId")
	if sessionID == "" || sessionID != transport.current.SessionID {
		transport.mu.Unlock()
		return true
	}
	state := listenplayback.PlaybackState(listenPayloadString(payload, "state"))
	position, hasPosition := listenPayloadFloat(payload, "currentTime")
	duration, hasDuration := listenPayloadFloat(payload, "duration")
	volume, hasVolume := listenPayloadFloat(payload, "volume")
	muted, hasMuted := listenPayloadBoolValue(payload, "muted")
	errorMessage := listenPayloadString(payload, "error")
	if errorMessage != "" {
		state = listenplayback.PlaybackStateError
	}
	listeners := make([]listenplayback.PlaybackBackendEventListener, 0, len(transport.subscribers))
	for _, listener := range transport.subscribers {
		listeners = append(listeners, listener)
	}
	transport.mu.Unlock()

	event := listenplayback.PlaybackBackendEvent{
		Provider:  listenplayback.PlaybackProviderLocal,
		SessionID: sessionID,
		State:     state,
		Position:  position,
		Duration:  duration,
		Volume:    volume,
		Muted:     muted,
		Error:     errorMessage,
		HasTiming: hasPosition || hasDuration,
		HasVolume: hasVolume || hasMuted,
	}
	for _, listener := range listeners {
		listener(event)
	}
	return true
}

func (transport *NativeLocalMediaWebviewTransport) ensureWindowLocked() (*application.WebviewWindow, error) {
	if transport.window != nil {
		return transport.window, nil
	}
	if transport.app == nil {
		return nil, fmt.Errorf("desktop local media transport is unavailable")
	}
	window := transport.app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{
		Name:          localMediaWindowName,
		Title:         "XiaDown Local Media",
		Width:         2,
		Height:        2,
		MinWidth:      1,
		MinHeight:     1,
		Hidden:        true,
		Frameless:     true,
		DisableResize: true,
		HTML:          localMediaHostHTML,
		JS:            localMediaBridgeScript,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
			Permissions: map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState{
				remoteMediaWebViewAutoplayPermissionKind: application.CoreWebView2PermissionStateAllow,
			},
		},
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				EnableAutoplayWithoutUserAction: application.Enabled,
			},
		},
	}))
	if window == nil {
		return nil, fmt.Errorf("failed to create local media player window")
	}
	registerWebViewRemoteCapabilityPolicy(window)
	bridgeHook, bridgeInstalled := attachListenYouTubeMusicBridge(window, localMediaBridgeScript)
	if !bridgeInstalled {
		releaseWebViewRemoteCapabilityPolicy(window)
		window.Close()
		return nil, fmt.Errorf("failed to install the local media WebView bridge")
	}
	parkingRegistered := transport.windows != nil &&
		transport.windows.mainWindow != nil &&
		registerListenMediaWebViewParking(window, transport.windows.mainWindow)
	if !parkingRegistered {
		log.Printf(
			"media WebView parking unavailable for %q; using hidden-window fallback",
			localMediaWindowName,
		)
		hideListenYouTubeMediaWindow(window)
	}
	transport.closeHook = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		hideListenYouTubeMediaWindow(window)
	})
	transport.bridgeHook = bridgeHook
	transport.window = window
	return window, nil
}

func (transport *NativeLocalMediaWebviewTransport) startCommandLocked() localMediaStartCommand {
	return localMediaStartCommand{
		SessionID:    transport.current.SessionID,
		URI:          transport.current.URI,
		Kind:         string(transport.current.Kind),
		StartSeconds: transport.current.StartSeconds,
		Volume:       transport.current.Volume,
		Muted:        transport.current.Muted,
		Autoplay:     transport.desiredPlaying,
	}
}

// readyActionLocked makes repeated bridge-ready messages idempotent. The
// bridge retries readiness while Go is still dispatching the first start; a
// duplicate must not translate to stop and tear down the media that just won
// the race.
func (transport *NativeLocalMediaWebviewTransport) readyActionLocked() (localMediaReadyAction, localMediaStartCommand) {
	if transport.current.SessionID == "" {
		return localMediaReadyStop, localMediaStartCommand{}
	}
	if transport.current.SessionID == transport.dispatchedSession {
		return localMediaReadyNoop, localMediaStartCommand{}
	}
	transport.dispatchedSession = transport.current.SessionID
	return localMediaReadyStart, transport.startCommandLocked()
}

func resolveNativeLocalMediaURI(baseURL string, rawURI string) (string, error) {
	value := strings.TrimSpace(rawURI)
	if value == "" {
		return "", fmt.Errorf("local media URI is required")
	}
	parsed, err := url.Parse(value)
	if err == nil {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
			return value, nil
		case "file":
			value, err = url.PathUnescape(parsed.Path)
			if err != nil {
				return "", fmt.Errorf("decode local media path: %w", err)
			}
			if parsed.Host != "" && parsed.Host != "localhost" {
				value = "//" + parsed.Host + value
			}
		case "":
		default:
			// url.Parse treats a Windows drive letter as a scheme.
			if len(parsed.Scheme) != 1 || len(value) < 3 || value[1] != ':' {
				return "", fmt.Errorf("unsupported local media URI scheme %q", parsed.Scheme)
			}
		}
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("local media HTTP base URL is unavailable")
	}
	return baseURL + "/api/library/asset/local-media?path=" + url.QueryEscape(value), nil
}

func localMediaStartScript(command localMediaStartCommand) string {
	payload, _ := json.Marshal(command)
	return fmt.Sprintf(`(function(){var p=window.__xiadownLocalMedia;if(p){p.start(%s);}})();`, payload)
}

func localMediaSimpleCommandScript(command string) string {
	encoded, _ := json.Marshal(command)
	return fmt.Sprintf(`(function(){var p=window.__xiadownLocalMedia;if(p&&typeof p[%s]==="function"){p[%s]();}})();`, encoded, encoded)
}

func localMediaNumberCommandScript(command string, value float64) string {
	encoded, _ := json.Marshal(command)
	return fmt.Sprintf(`(function(){var p=window.__xiadownLocalMedia;if(p&&typeof p[%s]==="function"){p[%s](%g);}})();`, encoded, encoded, value)
}

func localMediaVolumeScript(volume float64, muted bool) string {
	return fmt.Sprintf(`(function(){var p=window.__xiadownLocalMedia;if(p){p.volume(%g,%t);}})();`, volume, muted)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

const localMediaHostHTML = `<!doctype html><html><head><meta charset="utf-8"></head><body><video id="xiadown-local-media" playsinline></video></body></html>`

const localMediaBridgeScript = `(function(){
  if (window.__xiadownLocalMedia) return;
  const SOURCE = "native-local-media-player";
  let media = null;
  let sessionId = "";
  let requestedStart = 0;
  let stopRequested = false;
  let readyTimer = null;

  function post(payload) {
    const message = JSON.stringify(Object.assign({ source: SOURCE, sessionId }, payload));
    try {
      if (window._wails && typeof window._wails.invoke === "function") { window._wails.invoke(message); return; }
      if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.external) { window.webkit.messageHandlers.external.postMessage(message); return; }
      if (window.chrome && window.chrome.webview && typeof window.chrome.webview.postMessage === "function") { window.chrome.webview.postMessage(message); return; }
      if (window.wails && typeof window.wails.invoke === "function") { window.wails.invoke(message); }
    } catch (_) {}
  }

  function finite(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function snapshot(state, error) {
    if (!media) return;
    post({
      type: "state",
      state,
      currentTime: Math.max(0, finite(media.currentTime, 0)),
      duration: Math.max(0, finite(media.duration, 0)),
      volume: Math.max(0, Math.min(1, finite(media.volume, 1))),
      muted: !!media.muted,
      error: error || ""
    });
  }

  function install() {
    media = document.getElementById("xiadown-local-media");
    if (!media) return;
    media.preload = "auto";
    media.addEventListener("loadstart", () => snapshot("loading"));
    media.addEventListener("loadedmetadata", () => {
      if (requestedStart > 0) {
        try { media.currentTime = Math.min(requestedStart, finite(media.duration, requestedStart)); } catch (_) {}
      }
      snapshot(media.paused ? "paused" : "playing");
    });
    media.addEventListener("playing", () => snapshot("playing"));
    media.addEventListener("waiting", () => snapshot("buffering"));
    media.addEventListener("stalled", () => snapshot("buffering"));
    media.addEventListener("pause", () => { if (!stopRequested && !media.ended) snapshot("paused"); });
    media.addEventListener("ended", () => snapshot("ended"));
    media.addEventListener("timeupdate", () => snapshot(media.paused ? "paused" : "playing"));
    media.addEventListener("durationchange", () => snapshot(media.paused ? "paused" : "playing"));
    media.addEventListener("volumechange", () => snapshot(media.paused ? "paused" : "playing"));
    media.addEventListener("error", () => {
      const code = media.error && media.error.code ? String(media.error.code) : "unknown";
      snapshot("error", "HTMLMediaElement error " + code);
    });
    post({ type: "ready" });
    readyTimer = window.setInterval(() => post({ type: "ready" }), 250);
  }

  function play() {
    if (!media || !media.src) return;
    stopRequested = false;
    const pending = media.play();
    if (pending && typeof pending.catch === "function") {
      pending.catch((error) => snapshot("error", String(error && error.message ? error.message : error)));
    }
  }

  window.__xiadownLocalMedia = {
    start(request) {
      if (!media || !request) return;
      if (readyTimer !== null) { window.clearInterval(readyTimer); readyTimer = null; }
      sessionId = String(request.sessionId || "");
      requestedStart = Math.max(0, finite(request.startSeconds, 0));
      stopRequested = false;
      media.volume = Math.max(0, Math.min(1, finite(request.volume, 1)));
      media.muted = !!request.muted;
      media.src = String(request.uri || "");
      media.load();
      snapshot("loading");
      if (request.autoplay !== false) play();
    },
    play,
    pause() { if (media) media.pause(); },
    stop() {
      if (!media) return;
      if (readyTimer !== null) { window.clearInterval(readyTimer); readyTimer = null; }
      stopRequested = true;
      media.pause();
      media.removeAttribute("src");
      media.load();
      requestedStart = 0;
      sessionId = "";
    },
    seek(seconds) {
      if (!media) return;
      const target = Math.max(0, finite(seconds, 0));
      requestedStart = target;
      try { media.currentTime = media.duration > 0 ? Math.min(target, media.duration) : target; } catch (_) {}
      snapshot(media.paused ? "paused" : "playing");
    },
    volume(value, muted) {
      if (!media) return;
      media.volume = Math.max(0, Math.min(1, finite(value, 1)));
      media.muted = !!muted;
      snapshot(media.paused ? "paused" : "playing");
    }
  };

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", install, { once: true });
  else install();
})();`
