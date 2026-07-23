//go:build !windows

package wails

import (
	"os"
	"path/filepath"
)

func replaceRSSSavedImageFile(source string, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		return err
	}
	// The file was already fully written and synced before publication. Syncing
	// the containing directory is best effort because some Unix filesystems do
	// not support it, but it closes the power-loss window where available.
	directory, err := os.Open(filepath.Dir(destination))
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
