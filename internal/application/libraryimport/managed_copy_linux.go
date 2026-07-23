//go:build linux

package libraryimport

import (
	"os"

	"golang.org/x/sys/unix"
)

func atomicPublishNoReplaceAt(directoryFD int, sourceName, destinationName string) error {
	if err := unix.Renameat2(directoryFD, sourceName, directoryFD, destinationName, unix.RENAME_NOREPLACE); err != nil {
		if err == unix.EEXIST {
			return os.ErrExist
		}
		return err
	}
	return nil
}
