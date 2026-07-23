// Package firewall manages only XiaDown's Windows Private-network LAN rule.
package firewall

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf16"

	"xiadown/internal/infrastructure/processutil"
)

const windowsRuleName = "XiaDown Library LAN"

var (
	ErrUnavailable    = errors.New("firewall management unavailable")
	ErrRuleNotApplied = errors.New("firewall rule not applied")
)

type Rule struct {
	Program string
	Port    int
}

type Manager interface {
	Enable(context.Context, Rule) error
	Disable(context.Context) error
}

type CommandRunner interface {
	Run(context.Context, string, ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	processutil.ConfigureCLI(command)
	return command.Run()
}

func NewManager() Manager { return newPlatformManager(execRunner{}) }

func ValidateRule(rule Rule) (Rule, error) {
	rule.Program = strings.TrimSpace(rule.Program)
	if rule.Program == "" || rule.Port < 1 || rule.Port > 65535 {
		return Rule{}, fmt.Errorf("invalid firewall rule")
	}
	return rule, nil
}

func PowerShellEnableScript(rule Rule) (string, error) {
	rule, err := ValidateRule(rule)
	if err != nil {
		return "", err
	}
	program := strings.ReplaceAll(rule.Program, "'", "''")
	return fmt.Sprintf(`$ErrorActionPreference='Stop'; $name='%s'; Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue | Remove-NetFirewallRule; New-NetFirewallRule -DisplayName $name -Group 'XiaDown' -Direction Inbound -Action Allow -Enabled True -Profile Private -Protocol TCP -LocalPort %d -Program '%s' -RemoteAddress LocalSubnet | Out-Null`, windowsRuleName, rule.Port, program), nil
}

// PowerShellEnableCommand first performs a read-only exact-match check. It
// opens a UAC prompt only when the constrained rule must be created or
// repaired; a normal application launch with an already-correct rule remains
// silent. The elevated process receives an UTF-16LE EncodedCommand, avoiding
// shell interpolation of the executable path.
func PowerShellEnableCommand(rule Rule) (string, error) {
	rule, err := ValidateRule(rule)
	if err != nil {
		return "", err
	}
	mutation, err := PowerShellEnableScript(rule)
	if err != nil {
		return "", err
	}
	name := strings.ReplaceAll(windowsRuleName, "'", "''")
	program := strings.ReplaceAll(rule.Program, "'", "''")
	preflight := fmt.Sprintf(`$needsChange=$true; try { $rules=@(Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue); if ($rules.Count -eq 1) { $rule=$rules[0]; $ports=@($rule | Get-NetFirewallPortFilter); $apps=@($rule | Get-NetFirewallApplicationFilter); $addresses=@($rule | Get-NetFirewallAddressFilter); if ("$($rule.Enabled)" -eq 'True' -and "$($rule.Direction)" -eq 'Inbound' -and "$($rule.Action)" -eq 'Allow' -and "$($rule.Profile)" -eq 'Private' -and $ports.Count -eq 1 -and "$($ports[0].Protocol)" -eq 'TCP' -and "$($ports[0].LocalPort)" -eq '%d' -and $apps.Count -eq 1 -and "$($apps[0].Program)" -ieq '%s' -and $addresses.Count -eq 1 -and "$($addresses[0].RemoteAddress)" -eq 'LocalSubnet') { $needsChange=$false } } } catch { $needsChange=$true }; if (-not $needsChange) { exit 0 }`, name, rule.Port, program)
	return elevatedPowerShellCommand(preflight, mutation), nil
}

func PowerShellDisableScript() string {
	return fmt.Sprintf(`$ErrorActionPreference='Stop'; Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule`, windowsRuleName)
}

func PowerShellDisableCommand() string {
	name := strings.ReplaceAll(windowsRuleName, "'", "''")
	preflight := fmt.Sprintf(`$rules=@(Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue); if ($rules.Count -eq 0) { exit 0 }`, name)
	return elevatedPowerShellCommand(preflight, PowerShellDisableScript())
}

func elevatedPowerShellCommand(preflight, mutation string) string {
	encoded := encodePowerShellCommand(mutation)
	return fmt.Sprintf(`$ErrorActionPreference='Stop'; %s; $encoded='%s'; $identity=[Security.Principal.WindowsIdentity]::GetCurrent(); $principal=[Security.Principal.WindowsPrincipal]::new($identity); if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { & ([ScriptBlock]::Create([Text.Encoding]::Unicode.GetString([Convert]::FromBase64String($encoded)))); exit 0 }; $powershell=Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'; $process=Start-Process -FilePath $powershell -Verb RunAs -WindowStyle Hidden -Wait -PassThru -ArgumentList @('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded); if ($process.ExitCode -ne 0) { throw "elevated firewall command exited with code $($process.ExitCode)" }`, preflight, encoded)
}

func encodePowerShellCommand(value string) string {
	units := utf16.Encode([]rune(value))
	encoded := make([]byte, len(units)*2)
	for index, unit := range units {
		binary.LittleEndian.PutUint16(encoded[index*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}
