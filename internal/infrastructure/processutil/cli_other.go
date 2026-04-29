//go:build !windows

package processutil

import "os/exec"

func ConfigureCLI(_ *exec.Cmd) {}
