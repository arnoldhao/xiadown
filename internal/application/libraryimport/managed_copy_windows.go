//go:build windows

package libraryimport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsManagedShare = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

type windowsManagedDirectory struct {
	path   string
	handle windows.Handle
}

type windowsFileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func openManagedDirectory(
	root, relative string,
	create bool,
	mode os.FileMode,
) (managedDirectory, error) {
	_ = mode // Windows access is controlled by the selected root's DACL.
	relativeComponents, err := managedRelativeComponents(relative)
	if err != nil {
		return nil, err
	}
	volume, rootComponents, err := windowsAbsoluteComponents(root)
	if err != nil {
		return nil, err
	}
	volumePtr, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return nil, err
	}
	current, err := windows.CreateFile(
		volumePtr,
		windows.FILE_GENERIC_READ,
		windowsManagedShare,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	if err := requireWindowsDirectoryHandle(current); err != nil {
		_ = windows.CloseHandle(current)
		return nil, err
	}
	for _, component := range rootComponents {
		next, openErr := openWindowsDirectoryAt(current, component, false)
		_ = windows.CloseHandle(current)
		if openErr != nil {
			return nil, fmt.Errorf("open managed root component %q without following reparse points: %w", component, openErr)
		}
		current = next
	}
	for _, component := range relativeComponents {
		next, openErr := openWindowsDirectoryAt(current, component, create)
		_ = windows.CloseHandle(current)
		if openErr != nil {
			return nil, fmt.Errorf("open managed directory component %q without following reparse points: %w", component, openErr)
		}
		current = next
	}
	return &windowsManagedDirectory{path: filepath.Join(root, relative), handle: current}, nil
}

func windowsAbsoluteComponents(path string) (string, []string, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", nil, fmt.Errorf("managed root must be absolute")
	}
	volume := filepath.VolumeName(path)
	if volume == "" {
		return "", nil, fmt.Errorf("managed root has no Windows volume")
	}
	volumeRoot := volume + string(os.PathSeparator)
	remainder := strings.TrimPrefix(path[len(volume):], string(os.PathSeparator))
	components, err := managedRelativeComponents(remainder)
	if err != nil {
		return "", nil, err
	}
	return volumeRoot, components, nil
}

func openWindowsDirectoryAt(parent windows.Handle, name string, create bool) (windows.Handle, error) {
	if err := validateManagedLeafName(name); err != nil {
		return 0, err
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	disposition := uint32(windows.FILE_OPEN)
	if create {
		disposition = windows.FILE_OPEN_IF
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	var allocationSize int64
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ,
		attributes,
		&status,
		&allocationSize,
		windows.FILE_ATTRIBUTE_DIRECTORY,
		windowsManagedShare,
		disposition,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err != nil {
		return 0, err
	}
	if err := requireWindowsDirectoryHandle(handle); err != nil {
		_ = windows.CloseHandle(handle)
		return 0, err
	}
	return handle, nil
}

func requireWindowsDirectoryHandle(handle windows.Handle) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("managed path contains a Windows reparse point")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return fmt.Errorf("managed path component is not a directory")
	}
	return nil
}

func openWindowsFileAt(
	directory windows.Handle,
	name string,
	access, disposition uint32,
) (windows.Handle, error) {
	if err := validateManagedLeafName(name); err != nil {
		return 0, err
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: directory,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	var allocationSize int64
	if err := windows.NtCreateFile(
		&handle,
		access,
		attributes,
		&status,
		&allocationSize,
		windows.FILE_ATTRIBUTE_NORMAL,
		windowsManagedShare,
		disposition,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	); err != nil {
		return 0, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return 0, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		_ = windows.CloseHandle(handle)
		return 0, fmt.Errorf("managed file is a reparse point or directory")
	}
	return handle, nil
}

func (directory *windowsManagedDirectory) absolutePath() string { return directory.path }

func (directory *windowsManagedDirectory) openExisting(name string) (*os.File, error) {
	handle, err := openWindowsFileAt(directory.handle, name, windows.FILE_GENERIC_READ, windows.FILE_OPEN)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), filepath.Join(directory.path, name)), nil
}

func (directory *windowsManagedDirectory) createExclusive(name string, mode os.FileMode) (*os.File, error) {
	_ = mode
	handle, err := openWindowsFileAt(
		directory.handle,
		name,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE,
		windows.FILE_CREATE,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), filepath.Join(directory.path, name)), nil
}

func (directory *windowsManagedDirectory) remove(name string) error {
	handle, err := openWindowsFileAt(
		directory.handle,
		name,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_OPEN,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	deleteFile := byte(1)
	return windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteFile, 1)
}

func (directory *windowsManagedDirectory) publishNoReplace(sourceName, destinationName string) error {
	if err := validateManagedLeafName(destinationName); err != nil {
		return err
	}
	handle, err := openWindowsFileAt(
		directory.handle,
		sourceName,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_OPEN,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	destinationUTF16, err := windows.UTF16FromString(destinationName)
	if err != nil {
		return err
	}
	nameLength := (len(destinationUTF16) - 1) * 2
	var sample windowsFileRenameInformation
	bufferSize := int(unsafe.Offsetof(sample.FileName)) + nameLength
	buffer := make([]byte, bufferSize)
	info := (*windowsFileRenameInformation)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = directory.handle
	info.FileNameLength = uint32(nameLength)
	copy(
		(*[windows.MAX_LONG_PATH]uint16)(unsafe.Pointer(&info.FileName[0]))[:nameLength/2:nameLength/2],
		destinationUTF16[:len(destinationUTF16)-1],
	)
	var status windows.IO_STATUS_BLOCK
	err = windows.NtSetInformationFile(
		handle, &status, &buffer[0], uint32(bufferSize), windows.FileRenameInformation,
	)
	if errors.Is(err, windows.STATUS_OBJECT_NAME_COLLISION) ||
		errors.Is(err, windows.STATUS_OBJECT_NAME_EXISTS) ||
		errors.Is(err, windows.ERROR_ALREADY_EXISTS) ||
		errors.Is(err, windows.ERROR_FILE_EXISTS) {
		return os.ErrExist
	}
	return err
}

func (directory *windowsManagedDirectory) sync() error {
	err := windows.FlushFileBuffers(directory.handle)
	if errors.Is(err, windows.ERROR_INVALID_HANDLE) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil
	}
	return err
}

func (directory *windowsManagedDirectory) close() error {
	return windows.CloseHandle(directory.handle)
}
