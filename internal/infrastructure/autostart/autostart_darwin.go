//go:build darwin
// +build darwin

package autostart

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func setEnabled(_ string, label string, execPath string, launchArg string, enabled bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	plistPath := filepath.Join(launchAgentsDir, fmt.Sprintf("%s.plist", label))
	if !enabled {
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(launchAgentsDir, 0o755); err != nil {
		return err
	}

	plist := launchAgentPlist(label, execPath, launchArg)
	return os.WriteFile(plistPath, []byte(plist), 0o644)
}

func launchAgentPlist(label string, execPath string, launchArg string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, escapePlistString(label), escapePlistString(execPath), escapePlistString(launchArg))
}

func escapePlistString(value string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(value)); err != nil {
		return ""
	}
	return builder.String()
}
