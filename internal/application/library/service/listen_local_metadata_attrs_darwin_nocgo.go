//go:build darwin && !cgo

package service

import (
	"fmt"

	"xiadown/internal/domain/library"
)

func copyListenLocalMetadataExtendedAttributes(string, string) error {
	return fmt.Errorf("%w: preserving macOS ACLs requires the native runtime", library.ErrListenLocalMetadataUnsupported)
}
