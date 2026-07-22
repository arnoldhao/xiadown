//go:build darwin || linux

package libraryapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// openPublicAssetNoFollow walks from the filesystem root using directory file
// descriptors. Every component is opened relative to the already-open parent
// and with O_NOFOLLOW, so renaming or replacing an ancestor cannot redirect a
// public asset request to another file between validation and open.
func openPublicAssetNoFollow(path string) (*os.File, error) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(cleaned) || cleaned == string(os.PathSeparator) {
		return nil, fmt.Errorf("%w: Library asset path must be an absolute file", os.ErrNotExist)
	}
	components := strings.Split(strings.TrimPrefix(cleaned, string(os.PathSeparator)), string(os.PathSeparator))
	current, err := unix.Open(string(os.PathSeparator), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	for index, component := range components {
		if component == "" || component == "." || component == ".." {
			_ = unix.Close(current)
			return nil, fmt.Errorf("%w: invalid Library asset path", os.ErrNotExist)
		}
		// O_NONBLOCK prevents a catalogued path replaced by a FIFO/device from
		// hanging before the caller can fstat and reject non-regular content.
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if index < len(components)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(current, component, flags, 0)
		_ = unix.Close(current)
		if openErr != nil {
			if openErr == unix.ELOOP || openErr == unix.ENOTDIR {
				return nil, fmt.Errorf("%w: Library asset path contains a symbolic link or non-directory ancestor", os.ErrNotExist)
			}
			return nil, openErr
		}
		current = next
	}
	file := os.NewFile(uintptr(current), cleaned)
	if file == nil {
		_ = unix.Close(current)
		return nil, fmt.Errorf("open Library asset")
	}
	return file, nil
}
