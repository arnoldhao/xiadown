package wails

import (
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type fakeWindowPolicyEventSource struct {
	callback         func(*application.WindowEvent)
	eventType        events.WindowEventType
	active           bool
	emitOnSubscribe  bool
	unsubscribeCalls int
}

func (source *fakeWindowPolicyEventSource) OnWindowEvent(
	eventType events.WindowEventType,
	callback func(*application.WindowEvent),
) func() {
	source.eventType = eventType
	source.callback = callback
	source.active = true
	if source.emitOnSubscribe {
		callback(nil)
	}
	return func() {
		if !source.active {
			return
		}
		source.active = false
		source.unsubscribeCalls++
	}
}

func (source *fakeWindowPolicyEventSource) emit() {
	if source.active && source.callback != nil {
		source.callback(nil)
	}
}

func TestInstallWindowPolicyWhenReadyDefersUntilNativeWindowExists(t *testing.T) {
	source := &fakeWindowPolicyEventSource{}
	ready := false
	installCalls := 0

	installWindowPolicyWhenReady(
		source,
		events.Windows.WebViewNavigationCompleted,
		func() bool { return ready },
		func(func() bool) bool {
			installCalls++
			return true
		},
	)

	if installCalls != 0 {
		t.Fatalf("installer ran before the native window existed: calls = %d", installCalls)
	}
	if source.eventType != events.Windows.WebViewNavigationCompleted || !source.active {
		t.Fatal("readiness listener was not retained while the window was pending")
	}

	ready = true
	source.emit()
	if installCalls != 1 {
		t.Fatalf("installer calls after readiness = %d, want 1", installCalls)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("readiness listener was not removed after successful installation")
	}

	source.emit()
	if installCalls != 1 {
		t.Fatalf("installer ran again after success: calls = %d", installCalls)
	}
}

func TestInstallWindowPolicyWhenReadyInstallsImmediatelyWhenReady(t *testing.T) {
	source := &fakeWindowPolicyEventSource{}
	installCalls := 0

	installWindowPolicyWhenReady(
		source,
		events.Windows.WebViewNavigationCompleted,
		func() bool { return true },
		func(func() bool) bool {
			installCalls++
			return true
		},
	)

	if installCalls != 1 {
		t.Fatalf("installer calls = %d, want 1", installCalls)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("immediate installation did not remove its readiness listener")
	}
}

func TestInstallWindowPolicyWhenReadyRetriesFailedInstall(t *testing.T) {
	source := &fakeWindowPolicyEventSource{}
	installCalls := 0

	installWindowPolicyWhenReady(
		source,
		events.Windows.WebViewNavigationCompleted,
		func() bool { return true },
		func(func() bool) bool {
			installCalls++
			return installCalls >= 2
		},
	)

	if installCalls != 1 || !source.active {
		t.Fatal("failed initial installation did not retain its readiness listener")
	}
	source.emit()
	if installCalls != 2 {
		t.Fatalf("installer calls after retry = %d, want 2", installCalls)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("successful retry did not remove its readiness listener")
	}
}

func TestInstallWindowPolicyWhenReadyCancelsPendingInstall(t *testing.T) {
	source := &fakeWindowPolicyEventSource{}
	ready := false
	installCalls := 0

	cancel := installWindowPolicyWhenReady(
		source,
		events.Windows.WebViewNavigationCompleted,
		func() bool { return ready },
		func(func() bool) bool {
			installCalls++
			return true
		},
	)
	cancel()
	cancel()

	ready = true
	source.emit()
	if installCalls != 0 {
		t.Fatalf("cancelled installer ran after readiness: calls = %d", installCalls)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("cancelling a pending install did not remove its readiness listener exactly once")
	}
}

func TestWindowPolicyCancelDoesNotWaitForInFlightInstaller(t *testing.T) {
	installStarted := make(chan struct{})
	allowInstallToFinish := make(chan struct{})
	installFinished := make(chan struct{})
	state := newWindowPolicyInstallState(
		func() bool { return true },
		func(isActive func() bool) bool {
			close(installStarted)
			<-allowInstallToFinish
			return isActive()
		},
	)
	go func() {
		defer close(installFinished)
		state.tryInstall()
	}()
	<-installStarted

	cancelled := make(chan struct{})
	go func() {
		state.cancel()
		close(cancelled)
	}()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		close(allowInstallToFinish)
		<-installFinished
		t.Fatal("cancel waited for an in-flight installer")
	}

	close(allowInstallToFinish)
	select {
	case <-installFinished:
	case <-time.After(time.Second):
		t.Fatal("in-flight installer did not finish after cancellation")
	}
	if state.active() {
		t.Fatal("cancelled install state remained active")
	}
}

func TestInstallWindowPolicyWhenReadyHandlesReentrantReadiness(t *testing.T) {
	source := &fakeWindowPolicyEventSource{}
	installCalls := 0

	installWindowPolicyWhenReady(
		source,
		events.Windows.WebViewNavigationCompleted,
		func() bool { return true },
		func(func() bool) bool {
			installCalls++
			if installCalls == 1 {
				source.emit()
				return false
			}
			return true
		},
	)

	if installCalls != 2 {
		t.Fatalf("installer calls after reentrant readiness = %d, want 2", installCalls)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("reentrant readiness did not finish with exactly one listener cancellation")
	}
}

func TestInstallWindowPolicyWhenReadyCancelsListenerAfterPanic(t *testing.T) {
	source := &fakeWindowPolicyEventSource{}
	panicValue := any(nil)
	func() {
		defer func() {
			panicValue = recover()
		}()
		installWindowPolicyWhenReady(
			source,
			events.Windows.WebViewNavigationCompleted,
			func() bool { return true },
			func(func() bool) bool { panic("install failed") },
		)
	}()

	if panicValue != "install failed" {
		t.Fatalf("panic value = %v, want install failed", panicValue)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("panicking installer left its readiness listener active")
	}
}

func TestInstallWindowPolicyWhenReadyHandlesEventDuringSubscription(t *testing.T) {
	source := &fakeWindowPolicyEventSource{emitOnSubscribe: true}
	installCalls := 0

	installWindowPolicyWhenReady(
		source,
		events.Windows.WebViewNavigationCompleted,
		func() bool { return true },
		func(func() bool) bool {
			installCalls++
			return true
		},
	)

	if installCalls != 1 {
		t.Fatalf("installer calls = %d, want 1", installCalls)
	}
	if source.active || source.unsubscribeCalls != 1 {
		t.Fatal("listener was leaked when readiness arrived during subscription")
	}
}
