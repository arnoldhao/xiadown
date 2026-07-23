//go:build !windows

package firewall

import (
	"context"
	"fmt"
)

type unsupportedManager struct{}

func newPlatformManager(CommandRunner) Manager { return unsupportedManager{} }

func (unsupportedManager) Enable(context.Context, Rule) error {
	return fmt.Errorf("%w on this platform", ErrUnavailable)
}

func (unsupportedManager) Disable(context.Context) error { return nil }
