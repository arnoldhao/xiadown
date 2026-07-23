//go:build linux

package service

import (
	"bytes"
	"errors"

	"golang.org/x/sys/unix"
)

func copyListenLocalMetadataExtendedAttributes(source string, destination string) error {
	size, err := unix.Listxattr(source, nil)
	if err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) {
			return nil
		}
		return err
	}
	if size == 0 {
		return nil
	}
	names := make([]byte, size)
	size, err = unix.Listxattr(source, names)
	if err != nil {
		return err
	}
	for _, rawName := range bytes.Split(names[:size], []byte{0}) {
		if len(rawName) == 0 {
			continue
		}
		name := string(rawName)
		valueSize, err := unix.Getxattr(source, name, nil)
		if err != nil {
			return err
		}
		value := make([]byte, valueSize)
		if valueSize > 0 {
			valueSize, err = unix.Getxattr(source, name, value)
			if err != nil {
				return err
			}
			value = value[:valueSize]
		}
		if err := unix.Setxattr(destination, name, value, 0); err != nil {
			return err
		}
	}
	return nil
}
