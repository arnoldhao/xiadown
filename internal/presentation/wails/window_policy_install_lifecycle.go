package wails

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type windowPolicyEventSource interface {
	OnWindowEvent(
		eventType events.WindowEventType,
		callback func(event *application.WindowEvent),
	) func()
}

type windowPolicyInstallState struct {
	mu          sync.Mutex
	isReady     func() bool
	install     func(isActive func() bool) bool
	installed   bool
	cancelled   bool
	installing  bool
	retry       bool
	unsubscribe func()
}

func newWindowPolicyInstallState(
	isReady func() bool,
	install func(isActive func() bool) bool,
) *windowPolicyInstallState {
	return &windowPolicyInstallState{
		isReady: isReady,
		install: install,
	}
}

func (state *windowPolicyInstallState) active() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	return !state.cancelled
}

func (state *windowPolicyInstallState) tryInstall() {
	for {
		state.mu.Lock()
		if state.cancelled || state.installed {
			state.mu.Unlock()
			return
		}
		if state.installing {
			state.retry = true
			state.mu.Unlock()
			return
		}
		state.installing = true
		state.mu.Unlock()

		installed, panicValue := runWindowPolicyInstallAttempt(
			state.isReady,
			state.install,
			state.active,
		)

		state.mu.Lock()
		state.installing = false
		var unsubscribe func()
		if installed && !state.cancelled {
			state.installed = true
			unsubscribe = state.unsubscribe
			state.unsubscribe = nil
		}
		retry := state.retry && !state.cancelled && !state.installed && panicValue == nil
		state.retry = false
		state.mu.Unlock()

		if unsubscribe != nil {
			unsubscribe()
		}
		if panicValue != nil {
			state.cancel()
			panic(panicValue)
		}
		if !retry {
			return
		}
	}
}

func runWindowPolicyInstallAttempt(
	isReady func() bool,
	install func(isActive func() bool) bool,
	isActive func() bool,
) (
	installed bool,
	panicValue any,
) {
	defer func() {
		panicValue = recover()
	}()
	if !isActive() || !isReady() || !isActive() {
		return false, nil
	}
	return install(isActive), nil
}

// cancel prevents new attempts without waiting for an in-flight installer.
// Installers receive active and must check it on their serialized commit path;
// this avoids deadlocking a UI-thread release against an InvokeSync caller.
func (state *windowPolicyInstallState) cancel() {
	state.mu.Lock()
	state.cancelled = true
	unsubscribe := state.unsubscribe
	state.unsubscribe = nil
	state.mu.Unlock()

	if unsubscribe != nil {
		unsubscribe()
	}
}

// installWindowPolicyWhenReady subscribes before its first installation
// attempt so a readiness event cannot be lost between probing and listening.
// Failed attempts stay subscribed; the listener is removed only after the
// policy has been installed successfully.
func installWindowPolicyWhenReady(
	source windowPolicyEventSource,
	readyEvent events.WindowEventType,
	isReady func() bool,
	install func(isActive func() bool) bool,
) func() {
	if source == nil || isReady == nil || install == nil {
		return func() {}
	}

	state := newWindowPolicyInstallState(isReady, install)

	unsubscribe := source.OnWindowEvent(
		readyEvent,
		func(_ *application.WindowEvent) {
			state.tryInstall()
		},
	)

	// A custom event source may invoke the callback before OnWindowEvent
	// returns. In that case the successful callback could not unsubscribe yet,
	// so cancel the newly returned listener here.
	state.mu.Lock()
	if state.installed {
		state.mu.Unlock()
		if unsubscribe != nil {
			unsubscribe()
		}
		return state.cancel
	}
	state.unsubscribe = unsubscribe
	state.mu.Unlock()

	state.tryInstall()
	return state.cancel
}
