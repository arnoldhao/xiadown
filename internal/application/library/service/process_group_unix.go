//go:build unix

package service

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

func processGroupID(cmd *exec.Cmd) int {
	if cmd == nil || cmd.Process == nil {
		return 0
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Pid
	}
	return pgid
}

func terminateExternalProcessGroup(rootPID int, processGroupID int) error {
	if processGroupID > 0 {
		err := syscall.Kill(-processGroupID, syscall.SIGTERM)
		if err != nil && errors.Is(err, syscall.ESRCH) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
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
	time.Sleep(300 * time.Millisecond)
	err = syscall.Kill(rootPID, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
