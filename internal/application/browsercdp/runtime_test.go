package browsercdp

import (
	"strings"
	"testing"
)

func TestBuildLaunchArgs_HeadfulAllowsStartupPage(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if strings.Contains(joined, " --no-startup-window ") {
		t.Fatalf("expected headful launch to allow a startup page, args=%v", args)
	}
}

func TestAppendStartupPageArg_HeadfulAddsPageAfterExtraArgs(t *testing.T) {
	t.Parallel()

	args := appendStartupPageArg([]string{"--custom-flag"}, LaunchOptions{})

	if got := args[len(args)-1]; got != "about:blank" {
		t.Fatalf("expected startup page to be the final arg, got %q in %v", got, args)
	}
}

func TestBuildLaunchArgs_HeadlessKeepsHiddenMode(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{Headless: true})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --headless=new ") {
		t.Fatalf("expected headless launch flag, args=%v", args)
	}
	if strings.Contains(joined, " --new-window ") {
		t.Fatalf("expected headless launch to skip visible window, args=%v", args)
	}
}

func TestAppendStartupPageArg_HeadlessSkipsPageArg(t *testing.T) {
	t.Parallel()

	args := appendStartupPageArg([]string{"--custom-flag"}, LaunchOptions{Headless: true})

	if len(args) != 1 || args[0] != "--custom-flag" {
		t.Fatalf("expected headless launch args unchanged, got %v", args)
	}
}
