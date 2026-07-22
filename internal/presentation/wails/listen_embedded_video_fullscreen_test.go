package wails

import (
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestListenEmbeddedVideoFullscreenScriptUsesPlayerAndNativeFallbacks(t *testing.T) {
	script := listenEmbeddedVideoFullscreenScript(listenPlayerSource, 17, true)
	for _, expected := range []string{
		`const ACTION = "enter"`,
		`fullscreenButton.click()`,
		`video.webkitEnterFullscreen()`,
		`target.requestFullscreen`,
		listenEmbeddedVideoFullscreenResultType,
		listenEmbeddedVideoFullscreenChangeType,
		`requestId: String(REQUEST_ID)`,
		`__xiadownEmbeddedVideoFullscreenMonitor`,
		`video?.addEventListener("webkitbeginfullscreen", webkitBegin, true)`,
		`video?.addEventListener("webkitendfullscreen", webkitEnd, true)`,
		`publish(true, "webkitbeginfullscreen")`,
		`publish(false, "webkitendfullscreen")`,
		`ignoreSnapshotExitUntil`,
		`authoritativeFullscreenActive = true`,
		`!explicitExit && authoritativeFullscreenActive`,
		`!authoritativeFullscreenActive && (lastActive || Date.now() < ignoreSnapshotExitUntil)`,
		`reportKnown(true, "requestfullscreen-resolved")`,
		`check(true)`,
		`ignoreSnapshotExitUntil = Math.max(`,
		`}, 3000);`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("fullscreen script missing %q", expected)
		}
	}
	enterIndex := strings.Index(script, `if (WANT_FULLSCREEN) {`)
	policyIndex := strings.Index(script, `const isYouTubeStationWatchPage =`)
	elementIndex := strings.Index(script, `const target = (isYouTubeStationWatchPage || isRSSBilibiliPage)`)
	fallbackIndex := strings.Index(script, `if (!elementRequested && !clickFullscreenButton()) {`)
	if policyIndex < 0 || enterIndex < policyIndex || elementIndex < enterIndex || fallbackIndex < elementIndex {
		t.Fatal("fullscreen script must choose a stable station target before the video-only fallback")
	}
	for _, expected := range []string{
		`SOURCE === "listen-youtube-live-player"`,
		`location.hostname === "www.youtube.com"`,
		`location.pathname === "/watch"`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("YouTube Station root target must be limited by %q", expected)
		}
	}
	if !strings.Contains(script, `const target = (isYouTubeStationWatchPage || isRSSBilibiliPage)`) ||
		!strings.Contains(script, `? document.documentElement`) ||
		!strings.Contains(script, `: (player || video || document.documentElement)`) {
		t.Fatal("YouTube watch pages must fullscreen the document root while Music and /embed keep their player target")
	}
	for _, expected := range []string{
		`SOURCE === "rss-bilibili-video-player"`,
		`location.hostname === "www.bilibili.com"`,
		`const isRSSBilibiliVideoPage =`,
		`const isRSSBilibiliBangumiPage =`,
		`/^\/video\/(?:BV[0-9A-Za-z]+|av[0-9]+)`,
		`/^\/bangumi\/play\/(?:ep|ss)[1-9][0-9]*`,
		`isRSSBilibiliVideoPage || isRSSBilibiliBangumiPage`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("RSS Bilibili root target must be limited by %q", expected)
		}
	}
	if !strings.Contains(script, `isYouTubeStationWatchPage && fullscreenTarget === document.documentElement`) {
		t.Fatal("a failed watch-page document exit must not click YouTube's player-fullscreen button")
	}
	if strings.Contains(script, `player.classList.contains("ytp-fullscreen")`) {
		t.Fatal("fullscreen script must not treat YouTube's in-document CSS state as native fullscreen")
	}
	if strings.Contains(script, `ignoreSnapshotExitUntil = now +`) {
		t.Fatal("a begin event must not shorten the pending fullscreen hand-off window")
	}
	if strings.Contains(script, `}, 450);`) || strings.Contains(script, `fallbackTimer`) {
		t.Fatal("a successful YouTube fullscreen-button hand-off must not trigger a second native enter request")
	}
	if strings.Count(script, `document.addEventListener("fullscreenchange"`) < 2 {
		t.Fatal("fullscreen script must keep a monitor after the one-shot request listener is removed")
	}
	exitBranchIndex := strings.Index(script, "  } else {\n    let nativeRequested = false;")
	if exitBranchIndex < 0 {
		t.Fatal("fullscreen script is missing its exit branch")
	}
	exitBranch := script[exitBranchIndex:]
	if strings.Index(exitBranch, `document.exitFullscreen`) > strings.Index(exitBranch, `video.webkitExitFullscreen()`) {
		t.Fatal("fullscreen exit must leave element fullscreen before trying the video-only API")
	}
}

func TestListenEmbeddedVideoFullscreenExitScriptUsesNativeFallbacks(t *testing.T) {
	script := listenEmbeddedVideoFullscreenScript(listenLivePlayerSource, 23, false)
	for _, expected := range []string{
		`const ACTION = "exit"`,
		`video.webkitExitFullscreen()`,
		`document.exitFullscreen`,
		`result.catch(fallbackExitToButton)`,
		listenLivePlayerSource,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("fullscreen exit script missing %q", expected)
		}
	}
}

func TestListenEmbeddedVideoFullscreenChangeOwnsGeometryUntilExit(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		playbackSessionID:            "youtube-session",
		embeddedFullscreenTransition: true,
	}
	if !player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": true,
	}) {
		t.Fatal("fullscreen change payload was not handled")
	}
	if !player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
		t.Fatal("enter event must transfer geometry ownership to fullscreen")
	}
	version := player.embeddedFullscreenVersion
	player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": false,
	})
	if player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
		t.Fatal("exit event must return geometry ownership to the inline surface")
	}
	if player.embeddedFullscreenVersion <= version {
		t.Fatal("fullscreen signals must invalidate stale command completions")
	}
}

func TestListenEmbeddedVideoFullscreenIgnoresInactiveSnapshotDuringEnter(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		playbackSessionID:            "youtube-session",
		embeddedFullscreenTransition: true,
	}
	version := player.embeddedFullscreenVersion
	if !player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": false,
		"reason": "fullscreenchange",
	}) {
		t.Fatal("fullscreen change payload was not handled")
	}
	if !player.embeddedFullscreenTransition || player.embeddedFullscreenActive {
		t.Fatal("a stale inactive snapshot must not return geometry during enter")
	}
	if player.embeddedFullscreenVersion != version {
		t.Fatal("an ignored inactive snapshot must not invalidate the enter request")
	}
}

func TestListenEmbeddedVideoFullscreenIgnoresInactivePollAfterEnter(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		playbackSessionID: "youtube-session",
	}
	if !player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": true,
		"reason": "fullscreenchange",
	}) {
		t.Fatal("fullscreen enter payload was not handled")
	}
	version := player.embeddedFullscreenVersion
	player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": false,
		"reason": "poll",
	})
	if !player.embeddedFullscreenActive {
		t.Fatal("a false poll must not end an active native fullscreen presentation")
	}
	if player.embeddedFullscreenVersion != version {
		t.Fatal("an ignored false poll must not invalidate fullscreen ownership")
	}
	player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": false,
		"reason": "fullscreenchange",
	})
	if player.embeddedFullscreenActive {
		t.Fatal("an explicit fullscreenchange exit must return inline ownership")
	}
}

func TestListenEmbeddedVideoFullscreenNativeStateOwnsEnterThroughExit(t *testing.T) {
	window := &application.WebviewWindow{}
	player := &ListenYouTubeLivePlayer{
		window:                       window,
		playbackSessionID:            "youtube-session",
		embeddedFullscreenMonitor:    7,
		embeddedFullscreenTransition: true,
	}
	changed, finished, _, sessionID, valid :=
		player.applyNativeEmbeddedVideoFullscreenState(window, 7, true)
	if !valid || !changed || finished || sessionID != "youtube-session" {
		t.Fatalf("native enter transition = (%v, %v, %q, %v)", changed, finished, sessionID, valid)
	}
	if !player.embeddedFullscreenActive || !player.embeddedFullscreenNativeSeen || player.embeddedFullscreenTransition {
		t.Fatal("native entering/in-fullscreen state must own presentation")
	}
	version := player.embeddedFullscreenVersion
	changed, finished, _, _, valid =
		player.applyNativeEmbeddedVideoFullscreenState(window, 7, true)
	if !valid || changed || finished || player.embeddedFullscreenVersion != version {
		t.Fatal("repeated native fullscreen state must be idempotent")
	}
	changed, finished, _, _, valid =
		player.applyNativeEmbeddedVideoFullscreenState(window, 7, false)
	if !valid || !changed || !finished {
		t.Fatal("native NotInFullscreen must finish the observed presentation")
	}
	if player.embeddedFullscreenActive || player.embeddedFullscreenNativeSeen || player.embeddedFullscreenTransition {
		t.Fatal("native exit completion must return inline ownership")
	}
	if _, _, _, _, valid = player.applyNativeEmbeddedVideoFullscreenState(window, 6, true); valid {
		t.Fatal("a stale native monitor must not mutate fullscreen state")
	}
}

func TestListenNativeWindowFullscreenEventConfirmsEnter(t *testing.T) {
	if !listenEmbeddedVideoUsesNativeWindowFullscreen() {
		t.Skip("native player-window fullscreen is not used on this platform")
	}
	window := &application.WebviewWindow{}
	waiter := make(chan bool, 1)
	player := &ListenYouTubeLivePlayer{
		window:                         window,
		playbackSessionID:              "youtube-session",
		embeddedFullscreenMonitor:      9,
		embeddedFullscreenTransition:   true,
		embeddedNativeWindowFullscreen: true,
		embeddedNativeFullscreenWaiter: waiter,
	}
	player.handleNativeWindowFullscreenEvent(window, true)
	if !player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
		t.Fatal("the native did-enter event must confirm fullscreen ownership")
	}
	select {
	case active := <-waiter:
		if !active {
			t.Fatal("the enter waiter received an exit signal")
		}
	default:
		t.Fatal("the native did-enter event did not complete the pending request")
	}

	staleWindow := &application.WebviewWindow{}
	player.embeddedFullscreenActive = false
	player.handleNativeWindowFullscreenEvent(staleWindow, true)
	if player.embeddedFullscreenActive {
		t.Fatal("a stale player-window event must not acquire fullscreen ownership")
	}
}

func TestListenNativeWindowFullscreenExitWaitsPastDelayedEnter(t *testing.T) {
	waiter := make(chan bool, 2)
	waiter <- true
	waiter <- false
	var unexpected []bool
	if !waitForListenNativeWindowFullscreenState(
		waiter,
		false,
		time.Second,
		func(active bool) { unexpected = append(unexpected, active) },
	) {
		t.Fatal("the false did-exit event was not observed")
	}
	if len(unexpected) != 1 || !unexpected[0] {
		t.Fatalf("unexpected states = %v, want delayed did-enter true", unexpected)
	}
}

func TestListenEmbeddedVideoFullscreenDefersJSExitToNativeWatcher(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		playbackSessionID:            "youtube-session",
		embeddedFullscreenActive:     true,
		embeddedFullscreenNativeSeen: true,
	}
	version := player.embeddedFullscreenVersion
	player.handlePlaybackPayload(map[string]any{
		"type":   listenEmbeddedVideoFullscreenChangeType,
		"active": false,
		"reason": "fullscreenchange",
	})
	if !player.embeddedFullscreenActive || !player.embeddedFullscreenNativeSeen {
		t.Fatal("JavaScript exit must not outrun an observed native presentation")
	}
	if player.embeddedFullscreenVersion != version {
		t.Fatal("a deferred JavaScript exit must not invalidate native ownership")
	}
}

func TestListenEmbeddedVideoFullscreenOwnsPresentationDuringTransitionAndActiveState(t *testing.T) {
	for _, test := range []struct {
		active        bool
		transitioning bool
		want          bool
	}{
		{active: false, transitioning: false, want: false},
		{active: false, transitioning: true, want: true},
		{active: true, transitioning: false, want: true},
		{active: true, transitioning: true, want: true},
	} {
		if got := listenEmbeddedVideoFullscreenOwnsPresentation(test.active, test.transitioning); got != test.want {
			t.Fatalf("fullscreen ownership (%v, %v) = %v; want %v", test.active, test.transitioning, got, test.want)
		}
	}
}

func TestListenEmbeddedVideoInlineGeometryIsBlockedByFullscreenOwnership(t *testing.T) {
	for _, test := range []struct {
		name          string
		visible       bool
		active        bool
		transitioning bool
		nativeWindow  bool
		want          bool
	}{
		{name: "inline", visible: true, want: true},
		{name: "hidden", visible: false, want: false},
		{name: "element fullscreen", visible: true, active: true, want: false},
		{name: "element transition", visible: true, transitioning: true, want: false},
		{name: "native player window", visible: true, nativeWindow: true, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := listenEmbeddedVideoCanApplyInlineGeometry(
				test.visible,
				test.active,
				test.transitioning,
				test.nativeWindow,
			)
			if got != test.want {
				t.Fatalf("inline geometry eligibility = %v, want %v", got, test.want)
			}
		})
	}
}
