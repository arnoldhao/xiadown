//go:build !windows

package service

import "os/exec"

func configureCommand(cmd *exec.Cmd) {}
