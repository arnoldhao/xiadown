//go:build windows

package browsercdp

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Windows runners may create temporary paths with the Administrators group as
// their default owner. Chrome profile metadata is expected to be owned by the
// interactive user, so make the fixture model that production invariant rather
// than bypassing the ownership check in the test.
func secureTrustedCurrentBrowserTestPath(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION,
		user.User.Sid,
		nil,
		nil,
		nil,
	)
}
