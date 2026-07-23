//go:build windows

package service

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func assertPrivateResourceCookieJar(t *testing.T, path string, _ os.FileMode) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("inspect temporary cookie jar security: %v", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("resolve current Windows user: %v", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		t.Fatalf("inspect temporary cookie jar owner: %v", err)
	}
	if owner == nil || !owner.Equals(user.User.Sid) {
		t.Fatal("temporary cookie jar is not owned by the current Windows user")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("inspect temporary cookie jar controls: %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("temporary cookie jar DACL still inherits access from its parent")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("inspect temporary cookie jar DACL: %v", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatalf("resolve Windows LocalSystem SID: %v", err)
	}
	var userPermissions windows.ACCESS_MASK
	var systemPermissions windows.ACCESS_MASK
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			t.Fatalf("inspect temporary cookie jar ACE %d: %v", index, err)
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			t.Fatalf("temporary cookie jar ACE %d is not an allow entry", index)
		}
		if ace.Header.AceFlags != uint8(windows.NO_INHERITANCE) {
			t.Fatalf("temporary cookie jar ACE %d inherits access: %#x", index, ace.Header.AceFlags)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		switch {
		case sid.IsValid() && sid.Equals(user.User.Sid):
			userPermissions |= ace.Mask
		case sid.IsValid() && sid.Equals(systemSID):
			systemPermissions |= ace.Mask
		default:
			t.Fatalf("temporary cookie jar ACE %d grants an unexpected principal", index)
		}
	}
	if user.User.Sid.Equals(systemSID) {
		systemPermissions = userPermissions
	}
	if !resourceCookieWindowsMaskGrantsFullAccess(userPermissions) ||
		!resourceCookieWindowsMaskGrantsFullAccess(systemPermissions) {
		t.Fatalf(
			"temporary cookie jar permissions = user:%#x system:%#x, want full access",
			userPermissions,
			systemPermissions,
		)
	}
}

func resourceCookieWindowsMaskGrantsFullAccess(mask windows.ACCESS_MASK) bool {
	if mask&windows.GENERIC_ALL != 0 {
		return true
	}
	const fileAllAccess = windows.ACCESS_MASK(windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1ff)
	return mask&fileAllAccess == fileAllAccess
}
