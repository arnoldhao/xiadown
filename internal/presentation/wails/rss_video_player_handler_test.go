package wails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type rssVideoPlayerStub struct {
	response          RSSVideoPlayerPrepareResponse
	prepareRequest    RSSVideoPlayerPrepareRequest
	acceptRequestID   uint64
	cancelRequestID   uint64
	playSession       string
	pauseSession      string
	seekSession       string
	seekSeconds       float64
	volumeSession     string
	volume            float64
	muted             bool
	rateSession       string
	rate              float64
	captionToggle     string
	captionSession    string
	captionValue      string
	qualitySession    string
	qualityValue      string
	danmakuToggle     string
	fullscreenSession string
	exitSession       string
	status            RSSVideoPlayerStatus
	showRect          ListenEmbeddedVideoRect
	hideSeq           uint64
	closeSession      string
	rawHandled        bool
	err               error
}

func (stub *rssVideoPlayerStub) Prepare(
	_ context.Context,
	request RSSVideoPlayerPrepareRequest,
) (RSSVideoPlayerPrepareResponse, error) {
	stub.prepareRequest = request
	return stub.response, stub.err
}

func (stub *rssVideoPlayerStub) AcceptPrepare(requestID uint64) error {
	stub.acceptRequestID = requestID
	return stub.err
}

func (stub *rssVideoPlayerStub) CancelPrepare(requestID uint64) error {
	stub.cancelRequestID = requestID
	return stub.err
}

func (stub *rssVideoPlayerStub) Play(sessionID string) error {
	stub.playSession = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) Pause(sessionID string) error {
	stub.pauseSession = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) Seek(sessionID string, seconds float64) error {
	stub.seekSession = sessionID
	stub.seekSeconds = seconds
	return stub.err
}

func (stub *rssVideoPlayerStub) SetVolume(sessionID string, volume float64, muted bool) error {
	stub.volumeSession = sessionID
	stub.volume = volume
	stub.muted = muted
	return stub.err
}

func (stub *rssVideoPlayerStub) SetPlaybackRate(sessionID string, rate float64) error {
	stub.rateSession = sessionID
	stub.rate = rate
	return stub.err
}

func (stub *rssVideoPlayerStub) ToggleCaptions(sessionID string) error {
	stub.captionToggle = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) SelectCaption(sessionID string, value string) error {
	stub.captionSession = sessionID
	stub.captionValue = value
	return stub.err
}

func (stub *rssVideoPlayerStub) SelectQuality(sessionID string, value string) error {
	stub.qualitySession = sessionID
	stub.qualityValue = value
	return stub.err
}

func (stub *rssVideoPlayerStub) ToggleDanmaku(sessionID string) error {
	stub.danmakuToggle = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) RequestFullscreen(sessionID string) error {
	stub.fullscreenSession = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) ExitFullscreen(sessionID string) error {
	stub.exitSession = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) Status() RSSVideoPlayerStatus {
	return stub.status
}

func (stub *rssVideoPlayerStub) Show(rect ListenEmbeddedVideoRect) (bool, error) {
	stub.showRect = rect
	return stub.err == nil, stub.err
}

func (stub *rssVideoPlayerStub) Hide(sequence uint64) (bool, error) {
	stub.hideSeq = sequence
	return stub.err == nil, stub.err
}

func (stub *rssVideoPlayerStub) Close(sessionID string) error {
	stub.closeSession = sessionID
	return stub.err
}

func (stub *rssVideoPlayerStub) HandleRawMessage(
	_ application.Window,
	_ string,
	_ *application.OriginInfo,
) bool {
	return stub.rawHandled
}

func TestRSSVideoPlayerHandlerTransportContract(t *testing.T) {
	volume := 0.35
	stub := &rssVideoPlayerStub{
		response: RSSVideoPlayerPrepareResponse{
			Platform:        "bilibili",
			Adapter:         "video",
			PlatformVideoID: "BV1xx411c7mD",
			PlayerURL:       "https://www.bilibili.com/video/BV1xx411c7mD/",
			Authenticated:   true,
			SessionID:       "rss-session-1",
		},
		status: RSSVideoPlayerStatus{Provider: "bilibili", SessionID: "rss-session-1"},
	}
	handler := &RSSVideoPlayerHandler{player: stub}
	if handler.ServiceName() != "RSSVideoPlayerHandler" {
		t.Fatalf("ServiceName() = %q", handler.ServiceName())
	}
	response, err := handler.Prepare(context.Background(), RSSVideoPlayerPrepareRequest{
		RequestID:       101,
		PlatformVideoID: "BV1xx411c7mD",
		StartSeconds:    42,
		Volume:          &volume,
		Muted:           true,
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if stub.prepareRequest.PlatformVideoID != "BV1xx411c7mD" ||
		stub.prepareRequest.RequestID != 101 || stub.prepareRequest.StartSeconds != 42 || response.SessionID != "rss-session-1" ||
		response.Platform != "bilibili" || !response.Authenticated {
		t.Fatalf("unexpected Prepare contract: request=%#v response=%#v", stub.prepareRequest, response)
	}
	if response.Adapter != "video" {
		t.Fatalf("Prepare adapter = %q, want video", response.Adapter)
	}
	transaction := RSSVideoPlayerPrepareTransactionRequest{RequestID: 101}
	if err := handler.AcceptPrepare(context.Background(), transaction); err != nil {
		t.Fatal(err)
	}
	if err := handler.CancelPrepare(context.Background(), transaction); err != nil {
		t.Fatal(err)
	}
	if stub.acceptRequestID != 101 || stub.cancelRequestID != 101 {
		t.Fatalf("prepare transaction requests were not forwarded: %#v", stub)
	}

	session := RSSVideoPlayerSessionRequest{SessionID: response.SessionID}
	if err := handler.Play(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := handler.Pause(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := handler.Seek(context.Background(), RSSVideoPlayerSeekRequest{SessionID: response.SessionID, Seconds: 64}); err != nil {
		t.Fatal(err)
	}
	if err := handler.SetVolume(context.Background(), RSSVideoPlayerVolumeRequest{SessionID: response.SessionID, Volume: 0.8, Muted: false}); err != nil {
		t.Fatal(err)
	}
	if err := handler.SetPlaybackRate(context.Background(), RSSVideoPlayerRateRequest{SessionID: response.SessionID, Rate: 1.5}); err != nil {
		t.Fatal(err)
	}
	if err := handler.ToggleCaptions(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelectCaption(context.Background(), RSSVideoPlayerSelectionRequest{SessionID: response.SessionID, Value: ""}); err != nil {
		t.Fatal(err)
	}
	if err := handler.SelectQuality(context.Background(), RSSVideoPlayerSelectionRequest{SessionID: response.SessionID, Value: "80"}); err != nil {
		t.Fatal(err)
	}
	if err := handler.ToggleDanmaku(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := handler.RequestFullscreen(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := handler.ExitFullscreen(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	status, err := handler.Status(context.Background())
	if err != nil || status.SessionID != response.SessionID {
		t.Fatalf("Status() = %#v, %v", status, err)
	}
	if stub.playSession != response.SessionID || stub.pauseSession != response.SessionID ||
		stub.seekSession != response.SessionID || stub.seekSeconds != 64 ||
		stub.volumeSession != response.SessionID || stub.volume != 0.8 || stub.muted ||
		stub.rateSession != response.SessionID || stub.rate != 1.5 ||
		stub.captionToggle != response.SessionID || stub.captionSession != response.SessionID || stub.captionValue != "" ||
		stub.qualitySession != response.SessionID || stub.qualityValue != "80" || stub.danmakuToggle != response.SessionID ||
		stub.fullscreenSession != response.SessionID || stub.exitSession != response.SessionID {
		t.Fatalf("transport requests were not forwarded: %#v", stub)
	}

	if _, err := handler.Show(context.Background(), ListenEmbeddedVideoRect{X: 1, Y: 2, Width: 3, Height: 4, Sequence: 5}); err != nil {
		t.Fatalf("Show returned error: %v", err)
	}
	if stub.showRect.Sequence != 5 || stub.showRect.Width != 3 {
		t.Fatalf("Show request not forwarded: %#v", stub.showRect)
	}
	if _, err := handler.Hide(context.Background(), ListenEmbeddedVideoHideRequest{Sequence: 6}); err != nil {
		t.Fatalf("Hide returned error: %v", err)
	}
	if stub.hideSeq != 6 {
		t.Fatalf("Hide sequence = %d, want 6", stub.hideSeq)
	}
	if err := handler.Close(context.Background(), session); err != nil || stub.closeSession != response.SessionID {
		t.Fatalf("Close result: session=%q err=%v", stub.closeSession, err)
	}
}

func TestRSSVideoPlayerRawMessagesAreNotPartOfTheRendererService(t *testing.T) {
	stub := &rssVideoPlayerStub{rawHandled: true}
	service := &RSSVideoPlayerHandler{player: stub}
	if _, exposed := reflect.TypeOf(service).MethodByName("HandleRawMessage"); exposed {
		t.Fatal("raw message provenance handler must not be renderer-callable")
	}
	raw := NewRSSVideoPlayerRawMessageHandler(service)
	if !raw.HandleRawMessage(nil, "{}", nil) {
		t.Fatal("separate raw message handler did not preserve the native route")
	}
}

func TestRSSVideoPlayerHandlerPropagatesBackendErrors(t *testing.T) {
	want := errors.New("player failed")
	handler := &RSSVideoPlayerHandler{player: &rssVideoPlayerStub{err: want}}
	session := RSSVideoPlayerSessionRequest{SessionID: "rss-session-1"}
	checks := []struct {
		name string
		call func() error
	}{
		{name: "prepare", call: func() error {
			_, err := handler.Prepare(context.Background(), RSSVideoPlayerPrepareRequest{RequestID: 1, PlatformVideoID: "BV1xx411c7mD"})
			return err
		}},
		{name: "accept prepare", call: func() error {
			return handler.AcceptPrepare(context.Background(), RSSVideoPlayerPrepareTransactionRequest{RequestID: 1})
		}},
		{name: "cancel prepare", call: func() error {
			return handler.CancelPrepare(context.Background(), RSSVideoPlayerPrepareTransactionRequest{RequestID: 1})
		}},
		{name: "play", call: func() error { return handler.Play(context.Background(), session) }},
		{name: "pause", call: func() error { return handler.Pause(context.Background(), session) }},
		{name: "seek", call: func() error {
			return handler.Seek(context.Background(), RSSVideoPlayerSeekRequest{SessionID: session.SessionID})
		}},
		{name: "volume", call: func() error {
			return handler.SetVolume(context.Background(), RSSVideoPlayerVolumeRequest{SessionID: session.SessionID})
		}},
		{name: "rate", call: func() error {
			return handler.SetPlaybackRate(context.Background(), RSSVideoPlayerRateRequest{SessionID: session.SessionID, Rate: 1})
		}},
		{name: "toggle captions", call: func() error { return handler.ToggleCaptions(context.Background(), session) }},
		{name: "select caption off", call: func() error {
			return handler.SelectCaption(context.Background(), RSSVideoPlayerSelectionRequest{SessionID: session.SessionID})
		}},
		{name: "select quality", call: func() error {
			return handler.SelectQuality(context.Background(), RSSVideoPlayerSelectionRequest{SessionID: session.SessionID, Value: "80"})
		}},
		{name: "toggle danmaku", call: func() error { return handler.ToggleDanmaku(context.Background(), session) }},
		{name: "fullscreen", call: func() error { return handler.RequestFullscreen(context.Background(), session) }},
		{name: "exit fullscreen", call: func() error { return handler.ExitFullscreen(context.Background(), session) }},
		{name: "show", call: func() error { _, err := handler.Show(context.Background(), ListenEmbeddedVideoRect{}); return err }},
		{name: "hide", call: func() error {
			_, err := handler.Hide(context.Background(), ListenEmbeddedVideoHideRequest{})
			return err
		}},
		{name: "close", call: func() error { return handler.Close(context.Background(), session) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestRSSVideoPrepareCoordinatorLateOlderResolutionCannotReplaceNewSession(t *testing.T) {
	coordinator := &rssVideoPrepareCoordinator{}
	older, err := coordinator.begin(1_001)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := coordinator.begin(1_002)
	if err != nil {
		t.Fatal(err)
	}
	activeSession := ""
	newResponse, err := coordinator.commit(newer, func() (RSSVideoPlayerPrepareResponse, error) {
		activeSession = "new-session"
		return RSSVideoPlayerPrepareResponse{SessionID: activeSession}, nil
	})
	if err != nil || newResponse.SessionID != "new-session" {
		t.Fatalf("new Prepare commit = %#v, %v", newResponse, err)
	}
	oldCommitCalled := false
	_, err = coordinator.commit(older, func() (RSSVideoPlayerPrepareResponse, error) {
		oldCommitCalled = true
		activeSession = "old-session"
		return RSSVideoPlayerPrepareResponse{SessionID: activeSession}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late old Prepare error = %v, want context.Canceled", err)
	}
	if oldCommitCalled || activeSession != "new-session" {
		t.Fatalf("late old Prepare replaced the new session: called=%v active=%q", oldCommitCalled, activeSession)
	}
}

func TestRSSVideoPrepareCoordinatorRejectsOutOfOrderOlderBegin(t *testing.T) {
	coordinator := &rssVideoPrepareCoordinator{}
	newer, err := coordinator.begin(2_002)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.begin(2_001); !errors.Is(err, context.Canceled) {
		t.Fatalf("out-of-order old Begin error = %v, want context.Canceled", err)
	}
	commitCalled := false
	if _, err := coordinator.commit(newer, func() (RSSVideoPlayerPrepareResponse, error) {
		commitCalled = true
		return RSSVideoPlayerPrepareResponse{SessionID: "new-session"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !commitCalled {
		t.Fatal("newer request was not allowed to commit")
	}
}

func TestRSSVideoPrepareCoordinatorCleanupCancelsPendingAndCommittedRequests(t *testing.T) {
	coordinator := &rssVideoPrepareCoordinator{}
	pending, err := coordinator.begin(3_001)
	if err != nil {
		t.Fatal(err)
	}
	if sessionID, err := coordinator.cancel(pending.requestID); err != nil || sessionID != "" {
		t.Fatalf("pending cancel = %q, %v", sessionID, err)
	}
	commitCalled := false
	if _, err := coordinator.commit(pending, func() (RSSVideoPlayerPrepareResponse, error) {
		commitCalled = true
		return RSSVideoPlayerPrepareResponse{SessionID: "pending-session"}, nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled pending commit error = %v, want context.Canceled", err)
	}
	if commitCalled {
		t.Fatal("cleanup-canceled pending Prepare entered the destructive commit")
	}

	committed, err := coordinator.begin(3_002)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.commit(committed, func() (RSSVideoPlayerPrepareResponse, error) {
		return RSSVideoPlayerPrepareResponse{SessionID: "committed-session"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if sessionID, err := coordinator.cancel(committed.requestID); err != nil || sessionID != "committed-session" {
		t.Fatalf("committed cancel = %q, %v", sessionID, err)
	}
}

func TestRSSVideoPrepareCoordinatorCancelMayOvertakePrepareRPC(t *testing.T) {
	coordinator := &rssVideoPrepareCoordinator{}
	if sessionID, err := coordinator.cancel(4_002); err != nil || sessionID != "" {
		t.Fatalf("early cancel = %q, %v", sessionID, err)
	}
	if _, err := coordinator.begin(4_002); !errors.Is(err, context.Canceled) {
		t.Fatalf("delayed matching Prepare error = %v, want context.Canceled", err)
	}
	if _, err := coordinator.begin(4_001); !errors.Is(err, context.Canceled) {
		t.Fatalf("delayed older Prepare error = %v, want context.Canceled", err)
	}
}

func TestRSSVideoPrepareCoordinatorBoundsUnacceptedCommits(t *testing.T) {
	coordinator := &rssVideoPrepareCoordinator{}
	for requestID := uint64(5_001); requestID < 5_001+maxRSSVideoPrepareTransactions*3; requestID++ {
		ticket, err := coordinator.begin(requestID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := coordinator.commit(ticket, func() (RSSVideoPlayerPrepareResponse, error) {
			return RSSVideoPlayerPrepareResponse{SessionID: "unaccepted-session"}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	tracked := make(map[uint64]struct{})
	for requestID := range coordinator.seen {
		tracked[requestID] = struct{}{}
	}
	for requestID := range coordinator.finished {
		tracked[requestID] = struct{}{}
	}
	for requestID := range coordinator.canceled {
		tracked[requestID] = struct{}{}
	}
	for requestID := range coordinator.committed {
		tracked[requestID] = struct{}{}
	}
	if len(tracked) > maxRSSVideoPrepareTransactions {
		t.Fatalf("unaccepted Prepare transactions grew to %d, want <= %d", len(tracked), maxRSSVideoPrepareTransactions)
	}
}

func TestRSSBilibiliBridgeUsesCanonicalCapabilitiesAndStandardHTMLMediaTransport(t *testing.T) {
	script := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "session-1",
		Adapter:         "video",
		PlatformVideoID: "BV1xx411c7mD",
		StartSeconds:    25,
		Volume:          0.6,
		PlaybackRate:    1,
		Autoplay:        true,
	})
	for _, required := range []string{
		`"adapter":"video"`,
		`document.querySelectorAll("video")`,
		`media.play()`,
		`media.pause()`,
		`media.currentTime`,
		`media.volume`,
		`media.muted`,
		`media.playbackRate`,
		`video.buffered`,
		`"timeupdate"`,
		`"durationchange"`,
		`"volumechange"`,
		`"ratechange"`,
		`window.setInterval(reconcile, 500)`,
		`currentTime:`,
		`duration:`,
		`bufferedTime:`,
		`fullscreen:`,
		`controls: controlsFor(video, captions, quality, danmaku)`,
		`playbackRateOptions: RATE_OPTIONS`,
		`captionOptions: captions.options`,
		`qualityOptions: quality.options`,
		`danmakuEnabled: !!danmaku.enabled`,
		`script[type="application/ld+json"]`,
		`interactionStatistic`,
		`meta[itemprop="author"][content]`,
		`.pubdate-ip-text`,
		`.view-text`,
		`.video-like-info.video-toolbar-item-text`,
		`publisher: pageMetadata.publisher`,
		`publishedAt: pageMetadata.publishedAt`,
		`viewCount: pageMetadata.viewCount`,
		`likeCount: pageMetadata.likeCount`,
		`function configuredVideoIdentityState()`,
		`function normalizedBangumiIdentity(value)`,
		`function bangumiIdentityFromPath(pathname)`,
		`function configuredIdentityFromPath(pathname)`,
		`function ensureConfiguredVideoIdentity()`,
		`function failConfiguredVideoIdentity()`,
		"function attachMedia(nextMedia) {\n    if (!ensureConfiguredVideoIdentity()) return;",
		`function installHistoryIdentityGuard()`,
		`historyPort.pushState = guardedPushState`,
		`historyPort.replaceState = guardedReplaceState`,
		`window.addEventListener("popstate", verifyCurrentIdentity, true)`,
		`window.addEventListener("pageshow", verifyCurrentIdentity, true)`,
		`post({ type: "identity-violation" })`,
		`if (disposed || !ensureConfiguredVideoIdentity()) return`,
		`mainFrame: window.top === window`,
		`window.location.origin !== "https://www.bilibili.com"`,
		`const ACTIVE_VIDEO_ATTRIBUTE = "data-xiadown-rss-bilibili-active-video"`,
		`body>*{visibility:hidden!important`,
		`video.setAttribute(ACTIVE_VIDEO_ATTRIBUTE, CONFIG.sessionId)`,
		`video.removeAttribute(ACTIVE_VIDEO_ATTRIBUTE)`,
		`position:fixed!important`,
		`z-index:2147483640!important`,
		`pointer-events:none`,
		`window.player`,
		`player.getSupportedQualityList()`,
		`player.getQuality()`,
		`port.player.requestQuality(quality)`,
		`player.danmaku`,
		`BILIBILI_SUBTITLE_ITEM_SELECTOR`,
		`media.textTracks`,
		`Player_Quality_Rendered`,
		`Player_Danmaku_Change`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("Bilibili bridge missing %q", required)
		}
	}
	if strings.Contains(script, `body *{display:none`) {
		t.Fatal("Bilibili full-page bridge must keep hidden page DOM alive")
	}
	for _, forbidden := range []string{
		"createPlayer(",
		"window.player =",
		"getAvailablePlaybackRates",
		"setPlaybackRate(",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("Bilibili bridge must not call unpublished player API %q", forbidden)
		}
	}
}

func TestRSSBilibiliBridgeIsValidJavaScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "session-1",
		Adapter:         "video",
		PlatformVideoID: "BV1xx411c7mD",
		Volume:          1,
		PlaybackRate:    1,
		Autoplay:        true,
	})
	command := exec.Command(node, "--check")
	command.Stdin = strings.NewReader(script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bridge JavaScript syntax: %v\n%s", err, output)
	}
}

func TestRSSBilibiliBridgeRuntimeCapabilitiesAndCommands(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	bridge := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "session-1",
		Adapter:         "video",
		PlatformVideoID: "BV1xx411c7mD",
		Volume:          1,
		PlaybackRate:    1,
		Autoplay:        true,
	})
	staleQuality := rssBilibiliStringCommandScript("stale-session", "selectQuality", "116")
	activeCurrentQuality := rssBilibiliStringCommandScript("session-1", "selectQuality", "80")
	activeQuality := rssBilibiliStringCommandScript("session-1", "selectQuality", "116")
	runtime := `
const assert = require("node:assert/strict");
globalThis.window = globalThis;
window.top = window;
window.location = {
  origin: "https://www.bilibili.com",
  pathname: "/video/BV1xx411c7mD/",
  search: "",
};
Object.defineProperty(window.location, "href", {
  get() { return window.location.origin + window.location.pathname + window.location.search; },
});
window.history = {
  pushState(_state, _unused, target) {
    if (target == null) return;
    const resolved = new URL(String(target), window.location.href);
    window.location.pathname = resolved.pathname;
    window.location.search = resolved.search;
  },
  replaceState(_state, _unused, target) {
    if (target == null) return;
    const resolved = new URL(String(target), window.location.href);
    window.location.pathname = resolved.pathname;
    window.location.search = resolved.search;
  },
};
window.setInterval = () => 1;
window.clearInterval = () => {};
window.setTimeout = (callback) => { callback(); return 1; };
window.clearTimeout = () => {};
window.requestAnimationFrame = (callback) => callback();

class FakeMutationObserver {
  constructor(callback) { this.callback = callback; }
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = FakeMutationObserver;

class FakeEventTarget {
  constructor() { this.listeners = new Map(); }
  addEventListener(name, listener) { this.listeners.set(name, listener); }
  removeEventListener(name) { this.listeners.delete(name); }
}

class FakeClassList {
  constructor(...names) { this.names = new Set(names); }
  contains(name) { return this.names.has(name); }
  add(name) { this.names.add(name); }
  remove(name) { this.names.delete(name); }
}

class FakeNode extends FakeEventTarget {
  constructor({ attrs = {}, classes = [], text = "", click = null } = {}) {
    super();
    this.attrs = { ...attrs };
    this.classList = new FakeClassList(...classes);
    this.textContent = text;
    this.isConnected = true;
    this.onClick = click;
    this.labelNode = null;
  }
  getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  removeAttribute(name) { delete this.attrs[name]; }
  matches(selector) {
    if (selector.includes("[disabled]") && this.disabled) return true;
    if (selector.includes("[aria-disabled='true']") && this.attrs["aria-disabled"] === "true") return true;
    return false;
  }
  closest(selector) {
    if (selector === ".bui-disabled") return this.disabled ? this : null;
    if (selector === "a[href]") return null;
    if (selector.includes("bpx-state-active") || selector.includes("bui-checked")) {
      return this.classList.contains("bpx-state-active") || this.classList.contains("bui-checked") ? this : null;
    }
    return null;
  }
  querySelector(selector) {
    return selector === ".bpx-player-ctrl-subtitle-language-item-text" ? this.labelNode : null;
  }
  click() { if (this.onClick) this.onClick(); }
}

class FakeTextTrackList extends Array {
  constructor(...tracks) { super(...tracks); this.events = new FakeEventTarget(); }
  addEventListener(...args) { this.events.addEventListener(...args); }
  removeEventListener(...args) { this.events.removeEventListener(...args); }
}

class FakeVideo extends FakeNode {
  constructor() {
    super();
    this.tagName = "VIDEO";
    this.paused = true;
    this.ended = false;
    this.readyState = 4;
    this.duration = 120;
    this.currentTime = 0;
    this.volume = 1;
    this.muted = false;
    this.playbackRate = 1;
    this.seeking = false;
    this.error = null;
    this.buffered = { length: 0, end() { return 0; } };
    this.controls = false;
    this.playCalls = 0;
    this.textTracks = new FakeTextTrackList(
      { id: "en", language: "en", label: "English", mode: "disabled" },
      { id: "ja", language: "ja", label: "日本語", mode: "disabled" },
    );
  }
  play() { this.playCalls += 1; this.paused = false; return Promise.resolve(); }
  pause() { this.paused = true; }
}

const video = new FakeVideo();
let currentMedia = video;
let captionDOMAvailable = true;
let closeActive = true;
const captionEntries = ["zh-CN", "en-US"].map((language) => {
  const entry = new FakeNode({ attrs: { "data-lan": language } });
  entry.labelNode = new FakeNode({ text: language === "zh-CN" ? "中文" : "English" });
  entry.onClick = () => {
    closeActive = false;
    captionEntries.forEach((candidate) => candidate.classList.remove("bpx-state-active"));
    entry.classList.add("bpx-state-active");
  };
  return entry;
});
const captionClose = new FakeNode({ click: () => {
  closeActive = true;
  captionEntries.forEach((entry) => entry.classList.remove("bpx-state-active"));
} });
Object.defineProperty(captionClose, "classList", { value: {
  contains(name) { return name === "bpx-state-active" && closeActive; }
} });

let danmakuDOMClicks = 0;
const danmakuInput = new FakeNode({ click: () => { danmakuDOMClicks += 1; danmakuInput.checked = !danmakuInput.checked; } });
danmakuInput.checked = true;

const styles = new Map();
const head = { appendChild(node) { styles.set(node.id, node); node.isConnected = true; } };
const metadataScript = new FakeNode({ text: JSON.stringify({
  "@context": "https://schema.org",
  "@type": "VideoObject",
  author: { "@type": "Person", name: "Canonical publisher" },
  uploadDate: "2026-07-11T08:30:00+08:00",
  interactionStatistic: [
    {
      "@type": "InteractionCounter",
      interactionType: { "@type": "WatchAction" },
      userInteractionCount: "1250400",
    },
    {
      "@type": "InteractionCounter",
      interactionType: { "@type": "LikeAction" },
      userInteractionCount: 48200,
    },
  ],
}) });
const metadataNodes = new Map([
  ['meta[itemprop="author"][content]', new FakeNode({ attrs: { content: "DOM publisher" } })],
  ['meta[itemprop="uploadDate"][content]', new FakeNode({ attrs: { content: "" } })],
  ['meta[itemprop="datePublished"][content]', new FakeNode({ attrs: { content: "" } })],
  [".pubdate-ip-text", new FakeNode({ text: "发布于 2026-07-14 09:30" })],
  [".view-text", new FakeNode({ text: "1.2万" })],
  [".video-like-info.video-toolbar-item-text", new FakeNode({ text: "345" })],
]);
globalThis.document = new FakeEventTarget();
document.readyState = "complete";
document.title = "Bilibili test";
document.head = head;
document.documentElement = new FakeNode();
document.fullscreenElement = null;
document.webkitFullscreenElement = null;
document.getElementById = (id) => styles.get(id) || null;
document.createElement = () => new FakeNode();
document.querySelectorAll = (selector) => {
  if (selector === "video") return [currentMedia];
  if (selector === 'script[type="application/ld+json"]') return [metadataScript];
  if (selector.includes("bpx-player-ctrl-subtitle-language-item")) return captionDOMAvailable ? captionEntries : [];
  if (selector.includes("bpx-player-ctrl-quality-menu-item")) return [];
  return [];
};
document.querySelector = (selector) => {
  if (metadataNodes.has(selector)) return metadataNodes.get(selector);
  if (selector === ".bpx-player-ctrl-subtitle-close-switch") return captionDOMAvailable ? captionClose : null;
  if (selector.includes("bui-danmaku-switch-input")) return danmakuInput;
  return null;
};

const messages = [];
window._wails = { invoke(message) { messages.push(JSON.parse(message)); } };
const state = () => messages.filter((message) => message.type === "state").at(-1);

let currentQuality = 80;
const qualityRequests = [];
let rejectQuality = false;
let danmakuOpen = true;
const playerEvents = new Map();
let oldPlayerOffCount = 0;
window.nano = { EventType: {
  Player_Initialized: "initialized",
  Player_Connected: "connected",
  Player_LoadedMetadata: "metadata",
  Player_Quality_Changed: "quality-changed",
  Player_Quality_Rendered: "quality-rendered",
  Player_Danmaku_Change: "danmaku-changed",
} };
window.player = {
  mediaElement() { return currentMedia; },
  isInitialized() { return true; },
  on(name, listener) { playerEvents.set(name, listener); },
  off(name) { oldPlayerOffCount += 1; playerEvents.delete(name); },
  getSupportedQualityList() { return [64, 80, 116]; },
  getQuality() { return { nowQ: currentQuality, realQ: currentQuality }; },
  requestQuality(quality) {
    qualityRequests.push(quality);
    if (rejectQuality) return Promise.reject(new Error("permission denied"));
    currentQuality = quality;
    return Promise.resolve();
  },
  danmaku: {
    isOpen() { return danmakuOpen; },
    isDisabled() { return false; },
    open() { danmakuOpen = true; },
    close() { danmakuOpen = false; },
  },
};
`
	assertions := `
assert.equal(state().controls.quality, true);
assert.equal(video.playCalls, 1, "the prepared session may autoplay its first media element once");
assert.equal(state().controls.captions, true);
assert.equal(state().controls.danmaku, true);
assert.equal(state().publisher, "Canonical publisher");
assert.equal(state().publishedAt, "2026-07-11T08:30:00+08:00");
assert.equal(state().viewCount, 1250400);
assert.equal(state().likeCount, 48200);
assert.deepEqual(state().qualityOptions.map((entry) => entry.id), ["64", "80", "116"]);
assert.deepEqual(state().captionOptions.map((entry) => entry.id), ["bili:zh-CN", "bili:en-US"]);
assert.equal(state().selections.captionId, "");

metadataScript.textContent = "{}";
window.__xiadownRSSBilibiliPlayer.reconcile();
assert.equal(state().publisher, "DOM publisher");
assert.equal(state().publishedAt, "2026-07-14T09:30");
assert.equal(state().viewCount, 12000);
assert.equal(state().likeCount, 345);

window.__xiadownRSSBilibiliPlayer.toggleCaptions();
assert.equal(state().selections.captionId, "bili:zh-CN");
window.__xiadownRSSBilibiliPlayer.selectCaption("bili:en-US");
assert.equal(state().selections.captionId, "bili:en-US");
window.__xiadownRSSBilibiliPlayer.toggleCaptions();
assert.equal(state().selections.captionId, "");

captionDOMAvailable = false;
window.__xiadownRSSBilibiliPlayer.reconcile();
const textTrackID = state().captionOptions[0].id;
assert.match(textTrackID, /^track:0:/);
window.__xiadownRSSBilibiliPlayer.selectCaption(textTrackID);
assert.equal(video.textTracks[0].mode, "showing");
assert.equal(state().selections.captionId, textTrackID);
window.__xiadownRSSBilibiliPlayer.selectCaption("");
assert.equal(video.textTracks[0].mode, "disabled");
assert.equal(state().selections.captionId, "");

window.__xiadownRSSBilibiliPlayer.toggleDanmaku();
assert.equal(danmakuOpen, false);
window.player.danmaku = null;
window.__xiadownRSSBilibiliPlayer.reconcile();
window.__xiadownRSSBilibiliPlayer.toggleDanmaku();
assert.equal(danmakuDOMClicks, 1);
assert.equal(danmakuInput.checked, false);

` + staleQuality + `
assert.deepEqual(qualityRequests, []);
` + activeCurrentQuality + `
assert.deepEqual(qualityRequests, []);
` + activeQuality + `
assert.deepEqual(qualityRequests, [116]);
Promise.resolve().then(() => {
  assert.equal(state().selections.qualityId, "116");
  const messagesBeforeRejectedQuality = messages.length;
  rejectQuality = true;
  window.__xiadownRSSBilibiliPlayer.selectQuality("64");
  return Promise.resolve().then(() => {
    assert.deepEqual(qualityRequests, [116, 64]);
    assert.equal(state().selections.qualityId, "116");
    assert.ok(messages.length > messagesBeforeRejectedQuality);
    let replacementBindings = 0;
    window.player = {
      ...window.player,
      on() { replacementBindings += 1; },
      off() {},
    };
    window.__xiadownRSSBilibiliPlayer.reconcile();
    assert.ok(oldPlayerOffCount > 0);
    assert.ok(replacementBindings > 0);

    const qualityReplacement = new FakeVideo();
    currentMedia = qualityReplacement;
    window.__xiadownRSSBilibiliPlayer.reconcile();
    assert.equal(
      qualityReplacement.playCalls,
      0,
      "quality/media-node replacement must not replay host autoplay",
    );

    const endedCapture = document.listeners.get("ended");
    assert.equal(typeof endedCapture, "function");
    endedCapture({
      target: qualityReplacement,
      preventDefault() {},
      stopImmediatePropagation() {},
    });
    assert.equal(qualityReplacement.paused, true);
    assert.equal(state().state, "ended");

    const postEndedReplacement = new FakeVideo();
    currentMedia = postEndedReplacement;
    window.__xiadownRSSBilibiliPlayer.reconcile();
    assert.equal(postEndedReplacement.paused, true);
    assert.equal(postEndedReplacement.playCalls, 0);
    assert.equal(state().state, "ended", "terminal state survives a Bilibili media rebuild");

    window.__xiadownRSSBilibiliPlayer.play();
    assert.equal(postEndedReplacement.currentTime, 0);
    assert.equal(postEndedReplacement.playCalls, 1, "only an explicit replay clears terminal state");
    postEndedReplacement.listeners.get("playing")();
    assert.equal(state().state, "playing");

    window.history.replaceState({}, "", "/video/bv1xx411c7mD/?p=1");
    window.__xiadownRSSBilibiliPlayer.reconcile();
    assert.equal(state().available, true);
    assert.notEqual(state().state, "error");

    window.history.pushState({}, "", "?p=2&spm_id_from=333");
    window.__xiadownRSSBilibiliPlayer.reconcile();
    assert.equal(state().available, true);
    assert.notEqual(state().state, "error");

    currentMedia.paused = false;
    const messagesBeforeIdentityFailure = messages.length;
    window.history.replaceState({}, "", "/video/BV1xx411c7mM/");
    assert.ok(messages.length > messagesBeforeIdentityFailure);
    assert.equal(window.location.pathname, "/video/bv1xx411c7mD/");
    assert.equal(messages.filter((message) => message.type === "identity-violation").length, 1);
    const failureMessages = messages.slice(messagesBeforeIdentityFailure);
    assert.ok(
      failureMessages.findIndex((message) => message.type === "state" && message.state === "error") <
        failureMessages.findIndex((message) => message.type === "identity-violation"),
      "the renderer must observe unavailable/error before native session teardown",
    );
    assert.equal(currentMedia.paused, true);
    assert.equal(state().available, false);
    assert.equal(state().state, "error");
    assert.equal(state().publisher, "");
    assert.equal(state().publishedAt, "");
    assert.equal(state().viewCount, 0);
    assert.equal(state().likeCount, 0);
    assert.equal(state().controls.playPause, false);
    const messagesAfterIdentityFailure = messages.length;
    window.__xiadownRSSBilibiliPlayer.play();
    window.__xiadownRSSBilibiliPlayer.seek(30);
    window.__xiadownRSSBilibiliPlayer.snapshot();
    window.__xiadownRSSBilibiliPlayer.reconcile();
    assert.equal(messages.length, messagesAfterIdentityFailure);
    window.__xiadownRSSBilibiliPlayer.dispose();
  });
});
`
	command := exec.Command(node)
	command.Stdin = strings.NewReader(runtime + "\n" + bridge + "\n" + assertions)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bridge runtime contract: %v\n%s", err, output)
	}
}

func TestRSSBilibiliBangumiBridgePublishesReadyMediaStateAndLocksIdentity(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	bridge := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "session-bangumi",
		Adapter:         "bangumi",
		PlatformVideoID: "ep3854807",
		Volume:          0.75,
		PlaybackRate:    1,
		Autoplay:        false,
	})
	runtime := `
const assert = require("node:assert/strict");
globalThis.window = globalThis;
window.top = window;
window.location = {
  origin: "https://www.bilibili.com",
  pathname: "/bangumi/play/ep3854807/",
  search: "",
};
Object.defineProperty(window.location, "href", {
  get() { return window.location.origin + window.location.pathname + window.location.search; },
});
window.history = {
  pushState(_state, _unused, target) {
    if (target == null) return;
    const resolved = new URL(String(target), window.location.href);
    window.location.pathname = resolved.pathname;
    window.location.search = resolved.search;
  },
  replaceState(_state, _unused, target) {
    if (target == null) return;
    const resolved = new URL(String(target), window.location.href);
    window.location.pathname = resolved.pathname;
    window.location.search = resolved.search;
  },
};
const windowListeners = new Map();
window.addEventListener = (name, listener) => { windowListeners.set(name, listener); };
window.removeEventListener = (name) => { windowListeners.delete(name); };
window.setInterval = () => 1;
window.clearInterval = () => {};
window.setTimeout = () => 1;
window.clearTimeout = () => {};
window.requestAnimationFrame = () => {};

class FakeMutationObserver {
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = FakeMutationObserver;

class FakeEventTarget {
  constructor() { this.listeners = new Map(); }
  addEventListener(name, listener) { this.listeners.set(name, listener); }
  removeEventListener(name) { this.listeners.delete(name); }
}

class FakeNode extends FakeEventTarget {
  constructor() {
    super();
    this.attrs = {};
    this.isConnected = true;
    this.textContent = "";
  }
  getAttribute(name) { return this.attrs[name] ?? null; }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  removeAttribute(name) { delete this.attrs[name]; }
}

class FakeVideo extends FakeNode {
  constructor() {
    super();
    this.tagName = "VIDEO";
    this.paused = true;
    this.ended = false;
    this.readyState = 4;
    this.duration = 1450;
    this.currentTime = 12;
    this.volume = 1;
    this.muted = false;
    this.playbackRate = 1;
    this.seeking = false;
    this.error = null;
    this.buffered = { length: 1, end() { return 60; } };
    this.controls = true;
    this.textTracks = null;
  }
  play() { this.paused = false; return Promise.resolve(); }
  pause() { this.paused = true; }
}

const video = new FakeVideo();
const styles = new Map();
globalThis.document = new FakeEventTarget();
document.readyState = "complete";
document.title = "Bangumi episode";
document.documentElement = new FakeNode();
document.head = { appendChild(node) { styles.set(node.id, node); } };
document.getElementById = (id) => styles.get(id) || null;
document.createElement = () => new FakeNode();
document.querySelector = () => null;
document.querySelectorAll = (selector) => selector === "video" ? [video] : [];
document.fullscreenElement = null;
document.webkitFullscreenElement = null;

const messages = [];
window._wails = { invoke(message) { messages.push(JSON.parse(message)); } };
const states = () => messages.filter((message) => message.type === "state");
const state = () => states().at(-1);
`
	assertions := `
assert.ok(window.__xiadownRSSBilibiliPlayer);
assert.equal(state().platformVideoId, "ep3854807");
assert.equal(state().available, true);
assert.equal(state().state, "paused");
assert.equal(state().currentTime, 12);
assert.equal(state().duration, 1450);
assert.equal(state().controls.playPause, true);
assert.equal(video.getAttribute("data-xiadown-rss-bilibili-active-video"), "session-bangumi");

const statesBeforeCanPlay = states().length;
video.listeners.get("canplay")();
assert.ok(states().length > statesBeforeCanPlay, "canplay must publish a ready media snapshot");
assert.equal(state().available, true);
assert.equal(state().state, "paused");

window.history.replaceState({}, "", "/bangumi/play/EP3854807/?from=episode");
window.__xiadownRSSBilibiliPlayer.reconcile();
assert.equal(state().available, true);
assert.notEqual(state().state, "error");

const messagesBeforeEscape = messages.length;
window.history.pushState({}, "", "/bangumi/play/ss3854807/");
assert.equal(window.location.pathname, "/bangumi/play/EP3854807/");
assert.ok(messages.length > messagesBeforeEscape);
assert.equal(messages.filter((message) => message.type === "identity-violation").length, 1);
assert.equal(state().available, false);
assert.equal(state().state, "error");
assert.equal(video.paused, true);
`
	command := exec.Command(node)
	command.Stdin = strings.NewReader(runtime + "\n" + bridge + "\n" + assertions)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Bangumi bridge runtime contract: %v\n%s", err, output)
	}
}

func TestRSSBilibiliBridgeRejectsPathForWrongAdapterBeforeDOMAccess(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	tests := []struct {
		name            string
		adapter         string
		platformVideoID string
		pathname        string
	}{
		{
			name:            "video adapter on Bangumi path",
			adapter:         "video",
			platformVideoID: "BV1xx411c7mD",
			pathname:        "/bangumi/play/ep3854807/",
		},
		{
			name:            "Bangumi adapter on video path",
			adapter:         "bangumi",
			platformVideoID: "ep3854807",
			pathname:        "/video/BV1xx411c7mD/",
		},
		{
			name:            "unknown adapter",
			adapter:         "",
			platformVideoID: "ep3854807",
			pathname:        "/bangumi/play/ep3854807/",
		},
		{
			name:            "Bangumi child path",
			adapter:         "bangumi",
			platformVideoID: "ep3854807",
			pathname:        "/bangumi/play/ep3854807/comments",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bridge := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
				SessionID:       "wrong-adapter",
				Adapter:         test.adapter,
				PlatformVideoID: test.platformVideoID,
				Volume:          1,
				PlaybackRate:    1,
			})
			runtime := fmt.Sprintf(`
const assert = require("node:assert/strict");
globalThis.window = globalThis;
window.top = window;
window.location = { origin: "https://www.bilibili.com", pathname: %q };
`, test.pathname)
			command := exec.Command(node)
			command.Stdin = strings.NewReader(runtime + "\n" + bridge + `
assert.equal(window.__xiadownRSSBilibiliPlayer, undefined);
`)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("wrong adapter path guard: %v\n%s", err, output)
			}
		})
	}
}

func TestRSSBilibiliBridgeLocksAVIdentityToExactCanonicalPath(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	bridge := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "session-av",
		Adapter:         "video",
		PlatformVideoID: "av170001",
		Volume:          1,
		PlaybackRate:    1,
		Autoplay:        false,
	})
	runtime := `
const assert = require("node:assert/strict");
globalThis.window = globalThis;
window.top = window;
window.location = {
  origin: "https://www.bilibili.com",
  pathname: "/video/av170001/",
  search: "",
};
Object.defineProperty(window.location, "href", {
  get() { return window.location.origin + window.location.pathname + window.location.search; },
});
window.history = { pushState() {}, replaceState() {} };
const windowListeners = new Map();
window.addEventListener = (name, listener) => { windowListeners.set(name, listener); };
window.removeEventListener = (name) => { windowListeners.delete(name); };
window.__INITIAL_STATE__ = {};
window.setInterval = () => 1;
window.clearInterval = () => {};
window.setTimeout = () => 1;
window.clearTimeout = () => {};
window.requestAnimationFrame = () => {};

class FakeMutationObserver {
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = FakeMutationObserver;

class FakeEventTarget {
  addEventListener() {}
  removeEventListener() {}
}

class FakeNode extends FakeEventTarget {
  constructor() {
    super();
    this.attrs = {};
    this.isConnected = true;
    this.textContent = "";
  }
  getAttribute(name) { return this.attrs[name] ?? null; }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  removeAttribute(name) { delete this.attrs[name]; }
}

const styles = new Map();
globalThis.document = new FakeEventTarget();
document.readyState = "complete";
document.title = "Bilibili exact av test";
document.documentElement = new FakeNode();
document.head = { appendChild(node) { styles.set(node.id, node); } };
document.getElementById = (id) => styles.get(id) || null;
document.createElement = () => new FakeNode();
document.querySelector = () => null;
document.querySelectorAll = () => [];
document.fullscreenElement = null;
document.webkitFullscreenElement = null;

window.nano = { EventType: {} };
window.player = { isInitialized() { return true; } };
const messages = [];
window._wails = { invoke(message) { messages.push(JSON.parse(message)); } };
const state = () => messages.filter((message) => message.type === "state").at(-1);
`
	assertions := `
assert.ok(window.__xiadownRSSBilibiliPlayer);
assert.equal(state().state, "loading");
assert.equal(state().platformVideoId, "av170001");

window.location.search = "?p=2&spm_id_from=333";
windowListeners.get("popstate")();
assert.notEqual(state().state, "error");

window.location.pathname = "/video/BV17x411w7KC/";
window.__INITIAL_STATE__ = {
  aid: 170001,
  bvid: "BV17x411w7KC",
  videoData: { aid: 170001, bvid: "BV17x411w7KC" },
};
windowListeners.get("pageshow")();
assert.equal(messages.filter((message) => message.type === "identity-violation").length, 1);
assert.equal(state().available, false);
assert.equal(state().state, "error");
assert.equal(state().publisher, "");
assert.equal(state().viewCount, 0);
`
	command := exec.Command(node)
	command.Stdin = strings.NewReader(runtime + "\n" + bridge + "\n" + assertions)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("exact canonical av identity contract: %v\n%s", err, output)
	}
}

func TestRSSBilibiliBridgeRejectsNonCanonicalDocumentBeforeDOMAccess(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	bridge := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "session-1",
		Adapter:         "video",
		PlatformVideoID: "BV1xx411c7mD",
		Volume:          1,
		PlaybackRate:    1,
	})
	runtime := `
const assert = require("node:assert/strict");
globalThis.window = globalThis;
window.top = window;
window.location = { origin: "https://attacker.example", pathname: "/video/BV1xx411c7mD/" };
` + bridge + `
assert.equal(window.__xiadownRSSBilibiliPlayer, undefined);
`
	command := exec.Command(node)
	command.Stdin = strings.NewReader(runtime)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("non-canonical bridge guard: %v\n%s", err, output)
	}
}

func TestRSSBilibiliBridgeDefaultsAndCommandSessionGuards(t *testing.T) {
	config, err := normalizeRSSBilibiliBridgeConfig(RSSVideoPlayerPrepareRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Volume != 1 || !config.Autoplay || config.PlaybackRate != 1 {
		t.Fatalf("default bridge config = %#v", config)
	}
	volume := 2.0
	config, err = normalizeRSSBilibiliBridgeConfig(RSSVideoPlayerPrepareRequest{StartSeconds: -10, Volume: &volume})
	if err != nil || config.StartSeconds != 0 || config.Volume != 1 {
		t.Fatalf("normalized bridge config = %#v, %v", config, err)
	}
	if _, err := normalizeRSSBilibiliBridgeConfig(RSSVideoPlayerPrepareRequest{StartSeconds: math.NaN()}); err == nil {
		t.Fatal("non-finite start position must be rejected")
	}
	for _, script := range []string{
		rssBilibiliSimpleCommandScript("session-1", "play"),
		rssBilibiliSimpleCommandScript("session-1", "toggleCaptions"),
		rssBilibiliNumberCommandScript("session-1", "seek", 12),
		rssBilibiliVolumeCommandScript("session-1", 0.5, true),
		rssBilibiliStringCommandScript("session-1", "selectCaption", ""),
		rssBilibiliStringCommandScript("session-1", "selectQuality", "80"),
	} {
		if !strings.Contains(script, `p.sessionId==="session-1"`) {
			t.Fatalf("command lacks session guard: %s", script)
		}
	}
	if value, err := normalizeRSSBilibiliCaptionSelectionValue(""); err != nil || value != "" {
		t.Fatalf("caption Off selection = %q, %v", value, err)
	}
	if _, err := normalizeRSSBilibiliSelectionValue("quality", ""); err == nil {
		t.Fatal("empty quality selection must be rejected")
	}
	encoded, err := json.Marshal(newRSSVideoPlayerStatus())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"captionId":""`) {
		t.Fatalf("caption Off must remain an explicit empty status selection: %s", encoded)
	}
}

func TestRSSBilibiliRawMessageOriginPolicy(t *testing.T) {
	tests := []struct {
		name       string
		originInfo *application.OriginInfo
		marker     bool
		want       bool
	}{
		{
			name:       "macOS canonical main frame URL",
			originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com/video/BV1xx411c7mD/", IsMainFrame: true},
			marker:     true,
			want:       true,
		},
		{
			name:       "WebView2 equivalent main frame proof",
			originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com/video/BV1xx411c7mD/", TopOrigin: "https://www.bilibili.com/video/BV1xx411c7mD/"},
			marker:     true,
			want:       true,
		},
		{name: "missing origin", originInfo: nil, marker: true},
		{name: "about blank bootstrap", originInfo: &application.OriginInfo{Origin: "about:blank", IsMainFrame: true}, marker: true},
		{name: "subframe", originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com", IsMainFrame: false}, marker: true},
		{name: "bridge subframe marker", originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com", IsMainFrame: true}, marker: false},
		{name: "external-player origin", originInfo: &application.OriginInfo{Origin: "https://player.bilibili.com", IsMainFrame: true}, marker: true},
		{name: "lookalike origin", originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com.attacker.example", IsMainFrame: true}, marker: true},
		{name: "wrong top origin", originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com", TopOrigin: "https://player.bilibili.com", IsMainFrame: true}, marker: true},
		{name: "non default port", originInfo: &application.OriginInfo{Origin: "https://www.bilibili.com:8443", IsMainFrame: true}, marker: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rssBilibiliRawMessageOriginTrusted(test.originInfo, test.marker); got != test.want {
				t.Fatalf("trusted = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRSSBilibiliRawMessageHandlerChecksEveryIdentityBoundary(t *testing.T) {
	source, err := os.ReadFile("rss_video_player_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`windowName != rssBilibiliPlayerWindowName`,
		`listenPayloadString(payload, "source") != rssBilibiliPlayerSource`,
		`rssBilibiliRawMessageOriginTrusted(originInfo, listenPayloadBool(payload, "mainFrame"))`,
		`windowID != activeWindow.ID()`,
		`sessionID == "" || sessionID != activeSessionID`,
		`originInfo.IsMainFrame`,
		`topOrigin != "" && topOrigin == origin`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("raw-message identity boundary is missing %q", required)
		}
	}
	limitCheck := strings.Index(text, "len(message) > maxRSSBilibiliRawMessageBytes")
	decode := strings.Index(text, "json.Unmarshal([]byte(message), &payload)")
	if limitCheck < 0 || decode < 0 || limitCheck > decode {
		t.Fatal("raw-message size boundary must run before JSON decoding")
	}
}

func TestRSSBilibiliPlaybackStatusAndStaleCloseAreSessionBound(t *testing.T) {
	player := &rssBilibiliVideoPlayer{status: newRSSVideoPlayerStatus()}
	player.status.SessionID = "new-session"
	player.status.Available = true
	player.status.PlatformVideoID = "BV1xx411c7mD"
	player.updatePlaybackStatus("old-session", map[string]any{
		"state":       "playing",
		"publisher":   "Stale publisher",
		"publishedAt": "2020-01-01T00:00:00Z",
		"viewCount":   99.0,
		"likeCount":   88.0,
		"currentTime": 99.0,
		"available":   true,
	})
	if player.status.State != "idle" || player.status.CurrentTime != 0 ||
		player.status.Publisher != "" || player.status.ViewCount != 0 {
		t.Fatalf("stale state mutated player: %#v", player.status)
	}
	if player.closeCurrentWindow("old-session") {
		t.Fatal("stale close unexpectedly closed current session")
	}
	if player.status.SessionID != "new-session" {
		t.Fatalf("stale close reset current session: %#v", player.status)
	}

	player.updatePlaybackStatus("new-session", map[string]any{
		"state":        "playing",
		"title":        "Bilibili title",
		"publisher":    "Canonical publisher",
		"publishedAt":  "2026-07-11T08:30:00+08:00",
		"viewCount":    1250400.0,
		"likeCount":    48200.0,
		"currentTime":  12.5,
		"duration":     100.0,
		"bufferedTime": 44.0,
		"volume":       0.4,
		"muted":        true,
		"playbackRate": 1.5,
		"available":    true,
		"captionOptions": []any{
			map[string]any{"id": "bili:zh-CN", "label": "中文"},
		},
		"qualityOptions": []any{
			map[string]any{"id": "80", "label": "1080P 高清"},
		},
		"danmakuEnabled": true,
		"selections": map[string]any{
			"captionId": "bili:zh-CN",
			"qualityId": "80",
		},
		"controls": map[string]any{
			"playPause":    true,
			"seek":         true,
			"volume":       true,
			"playbackRate": true,
			"fullscreen":   true,
			"captions":     true,
			"quality":      true,
			"danmaku":      true,
		},
	})
	status := player.Status()
	if status.State != "playing" || status.Title != "Bilibili title" ||
		status.Publisher != "Canonical publisher" || status.PublishedAt != "2026-07-11T08:30:00+08:00" ||
		status.ViewCount != 1250400 || status.LikeCount != 48200 ||
		status.CurrentTime != 12.5 || status.Duration != 100 || status.BufferedTime != 44 ||
		status.Volume != 0.4 || !status.Muted || status.PlaybackRate != 1.5 ||
		status.Selections.PlaybackRateID != "1.5" || !status.Controls.PlayPause ||
		!status.Controls.Seek || !status.Controls.Volume || !status.Controls.PlaybackRate ||
		!status.Controls.Fullscreen || !status.Controls.Captions || !status.Controls.Quality || !status.Controls.Danmaku ||
		status.Selections.CaptionID != "bili:zh-CN" || status.Selections.QualityID != "80" ||
		len(status.CaptionOptions) != 1 || len(status.QualityOptions) != 1 || !status.DanmakuEnabled ||
		len(status.PlaybackRateOptions) != 6 {
		t.Fatalf("unexpected playback status: %#v", status)
	}
	player.updatePlaybackStatus("new-session", map[string]any{
		"publisher":   "",
		"publishedAt": "",
		"viewCount":   0.0,
		"likeCount":   0.0,
	})
	if cleared := player.Status(); cleared.Publisher != "" || cleared.PublishedAt != "" ||
		cleared.ViewCount != 0 || cleared.LikeCount != 0 {
		t.Fatalf("missing page metadata was not cleared: %#v", cleared)
	}
	player.updateFullscreenStatus("new-session", true)
	if !player.Status().Fullscreen {
		t.Fatal("fullscreen state was not recorded")
	}
	player.fullscreenTransition = true
	player.updatePlaybackStatus("new-session", map[string]any{"fullscreen": false})
	if !player.Status().Fullscreen || !player.fullscreenTransition {
		t.Fatal("periodic media snapshot ended the authoritative fullscreen transition")
	}
	player.updateFullscreenStatus("old-session", false)
	if !player.Status().Fullscreen {
		t.Fatal("stale session ended the active fullscreen presentation")
	}
	player.updateFullscreenStatus("new-session", false)
	if player.Status().Fullscreen || player.fullscreenTransition {
		t.Fatal("authoritative fullscreen exit did not clear transition state")
	}
}

func TestRSSBilibiliFullscreenScriptCarriesSessionIdentity(t *testing.T) {
	script := listenEmbeddedVideoFullscreenScriptForSession(
		rssBilibiliPlayerSource,
		"rss-session-1",
		7,
		true,
	)
	for _, required := range []string{
		`const SESSION_ID = "rss-session-1"`,
		`sessionId: SESSION_ID`,
		`mainFrame: window.top === window`,
		`const SOURCE = "rss-bilibili-video-player"`,
		`const isRSSBilibiliPage =`,
		`(isYouTubeStationWatchPage || isRSSBilibiliPage)`,
		`? document.documentElement`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("session fullscreen script missing %q", required)
		}
	}
}

func TestRSSBilibiliNativeFullscreenMirrorsYouTubeOwnershipContract(t *testing.T) {
	source, err := os.ReadFile("rss_video_player_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		`listenEmbeddedVideoUsesNativeWindowFullscreen()`,
		`requestNativeWindowFullscreenLocked`,
		`handleNativeWindowFullscreenEvent`,
		`restoreAfterNativeWindowFullscreenLocked`,
		`player.fullscreenTransition = true`,
		`player.fullscreenGeneration += 1`,
		`waitForListenNativeWindowFullscreenState`,
		`window.RegisterHook(events.Common.WindowFullscreen`,
		`window.RegisterHook(events.Common.WindowUnFullscreen`,
		`installRSSVideoPlayerNativeFullscreenEscape(window)`,
		`case rssBilibiliFullscreenExitRequestType:`,
		`go func() { _ = player.ExitFullscreen(sessionID) }()`,
		`case rssBilibiliIdentityViolationType:`,
		`_ = player.Close(sessionID)`,
		`if !active && !nativeWindowFullscreen && window != nil`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("native fullscreen ownership contract missing %q", required)
		}
	}
	bridgeSource, err := os.ReadFile("rss_video_transport_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	bridgeText := string(bridgeSource)
	for _, required := range []string{
		`.bpx-player-container`,
		`.bpx-player-primary-area`,
		`.bpx-player-video-area`,
		`position:fixed!important;inset:0!important;width:100vw!important;height:100vh!important`,
		`.bpx-player-control-wrap`,
		`.bpx-player-control-bottom`,
		`opacity:1!important`,
		`.bpx-player-ctrl-full`,
		`.bpx-player-ctrl-web`,
		`.bpx-player-ctrl-next`,
		`.bpx-player-ctrl-setting-autoplay`,
		`display:none!important;visibility:hidden!important;pointer-events:none!important`,
		`.bpx-player-sending-area .bpx-player-dm-switch`,
		`function requestHostFullscreenExit(event)`,
		`post({ type: "fullscreen-exit-request" })`,
		`FULLSCREEN_STORAGE_KEY`,
		`restoreFullscreenPresentation()`,
		`let terminalEnded = false`,
		`let initialAutoplayAttempted = false`,
		`function blockAutomaticAdvance(event)`,
	} {
		if !strings.Contains(bridgeText, required) {
			t.Fatalf("fullscreen bridge presentation missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`document.addEventListener("keydown"`,
		`event.key === "Escape"`,
	} {
		if strings.Contains(bridgeText, forbidden) {
			t.Fatalf("Bilibili bridge must leave native Escape handling intact: %q", forbidden)
		}
	}

	darwinSource, err := os.ReadFile("listen_player_webview_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	darwinText := string(darwinSource)
	for _, required := range []string{
		`addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown`,
		`event.keyCode != 53`,
		`listenRSSBilibiliAttemptFullscreenEscapeExit`,
		`NSWindowStyleMaskFullScreen`,
		`NSWindowWillEnterFullScreenNotification`,
		`listenRSSBilibiliFullscreenEnteringGeneration`,
		`!fullscreen && !listenRSSBilibiliFullscreenEscapeEntering`,
		`[window toggleFullScreen:nil]`,
		`listenRSSBilibiliFullscreenEscapeGeneration`,
		`listenRemoveRSSBilibiliFullscreenEscapeMonitor`,
	} {
		if !strings.Contains(darwinText, required) {
			t.Fatalf("native Escape monitor missing %q", required)
		}
	}

	command := rssBilibiliBooleanCommandScript(
		"rss-session-1",
		"fullscreenPresentation",
		true,
	)
	for _, required := range []string{
		`p.sessionId==="rss-session-1"`,
		`p["fullscreenPresentation"](true)`,
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("fullscreen presentation command missing %q", required)
		}
	}
}
