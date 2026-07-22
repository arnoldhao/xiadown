//go:build windows

package firewall

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type failingWindowsRunner struct{ err error }

func (runner failingWindowsRunner) Run(context.Context, string, ...string) error { return runner.err }

func TestWindowsManagerReportsActionableElevationFailure(t *testing.T) {
	manager := newPlatformManager(failingWindowsRunner{err: errors.New("access denied")})
	err := manager.Enable(context.Background(), Rule{
		Program: `C:\Program Files\XiaDown\xiadown.exe`,
		Port:    48123,
	})
	if !errors.Is(err, ErrRuleNotApplied) {
		t.Fatalf("Enable error = %v, want ErrRuleNotApplied", err)
	}
	message := strings.ToLower(err.Error())
	for _, required := range []string{"administrator", "private", "localsubnet", "48123"} {
		if !strings.Contains(message, required) {
			t.Fatalf("actionable error missing %q: %v", required, err)
		}
	}
}
