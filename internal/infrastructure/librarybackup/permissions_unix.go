//go:build !windows

package librarybackup

import "os"

func restrictBackupDirectory(path string) error {
	return os.Chmod(path, backupDirectoryMode)
}

func restrictBackupFile(path string) error {
	return os.Chmod(path, backupFileMode)
}
