package librarybackup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// reconcileBackupDirectoryLocked removes only artifacts whose names and state
// prove they belong to an interrupted XiaDown backup transaction. A published
// manifest is the commit record, so a database without one is uncommitted (or
// is the second half of an interrupted manifest-first delete). XiaDown owns a
// single Manager for this private directory, so startup does not delay removal
// based on an attacker-controlled or clock-sensitive modification timestamp.
// Invalid committed pairs are deliberately preserved for operator diagnosis.
func (manager *Manager) reconcileBackupDirectoryLocked() error {
	if err := ensurePrivateDirectory(manager.backupDirectory); err != nil {
		return err
	}
	entries, err := os.ReadDir(manager.backupDirectory)
	if err != nil {
		return err
	}
	protected, markerUncertain := manager.protectedBackupIDsLocked()
	removed := false
	var cleanupErr error
	for _, entry := range entries {
		artifactPath := manager.backupPath(entry.Name())
		if filepath.Clean(artifactPath) == filepath.Clean(manager.restoreMarkerPath) {
			continue
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("inspect backup artifact %q: %w", entry.Name(), err))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}

		remove := isBackupTemporaryName(entry.Name())
		if backupID, ok := backupIDFromDatabaseName(entry.Name()); ok {
			if markerUncertain || protected[backupID] {
				continue
			}
			manifestPath := manager.backupPath(backupPrefix + backupID + manifestSuffix)
			if _, err := os.Lstat(manifestPath); err == nil {
				// The manifest exists, even if it is invalid. Verification and the
				// management UI own diagnosis of committed-but-invalid pairs.
				continue
			} else if !os.IsNotExist(err) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("inspect backup commit record %q: %w", filepath.Base(manifestPath), err))
				continue
			}
			remove = true
		}
		if !remove {
			continue
		}
		if err := removeBackupArtifactDurably(artifactPath); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove interrupted backup artifact %q: %w", entry.Name(), err))
			continue
		}
		removed = true
	}
	if removed {
		cleanupErr = errors.Join(cleanupErr, syncDirectory(manager.backupDirectory))
	}
	return cleanupErr
}

func isBackupTemporaryName(name string) bool {
	if !strings.HasPrefix(name, "."+backupPrefix) {
		return false
	}
	return strings.HasSuffix(name, ".sqlite.tmp") || strings.HasSuffix(name, ".manifest.tmp") ||
		strings.HasSuffix(name, ".delete.tmp")
}
