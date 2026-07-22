//go:build windows

package libraryapi

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

type publicMusicPrincipalAccess struct {
	effective  windows.ACCESS_MASK
	objects    windows.ACCESS_MASK
	containers windows.ACCESS_MASK
}

func assertPrivatePublicMusicResourceDirectory(t *testing.T, path string, info os.FileInfo) {
	t.Helper()
	if info == nil || !info.IsDir() {
		t.Fatalf("Music resource cache is not a directory: %v", info)
	}
	assertPublicMusicResourceWindowsACL(t, path, true, false)
}

func assertProtectedPublicMusicResourceBlob(t *testing.T, path string, info os.FileInfo) {
	t.Helper()
	if info == nil || !info.Mode().IsRegular() {
		t.Fatalf("Music resource CAS blob is not regular: %v", info)
	}
	assertPublicMusicResourceWindowsACL(t, path, false, true)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err == nil {
		_ = file.Close()
		t.Fatal("current Windows user can open verified Music resource blob for writing")
	}
}

func assertPublicMusicResourceWindowsACL(t *testing.T, path string, directory, verified bool) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("inspect Music resource ACL: %v", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		t.Fatalf("inspect Music resource owner: %v", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("resolve current Windows user: %v", err)
	}
	if owner == nil || !owner.Equals(user.User.Sid) {
		t.Fatal("Music resource path is not owned by the current Windows user")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("inspect Music resource ACL controls: %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("Music resource DACL still inherits access from its parent")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("inspect Music resource DACL: %v", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatalf("resolve Windows LocalSystem SID: %v", err)
	}
	userAccess := publicMusicPrincipalAccess{}
	systemAccess := publicMusicPrincipalAccess{}
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			t.Fatalf("inspect Music resource ACE %d: %v", index, err)
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			t.Fatalf("Music resource ACE %d is not an allow entry", index)
		}
		flags := ace.Header.AceFlags
		allowedFlags := uint8(windows.NO_INHERITANCE)
		if directory {
			allowedFlags = uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE | windows.INHERIT_ONLY_ACE)
		}
		if flags & ^allowedFlags != 0 ||
			flags&uint8(windows.INHERIT_ONLY_ACE) != 0 &&
				flags&uint8(windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT) == 0 {
			t.Fatalf("Music resource ACE %d has invalid inheritance flags %#x", index, flags)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.IsValid() {
			t.Fatalf("Music resource ACE %d has an invalid SID", index)
		}
		var access *publicMusicPrincipalAccess
		switch {
		case sid.Equals(user.User.Sid):
			access = &userAccess
		case sid.Equals(systemSID):
			access = &systemAccess
		default:
			t.Fatalf("Music resource ACE %d grants an unexpected principal", index)
		}
		if flags&uint8(windows.INHERIT_ONLY_ACE) == 0 {
			access.effective |= ace.Mask
		}
		if flags&uint8(windows.OBJECT_INHERIT_ACE) != 0 {
			access.objects |= ace.Mask
		}
		if flags&uint8(windows.CONTAINER_INHERIT_ACE) != 0 {
			access.containers |= ace.Mask
		}
	}
	if user.User.Sid.Equals(systemSID) {
		systemAccess = userAccess
	}
	if verified {
		if !publicMusicWindowsMaskGrantsRead(userAccess.effective) ||
			userAccess.effective&windows.ACCESS_MASK(windows.DELETE) == 0 ||
			publicMusicWindowsMaskGrantsContentWrite(userAccess.effective) ||
			userAccess.effective&windows.ACCESS_MASK(windows.WRITE_DAC|windows.WRITE_OWNER) != 0 {
			t.Fatalf("verified Music resource user permissions=%#x", userAccess.effective)
		}
		if !user.User.Sid.Equals(systemSID) && !publicMusicWindowsMaskGrantsFullAccess(systemAccess.effective) {
			t.Fatalf("Music resource LocalSystem permissions=%#x", systemAccess.effective)
		}
		return
	}
	assertPublicMusicPrincipalFullAccess(t, "current user", userAccess, directory)
	assertPublicMusicPrincipalFullAccess(t, "LocalSystem", systemAccess, directory)
}

func assertPublicMusicPrincipalFullAccess(
	t *testing.T,
	principal string,
	access publicMusicPrincipalAccess,
	directory bool,
) {
	t.Helper()
	if !publicMusicWindowsMaskGrantsFullAccess(access.effective) {
		t.Fatalf("Music resource %s effective permissions=%#x, want full access", principal, access.effective)
	}
	if directory && (!publicMusicWindowsMaskGrantsFullAccess(access.objects) ||
		!publicMusicWindowsMaskGrantsFullAccess(access.containers)) {
		t.Fatalf(
			"Music resource %s inherited permissions=objects:%#x containers:%#x, want full access",
			principal,
			access.objects,
			access.containers,
		)
	}
}
