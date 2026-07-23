//go:build windows

package processutil

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureCLIPreventsAConsoleWindowAndPreservesExistingFlags(t *testing.T) {
	command := exec.Command("cmd.exe", "/c", "exit", "0")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}

	ConfigureCLI(command)

	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow {
		t.Fatal("ConfigureCLI must hide the child process window")
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("ConfigureCLI must prevent Windows from allocating a child console")
	}
	if command.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("ConfigureCLI must preserve existing creation flags")
	}
}
