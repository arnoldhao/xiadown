//go:build windows

package libraryapi

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows does not implement POSIX confidentiality through os.Chmod. Protect
// the CAS root from inherited parent access and let its children inherit only
// the interactive user and LocalSystem entries.
func securePublicMusicResourceDirectory(path string) error {
	return setPublicMusicResourceWindowsACL(path, true, false)
}

func securePublicMusicResourceTemporaryFile(path string) error {
	return setPublicMusicResourceWindowsACL(path, false, false)
}

// Verified blobs remain deletable so bounded-cache eviction works, but the
// application user receives no right to modify or append their content.
// LocalSystem retains full access, matching the administrative override that
// exists for a 0400 file on POSIX systems.
func securePublicMusicResourceVerifiedBlob(path string) error {
	return setPublicMusicResourceWindowsACL(path, false, true)
}

func setPublicMusicResourceWindowsACL(path string, directory, verified bool) error {
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
	entry := func(sid *windows.SID, permissions windows.ACCESS_MASK) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{
			AccessPermissions: permissions,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		}
	}
	userPermissions := windows.ACCESS_MASK(windows.GENERIC_ALL)
	if verified {
		userPermissions = windows.ACCESS_MASK(windows.GENERIC_READ | windows.DELETE)
	}
	entries := []windows.EXPLICIT_ACCESS{entry(user.User.Sid, userPermissions)}
	if !user.User.Sid.Equals(systemSID) {
		entries = append(entries, entry(systemSID, windows.ACCESS_MASK(windows.GENERIC_ALL)))
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("build private Music resource ACL: %w", err)
	}
	securityInformation := windows.SECURITY_INFORMATION(
		windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	var owner *windows.SID
	if !verified {
		// The directory and writable staging file still grant GENERIC_ALL, so
		// explicitly normalizing their owner is safe even on runners that make
		// Administrators the default owner. A verified file is already owned by
		// this user and intentionally no longer grants WRITE_OWNER.
		securityInformation |= windows.OWNER_SECURITY_INFORMATION
		owner = user.User.Sid
	}
	if err := windows.SetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		securityInformation,
		owner,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("apply private Music resource ACL: %w", err)
	}
	return nil
}

func publicMusicResourceBlobIsProtected(path string, info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return false
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || !owner.Equals(user.User.Sid) {
		return false
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return false
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return false
	}
	var userPermissions windows.ACCESS_MASK
	var systemPermissions windows.ACCESS_MASK
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil || ace == nil ||
			ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
			ace.Header.AceFlags != uint8(windows.NO_INHERITANCE) {
			return false
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.IsValid() {
			return false
		}
		switch {
		case sid.Equals(user.User.Sid):
			userPermissions |= ace.Mask
		case sid.Equals(systemSID):
			systemPermissions |= ace.Mask
		default:
			return false
		}
	}
	if user.User.Sid.Equals(systemSID) {
		systemPermissions = userPermissions
	}
	if !publicMusicWindowsMaskGrantsRead(userPermissions) ||
		userPermissions&windows.ACCESS_MASK(windows.DELETE) == 0 ||
		publicMusicWindowsMaskGrantsContentWrite(userPermissions) ||
		userPermissions&windows.ACCESS_MASK(windows.WRITE_DAC|windows.WRITE_OWNER) != 0 {
		return false
	}
	return user.User.Sid.Equals(systemSID) || publicMusicWindowsMaskGrantsFullAccess(systemPermissions)
}

func publicMusicWindowsMaskGrantsRead(mask windows.ACCESS_MASK) bool {
	return mask&windows.GENERIC_READ != 0 ||
		mask&windows.ACCESS_MASK(windows.FILE_GENERIC_READ) == windows.ACCESS_MASK(windows.FILE_GENERIC_READ)
}

func publicMusicWindowsMaskGrantsContentWrite(mask windows.ACCESS_MASK) bool {
	const contentWrite = windows.GENERIC_WRITE | windows.GENERIC_ALL |
		windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA |
		windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES
	return mask&windows.ACCESS_MASK(contentWrite) != 0
}

func publicMusicWindowsMaskGrantsFullAccess(mask windows.ACCESS_MASK) bool {
	if mask&windows.GENERIC_ALL != 0 {
		return true
	}
	return publicMusicWindowsMaskGrantsRead(mask) &&
		mask&windows.ACCESS_MASK(windows.FILE_GENERIC_WRITE) == windows.ACCESS_MASK(windows.FILE_GENERIC_WRITE) &&
		mask&windows.ACCESS_MASK(windows.FILE_GENERIC_EXECUTE) == windows.ACCESS_MASK(windows.FILE_GENERIC_EXECUTE) &&
		mask&windows.ACCESS_MASK(windows.DELETE|windows.WRITE_DAC|windows.WRITE_OWNER) ==
			windows.ACCESS_MASK(windows.DELETE|windows.WRITE_DAC|windows.WRITE_OWNER)
}
