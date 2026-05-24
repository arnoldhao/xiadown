//go:build windows

package browsercdp

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"

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

func runtimeProcessIDs(cmd *exec.Cmd) (int, int) {
	if cmd == nil || cmd.Process == nil {
		return 0, 0
	}
	return cmd.Process.Pid, cmd.Process.Pid
}

func terminateRuntimeProcessGroup(rootPID int, _ int) error {
	return terminateRuntimeProcessGroupWithGrace(rootPID, 0, 0)
}

func terminateRuntimeProcessGroupWithGrace(rootPID int, _ int, _ time.Duration) error {
	if rootPID <= 0 {
		return nil
	}
	command := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(rootPID))
	processutil.ConfigureCLI(command)
	return command.Run()
}
