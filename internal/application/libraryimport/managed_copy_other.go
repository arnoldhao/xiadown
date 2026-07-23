//go:build !darwin && !linux && !windows

package libraryimport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type genericManagedDirectory struct{ path string }

func openManagedDirectory(
	root, relative string,
	create bool,
	mode os.FileMode,
) (managedDirectory, error) {
	components, err := managedRelativeComponents(relative)
	if err != nil {
		return nil, err
	}
	current := root
	if err := requireGenericDirectoryNoLinks(current); err != nil {
		return nil, err
	}
	for _, component := range components {
		current = filepath.Join(current, component)
		if create {
			if err := os.Mkdir(current, mode); err != nil && !errors.Is(err, os.ErrExist) {
				return nil, err
			}
		}
		if err := requireGenericDirectoryNoLinks(current); err != nil {
			return nil, err
		}
	}
	return &genericManagedDirectory{path: current}, nil
}

func requireGenericDirectoryNoLinks(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed path component is not a real directory")
	}
	return nil
}

func (directory *genericManagedDirectory) absolutePath() string { return directory.path }

func (directory *genericManagedDirectory) openExisting(name string) (*os.File, error) {
	if err := validateManagedLeafName(name); err != nil {
		return nil, err
	}
	path := filepath.Join(directory.path, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("managed file is a symlink")
	}
	return os.Open(path)
}

func (directory *genericManagedDirectory) createExclusive(name string, mode os.FileMode) (*os.File, error) {
	if err := validateManagedLeafName(name); err != nil {
		return nil, err
	}
	if err := requireGenericDirectoryNoLinks(directory.path); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(directory.path, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
}

func (directory *genericManagedDirectory) remove(name string) error {
	if err := validateManagedLeafName(name); err != nil {
		return err
	}
	path := filepath.Join(directory.path, name)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed file is a symlink")
	}
	return os.Remove(path)
}

func (directory *genericManagedDirectory) publishNoReplace(sourceName, destinationName string) error {
	if err := validateManagedLeafName(sourceName); err != nil {
		return err
	}
	if err := validateManagedLeafName(destinationName); err != nil {
		return err
	}
	if err := requireGenericDirectoryNoLinks(directory.path); err != nil {
		return err
	}
	if err := os.Link(filepath.Join(directory.path, sourceName), filepath.Join(directory.path, destinationName)); err != nil {
		return err
	}
	return directory.remove(sourceName)
}

func (directory *genericManagedDirectory) sync() error {
	handle, err := os.Open(directory.path)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}

func (*genericManagedDirectory) close() error { return nil }
