//go:build windows

package libraryapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openPublicAssetNoFollow(path string) (*os.File, error) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(cleaned) {
		return nil, fmt.Errorf("%w: Library asset path must be absolute", os.ErrNotExist)
	}
	volume := filepath.VolumeName(cleaned)
	if volume == "" {
		return nil, fmt.Errorf("%w: Library asset volume is missing", os.ErrNotExist)
	}
	rootPath := volume + string(os.PathSeparator)
	relative := strings.TrimPrefix(cleaned[len(volume):], string(os.PathSeparator))
	components := strings.Split(relative, string(os.PathSeparator))
	if relative == "" || len(components) == 0 {
		return nil, fmt.Errorf("%w: Library asset path must name a file", os.ErrNotExist)
	}

	rootPointer, err := windows.UTF16PtrFromString(rootPath)
	if err != nil {
		return nil, err
	}
	current, err := windows.CreateFile(
		rootPointer,
		windows.FILE_GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	if err := requireWindowsAssetHandle(current, true); err != nil {
		_ = windows.CloseHandle(current)
		return nil, err
	}

	for index, component := range components {
		if component == "" || component == "." || component == ".." || strings.ContainsRune(component, ':') {
			_ = windows.CloseHandle(current)
			return nil, fmt.Errorf("%w: invalid Library asset path component", os.ErrNotExist)
		}
		objectName, nameErr := windows.NewNTUnicodeString(component)
		if nameErr != nil {
			_ = windows.CloseHandle(current)
			return nil, nameErr
		}
		attributes := &windows.OBJECT_ATTRIBUTES{
			RootDirectory: current,
			ObjectName:    objectName,
			Attributes:    windows.OBJ_CASE_INSENSITIVE,
		}
		attributes.Length = uint32(unsafe.Sizeof(*attributes))
		options := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT)
		directory := index < len(components)-1
		if directory {
			options |= windows.FILE_DIRECTORY_FILE
		} else {
			options |= windows.FILE_NON_DIRECTORY_FILE
		}
		var next windows.Handle
		openErr := windows.NtCreateFile(
			&next,
			windows.FILE_GENERIC_READ,
			attributes,
			&windows.IO_STATUS_BLOCK{},
			nil,
			windows.FILE_ATTRIBUTE_NORMAL,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			windows.FILE_OPEN,
			options,
			0,
			0,
		)
		_ = windows.CloseHandle(current)
		if openErr != nil {
			return nil, openErr
		}
		if handleErr := requireWindowsAssetHandle(next, directory); handleErr != nil {
			_ = windows.CloseHandle(next)
			return nil, handleErr
		}
		current = next
	}

	file := os.NewFile(uintptr(current), cleaned)
	if file == nil {
		_ = windows.CloseHandle(current)
		return nil, fmt.Errorf("open Library asset handle")
	}
	return file, nil
}

func requireWindowsAssetHandle(handle windows.Handle, directory bool) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("%w: Library asset path contains a reparse point", os.ErrNotExist)
	}
	isDirectory := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if isDirectory != directory {
		return fmt.Errorf("%w: invalid Library asset path component type", os.ErrNotExist)
	}
	return nil
}
