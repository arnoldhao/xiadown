package firewall

import (
	"encoding/base64"
	"encoding/binary"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestWindowsRuleIsPrivateProgramPortAndLocalSubnetScoped(t *testing.T) {
	script, err := PowerShellEnableScript(Rule{Program: `C:\Program Files\XiaDown\xiadown.exe`, Port: 48123})
	if err != nil {
		t.Fatalf("PowerShellEnableScript: %v", err)
	}
	for _, required := range []string{
		"$ErrorActionPreference='Stop'",
		"-Direction Inbound", "-Action Allow", "-Profile Private", "-Protocol TCP",
		"-LocalPort 48123", "-Program 'C:\\Program Files\\XiaDown\\xiadown.exe'", "-RemoteAddress LocalSubnet",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("script missing %q: %s", required, script)
		}
	}
	for _, forbidden := range []string{"-Profile Public", "-RemoteAddress Any", "Set-NetFirewallProfile"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("script contains forbidden scope %q: %s", forbidden, script)
		}
	}
}

func TestDisableRemovesOnlyXiaDownRule(t *testing.T) {
	script := PowerShellDisableScript()
	if !strings.Contains(script, "-DisplayName 'XiaDown Library LAN'") || strings.Contains(script, "Set-NetFirewallProfile") {
		t.Fatalf("unsafe disable script: %s", script)
	}
}

func TestWindowsRuleCommandUsesUACOnlyAfterExactScopedPreflight(t *testing.T) {
	command, err := PowerShellEnableCommand(Rule{Program: `C:\Program Files\XiaDown\xiadown.exe`, Port: 48123})
	if err != nil {
		t.Fatalf("PowerShellEnableCommand: %v", err)
	}
	for _, required := range []string{
		"$needsChange", "Get-NetFirewallPortFilter", "Get-NetFirewallApplicationFilter",
		"Get-NetFirewallAddressFilter", "LocalSubnet", "Private", "48123",
		"-Verb RunAs", "-WindowStyle Hidden", "-Wait", "-PassThru", "-EncodedCommand",
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("elevated command missing %q: %s", required, command)
		}
	}
	decoded := decodeEmbeddedPowerShellCommand(t, command)
	for _, required := range []string{
		"-Profile Private", "-Protocol TCP", "-LocalPort 48123",
		"-Program 'C:\\Program Files\\XiaDown\\xiadown.exe'", "-RemoteAddress LocalSubnet",
	} {
		if !strings.Contains(decoded, required) {
			t.Fatalf("decoded elevated mutation missing %q: %s", required, decoded)
		}
	}
}

func TestWindowsDisableCommandIsNoOpWithoutRuleAndOtherwiseElevates(t *testing.T) {
	command := PowerShellDisableCommand()
	for _, required := range []string{"$rules.Count -eq 0", "-Verb RunAs", "-WindowStyle Hidden", "-EncodedCommand"} {
		if !strings.Contains(command, required) {
			t.Fatalf("disable command missing %q: %s", required, command)
		}
	}
	decoded := decodeEmbeddedPowerShellCommand(t, command)
	if !strings.Contains(decoded, "-DisplayName 'XiaDown Library LAN'") || strings.Contains(decoded, "Set-NetFirewallProfile") {
		t.Fatalf("unsafe decoded disable command: %s", decoded)
	}
}

func decodeEmbeddedPowerShellCommand(t *testing.T, command string) string {
	t.Helper()
	match := regexp.MustCompile(`\$encoded='([A-Za-z0-9+/=]+)'`).FindStringSubmatch(command)
	if len(match) != 2 {
		t.Fatalf("embedded encoded command missing: %s", command)
	}
	data, err := base64.StdEncoding.DecodeString(match[1])
	if err != nil || len(data)%2 != 0 {
		t.Fatalf("decode embedded command: %v", err)
	}
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(data[index*2:])
	}
	return string(utf16.Decode(units))
}
