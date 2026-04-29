//go:build darwin
// +build darwin

package autostart

import (
	"strings"
	"testing"
)

func TestLaunchAgentPlistEscapesStringValues(t *testing.T) {
	plist := launchAgentPlist(
		"com.xiadown.app&dev",
		"/Applications/XiaDown & Tools.app/Contents/MacOS/XiaDown",
		"--autostart",
	)

	for _, expected := range []string{
		"<string>com.xiadown.app&amp;dev</string>",
		"<string>/Applications/XiaDown &amp; Tools.app/Contents/MacOS/XiaDown</string>",
		"<string>--autostart</string>",
	} {
		if !strings.Contains(plist, expected) {
			t.Fatalf("expected plist to contain %q, got:\n%s", expected, plist)
		}
	}
}
