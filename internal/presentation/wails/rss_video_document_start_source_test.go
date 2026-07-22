package wails

import (
	"os"
	"strings"
	"testing"
)

func TestRSSBilibiliDocumentStartBridgePlatformContracts(t *testing.T) {
	t.Parallel()

	darwin := readRSSVideoDocumentStartSource(t, "listen_player_webview_darwin.go")
	for _, required := range []string{
		"listenInstallRSSBilibiliDocumentStartScript",
		"WKUserScriptInjectionTimeAtDocumentStart",
		"forMainFrameOnly:YES",
		"listenEvaluateRSSBilibiliAssociatedBridgeForTrustedDocument",
		"didCommitNavigation",
		"didFinishNavigation",
		"listenRSSBilibiliAllowsTopLevelURL(webView.URL, expectedVideoID)",
		"[delegate webView:webView didCommitNavigation:navigation]",
		"[delegate webView:webView didFinishNavigation:navigation]",
		"listenRemoveRSSBilibiliDocumentStartScript",
		"removeAllUserScripts",
		"if (![script.source isEqualToString:source])",
	} {
		if !strings.Contains(darwin, required) {
			t.Fatalf("macOS RSS document-start bridge is missing %q", required)
		}
	}

	windowsSource := readRSSVideoDocumentStartSource(t, "listen_player_webview_windows.go")
	for _, required := range []string{
		"listenWindowsWebViewForWindow(window)",
		"webview.AddScriptToExecuteOnDocumentCreated(script)",
		"webview.WrapNavigationCompleted",
		"webview.Controller().PutIsVisible(true)",
		"efficiency mode",
		"registration failure is",
		"WebView releases the CoreWebView2",
	} {
		if !strings.Contains(windowsSource, required) {
			t.Fatalf("Windows RSS document-start bridge is missing %q", required)
		}
	}
	windowsAttach := rssVideoFunctionSource(
		t,
		windowsSource,
		"func attachRSSVideoPlayerDocumentStartBridge(",
	)
	for _, forbidden := range []string{"WebViewNavigationCompleted", "ExecuteScript"} {
		if strings.Contains(windowsAttach, forbidden) {
			t.Fatalf("Windows RSS document-start install still depends on %q", forbidden)
		}
	}
}

func TestRSSBilibiliBridgeIsRegisteredBeforePlayerNavigation(t *testing.T) {
	t.Parallel()

	source := readRSSVideoDocumentStartSource(t, "rss_video_player_handler.go")
	if strings.Contains(source, "JS:                         bridgeScript") {
		t.Fatal("RSS player still relies on Wails' post-navigation JS option")
	}
	create := rssVideoFunctionSource(t, source, "func (player *rssBilibiliVideoPlayer) createWindow(")
	if !strings.Contains(create, "attachRSSVideoPlayerDocumentStartBridge(window, bridgeScript)") ||
		!strings.Contains(create, "failed to install RSS Bilibili document-start bridge") {
		t.Fatal("RSS player window does not fail closed around native document-start registration")
	}
	installCall := strings.Index(source, "window, bridgeHook, err := player.createWindow(bridgeScript)")
	navigateCall := strings.Index(source, "loadRSSVideoPlayerURL(window, descriptor.PlayerURL, descriptor.Cookies)")
	if installCall < 0 || navigateCall < 0 || installCall >= navigateCall {
		t.Fatal("RSS Bilibili bridge must be registered before the authenticated player navigation")
	}
}

func TestRSSBilibiliBridgeHidesRemoteChromeAtDocumentStart(t *testing.T) {
	t.Parallel()

	script := rssBilibiliHTMLMediaBridgeScript(rssBilibiliBridgeConfig{
		SessionID:       "rss-session-document-start",
		Adapter:         "video",
		PlatformVideoID: "BV1DocumentStart",
		Volume:          1,
		PlaybackRate:    1,
	})
	for _, required := range []string{
		"if (window.top !== window) return;",
		"installVideoOnlyStyleAtDocumentStart",
		"documentRootObserver.observe(document, { childList: true, subtree: true })",
		"documentRootObserver.disconnect()",
		"xiadown-rss-bilibili-video-only",
		`body>*{visibility:hidden!important`,
		`data-xiadown-rss-bilibili-active-video`,
		`position:fixed!important`,
		`width:100vw!important`,
		`height:100vh!important`,
		`object-fit:contain!important`,
		`video.setAttribute(ACTIVE_VIDEO_ATTRIBUTE, CONFIG.sessionId)`,
		`unmarkMedia(media)`,
		`.bpx-player-render-dm-wrap{visibility:visible!important;position:fixed!important`,
		`.bpx-player-subtitle-wrap{visibility:visible!important;position:fixed!important`,
		`.bpx-player-render-dm-wrap *,.bpx-player-subtitle-wrap *{pointer-events:none!important`,
		`.bpx-player-control-wrap,.bpx-player-control-mask`,
		`.bpx-player-cmd-dm-wrap{display:none!important`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("RSS Bilibili bridge is missing early-style contract %q", required)
		}
	}
	if strings.Contains(script, `body *{display:none`) {
		t.Fatal("full-page isolation must preserve Bilibili's DOM and layout initialization")
	}
	for _, forbidden := range []string{
		`.bpx-player-render-dm-wrap *{visibility:visible!important`,
		`.bpx-player-subtitle-wrap *{visibility:visible!important`,
		`.bpx-player-video-perch{display:none!important`,
		`.bpx-player-render-dm-wrap{display:block!important`,
		`.bpx-player-subtitle-wrap{display:block!important`,
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("overlay allowlist must preserve Bilibili's own hidden/off state: %q", forbidden)
		}
	}
	if guard := strings.Index(script, "if (window.top !== window) return;"); guard < 0 || guard > strings.Index(script, "const CONFIG =") {
		t.Fatal("RSS Bilibili main-frame guard must run before bridge configuration")
	}
	waitForDOM := strings.LastIndex(script, `if (document.readyState === "loading")`)
	if waitForDOM < 0 ||
		strings.LastIndex(script[:waitForDOM], "installVideoOnlyStyleAtDocumentStart();") < 0 {
		t.Fatal("video-only CSS must be attempted before waiting for DOMContentLoaded")
	}
	for _, required := range []string{
		`previous.sessionId === CONFIG.sessionId`,
		`previous.disposed !== true`,
		`typeof previous.reconcile === "function"`,
		`get disposed() { return disposed; }`,
		`reconcile() { reconcile(); }`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("RSS Bilibili bridge is missing idempotent recovery contract %q", required)
		}
	}
}

func readRSSVideoDocumentStartSource(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func rssVideoFunctionSource(t *testing.T, source string, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("source is missing function %q", signature)
	}
	tail := source[start+len(signature):]
	if next := strings.Index(tail, "\nfunc "); next >= 0 {
		tail = tail[:next]
	}
	return signature + tail
}
