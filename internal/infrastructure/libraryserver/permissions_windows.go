//go:build windows

package libraryserver

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func restrictTLSFile(path string, mode os.FileMode) error {
	if mode.Perm() != 0o600 {
		return nil
	}
	return restrictTLSPrivateKey(path)
}

// restrictTLSPrivateKey replaces inherited permissions with a protected DACL
// granting full access only to the current user and LocalSystem. The current
// user is also made the explicit owner because elevated Windows environments
// can otherwise assign ownership to the Administrators group. os.Chmod on
// Windows changes only the read-only attribute and is not a confidentiality
// boundary.
func restrictTLSPrivateKey(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("resolve current Windows user: %w", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("resolve Windows LocalSystem SID: %w", err)
	}
	entry := func(sid *windows.SID) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.ACCESS_MASK(windows.GENERIC_ALL),
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
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
		return fmt.Errorf("build private Windows TLS key ACL: %w", err)
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
		return fmt.Errorf("apply private Windows TLS key ACL: %w", err)
	}
	return nil
}
