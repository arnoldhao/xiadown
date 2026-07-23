//go:build windows

package service

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"xiadown/internal/domain/library"
)

const replaceFileWriteThrough = 0x00000001

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func replaceListenLocalMetadataFile(source string, destination string) error {
	replacement, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	replaced, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(replaced)),
		uintptr(unsafe.Pointer(replacement)),
		0,
		replaceFileWriteThrough,
		0,
		0,
	)
	if result != 0 {
		return nil
	}
	return classifyListenLocalMetadataWindowsReplaceError(callErr)
}

func classifyListenLocalMetadataWindowsReplaceError(err error) error {
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.ERROR_USER_MAPPED_FILE) ||
		errors.Is(err, windows.ERROR_UNABLE_TO_REMOVE_REPLACED) ||
		errors.Is(err, windows.ERROR_UNABLE_TO_MOVE_REPLACEMENT) ||
		errors.Is(err, windows.ERROR_UNABLE_TO_MOVE_REPLACEMENT_2) {
		return fmt.Errorf("%w: %v", library.ErrListenLocalFileBusy, err)
	}
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return fmt.Errorf("%w: %v", library.ErrListenLocalFilePermission, err)
	}
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return fmt.Errorf("%w: %v", library.ErrListenLocalFileChanged, err)
	}
	return err
}
