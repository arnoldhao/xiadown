//go:build darwin

package browsercdp

import (
	"strings"

	"golang.org/x/sys/unix"
)

func currentChromePIDMatches(pid int, _ Candidate) bool {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || process == nil || int(process.Proc.P_pid) != pid {
		return false
	}
	name := strings.TrimRight(string(process.Proc.P_comm[:]), "\x00")
	return name == "Google Chrome"
}
