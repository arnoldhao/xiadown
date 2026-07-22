//go:build windows

package librarybackup

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Windows ignores the Unix permission bits passed to OpenFile and MkdirAll.
// Use a protected DACL so sensitive metadata snapshots do not inherit broad
// access from a parent directory. LocalSystem is retained for OS maintenance;
// administrators can still take ownership through the normal Windows model.
func restrictBackupDirectory(path string) error {
	return setPrivateBackupDACL(path, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
}

func restrictBackupFile(path string) error {
	return setPrivateBackupDACL(path, windows.NO_INHERITANCE)
}

func setPrivateBackupDACL(path string, inheritance uint32) error {
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
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		}
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		entry(user.User.Sid),
		entry(systemSID),
	}, nil)
	if err != nil {
		return fmt.Errorf("build private Windows backup ACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("apply private Windows backup ACL: %w", err)
	}
	return nil
}
