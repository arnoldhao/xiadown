//go:build (!darwin && !windows) || ios

package wails

import (
	"runtime"
	"sync"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var listenYouTubeMusicRuntimeReadyWindowIDs sync.Map

func listenYouTubeMusicUserAgent() string {
	return ""
}

func syncListenNativeVideoHostBackground(_ *application.WebviewWindow, _ application.RGBA) {}

func configureListenYouTubeMusicNativeWindow(_ unsafe.Pointer, _ string) {}

func installRSSVideoPlayerNativeFullscreenEscape(_ *application.WebviewWindow) func() {
	return nil
}

func installListenNativeWindowFullscreenEscape(_ *application.WebviewWindow) func() {
	return nil
}

func showListenNativeAirPlayPicker(_ unsafe.Pointer, _ ListenAirPlayAnchor) bool {
	return false
}

func showListenNativeEmbeddedWebView(_ unsafe.Pointer, _ unsafe.Pointer, _ ListenEmbeddedVideoRect) bool {
	return false
}

func showListenNativeEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	return showListenNativeEmbeddedWebView(playerWindow.NativeWindow(), hostWindow.NativeWindow(), rect)
}

func showRSSNativeEmbeddedWebView(_ unsafe.Pointer, _ unsafe.Pointer, _ ListenEmbeddedVideoRect) bool {
	return false
}

func showRSSNativeEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	return showRSSNativeEmbeddedWebView(playerWindow.NativeWindow(), hostWindow.NativeWindow(), rect)
}

func showRSSNativeInteractiveEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	rect.Interactive = true
	return showRSSNativeEmbeddedWebView(playerWindow.NativeWindow(), hostWindow.NativeWindow(), rect)
}

func hideListenNativeEmbeddedWebView(_ unsafe.Pointer) bool {
	return false
}

func detachListenNativeEmbeddedWebViewForFullscreen(_ unsafe.Pointer) bool {
	return false
}

func listenNativeEmbeddedVideoFullscreenOwnsPresentation(_ unsafe.Pointer) (bool, bool) {
	return false, false
}

func listenEmbeddedVideoUsesNativeWindowFullscreen() bool {
	return false
}

func listenEmbeddedVideoFullscreenAllowsHostGeometry() bool {
	return false
}

func loadListenYouTubeMusicURL(window *application.WebviewWindow, targetURL string, _ []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}
	window.SetURL(targetURL)
}

func loadRSSVideoPlayerURL(window *application.WebviewWindow, targetURL string, _ []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}
	window.SetURL(targetURL)
}

func loadRSSSitePlayerURL(
	window *application.WebviewWindow,
	targetURL string,
	_ string,
	_ []appcookies.Record,
	allowedDomains []string,
	registrableSite string,
) {
	if window == nil || targetURL == "" {
		return
	}
	policy, allowed := webViewRemoteNavigationPolicyForRSSSite(targetURL, allowedDomains, registrableSite)
	if !allowed || !policy.allows(targetURL) {
		return
	}
	window.SetURL(targetURL)
}

func releaseRSSVideoPlayerWindowFeatures(_ *application.WebviewWindow) {}

func releaseRSSSitePlayerWindowFeatures(_ *application.WebviewWindow) {}

func execListenYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}
	markListenYouTubeMusicRuntimeReady(window)
	window.ExecJS(script)
}

func hideListenYouTubeMediaWindow(window *application.WebviewWindow) bool {
	if window == nil {
		return false
	}
	window.Hide()
	return parkListenMediaWebView(window)
}

// Persistent media WebViews use a native parking host on macOS and Windows.
// Other backends keep their existing hidden-window lifecycle, so registration
// and presentation transitions are successful no-ops here.
func registerListenMediaWebViewParking(
	playerWindow *application.WebviewWindow,
	hostWindow *application.WebviewWindow,
) bool {
	return playerWindow != nil && hostWindow != nil
}

func parkListenMediaWebView(playerWindow *application.WebviewWindow) bool {
	if playerWindow == nil {
		return false
	}
	playerWindow.Hide()
	return true
}

func unparkListenMediaWebView(playerWindow *application.WebviewWindow) bool {
	return playerWindow != nil
}

func reassertListenMediaWebViewParking(_ *application.WebviewWindow) {}

func releaseListenMediaWebViewParking(_ *application.WebviewWindow) {}

func attachListenYouTubeMusicBridge(window *application.WebviewWindow, script string) (func(), bool) {
	if window == nil || script == "" {
		return nil, false
	}

	var eventType events.WindowEventType
	switch runtime.GOOS {
	case "windows":
		eventType = events.Windows.WebViewNavigationCompleted
	case "linux":
		eventType = events.Linux.WindowLoadFinished
	default:
		// Mobile/other Wails backends keep using WebviewWindowOptions.JS as
		// before; they have no additional native navigation hook to register.
		return nil, true
	}

	return window.OnWindowEvent(eventType, func(_ *application.WindowEvent) {
		execListenYouTubeMusicJS(window, script)
	}), true
}

func attachRSSVideoPlayerDocumentStartBridge(
	window *application.WebviewWindow,
	script string,
) (func(), bool) {
	if window == nil || script == "" {
		return nil, false
	}
	// Wails does not expose a document-created hook on the remaining desktop
	// backend. Keep the load-finished fallback for compilation/support parity;
	// macOS and Windows use their native document-start implementations.
	var eventType events.WindowEventType
	switch runtime.GOOS {
	case "linux":
		eventType = events.Linux.WindowLoadFinished
	default:
		return nil, false
	}
	return window.OnWindowEvent(eventType, func(_ *application.WindowEvent) {
		execListenYouTubeMusicJS(window, script)
	}), true
}

func markListenYouTubeMusicRuntimeReady(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	if _, loaded := listenYouTubeMusicRuntimeReadyWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	window.HandleMessage("wails:runtime:ready")
}
