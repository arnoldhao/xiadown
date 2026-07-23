//go:build windows

package service

import (
	"fmt"
	"os"
	"syscall"
)

func listenLocalFileChangeToken(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || stat == nil {
		return ""
	}
	return fmt.Sprintf(
		"%d:%d:%d:%d",
		stat.FileAttributes,
		stat.CreationTime.HighDateTime,
		stat.CreationTime.LowDateTime,
		uint64(stat.FileSizeHigh)<<32|uint64(stat.FileSizeLow),
	)
}
