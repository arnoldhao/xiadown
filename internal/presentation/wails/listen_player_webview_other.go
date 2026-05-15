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

func configureListenYouTubeMusicNativeWindow(_ unsafe.Pointer, _ string) {}

func showListenNativeAirPlayPicker(_ unsafe.Pointer, _ ListenAirPlayAnchor) bool {
	return false
}

func showListenNativeEmbeddedWebView(_ unsafe.Pointer, _ unsafe.Pointer, _ ListenEmbeddedVideoRect) bool {
	return false
}

func hideListenNativeEmbeddedWebView(_ unsafe.Pointer) bool {
	return false
}

func loadListenYouTubeMusicURL(window *application.WebviewWindow, targetURL string, _ []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}
	window.SetURL(targetURL)
}

func execListenYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}
	markListenYouTubeMusicRuntimeReady(window)
	window.ExecJS(script)
}

func attachListenYouTubeMusicBridge(window *application.WebviewWindow, script string) func() {
	if window == nil || script == "" {
		return nil
	}

	var eventType events.WindowEventType
	switch runtime.GOOS {
	case "windows":
		eventType = events.Windows.WebViewNavigationCompleted
	case "linux":
		eventType = events.Linux.WindowLoadFinished
	default:
		return nil
	}

	return window.OnWindowEvent(eventType, func(_ *application.WindowEvent) {
		execListenYouTubeMusicJS(window, script)
	})
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
