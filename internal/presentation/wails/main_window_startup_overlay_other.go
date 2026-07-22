//go:build !darwin || !cgo || ios || server

package wails

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func registerMainWindowStartupOverlayEvents(
	_ *WindowManager,
	_ *application.WebviewWindow,
) {
}

func supportsMainWindowStartupOverlay() bool { return false }

func installMainWindowStartupOverlay(_ unsafe.Pointer, _ []byte, _ string) bool {
	return false
}

func dismissMainWindowStartupOverlay(_ unsafe.Pointer) {
}
