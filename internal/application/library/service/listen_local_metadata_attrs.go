package service

import (
	"fmt"
	"os"

	"xiadown/internal/domain/library"
)

func prepareListenLocalMetadataReplacement(originalPath string, replacementPath string, original os.FileInfo) error {
	if err := copyListenLocalMetadataOwnership(replacementPath, original); err != nil {
		return err
	}
	mode := original.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if err := os.Chmod(replacementPath, mode); err != nil {
		return err
	}
	if err := copyListenLocalMetadataExtendedAttributes(originalPath, replacementPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("%w: preserve filesystem metadata: %v", library.ErrListenLocalFilePermission, err)
		}
		return listenLocalMetadataPreservationError(fmt.Sprintf("filesystem metadata: %v", err))
	}
	return nil
}
