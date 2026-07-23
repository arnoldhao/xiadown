//go:build windows

package browsercdp

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

type runtimeRegistryPrincipalAccess struct {
	effective  windows.ACCESS_MASK
	objects    windows.ACCESS_MASK
	containers windows.ACCESS_MASK
}

func assertPrivateRuntimeRegistryPath(t *testing.T, path string, directory bool, _ os.FileMode) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		filepath.Clean(path),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("inspect private runtime registry ACL: %v", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		t.Fatalf("inspect private runtime registry owner: %v", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("resolve current Windows user: %v", err)
	}
	if owner == nil || !owner.Equals(user.User.Sid) {
		t.Fatal("runtime registry path is not owned by the current Windows user")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("inspect private runtime registry controls: %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("runtime registry DACL still inherits access from its parent")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("inspect private runtime registry DACL: %v", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatalf("resolve Windows LocalSystem SID: %v", err)
	}
	userAccess := runtimeRegistryPrincipalAccess{}
	systemAccess := runtimeRegistryPrincipalAccess{}
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			t.Fatalf("inspect runtime registry ACE %d: %v", index, err)
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			t.Fatalf("runtime registry ACE %d is not an allow entry", index)
		}
		flags := ace.Header.AceFlags
		allowedFlags := uint8(windows.NO_INHERITANCE)
		if directory {
			allowedFlags = uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE | windows.INHERIT_ONLY_ACE)
		}
		if flags & ^allowedFlags != 0 ||
			flags&uint8(windows.INHERIT_ONLY_ACE) != 0 &&
				flags&uint8(windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT) == 0 {
			t.Fatalf("runtime registry ACE %d has invalid inheritance flags %#x", index, flags)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.IsValid() {
			t.Fatalf("runtime registry ACE %d has an invalid SID", index)
		}
		var access *runtimeRegistryPrincipalAccess
		switch {
		case sid.Equals(user.User.Sid):
			access = &userAccess
		case sid.Equals(systemSID):
			access = &systemAccess
		default:
			t.Fatalf("runtime registry ACE %d grants an unexpected principal", index)
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
	assertRuntimeRegistryPrincipalAccess(t, "current user", userAccess, directory)
	assertRuntimeRegistryPrincipalAccess(t, "LocalSystem", systemAccess, directory)
}

func assertRuntimeRegistryPrincipalAccess(
	t *testing.T,
	principal string,
	access runtimeRegistryPrincipalAccess,
	directory bool,
) {
	t.Helper()
	if !runtimeRegistryWindowsMaskGrantsFullAccess(access.effective) {
		t.Fatalf("runtime registry %s effective permissions = %#x, want full access", principal, access.effective)
	}
	if directory && (!runtimeRegistryWindowsMaskGrantsFullAccess(access.objects) ||
		!runtimeRegistryWindowsMaskGrantsFullAccess(access.containers)) {
		t.Fatalf(
			"runtime registry %s inherited permissions = objects:%#x containers:%#x, want full access",
			principal,
			access.objects,
			access.containers,
		)
	}
}

func runtimeRegistryWindowsMaskGrantsFullAccess(mask windows.ACCESS_MASK) bool {
	if mask&windows.GENERIC_ALL != 0 {
		return true
	}
	const fileAllAccess = windows.ACCESS_MASK(windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1ff)
	return mask&fileAllAccess == fileAllAccess
}
