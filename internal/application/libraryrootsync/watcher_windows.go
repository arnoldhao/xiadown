//go:build windows

package libraryrootsync

import (
	"context"
	"errors"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsNativeWatcher struct{}

func platformNativeWatcher() nativeWatcher {
	return windowsNativeWatcher{}
}

func (windowsNativeWatcher) Available() bool      { return true }
func (windowsNativeWatcher) SupportsReplay() bool { return false }

func (windowsNativeWatcher) Watch(
	ctx context.Context,
	rootPath string,
	_ uint64,
	emit func(watchEvent),
) error {
	path, err := windows.UTF16PtrFromString(filepath.Clean(rootPath))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(
		path,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event)

	buffer := make([]byte, 64*1024)
	const mask = windows.FILE_NOTIFY_CHANGE_FILE_NAME |
		windows.FILE_NOTIFY_CHANGE_DIR_NAME |
		windows.FILE_NOTIFY_CHANGE_SIZE |
		windows.FILE_NOTIFY_CHANGE_LAST_WRITE |
		windows.FILE_NOTIFY_CHANGE_CREATION
	for {
		if err := windows.ResetEvent(event); err != nil {
			return err
		}
		var bytesReturned uint32
		overlapped := windows.Overlapped{HEvent: event}
		err = windows.ReadDirectoryChanges(
			handle,
			&buffer[0],
			uint32(len(buffer)),
			true,
			mask,
			&bytesReturned,
			&overlapped,
			0,
		)
		if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
			if errors.Is(err, windows.ERROR_NOTIFY_ENUM_DIR) {
				emit(watchEvent{overflow: true})
				continue
			}
			return err
		}
		for {
			if ctx.Err() != nil {
				_ = windows.CancelIoEx(handle, &overlapped)
				return ctx.Err()
			}
			waitResult, waitErr := windows.WaitForSingleObject(event, 250)
			if waitErr != nil {
				return waitErr
			}
			if waitResult == uint32(windows.WAIT_TIMEOUT) {
				continue
			}
			if waitResult != windows.WAIT_OBJECT_0 {
				return errors.New("unexpected ReadDirectoryChangesW wait result")
			}
			break
		}
		if err := windows.GetOverlappedResult(
			handle,
			&overlapped,
			&bytesReturned,
			false,
		); err != nil {
			if ctx.Err() != nil ||
				errors.Is(err, windows.ERROR_OPERATION_ABORTED) ||
				errors.Is(err, windows.ERROR_INVALID_HANDLE) {
				return ctx.Err()
			}
			if errors.Is(err, windows.ERROR_NOTIFY_ENUM_DIR) {
				emit(watchEvent{overflow: true})
				continue
			}
			return err
		}
		if bytesReturned == 0 {
			emit(watchEvent{overflow: true})
			continue
		}
		parseWindowsNotifyBuffer(
			buffer[:bytesReturned],
			rootPath,
			emit,
		)
	}
}

func parseWindowsNotifyBuffer(
	buffer []byte,
	rootPath string,
	emit func(watchEvent),
) {
	for offset := uint32(0); offset+12 <= uint32(len(buffer)); {
		record := (*windows.FileNotifyInformation)(
			unsafe.Pointer(&buffer[offset]),
		)
		nameOffset := offset + 12
		nameLength := record.FileNameLength
		if nameLength%2 != 0 ||
			nameOffset+nameLength > uint32(len(buffer)) {
			emit(watchEvent{overflow: true})
			return
		}
		units := unsafe.Slice(
			(*uint16)(unsafe.Pointer(&buffer[nameOffset])),
			nameLength/2,
		)
		emit(watchEvent{path: filepath.Join(rootPath, windows.UTF16ToString(units))})
		if record.NextEntryOffset == 0 {
			return
		}
		if record.NextEntryOffset < 12 ||
			offset+record.NextEntryOffset > uint32(len(buffer)) {
			emit(watchEvent{overflow: true})
			return
		}
		offset += record.NextEntryOffset
	}
}
