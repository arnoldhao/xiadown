//go:build windows

package update

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
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
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// A fresh process group is enough here. Using DETACHED_PROCESS with
		// powershell.exe has proven unreliable for the update helper.
		CreationFlags: createNewProcessGroup,
		HideWindow:    true,
	}
	return cmd.Start()
}
