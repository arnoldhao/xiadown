//go:build windows

package wails

import (
	"fmt"
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	comSFalse         = uintptr(0x00000001)
	comRPCChangedMode = uintptr(0x80010106)
)

func prepareOSNotificationServiceStartup() (func(), error) {
	runtime.LockOSThread()
	release := runtime.UnlockOSThread
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err == nil || isCOMResult(err, comSFalse) || isCOMResult(err, comRPCChangedMode) {
		return release, nil
	}
	return release, fmt.Errorf("initialize COM for Windows notifications: %w", err)
}

func isCOMResult(err error, code uintptr) bool {
	if err == nil {
		return false
	}
	errno, ok := err.(syscall.Errno)
	return ok && uintptr(errno) == code
}
