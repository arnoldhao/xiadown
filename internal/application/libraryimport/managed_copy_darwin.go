//go:build darwin

package libraryimport

import (
	"os"

	"golang.org/x/sys/unix"
)

func atomicPublishNoReplaceAt(directoryFD int, sourceName, destinationName string) error {
	if err := unix.RenameatxNp(directoryFD, sourceName, directoryFD, destinationName, unix.RENAME_EXCL); err != nil {
		if err == unix.EEXIST {
			return os.ErrExist
		}
		return err
	}
	return nil
}
