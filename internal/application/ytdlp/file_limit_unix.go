//go:build unix

package ytdlp

import "syscall"

func ensureYTDLPFileLimit(minimum uint64) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return
	}
	target := ytdlpFileLimitTarget(limit.Cur, limit.Max, minimum)
	if target <= limit.Cur {
		return
	}
	limit.Cur = target
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit)
}
