//go:build windows

package browserprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProfileStateForErrorClassifiesWindowsSharingViolationAsBrowserRunning(t *testing.T) {
	err := fmt.Errorf("open cookies: %w", windows.ERROR_SHARING_VIOLATION)
	if state := profileStateForError(err); state != ProfileStateBrowserRunning {
		t.Fatalf("sharing violation state = %q, want %q", state, ProfileStateBrowserRunning)
	}
}

func TestProfileCookieDiscoveryReportsExclusivelyLockedDatabaseAsBrowserRunning(t *testing.T) {
	profile := t.TempDir()
	cookies := filepath.Join(profile, "Network", "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookies), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookies, []byte("SQLite format 3\x00fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(cookies)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		path,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)

	hasCookies, readErr := profileHasCookieStoreDetailed(profile)
	if hasCookies || readErr == nil {
		t.Fatalf("exclusively locked Cookies result = %t, %v", hasCookies, readErr)
	}
	if state := profileStateForError(readErr); state != ProfileStateBrowserRunning {
		t.Fatalf("locked Cookies state = %q, want %q", state, ProfileStateBrowserRunning)
	}
}
