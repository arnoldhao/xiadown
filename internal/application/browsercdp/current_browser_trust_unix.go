//go:build darwin || linux

package browsercdp

import (
	"errors"
	"os"
	"syscall"
)

func validateTrustedCurrentBrowserOwner(path string, info os.FileInfo, _ bool) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || int(stat.Uid) != os.Getuid() {
		return trustedCurrentBrowserOwnerError(path)
	}
	// The endpoint may be readable by the user's primary group on some
	// distributions, but no other account may be able to replace it.
	if info.Mode().Perm()&0o022 != 0 {
		return errors.New("current browser metadata is writable by another account")
	}
	return nil
}

func currentChromePlatformProcessRunning(_ Candidate) bool { return false }
