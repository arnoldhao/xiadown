//go:build darwin || linux

package service

import (
	"fmt"
	"os"
	"syscall"

	"xiadown/internal/domain/library"
)

func copyListenLocalMetadataOwnership(replacementPath string, original os.FileInfo) error {
	originalStat, ok := original.Sys().(*syscall.Stat_t)
	if !ok || originalStat == nil {
		return nil
	}
	replacement, err := os.Stat(replacementPath)
	if err != nil {
		return err
	}
	replacementStat, ok := replacement.Sys().(*syscall.Stat_t)
	if ok && replacementStat != nil && replacementStat.Uid == originalStat.Uid && replacementStat.Gid == originalStat.Gid {
		return nil
	}
	if err := os.Chown(replacementPath, int(originalStat.Uid), int(originalStat.Gid)); err != nil {
		return fmt.Errorf("%w: preserve file ownership: %v", library.ErrListenLocalFilePermission, err)
	}
	return nil
}
