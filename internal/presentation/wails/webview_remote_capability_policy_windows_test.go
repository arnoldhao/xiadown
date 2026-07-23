//go:build windows && !server

package wails

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const windowsPopupPolicyBeforeRunChild = "XIADOWN_TEST_WINDOWS_POPUP_POLICY_BEFORE_RUN"

type windowsPopupPolicyTestTransport struct{}

func (windowsPopupPolicyTestTransport) Start(context.Context, *application.MessageProcessor) error {
	return nil
}

func (windowsPopupPolicyTestTransport) JSClient() []byte { return nil }

func (windowsPopupPolicyTestTransport) Stop() error { return nil }

func TestWindowsPopupPolicyDoesNotInvokeMainThreadBeforeRun(t *testing.T) {
	if os.Getenv(windowsPopupPolicyBeforeRunChild) == "1" {
		testWindowsPopupPolicyBeforeRunChild(t)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestWindowsPopupPolicyDoesNotInvokeMainThreadBeforeRun$",
		"-test.count=1",
	)
	command.Env = append(os.Environ(), windowsPopupPolicyBeforeRunChild+"=1")
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("pre-Run popup policy test timed out: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("pre-Run popup policy child failed: %v\n%s", err, output)
	}
}

func TestWindowsPopupPolicyRegistrationCancelsLateCallbacks(t *testing.T) {
	registration := &windowsRemoteCapabilityPolicyRegistration{}
	registration.cancel()
	readyCancels := 0
	closeCancels := 0
	registration.setCancelReady(func() { readyCancels++ })
	registration.setCancelClose(func() { closeCancels++ })

	if registration.active() {
		t.Fatal("cancelled registration remained active")
	}
	if readyCancels != 1 || closeCancels != 1 {
		t.Fatalf("late callbacks were not cancelled exactly once: ready=%d close=%d", readyCancels, closeCancels)
	}
}

func testWindowsPopupPolicyBeforeRunChild(t *testing.T) {
	app := application.New(application.Options{
		Name:                        "XiaDown popup policy lifecycle test",
		Assets:                      application.AlphaAssets,
		Transport:                   windowsPopupPolicyTestTransport{},
		DisableDefaultSignalHandler: true,
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "popup-policy-pre-run",
		URL:    "about:blank",
		Hidden: true,
	})
	if window.NativeWindow() != nil {
		t.Fatal("pending window unexpectedly has a native handle before Application.Run")
	}
	if installListenWindowsPersistentPopupPolicy(window) {
		t.Fatal("popup policy reported success before the native WebView existed")
	}

	// This is the production startup call. It must only subscribe for the later
	// WebView readiness event and must not enter application.InvokeSync yet.
	registerWebViewRemoteCapabilityPolicy(window)
	releaseListenWindowsPersistentPopupPolicy(window)

	// Both release paths must also be safe if window ownership ends before Run.
	registerWebViewRemoteCapabilityPolicy(window)
	releaseListenWindowsRemoteNavigationPolicy(window)
}
