//go:build windows

package opener

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sFalseHRESULT   = uintptr(0x00000001)
	rpcEChangedMode = uintptr(0x80010106)
)

var (
	shell32                        = windows.NewLazySystemDLL("shell32.dll")
	procILCreateFromPathW          = shell32.NewProc("ILCreateFromPathW")
	procILFree                     = shell32.NewProc("ILFree")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
	windowsOpenVerb                = windows.StringToUTF16Ptr("open")
)

func openDirectoryWindows(path string) error {
	return shellExecuteWindows(resolveWindowsShellPath(path))
}

func openURLWindows(rawURL string) error {
	return shellExecuteWindows(rawURL)
}

func revealPathWindows(path string) error {
	resolved := resolveWindowsShellPath(path)
	pidl, err := windowsItemIDListFromPath(resolved)
	if err == nil {
		defer windowsFreeItemIDList(pidl)
		if err := windowsOpenFolderAndSelectItems(pidl); err == nil {
			return nil
		}
	}
	return openDirectoryWindows(filepath.Dir(resolved))
}

func shellExecuteWindows(target string) error {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("encode shell target: %w", err)
	}
	err = windows.ShellExecute(0, windowsOpenVerb, targetPtr, nil, nil, windows.SW_SHOWNORMAL)
	if err == nil {
		return nil
	}
	return fmt.Errorf("open shell target %q: %w", target, err)
}

func resolveWindowsShellPath(path string) string {
	if resolved, err := filepath.Abs(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func windowsItemIDListFromPath(path string) (uintptr, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("encode shell path: %w", err)
	}
	pidl, _, callErr := procILCreateFromPathW.Call(uintptr(unsafe.Pointer(pathPtr)))
	if pidl == 0 {
		if callErr != windows.ERROR_SUCCESS {
			return 0, fmt.Errorf("ILCreateFromPathW %q: %w", path, callErr)
		}
		return 0, fmt.Errorf("ILCreateFromPathW %q returned null", path)
	}
	return pidl, nil
}

func windowsFreeItemIDList(pidl uintptr) {
	if pidl != 0 {
		procILFree.Call(pidl)
	}
}

func windowsOpenFolderAndSelectItems(pidl uintptr) error {
	uninitialize, err := initializeWindowsShellCOM()
	if err != nil {
		return err
	}
	defer uninitialize()

	result, _, callErr := procSHOpenFolderAndSelectItems.Call(pidl, 0, 0, 0)
	if int32(result) >= 0 {
		return nil
	}
	if callErr != windows.ERROR_SUCCESS {
		return fmt.Errorf("SHOpenFolderAndSelectItems failed: %w", callErr)
	}
	return fmt.Errorf("SHOpenFolderAndSelectItems failed: HRESULT 0x%08x", uint32(result))
}

func initializeWindowsShellCOM() (func(), error) {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err == nil || isSyscallCode(err, sFalseHRESULT) {
		return windows.CoUninitialize, nil
	}
	if isSyscallCode(err, rpcEChangedMode) {
		return func() {}, nil
	}
	return func() {}, fmt.Errorf("initialize shell COM: %w", err)
}

func isSyscallCode(err error, code uintptr) bool {
	if err == nil {
		return false
	}
	errno, ok := err.(syscall.Errno)
	return ok && uintptr(errno) == code
}
