//go:build unix

package browsercdp

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func runtimeProcessIDs(cmd *exec.Cmd) (int, int) {
	if cmd == nil || cmd.Process == nil {
		return 0, 0
	}
	rootPID := cmd.Process.Pid
	pgid, err := syscall.Getpgid(rootPID)
	if err != nil || pgid <= 0 {
		return rootPID, rootPID
	}
	return rootPID, pgid
}

func terminateRuntimeProcessGroup(rootPID int, processGroupID int) error {
	return terminateRuntimeProcessGroupWithGrace(rootPID, processGroupID, 300*time.Millisecond)
}

func terminateRuntimeProcessGroupWithGrace(rootPID int, processGroupID int, killDelay time.Duration) error {
	if killDelay <= 0 {
		killDelay = 300 * time.Millisecond
	}
	if processGroupID > 0 {
		err := syscall.Kill(-processGroupID, syscall.SIGTERM)
		if err != nil && errors.Is(err, syscall.ESRCH) {
			return nil
		}
		time.Sleep(killDelay)
		err = syscall.Kill(-processGroupID, syscall.SIGKILL)
		if err == nil || errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	if rootPID <= 0 {
		return nil
	}
	err := syscall.Kill(rootPID, syscall.SIGTERM)
	if err != nil && errors.Is(err, syscall.ESRCH) {
		return nil
	}
	time.Sleep(killDelay)
	err = syscall.Kill(rootPID, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
