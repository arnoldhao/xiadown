//go:build windows

package browserprofile

import (
	"errors"

	"golang.org/x/sys/windows"
)

// Chromium may deny every new file handle to its Cookies database while it is
// running on Windows. Keep this distinct from corrupt profile data so callers
// can ask the user to close the browser and retry without exposing a path or a
// native Windows error across the application boundary.
func isBrowserProfileInUseError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
