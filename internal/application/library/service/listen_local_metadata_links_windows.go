//go:build windows

package service

import (
	"os"

	"golang.org/x/sys/windows"
)

func listenLocalFileLinkCount(path string, _ os.FileInfo) (uint64, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return 0, classifyListenLocalMetadataWindowsReplaceError(err)
	}
	defer windows.CloseHandle(handle)
	information := windows.ByHandleFileInformation{}
	if err := windows.GetFileInformationByHandle(handle, &information); err != nil {
		return 0, err
	}
	return uint64(information.NumberOfLinks), nil
}
