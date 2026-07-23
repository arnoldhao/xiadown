//go:build windows

package processutil

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW prevents a console application from allocating a console;
// HideWindow remains set as a fallback for child processes that ignore it.
const createNoWindow = 0x08000000

func ConfigureCLI(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
