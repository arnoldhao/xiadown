//go:build linux

package service

import (
	"fmt"
	"os"
	"syscall"
)

func listenLocalFileChangeToken(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return ""
	}
	return fmt.Sprintf(
		"%d:%d:%d:%d:%d",
		stat.Uid,
		stat.Gid,
		stat.Nlink,
		stat.Ctim.Sec,
		stat.Ctim.Nsec,
	)
}
