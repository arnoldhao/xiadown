//go:build windows

package firewall

import (
	"context"
	"fmt"
)

type windowsManager struct{ runner CommandRunner }

func newPlatformManager(runner CommandRunner) Manager { return &windowsManager{runner: runner} }

func (manager *windowsManager) Enable(ctx context.Context, rule Rule) error {
	script, err := PowerShellEnableCommand(rule)
	if err != nil {
		return err
	}
	if err := manager.runner.Run(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf(
			"%w: automatic Windows firewall setup failed; approve administrator access and retry, or add an inbound TCP rule for XiaDown port %d scoped to the Private profile and LocalSubnet only: %v",
			ErrRuleNotApplied,
			rule.Port,
			err,
		)
	}
	return nil
}

func (manager *windowsManager) Disable(ctx context.Context) error {
	return manager.runner.Run(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", PowerShellDisableCommand())
}
