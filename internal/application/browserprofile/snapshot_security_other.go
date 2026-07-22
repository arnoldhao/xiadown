//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package browserprofile

import (
	"fmt"
	"os"
)

// Unsupported platforms fail closed because ownership cannot be established.
func snapshotOwnedByCurrentUser(_ string, _ os.FileInfo) (bool, error) {
	return false, fmt.Errorf("browser snapshot ownership is unsupported")
}

func secureSnapshotDirectory(_ string) error {
	return fmt.Errorf("browser snapshot permissions are unsupported")
}
