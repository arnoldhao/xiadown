//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package browserprofile

import (
	"fmt"
	"os"
	"syscall"
)

func snapshotOwnedByCurrentUser(_ string, info os.FileInfo) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("browser snapshot owner metadata is unavailable")
	}
	return stat.Uid == uint32(os.Geteuid()), nil
}

func secureSnapshotDirectory(path string) error {
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure browser snapshot directory: %w", err)
	}
	return nil
}
