package update

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	domainupdate "xiadown/internal/domain/update"
)

var ErrPreparedUpdateNotFound = fmt.Errorf("prepared update not found")

type installKind string

const (
	installKindInstalled installKind = "installed"
	installKindPortable  installKind = "portable"
	installKindUnknown   installKind = "unknown"
)

type PlatformInstaller struct {
	stateDir            string
	planPath            string
	whatsNewPendingPath string
	whatsNewSeenPath    string
	goos                string
	goarch              string
	executablePath      func() (string, error)
	startDetached       func(name string, args []string) error
}

type stagedPlan struct {
	Platform     string `json:"platform"`
	Mode         string `json:"mode"`
	StageDir     string `json:"stageDir"`
	SourcePath   string `json:"sourcePath"`
	TargetPath   string `json:"targetPath"`
	RelaunchPath string `json:"relaunchPath"`
	FallbackPath string `json:"fallbackPath,omitempty"`
	InstallDir   string `json:"installDir,omitempty"`
	Version      string `json:"version,omitempty"`
	Changelog    string `json:"changelog,omitempty"`
}

type whatsNewSeenState struct {
	Version string `json:"version"`
	SeenAt  string `json:"seenAt,omitempty"`
}

func NewInstaller(statePath string) (*PlatformInstaller, error) {
	stateDir := strings.TrimSpace(filepath.Dir(statePath))
	if stateDir == "" || stateDir == "." {
		configDir, err := os.UserConfigDir()
		if err != nil {
			stateDir = filepath.Join(os.TempDir(), "xiadown")
		} else {
			stateDir = filepath.Join(configDir, "xiadown")
		}
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	return &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
		goos:                runtime.GOOS,
		goarch:              runtime.GOARCH,
		executablePath:      os.Executable,
		startDetached:       startDetachedCommand,
	}, nil
}

func (installer *PlatformInstaller) SelectDownloadURLs(_ context.Context, urls []string) []string {
	if installer == nil || installer.goos != "windows" || len(urls) == 0 {
		return slices.Clone(urls)
	}
	currentExe, err := installer.currentExecutable()
	if err != nil {
		return preferWindowsPortableDownloadURLs(urls)
	}
	if detectWindowsInstallKind(currentExe) == installKindInstalled {
		return preferWindowsInstallerDownloadURLs(urls)
	}
	return preferWindowsPortableDownloadURLs(urls)
}

func (installer *PlatformInstaller) Install(ctx context.Context, artifactPath string, prepared domainupdate.Info) error {
	if installer == nil {
		return fmt.Errorf("installer not configured")
	}
	normalizedArtifact := strings.TrimSpace(artifactPath)
	if normalizedArtifact == "" {
		return fmt.Errorf("artifact path is empty")
	}

	previousPlan, err := installer.loadPlan()
	hasPreviousPlan := err == nil
	if err != nil && !errors.Is(err, ErrPreparedUpdateNotFound) {
		return err
	}

	var installErr error
	switch installer.goos {
	case "windows":
		installErr = installer.prepareWindowsUpdate(normalizedArtifact, prepared)
	case "darwin":
		installErr = installer.prepareMacUpdate(ctx, normalizedArtifact, prepared)
	default:
		installErr = fmt.Errorf("update install is not supported on %s", installer.goos)
	}
	if installErr != nil {
		return installErr
	}
	if hasPreviousPlan && strings.TrimSpace(previousPlan.StageDir) != "" {
		_ = os.RemoveAll(previousPlan.StageDir)
	}
	return nil
}

func (installer *PlatformInstaller) RestartToApply(_ context.Context) error {
	if installer == nil {
		return fmt.Errorf("installer not configured")
	}
	plan, err := installer.loadPlan()
	if err != nil {
		return err
	}

	switch plan.Platform {
	case "windows":
		return installer.restartWindows(plan)
	case "darwin":
		return installer.restartDarwin(plan)
	default:
		return fmt.Errorf("unsupported update platform %q", plan.Platform)
	}
}

func (installer *PlatformInstaller) prepareWindowsUpdate(artifactPath string, prepared domainupdate.Info) error {
	currentExe, err := installer.currentExecutable()
	if err != nil {
		return err
	}

	stageDir, err := installer.newStageDir()
	if err != nil {
		return err
	}

	artifactName := filepath.Base(artifactPath)
	plan := stagedPlan{
		Platform:     "windows",
		StageDir:     stageDir,
		TargetPath:   currentExe,
		RelaunchPath: currentExe,
		InstallDir:   filepath.Dir(currentExe),
		Version:      strings.TrimSpace(prepared.LatestVersion),
		Changelog:    prepared.Changelog,
	}

	switch strings.ToLower(filepath.Ext(artifactName)) {
	case ".exe":
		stagedInstaller := filepath.Join(stageDir, artifactName)
		if err := copyFile(artifactPath, stagedInstaller); err != nil {
			return err
		}
		plan.Mode = "installer"
		plan.SourcePath = stagedInstaller
	case ".zip":
		execName := filepath.Base(currentExe)
		stagedExe, err := extractZipExecutable(artifactPath, filepath.Join(stageDir, "portable"), execName)
		if err != nil {
			return err
		}
		plan.Mode = "portable"
		plan.SourcePath = stagedExe
	default:
		return fmt.Errorf("unsupported windows update artifact %q", artifactName)
	}

	return installer.savePlan(plan)
}

func (installer *PlatformInstaller) prepareMacUpdate(ctx context.Context, artifactPath string, prepared domainupdate.Info) error {
	currentExe, err := installer.currentExecutable()
	if err != nil {
		return err
	}
	currentBundle, err := resolveAppBundle(currentExe)
	if err != nil {
		return fmt.Errorf("automatic update requires a macOS app bundle: %w", err)
	}

	stageDir, err := installer.newStageDir()
	if err != nil {
		return err
	}
	extractDir := filepath.Join(stageDir, "bundle")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := extractMacArchive(ctx, artifactPath, extractDir); err != nil {
		return err
	}
	stagedBundle, err := findFirstAppBundle(extractDir)
	if err != nil {
		return err
	}
	_ = removeMacQuarantine(stagedBundle)
	targetBundle := resolveMacTargetBundle(currentBundle)

	return installer.savePlan(stagedPlan{
		Platform:     "darwin",
		Mode:         "bundle",
		StageDir:     stageDir,
		SourcePath:   stagedBundle,
		TargetPath:   targetBundle,
		RelaunchPath: targetBundle,
		FallbackPath: currentBundle,
		Version:      strings.TrimSpace(prepared.LatestVersion),
		Changelog:    prepared.Changelog,
	})
}

func (installer *PlatformInstaller) PreparedUpdate(_ context.Context) (domainupdate.Info, bool, error) {
	if installer == nil {
		return domainupdate.Info{}, false, fmt.Errorf("installer not configured")
	}
	plan, err := installer.loadPlan()
	if err != nil {
		if errors.Is(err, ErrPreparedUpdateNotFound) {
			return domainupdate.Info{}, false, nil
		}
		return domainupdate.Info{}, false, err
	}
	return domainupdate.Info{
		Kind:              domainupdate.KindApp,
		Status:            domainupdate.StatusReadyToRestart,
		LatestVersion:     strings.TrimSpace(plan.Version),
		PreparedVersion:   strings.TrimSpace(plan.Version),
		Changelog:         plan.Changelog,
		PreparedChangelog: plan.Changelog,
		Progress:          100,
	}, true, nil
}

func (installer *PlatformInstaller) ClearPreparedUpdate(_ context.Context) error {
	if installer == nil {
		return fmt.Errorf("installer not configured")
	}
	return installer.cleanupStagedUpdate()
}

func (installer *PlatformInstaller) PendingWhatsNew(_ context.Context) (domainupdate.WhatsNew, bool, error) {
	if installer == nil {
		return domainupdate.WhatsNew{}, false, fmt.Errorf("installer not configured")
	}
	data, err := os.ReadFile(installer.whatsNewPendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domainupdate.WhatsNew{}, false, nil
		}
		return domainupdate.WhatsNew{}, false, err
	}

	var plan stagedPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return domainupdate.WhatsNew{}, false, err
	}
	version := strings.TrimSpace(plan.Version)
	if version == "" {
		return domainupdate.WhatsNew{}, false, nil
	}
	return domainupdate.WhatsNew{
		Version:   version,
		Changelog: plan.Changelog,
	}, true, nil
}

func (installer *PlatformInstaller) SeenWhatsNewVersion(_ context.Context) (string, error) {
	if installer == nil {
		return "", fmt.Errorf("installer not configured")
	}
	data, err := os.ReadFile(installer.whatsNewSeenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var seen whatsNewSeenState
	if err := json.Unmarshal(data, &seen); err != nil {
		return "", err
	}
	return strings.TrimSpace(seen.Version), nil
}

func (installer *PlatformInstaller) MarkWhatsNewSeen(_ context.Context, version string) error {
	if installer == nil {
		return fmt.Errorf("installer not configured")
	}
	normalized := strings.TrimSpace(version)
	if normalized == "" {
		return nil
	}
	data, err := json.MarshalIndent(whatsNewSeenState{
		Version: normalized,
		SeenAt:  time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(installer.whatsNewSeenPath, data, 0o600); err != nil {
		return err
	}
	pending, found, err := installer.PendingWhatsNew(context.Background())
	if err != nil {
		return err
	}
	if found && domainupdate.CompareVersion(pending.Version, normalized) <= 0 {
		_ = os.Remove(installer.whatsNewPendingPath)
	}
	return nil
}

func (installer *PlatformInstaller) restartWindows(plan stagedPlan) error {
	scriptPath := filepath.Join(installer.stateDir, "apply_update.ps1")
	if err := os.WriteFile(scriptPath, []byte(windowsApplyScript), 0o600); err != nil {
		return err
	}

	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		strconv.Itoa(os.Getpid()),
		plan.Mode,
		plan.SourcePath,
		plan.TargetPath,
		plan.InstallDir,
		plan.StageDir,
		installer.planPath,
		installer.whatsNewPendingPath,
	}
	return installer.startDetached("powershell.exe", args)
}

func (installer *PlatformInstaller) restartDarwin(plan stagedPlan) error {
	scriptPath := filepath.Join(installer.stateDir, "apply_update.sh")
	if err := os.WriteFile(scriptPath, []byte(darwinApplyScript), 0o700); err != nil {
		return err
	}

	relaunchPath := strings.TrimSpace(plan.RelaunchPath)
	if relaunchPath == "" {
		relaunchPath = plan.TargetPath
	}
	fallbackPath := strings.TrimSpace(plan.FallbackPath)
	if fallbackPath == "" {
		fallbackPath = plan.TargetPath
	}

	args := []string{
		scriptPath,
		strconv.Itoa(os.Getpid()),
		plan.SourcePath,
		plan.TargetPath,
		relaunchPath,
		fallbackPath,
		plan.StageDir,
		installer.planPath,
		installer.whatsNewPendingPath,
	}
	return installer.startDetached("/bin/sh", args)
}

func (installer *PlatformInstaller) currentExecutable() (string, error) {
	if installer.executablePath == nil {
		return "", fmt.Errorf("executable path resolver not configured")
	}
	path, err := installer.executablePath()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("executable path is empty")
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
		path = resolved
	}
	return path, nil
}

func (installer *PlatformInstaller) newStageDir() (string, error) {
	stageRoot := filepath.Join(installer.stateDir, "update-stage")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(stageRoot, "prepared-*")
}

func (installer *PlatformInstaller) cleanupStagedUpdate() error {
	plan, err := installer.loadPlan()
	if err != nil {
		if errors.Is(err, ErrPreparedUpdateNotFound) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(plan.StageDir) != "" {
		_ = os.RemoveAll(plan.StageDir)
	}
	_ = os.Remove(installer.planPath)
	return nil
}

func (installer *PlatformInstaller) loadPlan() (stagedPlan, error) {
	data, err := os.ReadFile(installer.planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return stagedPlan{}, ErrPreparedUpdateNotFound
		}
		return stagedPlan{}, err
	}
	var plan stagedPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return stagedPlan{}, err
	}
	if strings.TrimSpace(plan.SourcePath) == "" || strings.TrimSpace(plan.TargetPath) == "" {
		return stagedPlan{}, fmt.Errorf("prepared update plan is incomplete")
	}
	return plan, nil
}

func (installer *PlatformInstaller) savePlan(plan stagedPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(installer.planPath, data, 0o600)
}

func resolveAppBundle(executablePath string) (string, error) {
	current := filepath.Dir(executablePath)
	for {
		if strings.HasSuffix(strings.ToLower(current), ".app") {
			return current, nil
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return "", fmt.Errorf("mac app bundle not found for executable %q", executablePath)
}

func resolveMacTargetBundle(currentBundle string) string {
	normalized := filepath.Clean(strings.TrimSpace(currentBundle))
	if isWithinDir(normalized, "/Applications") {
		return normalized
	}
	return filepath.Join("/Applications", filepath.Base(normalized))
}

func isWithinDir(path string, root string) bool {
	cleanedPath := filepath.Clean(strings.TrimSpace(path))
	cleanedRoot := filepath.Clean(strings.TrimSpace(root))
	if cleanedPath == "" || cleanedRoot == "" {
		return false
	}
	rel, err := filepath.Rel(cleanedRoot, cleanedPath)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func findFirstAppBundle(root string) (string, error) {
	var match string
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
			match = path
			return fs.SkipDir
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if match == "" {
		return "", fmt.Errorf("no .app bundle found in %q", root)
	}
	return match, nil
}

func extractMacArchive(ctx context.Context, archivePath string, destDir string) error {
	if !strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return fmt.Errorf("unsupported mac update artifact %q", archivePath)
	}
	cmd := exec.CommandContext(ctx, "ditto", "-x", "-k", archivePath, destDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("extract mac archive: %s", message)
	}
	return nil
}

func removeMacQuarantine(path string) error {
	cmd := exec.Command("xattr", "-dr", "com.apple.quarantine", path)
	return cmd.Run()
}

func extractZipExecutable(archivePath, destDir, execName string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var candidate string
	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		src, err := file.Open()
		if err != nil {
			return "", err
		}
		dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = src.Close()
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			_ = src.Close()
			return "", err
		}
		_ = dst.Close()
		_ = src.Close()
		if strings.EqualFold(filepath.Base(path), execName) {
			candidate = path
		}
	}

	if candidate == "" {
		return "", fmt.Errorf("executable %s not found in archive", execName)
	}
	return candidate, nil
}

func copyFile(src string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func preferWindowsInstallerDownloadURLs(urls []string) []string {
	result := make([]string, 0, len(urls)*2)
	seen := make(map[string]struct{}, len(urls)*2)
	add := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	for _, raw := range urls {
		if strings.Contains(strings.ToLower(raw), "-installer.exe") {
			add(raw)
			continue
		}
		if installerURL := installerWindowsDownloadURL(raw); installerURL != "" {
			add(installerURL)
		}
	}
	if len(result) == 0 {
		return slices.Clone(urls)
	}
	return result
}

func preferWindowsPortableDownloadURLs(urls []string) []string {
	result := make([]string, 0, len(urls)*2)
	seen := make(map[string]struct{}, len(urls)*2)
	add := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	for _, raw := range urls {
		if portable := portableWindowsDownloadURL(raw); portable != "" {
			add(portable)
			continue
		}
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(raw)), ".zip") {
			add(raw)
		}
	}
	if len(result) == 0 {
		return slices.Clone(urls)
	}
	return result
}

func installerWindowsDownloadURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	const portableSuffix = ".zip"
	if !strings.HasSuffix(strings.ToLower(trimmed), portableSuffix) {
		return ""
	}
	return trimmed[:len(trimmed)-len(portableSuffix)] + "-installer.exe"
}

func portableWindowsDownloadURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	const installerSuffix = "-installer.exe"
	index := strings.LastIndex(strings.ToLower(trimmed), installerSuffix)
	if index < 0 {
		return ""
	}
	return trimmed[:index] + ".zip" + trimmed[index+len(installerSuffix):]
}

const windowsApplyScript = `param(
  [Parameter(Mandatory = $true)][int]$ParentPid,
  [Parameter(Mandatory = $true)][string]$Mode,
  [Parameter(Mandatory = $true)][string]$SourcePath,
  [Parameter(Mandatory = $true)][string]$TargetPath,
  [Parameter(Mandatory = $true)][string]$InstallDir,
  [Parameter(Mandatory = $true)][string]$StageDir,
  [Parameter(Mandatory = $true)][string]$PlanPath,
  [Parameter(Mandatory = $true)][string]$PendingWhatsNewPath
)

$ErrorActionPreference = "Stop"

function ConvertTo-PSLiteral {
  param([string]$Value)
  return "'" + ($Value -replace "'", "''") + "'"
}

function Copy-PortableUpdate {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Target
  )

  $backupPath = $Target + ".old"
  $targetDir = Split-Path -Parent $Target
  if (-not [string]::IsNullOrWhiteSpace($targetDir)) {
    New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
  }
  Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $Target) {
    Move-Item -LiteralPath $Target -Destination $backupPath -Force
  }

  try {
    Copy-Item -LiteralPath $Source -Destination $Target -Force -ErrorAction Stop
    Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue
  } catch {
    Remove-Item -LiteralPath $Target -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $backupPath) {
      Move-Item -LiteralPath $backupPath -Destination $Target -Force -ErrorAction SilentlyContinue
    }
    throw
  }
}

function Copy-PortableUpdateElevated {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Target
  )

  $command = @'
$ErrorActionPreference = "Stop"
$source = __SOURCE__
$target = __TARGET__
$backupPath = $target + ".old"
$targetDir = Split-Path -Parent $target
if (-not [string]::IsNullOrWhiteSpace($targetDir)) {
  New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
}
Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue
if (Test-Path -LiteralPath $target) {
  Move-Item -LiteralPath $target -Destination $backupPath -Force
}
try {
  Copy-Item -LiteralPath $source -Destination $target -Force -ErrorAction Stop
  Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue
} catch {
  Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $backupPath) {
    Move-Item -LiteralPath $backupPath -Destination $target -Force -ErrorAction SilentlyContinue
  }
  throw
}
'@
  $command = $command.Replace("__SOURCE__", (ConvertTo-PSLiteral $Source)).Replace("__TARGET__", (ConvertTo-PSLiteral $Target))
  $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
  $result = Start-Process -FilePath "powershell.exe" -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", $encoded) -Verb RunAs -Wait -PassThru
  if ($null -ne $result.ExitCode -and $result.ExitCode -ne 0) {
    throw ("elevated portable copy exited with code " + $result.ExitCode)
  }
}

for ($i = 0; $i -lt 480; $i++) {
  $proc = Get-Process -Id $ParentPid -ErrorAction SilentlyContinue
  if (-not $proc) {
    break
  }
  Start-Sleep -Milliseconds 250
}

try {
  switch ($Mode) {
    "installer" {
      $result = Start-Process -FilePath $SourcePath -ArgumentList @("/S", "/D=" + $InstallDir) -Verb RunAs -Wait -PassThru
      if ($null -ne $result.ExitCode -and $result.ExitCode -ne 0) {
        throw ("installer exited with code " + $result.ExitCode)
      }
    }
    "portable" {
      try {
        Copy-PortableUpdate -Source $SourcePath -Target $TargetPath
      } catch {
        Copy-PortableUpdateElevated -Source $SourcePath -Target $TargetPath
      }
    }
    default {
      throw ("unsupported update mode: " + $Mode)
    }
  }

  try {
    Copy-Item -LiteralPath $PlanPath -Destination $PendingWhatsNewPath -Force -ErrorAction Stop
  } catch {
  }

  Start-Process -FilePath $TargetPath -WorkingDirectory $InstallDir | Out-Null
  Remove-Item -LiteralPath $PlanPath -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $StageDir -Recurse -Force -ErrorAction SilentlyContinue
} catch {
  try {
    if (Test-Path -LiteralPath $TargetPath) {
      Start-Process -FilePath $TargetPath -ArgumentList @("--skip-prepared-update-once") -WorkingDirectory $InstallDir | Out-Null
    }
  } catch {
  }
  exit 1
}
`

const darwinApplyScript = `#!/bin/sh
set -eu

PARENT_PID="$1"
SOURCE_APP="$2"
TARGET_APP="$3"
RELAUNCH_APP="$4"
FALLBACK_APP="$5"
STAGE_DIR="$6"
PLAN_PATH="$7"
PENDING_WHATS_NEW_PATH="$8"
BACKUP_APP="${TARGET_APP}.old"

while kill -0 "$PARENT_PID" 2>/dev/null; do
  sleep 0.25
done

relaunch_app() {
  APP_PATH="$1"
  shift || true
  if [ -n "$APP_PATH" ] && [ -d "$APP_PATH" ]; then
    if [ "$#" -gt 0 ]; then
      open -a "$APP_PATH" --args "$@" >/dev/null 2>&1 || true
    else
      open "$APP_PATH" >/dev/null 2>&1 || true
    fi
  fi
}

restore_backup() {
  if [ -d "$BACKUP_APP" ]; then
    rm -rf "$TARGET_APP"
    mv "$BACKUP_APP" "$TARGET_APP"
  fi
}

relaunch_fallback() {
  relaunch_app "$FALLBACK_APP" "--skip-prepared-update-once"
  if [ "$FALLBACK_APP" != "$TARGET_APP" ]; then
    relaunch_app "$TARGET_APP" "--skip-prepared-update-once"
  fi
}

install_direct() {
  mkdir -p "$(dirname "$TARGET_APP")"
  rm -rf "$BACKUP_APP"
  if [ -d "$TARGET_APP" ]; then
    mv "$TARGET_APP" "$BACKUP_APP"
  fi
  if /usr/bin/ditto "$SOURCE_APP" "$TARGET_APP"; then
    return 0
  fi
  rm -rf "$TARGET_APP"
  if [ -d "$BACKUP_APP" ]; then
    mv "$BACKUP_APP" "$TARGET_APP"
  fi
  return 1
}

install_privileged() {
  /usr/bin/osascript - "$SOURCE_APP" "$TARGET_APP" "$BACKUP_APP" <<'APPLESCRIPT'
on run argv
  set sourceApp to item 1 of argv
  set targetApp to item 2 of argv
  set backupApp to item 3 of argv
  set commandText to "set -e; rm -rf " & quoted form of backupApp & "; " & ¬
    "if [ -d " & quoted form of targetApp & " ]; then mv " & quoted form of targetApp & " " & quoted form of backupApp & "; fi; " & ¬
    "if /usr/bin/ditto " & quoted form of sourceApp & " " & quoted form of targetApp & "; then " & ¬
    "/usr/bin/xattr -dr com.apple.quarantine " & quoted form of targetApp & " >/dev/null 2>&1 || true; " & ¬
    "rm -rf " & quoted form of backupApp & "; " & ¬
    "else rm -rf " & quoted form of targetApp & "; if [ -d " & quoted form of backupApp & " ]; then mv " & quoted form of backupApp & " " & quoted form of targetApp & "; fi; exit 1; fi"
  do shell script commandText with administrator privileges
end run
APPLESCRIPT
}

if ! install_direct; then
  if ! install_privileged; then
    restore_backup
    relaunch_fallback
    exit 1
  fi
fi

/usr/bin/xattr -dr com.apple.quarantine "$TARGET_APP" >/dev/null 2>&1 || true
cp "$PLAN_PATH" "$PENDING_WHATS_NEW_PATH" >/dev/null 2>&1 || true
if ! open "$RELAUNCH_APP"; then
  relaunch_fallback
  exit 1
fi
rm -rf "$BACKUP_APP"
rm -f "$PLAN_PATH"
rm -rf "$STAGE_DIR"
`

var _ interface {
	Install(context.Context, string, domainupdate.Info) error
	RestartToApply(context.Context) error
	SelectDownloadURLs(context.Context, []string) []string
	PreparedUpdate(context.Context) (domainupdate.Info, bool, error)
	ClearPreparedUpdate(context.Context) error
	PendingWhatsNew(context.Context) (domainupdate.WhatsNew, bool, error)
	SeenWhatsNewVersion(context.Context) (string, error)
	MarkWhatsNewSeen(context.Context, string) error
} = (*PlatformInstaller)(nil)
