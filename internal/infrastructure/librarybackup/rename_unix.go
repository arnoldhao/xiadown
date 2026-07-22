//go:build !windows

package librarybackup

import (
	"fmt"
	"os"
)

// durableRename performs the namespace transition used by backup publication
// and restore swaps. Callers sync the containing directory after a complete
// multi-file transition.
func durableRename(source, destination string, replace bool) error {
	if !replace {
		if _, err := os.Lstat(destination); err == nil {
			return fmt.Errorf("rename destination already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(source, destination)
}
