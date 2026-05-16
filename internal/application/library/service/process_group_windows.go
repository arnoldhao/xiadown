//go:build windows

package service

import (
	"os/exec"
	"strconv"
	"syscall"

	"xiadown/internal/infrastructure/processutil"
)

func configureProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func processGroupID(cmd *exec.Cmd) int {
	if cmd == nil || cmd.Process == nil {
		return 0
	}
	return cmd.Process.Pid
}

func terminateExternalProcessGroup(rootPID int, _ int) error {
	if rootPID <= 0 {
		return nil
	}
	command := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(rootPID))
	processutil.ConfigureCLI(command)
	return command.Run()
}
