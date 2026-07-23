//go:build (!darwin && !windows) || ios

package wails

import (
	"context"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func connectorAppSessionNativeSupported() bool {
	return false
}

func loadNativeYouTubeRuntimeCookies() ([]appcookies.Record, error) {
	return nil, appsessions.ErrUnsupported
}

func connectorAppSessionCaptureBeforeClose() bool {
	return false
}

func prepareConnectorAppSessionNativeWindow(_ *application.WebviewWindow, _ string, _ string, _ []appcookies.Record, _ []string) {
}

func configureConnectorAppSessionNativeWindow(_ unsafe.Pointer, _ string) {
}

func loadConnectorAppSessionNativeURL(window *application.WebviewWindow, targetURL string) {
	if window == nil || targetURL == "" {
		return
	}
	window.SetURL(targetURL)
}

func setConnectorAppSessionNativeCookies(_ unsafe.Pointer, _ string, _ []appcookies.Record) {
}

func readConnectorAppSessionNativeCookies(_ context.Context, _ unsafe.Pointer) ([]appcookies.Record, error) {
	return nil, appsessions.ErrUnsupported
}

func readConnectorAppSessionNativeWindowCookies(_ context.Context, _ *application.WebviewWindow, _ []string) ([]appcookies.Record, error) {
	return nil, appsessions.ErrUnsupported
}

func clearConnectorAppSessionNativeRuntimeData(_ context.Context, _ *application.App, _ string, _ []string) error {
	return appsessions.ErrUnsupported
}
