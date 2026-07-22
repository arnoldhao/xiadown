package librarybackup

import (
	"fmt"
	"os"
	"path/filepath"
)

// removeBackupArtifactDurably first performs a durable same-directory rename
// away from the artifact's meaningful name. This gives Windows the same
// crash-safe namespace transition as publication (MOVEFILE_WRITE_THROUGH) and
// gives Unix a transition that can be fsynced before unlink. If the final
// unlink is interrupted, startup reconciliation recognizes the hidden
// .delete.tmp tombstone and retries without resurrecting a committed backup.
func removeBackupArtifactDurably(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() || (!info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0) {
		return fmt.Errorf("backup artifact cannot be retired safely")
	}
	directory := filepath.Dir(path)
	tombstone := filepath.Join(directory, "."+backupPrefix+"delete-"+randomSuffix()+".delete.tmp")
	if err := durableRename(path, tombstone, false); err != nil {
		return fmt.Errorf("retire backup artifact: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync retired backup artifact: %w", err)
	}
	if err := os.Remove(tombstone); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unlink retired backup artifact: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync removed backup artifact: %w", err)
	}
	return nil
}
