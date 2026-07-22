//go:build !windows

package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"xiadown/internal/domain/library"
)

func replaceListenLocalMetadataFile(source string, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		switch {
		case errors.Is(err, syscall.EBUSY), errors.Is(err, syscall.ETXTBSY):
			return fmt.Errorf("%w: %v", library.ErrListenLocalFileBusy, err)
		case os.IsPermission(err):
			return fmt.Errorf("%w: %v", library.ErrListenLocalFilePermission, err)
		default:
			return err
		}
	}
	// The replacement has already succeeded, so a directory fsync failure must
	// not be reported as a failed edit (which would leave the database stale).
	// Best effort still closes the power-loss window on filesystems that support
	// syncing directory entries.
	directory, err := os.Open(filepath.Dir(destination))
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
