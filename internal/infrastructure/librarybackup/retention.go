package librarybackup

import (
	"fmt"
	"os"
	"sort"

	domainbackup "xiadown/internal/domain/librarybackup"
)

func (manager *Manager) pruneLocked() error {
	if err := manager.reconcileBackupDirectoryLocked(); err != nil {
		return err
	}
	manifests, err := manager.retentionCandidatesLocked()
	if err != nil {
		return err
	}
	protected, markerUncertain := manager.protectedBackupIDsLocked()
	if markerUncertain {
		// An unreadable/tampered marker is an operator-visible recovery problem.
		// Avoid deleting any committed backup until that marker is resolved.
		return nil
	}
	cutoff := manager.clock().UTC().Add(-manager.retention.MaxAge)
	kept := 0
	for _, manifest := range manifests {
		if protected[manifest.BackupID] {
			kept++
			continue
		}
		removeForCount := kept >= manager.retention.MaxBackups
		// Always retain at least the newest valid backup, even if the machine
		// has not run long enough to create a fresh one within MaxAge.
		removeForAge := kept > 0 && manifest.CreatedAt.Before(cutoff)
		if !removeForCount && !removeForAge {
			kept++
			continue
		}
		if err := manager.removeBackupLocked(manifest); err != nil {
			return err
		}
	}
	return nil
}

func (manager *Manager) retentionCandidatesLocked() ([]domainbackup.Manifest, error) {
	entries, err := os.ReadDir(manager.backupDirectory)
	if err != nil {
		return nil, err
	}
	manifests := make([]domainbackup.Manifest, 0)
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isManifestName(entry.Name()) {
			continue
		}
		backupID, _ := backupIDFromManifestName(entry.Name())
		manifest, err := readManifest(manager.backupPath(entry.Name()))
		if err != nil || validateManifestNames(manifest, backupID, entry.Name()) != nil {
			// Retention never guesses about an invalid artifact.
			continue
		}
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(i, j int) bool {
		if manifests[i].CreatedAt.Equal(manifests[j].CreatedAt) {
			return manifests[i].BackupID > manifests[j].BackupID
		}
		return manifests[i].CreatedAt.After(manifests[j].CreatedAt)
	})
	return manifests, nil
}

func (manager *Manager) protectedBackupIDsLocked() (map[string]bool, bool) {
	result := make(map[string]bool)
	if _, err := os.Lstat(manager.restoreMarkerPath); err != nil {
		if os.IsNotExist(err) {
			return result, false
		}
		return result, true
	}
	marker, err := manager.readRestoreMarkerLocked()
	if err != nil {
		return result, true
	}
	if _, _, err := resolveMarkerArtifacts(StartupRestoreConfig{
		BackupDirectory: manager.backupDirectory,
	}, marker); err != nil {
		return result, true
	}
	result[marker.BackupID] = true
	result[marker.RollbackBackupID] = true
	return result, false
}

func (manager *Manager) removeBackupLocked(manifest domainbackup.Manifest) error {
	manifestPath := manager.backupPath(backupPrefix + manifest.BackupID + manifestSuffix)
	databasePath := manager.backupPath(manifest.Database.FileName)
	// Durably move the commit record out of its discoverable name first so an
	// interruption never leaves a manifest pointing at a missing snapshot. A
	// crash may leave only a hidden delete tombstone, which startup reconciliation
	// owns and which cannot be mistaken for a committed backup.
	if err := removeBackupArtifactDurably(manifestPath); err != nil {
		return fmt.Errorf("remove expired backup manifest: %w", err)
	}
	if err := removeBackupArtifactDurably(databasePath); err != nil {
		return fmt.Errorf("remove expired metadata snapshot: %w", err)
	}
	return nil
}

func (manager *Manager) backupPath(name string) string {
	return manager.backupDirectory + string(os.PathSeparator) + name
}
