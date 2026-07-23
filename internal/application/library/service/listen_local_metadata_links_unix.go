//go:build darwin || linux

package service

import (
	"os"
	"syscall"
)

func listenLocalFileLinkCount(_ string, info os.FileInfo) (uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 1, nil
	}
	return uint64(stat.Nlink), nil
}
