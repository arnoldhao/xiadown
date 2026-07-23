//go:build darwin || linux

package libraryimport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type unixManagedDirectory struct {
	path   string
	handle *os.File
}

func openManagedDirectory(
	root, relative string,
	create bool,
	mode os.FileMode,
) (managedDirectory, error) {
	components, err := managedRelativeComponents(relative)
	if err != nil {
		return nil, err
	}
	current, err := openAbsoluteDirectoryNoFollow(root)
	if err != nil {
		return nil, err
	}
	for _, component := range components {
		if create {
			err := unix.Mkdirat(int(current.Fd()), component, uint32(mode.Perm()))
			if err != nil && !errors.Is(err, unix.EEXIST) {
				_ = current.Close()
				return nil, err
			}
		}
		fd, openErr := unix.Openat(
			int(current.Fd()), component,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
			0,
		)
		_ = current.Close()
		if openErr != nil {
			return nil, fmt.Errorf("open managed directory component %q without following links: %w", component, openErr)
		}
		current = os.NewFile(uintptr(fd), component)
		if current == nil {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("open managed directory component %q", component)
		}
	}
	return &unixManagedDirectory{path: filepath.Join(root, relative), handle: current}, nil
}

func openAbsoluteDirectoryNoFollow(path string) (*os.File, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("managed root must be absolute")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(fd), "/")
	if current == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open filesystem root")
	}
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if component == "" {
			continue
		}
		nextFD, openErr := unix.Openat(
			int(current.Fd()), component,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
			0,
		)
		_ = current.Close()
		if openErr != nil {
			return nil, fmt.Errorf("open managed root component %q without following links: %w", component, openErr)
		}
		current = os.NewFile(uintptr(nextFD), component)
		if current == nil {
			_ = unix.Close(nextFD)
			return nil, fmt.Errorf("open managed root component %q", component)
		}
	}
	return current, nil
}

func (directory *unixManagedDirectory) absolutePath() string { return directory.path }

func (directory *unixManagedDirectory) openExisting(name string) (*os.File, error) {
	if err := validateManagedLeafName(name); err != nil {
		return nil, err
	}
	fd, err := unix.Openat(
		int(directory.handle.Fd()), name,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(directory.path, name))
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open managed file %q", name)
	}
	return file, nil
}

func (directory *unixManagedDirectory) createExclusive(name string, mode os.FileMode) (*os.File, error) {
	if err := validateManagedLeafName(name); err != nil {
		return nil, err
	}
	fd, err := unix.Openat(
		int(directory.handle.Fd()), name,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		uint32(mode.Perm()),
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(directory.path, name))
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("create managed file %q", name)
	}
	return file, nil
}

func (directory *unixManagedDirectory) remove(name string) error {
	if err := validateManagedLeafName(name); err != nil {
		return err
	}
	return unix.Unlinkat(int(directory.handle.Fd()), name, 0)
}

func (directory *unixManagedDirectory) publishNoReplace(sourceName, destinationName string) error {
	if err := validateManagedLeafName(sourceName); err != nil {
		return err
	}
	if err := validateManagedLeafName(destinationName); err != nil {
		return err
	}
	return atomicPublishNoReplaceAt(int(directory.handle.Fd()), sourceName, destinationName)
}

func (directory *unixManagedDirectory) sync() error { return directory.handle.Sync() }

func (directory *unixManagedDirectory) close() error { return directory.handle.Close() }
