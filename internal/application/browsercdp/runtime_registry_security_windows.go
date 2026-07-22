//go:build windows

package browsercdp

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// secureRuntimeRegistryPath uses a protected Windows DACL instead of POSIX
// mode bits, which os.Chmod cannot represent on Windows. Only the interactive
// user and LocalSystem retain access, and the user is made the explicit owner
// even when an elevated runner defaults new paths to the Administrators group.
func secureRuntimeRegistryPath(path string, directory bool) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("resolve current Windows user: %w", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("resolve Windows LocalSystem SID: %w", err)
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if directory {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entry := func(sid *windows.SID) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.ACCESS_MASK(windows.GENERIC_ALL),
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		}
	}
	entries := []windows.EXPLICIT_ACCESS{entry(user.User.Sid)}
	if !user.User.Sid.Equals(systemSID) {
		entries = append(entries, entry(systemSID))
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("build private browser runtime ACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		user.User.Sid,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("apply private browser runtime ACL: %w", err)
	}
	return nil
}
