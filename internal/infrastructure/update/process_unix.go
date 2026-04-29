//go:build !windows

package update

import (
	"os"
	"os/exec"
	"syscall"
)

func startDetachedCommand(name string, args []string) error {
	cmd := exec.Command(name, args...)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}
