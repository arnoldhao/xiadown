//go:build !darwin && !linux && !windows

package libraryapi

import (
	"fmt"
	"os"
)

func openPublicAssetNoFollow(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: Library asset is a symbolic link", os.ErrNotExist)
	}
	return os.Open(path)
}
