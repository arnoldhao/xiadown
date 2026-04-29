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

var dreamFMYouTubeMusicRuntimeReadyWindowIDs sync.Map

func dreamFMYouTubeMusicUserAgent() string {
	return ""
}

func configureDreamFMYouTubeMusicNativeWindow(_ unsafe.Pointer, _ string) {}

func showDreamFMNativeAirPlayPicker(_ unsafe.Pointer, _ DreamFMAirPlayAnchor) bool {
	return false
}

func loadDreamFMYouTubeMusicURL(window *application.WebviewWindow, targetURL string, _ []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}
	window.SetURL(targetURL)
}

func execDreamFMYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}
	markDreamFMYouTubeMusicRuntimeReady(window)
	window.ExecJS(script)
}

func attachDreamFMYouTubeMusicBridge(window *application.WebviewWindow, script string) func() {
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
		execDreamFMYouTubeMusicJS(window, script)
	})
}

func markDreamFMYouTubeMusicRuntimeReady(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	if _, loaded := dreamFMYouTubeMusicRuntimeReadyWindowIDs.LoadOrStore(window.ID(), struct{}{}); loaded {
		return
	}
	window.HandleMessage("wails:runtime:ready")
}
