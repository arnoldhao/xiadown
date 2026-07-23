//go:build windows

package browsercdp

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validateTrustedCurrentBrowserOwner(path string, _ os.FileInfo, _ bool) error {
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.IsValid() {
		return trustedCurrentBrowserOwnerError(path)
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil || !owner.Equals(user.User.Sid) {
		return trustedCurrentBrowserOwnerError(path)
	}
	return nil
}

func currentChromePlatformProcessRunning(_ Candidate) bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafeSizeofProcessEntry32())}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}
	for {
		name := strings.ToLower(strings.TrimSpace(windows.UTF16ToString(entry.ExeFile[:])))
		if name == "chrome.exe" {
			return true
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			return false
		}
	}
}

func currentChromePIDMatches(_ int, _ Candidate) bool { return false }

func unsafeSizeofProcessEntry32() uintptr {
	var entry windows.ProcessEntry32
	return unsafe.Sizeof(entry)
}
