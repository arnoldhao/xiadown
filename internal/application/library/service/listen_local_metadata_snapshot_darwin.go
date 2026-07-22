//go:build darwin

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
		"%d:%d:%d:%d:%d:%d:%d",
		stat.Uid,
		stat.Gid,
		stat.Nlink,
		stat.Ctimespec.Sec,
		stat.Ctimespec.Nsec,
		stat.Flags,
		stat.Gen,
	)
}
