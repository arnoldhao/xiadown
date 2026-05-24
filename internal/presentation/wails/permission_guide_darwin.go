//go:build darwin && cgo && !ios

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=10.15 -x objective-c
#cgo LDFLAGS: -framework AppKit -framework CoreGraphics -framework Foundation -framework QuartzCore

#include <stdlib.h>
#import "permission_guide_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func openPermissionGuide(request permissionGuideRequest) error {
	cSettingsURL := C.CString(request.SettingsURL)
	cPermissionName := C.CString(request.PermissionName)
	cHint := C.CString(request.Hint)
	defer C.free(unsafe.Pointer(cSettingsURL))
	defer C.free(unsafe.Pointer(cPermissionName))
	defer C.free(unsafe.Pointer(cHint))

	if C.xiadownOpenPermissionGuide(cSettingsURL, cPermissionName, cHint) != 1 {
		return fmt.Errorf("xiadown app bundle is unavailable")
	}
	return nil
}
