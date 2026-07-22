//go:build windows

package service

import (
	"os/exec"

	"xiadown/internal/infrastructure/processutil"
)

func configureCommand(cmd *exec.Cmd) {
	processutil.ConfigureCLI(cmd)
}
