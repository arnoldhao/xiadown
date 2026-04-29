//go:build windows

package ytdlp

import (
	"os/exec"
	"strconv"
	"syscall"

	"xiadown/internal/infrastructure/processutil"
)

func ConfigureProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func terminateProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	killCmd := exec.Command("taskkill", "/T", "/F", "/PID", pid)
	processutil.ConfigureCLI(killCmd)
	return killCmd.Run()
}
