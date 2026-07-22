//go:build windows && !server

package wails

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var windowsRemoteCapabilityPolicyRegistrations sync.Map

type windowsRemoteCapabilityPolicyRegistration struct {
	mu          sync.Mutex
	cancelled   bool
	cancelReady func()
	cancelClose func()
}

func (registration *windowsRemoteCapabilityPolicyRegistration) cancel() {
	if registration == nil {
		return
	}
	registration.mu.Lock()
	if registration.cancelled {
		registration.mu.Unlock()
		return
	}
	registration.cancelled = true
	cancelReady := registration.cancelReady
	cancelClose := registration.cancelClose
	registration.cancelReady = nil
	registration.cancelClose = nil
	registration.mu.Unlock()

	if cancelClose != nil {
		cancelClose()
	}
	if cancelReady != nil {
		cancelReady()
	}
}

func (registration *windowsRemoteCapabilityPolicyRegistration) active() bool {
	registration.mu.Lock()
	defer registration.mu.Unlock()
	return !registration.cancelled
}

func (registration *windowsRemoteCapabilityPolicyRegistration) setCancelReady(cancel func()) {
	registration.mu.Lock()
	if !registration.cancelled {
		registration.cancelReady = cancel
		registration.mu.Unlock()
		return
	}
	registration.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (registration *windowsRemoteCapabilityPolicyRegistration) setCancelClose(cancel func()) {
	registration.mu.Lock()
	if !registration.cancelled {
		registration.cancelClose = cancel
		registration.mu.Unlock()
		return
	}
	registration.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// registerWebViewRemoteCapabilityPolicy gives every persistent Windows WebView
// a native popup sink. This includes local shell/transport documents because a
// remote iframe can raise NewWindowRequested even when the top-level document
// itself never leaves the Wails asset origin. A normal closing listener only
// cancels pending readiness work: Wails dispatches closing listeners in
// parallel with native destruction, so COM removal remains an explicit owner
// cleanup performed before Window.Close. Intercepted closes never reach the
// listener, allowing main/settings/tray windows to keep the same WebView.
func registerWebViewRemoteCapabilityPolicy(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	// Wails alpha2.117 has no public CoreWebView2-created event. Windows that
	// already have a Core install immediately; pending startup windows retry on
	// the earliest stable public WebView readiness event.
	registration := &windowsRemoteCapabilityPolicyRegistration{}
	cancelClose := window.OnWindowEvent(
		events.Common.WindowClosing,
		func(_ *application.WindowEvent) {
			windowsRemoteCapabilityPolicyRegistrations.CompareAndDelete(window.ID(), registration)
			registration.cancel()
		},
	)
	registration.setCancelClose(cancelClose)
	if !registration.active() {
		return
	}
	if previous, loaded := windowsRemoteCapabilityPolicyRegistrations.Swap(window.ID(), registration); loaded {
		if previousRegistration, ok := previous.(*windowsRemoteCapabilityPolicyRegistration); ok {
			previousRegistration.cancel()
		}
	}
	if !registration.active() {
		windowsRemoteCapabilityPolicyRegistrations.CompareAndDelete(window.ID(), registration)
		return
	}
	cancelReady := installWindowPolicyWhenReady(
		window,
		events.Windows.WebViewNavigationCompleted,
		func() bool {
			return window.NativeWindow() != nil
		},
		func(installActive func() bool) bool {
			return installListenWindowsPersistentPopupPolicyWhileActive(
				window,
				func() bool {
					return installActive() && registration.active()
				},
			)
		},
	)
	registration.setCancelReady(cancelReady)
	if !registration.active() {
		windowsRemoteCapabilityPolicyRegistrations.CompareAndDelete(window.ID(), registration)
	}
}

func cancelWebViewRemoteCapabilityPolicyRegistration(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	value, loaded := windowsRemoteCapabilityPolicyRegistrations.LoadAndDelete(window.ID())
	if !loaded {
		return
	}
	registration, ok := value.(*windowsRemoteCapabilityPolicyRegistration)
	if ok {
		registration.cancel()
	}
}

// releaseWebViewRemoteCapabilityPolicy is for window owners that are about to
// call Window.Close. It must run before Close because Wails dispatches ordinary
// closing listeners concurrently with its native destruction listener.
func releaseWebViewRemoteCapabilityPolicy(window *application.WebviewWindow) {
	releaseListenWindowsPersistentPopupPolicy(window)
}
