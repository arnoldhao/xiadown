//go:build linux && cgo && !android && !server

package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// registerWebViewRemoteCapabilityPolicy restricts browser-only network
// features which can create a direct UDP/DNS path outside the HTTP gateway.
// MediaSource and HTMLMediaElement playback remain available; WebRTC and
// MediaStream are not required for XiaDown's YouTube/Bilibili players.
func registerWebViewRemoteCapabilityPolicy(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	applyShellRemoteCapabilityPolicy(window.NativeWindow())
	window.OnWindowEvent(events.Linux.WindowLoadStarted, func(_ *application.WindowEvent) {
		applyShellRemoteCapabilityPolicy(window.NativeWindow())
	})
}
