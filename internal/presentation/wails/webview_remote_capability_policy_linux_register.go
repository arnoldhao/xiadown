//go:build linux && cgo && !android && !server

package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// registerWebViewRemoteCapabilityPolicy restricts browser-only network
// capabilities XiaDown's players do not need. This is independent of proxy
// routing: MediaSource and HTMLMediaElement keep the native WebView network,
// while WebRTC and MediaStream remain unavailable.
func registerWebViewRemoteCapabilityPolicy(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	applyShellRemoteCapabilityPolicy(window.NativeWindow())
	window.OnWindowEvent(events.Linux.WindowLoadStarted, func(_ *application.WindowEvent) {
		applyShellRemoteCapabilityPolicy(window.NativeWindow())
	})
}
