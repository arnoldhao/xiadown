package update

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"xiadown/internal/application/softwareupdate"
	domainupdate "xiadown/internal/domain/update"
)

var (
	ErrPreparedUpdateNotFound = errors.New("prepared update not found")
	errPlistKeyNotFound       = errors.New("plist key not found")
)

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
	commandOutput       func(ctx context.Context, name string, args ...string) ([]byte, error)
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
	BundleID     string `json:"bundleId,omitempty"`
	TeamID       string `json:"teamId,omitempty"`
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
		commandOutput:       defaultCommandOutput,
	}, nil
}

func (installer *PlatformInstaller) SelectDownloadAsset(_ context.Context, asset softwareupdate.Asset) softwareupdate.Asset {
	if installer == nil || installer.goos != "windows" {
		return asset
	}
	currentExe, err := installer.currentExecutable()
	if err != nil {
		return softwareupdate.Asset{}
	}
	selected, ok := selectWindowsDownloadAsset(detectWindowsInstallKind(currentExe), asset)
	if !ok {
		return softwareupdate.Asset{}
	}
	return selected
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
		_ = installer.removeStageDir(previousPlan.StageDir)
	}
	return nil
}

func (installer *PlatformInstaller) RestartToApply(ctx context.Context) error {
	if installer == nil {
		return fmt.Errorf("installer not configured")
	}
	plan, err := installer.loadPlan()
	if err != nil {
		return err
	}

	switch plan.Platform {
	case "windows":
		return installer.restartWindows(ctx, plan)
	case "darwin":
		return installer.restartDarwin(ctx, plan)
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
	keepStageDir := false
	defer func() {
		if !keepStageDir {
			_ = installer.removeStageDir(stageDir)
		}
	}()

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

	if err := installer.savePlan(plan); err != nil {
		return err
	}
	keepStageDir = true
	return nil
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
	keepStageDir := false
	defer func() {
		if !keepStageDir {
			_ = installer.removeStageDir(stageDir)
		}
	}()
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
	validation, err := installer.validateMacUpdateBundle(ctx, currentBundle, stagedBundle)
	if err != nil {
		return err
	}
	targetBundle := resolveMacTargetBundle(currentBundle)

	if err := installer.savePlan(stagedPlan{
		Platform:     "darwin",
		Mode:         "bundle",
		StageDir:     stageDir,
		SourcePath:   stagedBundle,
		TargetPath:   targetBundle,
		RelaunchPath: targetBundle,
		FallbackPath: currentBundle,
		BundleID:     validation.BundleID,
		TeamID:       validation.TeamID,
		Version:      strings.TrimSpace(prepared.LatestVersion),
		Changelog:    prepared.Changelog,
	}); err != nil {
		return err
	}
	keepStageDir = true
	return nil
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

func (installer *PlatformInstaller) restartWindows(ctx context.Context, plan stagedPlan) error {
	validated, err := installer.validateRestartPlan(ctx, plan, "windows")
	if err != nil {
		return err
	}
	plan = validated

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

func (installer *PlatformInstaller) restartDarwin(ctx context.Context, plan stagedPlan) error {
	validated, err := installer.validateRestartPlan(ctx, plan, "darwin")
	if err != nil {
		return err
	}
	plan = validated

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
		plan.BundleID,
		plan.TeamID,
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
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	path = filepath.Clean(absolutePath)
	if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
		path = filepath.Clean(resolved)
	}
	return path, nil
}

func (installer *PlatformInstaller) output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if installer != nil && installer.commandOutput != nil {
		return installer.commandOutput(ctx, name, args...)
	}
	return defaultCommandOutput(ctx, name, args...)
}

func defaultCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (installer *PlatformInstaller) newStageDir() (string, error) {
	stageRoot, err := installer.stageRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return "", err
	}
	stageDir, err := os.MkdirTemp(stageRoot, "prepared-*")
	if err != nil {
		return "", err
	}
	validated, err := installer.validateStageDir(stageDir)
	if err != nil {
		return "", err
	}
	return validated, nil
}

func (installer *PlatformInstaller) stageRoot() (string, error) {
	if installer == nil {
		return "", fmt.Errorf("installer not configured")
	}
	stateDir := strings.TrimSpace(installer.stateDir)
	if stateDir == "" {
		return "", fmt.Errorf("installer state directory is empty")
	}
	root, err := filepath.Abs(filepath.Join(stateDir, "update-stage"))
	if err != nil {
		return "", fmt.Errorf("resolve update stage root: %w", err)
	}
	return filepath.Clean(root), nil
}

func (installer *PlatformInstaller) validateStageDir(rawStageDir string) (string, error) {
	stageDir, err := absoluteDeclaredPath(rawStageDir, "stage directory")
	if err != nil {
		return "", err
	}
	stageRoot, err := installer.stageRoot()
	if err != nil {
		return "", err
	}
	if !isWithinDir(stageDir, stageRoot) {
		return "", fmt.Errorf("stage directory %q is outside update stage root %q", stageDir, stageRoot)
	}
	if err := rejectSymlinkDescendant(stageRoot, stageDir); err != nil {
		return "", fmt.Errorf("validate stage directory path: %w", err)
	}

	rootInfo, err := os.Stat(stageRoot)
	if err != nil {
		return "", fmt.Errorf("inspect update stage root: %w", err)
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("update stage root %q is not a directory", stageRoot)
	}
	stageInfo, err := os.Stat(stageDir)
	if err != nil {
		return "", fmt.Errorf("inspect stage directory: %w", err)
	}
	if !stageInfo.IsDir() {
		return "", fmt.Errorf("stage path %q is not a directory", stageDir)
	}

	resolvedRoot, err := filepath.EvalSymlinks(stageRoot)
	if err != nil {
		return "", fmt.Errorf("resolve update stage root: %w", err)
	}
	resolvedStage, err := filepath.EvalSymlinks(stageDir)
	if err != nil {
		return "", fmt.Errorf("resolve stage directory: %w", err)
	}
	if !isWithinDir(resolvedStage, resolvedRoot) {
		return "", fmt.Errorf("resolved stage directory %q escapes update stage root %q", resolvedStage, resolvedRoot)
	}
	return stageDir, nil
}

func (installer *PlatformInstaller) validateStagedPlan(plan stagedPlan) (stagedPlan, error) {
	plan.Platform = strings.ToLower(strings.TrimSpace(plan.Platform))
	plan.Mode = strings.ToLower(strings.TrimSpace(plan.Mode))
	targetPath, err := absoluteDeclaredPath(plan.TargetPath, "target path")
	if err != nil {
		return stagedPlan{}, err
	}
	plan.TargetPath = targetPath

	stageDir, err := installer.validateStageDir(plan.StageDir)
	if err != nil {
		return stagedPlan{}, err
	}
	sourcePath, sourceInfo, err := validateExistingStageDescendant(plan.SourcePath, stageDir)
	if err != nil {
		return stagedPlan{}, fmt.Errorf("validate staged source: %w", err)
	}
	plan.StageDir = stageDir
	plan.SourcePath = sourcePath

	switch plan.Platform {
	case "windows":
		if plan.Mode != "installer" && plan.Mode != "portable" {
			return stagedPlan{}, fmt.Errorf("unsupported windows update mode %q", plan.Mode)
		}
		if !sourceInfo.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(sourcePath), ".exe") {
			return stagedPlan{}, fmt.Errorf("windows staged source must be a regular .exe file")
		}
		if plan.Mode == "installer" {
			if !samePath(filepath.Dir(sourcePath), stageDir, true) {
				return stagedPlan{}, fmt.Errorf("windows installer must be stored directly in the stage directory")
			}
		} else {
			portableRoot := filepath.Join(stageDir, "portable")
			if !isWithinDir(sourcePath, portableRoot) {
				return stagedPlan{}, fmt.Errorf("portable executable must be stored below %q", portableRoot)
			}
		}
	case "darwin":
		if plan.Mode != "bundle" {
			return stagedPlan{}, fmt.Errorf("unsupported macOS update mode %q", plan.Mode)
		}
		bundleRoot := filepath.Join(stageDir, "bundle")
		if !isWithinDir(sourcePath, bundleRoot) || !sourceInfo.IsDir() || !strings.EqualFold(filepath.Ext(sourcePath), ".app") {
			return stagedPlan{}, fmt.Errorf("macOS staged source must be an .app bundle below %q", bundleRoot)
		}
		info, err := os.Stat(filepath.Join(sourcePath, "Contents", "Info.plist"))
		if err != nil {
			return stagedPlan{}, fmt.Errorf("inspect staged app Info.plist: %w", err)
		}
		if !info.Mode().IsRegular() {
			return stagedPlan{}, fmt.Errorf("staged app Info.plist is not a regular file")
		}
	default:
		return stagedPlan{}, fmt.Errorf("unsupported update platform %q", plan.Platform)
	}
	return plan, nil
}

func (installer *PlatformInstaller) validateRestartPlan(ctx context.Context, plan stagedPlan, expectedPlatform string) (stagedPlan, error) {
	validated, err := installer.validateStagedPlan(plan)
	if err != nil {
		return stagedPlan{}, fmt.Errorf("validate restart plan: %w", err)
	}
	if validated.Platform != expectedPlatform {
		return stagedPlan{}, fmt.Errorf("prepared update platform %q does not match restart helper %q", validated.Platform, expectedPlatform)
	}
	if installer.goos != expectedPlatform {
		return stagedPlan{}, fmt.Errorf("prepared update platform %q does not match current platform %q", validated.Platform, installer.goos)
	}

	currentExe, err := installer.currentExecutable()
	if err != nil {
		return stagedPlan{}, fmt.Errorf("resolve current executable for prepared update: %w", err)
	}
	switch expectedPlatform {
	case "windows":
		if !strings.EqualFold(filepath.Ext(currentExe), ".exe") {
			return stagedPlan{}, fmt.Errorf("current Windows executable %q is not an .exe", currentExe)
		}
		if !sameDeclaredPath(validated.TargetPath, currentExe, true) {
			return stagedPlan{}, fmt.Errorf("prepared update target %q does not match current executable %q", validated.TargetPath, currentExe)
		}
		validated.TargetPath = currentExe
		validated.RelaunchPath = currentExe
		validated.FallbackPath = ""
		validated.InstallDir = filepath.Dir(currentExe)
	case "darwin":
		currentBundle, err := resolveAppBundle(currentExe)
		if err != nil {
			return stagedPlan{}, fmt.Errorf("resolve current app bundle for prepared update: %w", err)
		}
		targetBundle := resolveMacTargetBundle(currentBundle)
		if !sameDeclaredPath(validated.TargetPath, targetBundle, false) {
			return stagedPlan{}, fmt.Errorf("prepared update target %q does not match current app target %q", validated.TargetPath, targetBundle)
		}
		bundleValidation, err := installer.validateMacUpdateBundle(ctx, currentBundle, validated.SourcePath)
		if err != nil {
			return stagedPlan{}, fmt.Errorf("revalidate prepared macOS update: %w", err)
		}
		validated.TargetPath = targetBundle
		validated.RelaunchPath = targetBundle
		validated.FallbackPath = currentBundle
		validated.InstallDir = filepath.Dir(targetBundle)
		validated.BundleID = bundleValidation.BundleID
		validated.TeamID = bundleValidation.TeamID
	default:
		return stagedPlan{}, fmt.Errorf("unsupported update platform %q", expectedPlatform)
	}
	return validated, nil
}

func (installer *PlatformInstaller) removeStageDir(rawStageDir string) error {
	stageDir, err := installer.validateStageDir(rawStageDir)
	if err != nil {
		return fmt.Errorf("refuse unsafe staged update cleanup: %w", err)
	}
	if err := os.RemoveAll(stageDir); err != nil {
		return fmt.Errorf("remove staged update %q: %w", stageDir, err)
	}
	return nil
}

func validateExistingStageDescendant(rawPath string, stageDir string) (string, fs.FileInfo, error) {
	path, err := absoluteDeclaredPath(rawPath, "staged source path")
	if err != nil {
		return "", nil, err
	}
	if !isWithinDir(path, stageDir) {
		return "", nil, fmt.Errorf("path %q is outside stage directory %q", path, stageDir)
	}
	if err := rejectSymlinkDescendant(stageDir, path); err != nil {
		return "", nil, err
	}
	resolvedStage, err := filepath.EvalSymlinks(stageDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve stage directory: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve staged source: %w", err)
	}
	if !isWithinDir(resolvedPath, resolvedStage) {
		return "", nil, fmt.Errorf("resolved staged source %q escapes stage directory %q", resolvedPath, resolvedStage)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	return path, info, nil
}

func absoluteDeclaredPath(rawPath string, label string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", fmt.Errorf("%s is empty", label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%s %q is not absolute", label, rawPath)
	}
	return filepath.Clean(trimmed), nil
}

func sameDeclaredPath(declared string, expected string, caseInsensitive bool) bool {
	declaredPath, err := absoluteDeclaredPath(declared, "declared path")
	if err != nil {
		return false
	}
	expectedPath, err := absoluteDeclaredPath(expected, "expected path")
	if err != nil {
		return false
	}
	return samePath(declaredPath, expectedPath, caseInsensitive)
}

func samePath(left string, right string, caseInsensitive bool) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if caseInsensitive {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func rejectSymlinkDescendant(root string, descendant string) error {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic link is not allowed in staged path %q", root)
	}
	relative, err := filepath.Rel(root, descendant)
	if err != nil {
		return err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q is not a strict descendant of %q", descendant, root)
	}
	current := root
	for _, element := range strings.Split(relative, string(os.PathSeparator)) {
		current = filepath.Join(current, element)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is not allowed in staged path %q", current)
		}
	}
	return nil
}

func (installer *PlatformInstaller) cleanupStagedUpdate() error {
	plan, err := installer.loadPlan()
	if err != nil {
		if errors.Is(err, ErrPreparedUpdateNotFound) {
			return nil
		}
		// A corrupt plan must never turn into an arbitrary recursive delete. The
		// fixed-name plan itself is safe to discard so the user can recover.
		if removeErr := os.Remove(installer.planPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return errors.Join(err, fmt.Errorf("remove invalid prepared update plan: %w", removeErr))
		}
		return nil
	}
	if strings.TrimSpace(plan.StageDir) != "" {
		if err := installer.removeStageDir(plan.StageDir); err != nil {
			return err
		}
	}
	if err := os.Remove(installer.planPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (installer *PlatformInstaller) loadPlan() (stagedPlan, error) {
	file, err := os.Open(installer.planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return stagedPlan{}, ErrPreparedUpdateNotFound
		}
		return stagedPlan{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return stagedPlan{}, err
	}
	if !info.Mode().IsRegular() {
		return stagedPlan{}, fmt.Errorf("prepared update plan is not a regular file")
	}
	const maxPreparedPlanSize = 4 << 20
	if info.Size() > maxPreparedPlanSize {
		return stagedPlan{}, fmt.Errorf("prepared update plan exceeds %d bytes", maxPreparedPlanSize)
	}

	var plan stagedPlan
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return stagedPlan{}, fmt.Errorf("decode prepared update plan: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return stagedPlan{}, fmt.Errorf("prepared update plan contains trailing JSON data")
		}
		return stagedPlan{}, fmt.Errorf("decode prepared update plan trailing data: %w", err)
	}
	validated, err := installer.validateStagedPlan(plan)
	if err != nil {
		return stagedPlan{}, fmt.Errorf("validate prepared update plan: %w", err)
	}
	return validated, nil
}

func (installer *PlatformInstaller) savePlan(plan stagedPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(installer.planPath, data, 0o600)
}

type macUpdateBundleValidation struct {
	BundleID string
	TeamID   string
}

func (installer *PlatformInstaller) validateMacUpdateBundle(ctx context.Context, currentBundle string, stagedBundle string) (macUpdateBundleValidation, error) {
	currentBundleID, err := installer.macBundleIdentifier(ctx, currentBundle)
	if err != nil {
		return macUpdateBundleValidation{}, fmt.Errorf("read current mac app bundle identifier: %w", err)
	}
	stagedBundleID, err := installer.macBundleIdentifier(ctx, stagedBundle)
	if err != nil {
		return macUpdateBundleValidation{}, fmt.Errorf("read update mac app bundle identifier: %w", err)
	}
	if currentBundleID == "" || stagedBundleID == "" || currentBundleID != stagedBundleID {
		return macUpdateBundleValidation{}, fmt.Errorf("update bundle identifier mismatch: current=%q update=%q", currentBundleID, stagedBundleID)
	}

	if err := installer.verifyMacCodeSignature(ctx, stagedBundle); err != nil {
		return macUpdateBundleValidation{}, err
	}
	stagedTeamID, err := installer.macBundleTeamID(ctx, stagedBundle)
	if err != nil {
		return macUpdateBundleValidation{}, fmt.Errorf("read update mac code signing team: %w", err)
	}
	if isUnsetMacTeamID(stagedTeamID) {
		return macUpdateBundleValidation{}, fmt.Errorf("update mac app is not signed with a Developer ID team")
	}
	if currentTeamID, err := installer.macBundleTeamID(ctx, currentBundle); err == nil && !isUnsetMacTeamID(currentTeamID) && currentTeamID != stagedTeamID {
		return macUpdateBundleValidation{}, fmt.Errorf("update signing team mismatch: current=%q update=%q", currentTeamID, stagedTeamID)
	}
	if err := installer.assessMacGatekeeper(ctx, stagedBundle); err != nil {
		return macUpdateBundleValidation{}, err
	}

	return macUpdateBundleValidation{BundleID: stagedBundleID, TeamID: stagedTeamID}, nil
}

func isUnsetMacTeamID(teamID string) bool {
	trimmed := strings.TrimSpace(teamID)
	return trimmed == "" || strings.EqualFold(trimmed, "not set")
}

func (installer *PlatformInstaller) macBundleIdentifier(ctx context.Context, bundlePath string) (string, error) {
	infoPath := filepath.Join(bundlePath, "Contents", "Info.plist")
	if identifier, err := readXMLPlistString(infoPath, "CFBundleIdentifier"); err == nil && identifier != "" {
		return identifier, nil
	}

	output, err := installer.output(ctx, "/usr/libexec/PlistBuddy", "-c", "Print :CFBundleIdentifier", infoPath)
	if err != nil {
		return "", commandOutputError("read bundle identifier", output, err)
	}
	identifier := strings.TrimSpace(string(output))
	if identifier == "" {
		return "", fmt.Errorf("bundle identifier is empty")
	}
	return identifier, nil
}

func readXMLPlistString(path string, key string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	var lastKey string
	sawPlist := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "plist":
			sawPlist = true
		case "key":
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return "", err
			}
			lastKey = strings.TrimSpace(value)
		case "string":
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return "", err
			}
			if lastKey == key {
				return strings.TrimSpace(value), nil
			}
			lastKey = ""
		default:
			if lastKey != "" && start.Name.Local != "dict" && start.Name.Local != "plist" {
				lastKey = ""
			}
		}
	}
	if !sawPlist {
		return "", fmt.Errorf("invalid XML plist")
	}
	return "", fmt.Errorf("%w: %q", errPlistKeyNotFound, key)
}

func (installer *PlatformInstaller) verifyMacCodeSignature(ctx context.Context, bundlePath string) error {
	output, err := installer.output(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", "--verbose=2", bundlePath)
	if err != nil {
		return commandOutputError("verify update code signature", output, err)
	}
	return nil
}

func (installer *PlatformInstaller) assessMacGatekeeper(ctx context.Context, bundlePath string) error {
	output, err := installer.output(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=4", bundlePath)
	if err != nil {
		return commandOutputError("assess update with Gatekeeper", output, err)
	}
	return nil
}

func (installer *PlatformInstaller) macBundleTeamID(ctx context.Context, bundlePath string) (string, error) {
	output, err := installer.output(ctx, "/usr/bin/codesign", "-dv", "--verbose=4", bundlePath)
	if err != nil {
		return "", commandOutputError("read code signing metadata", output, err)
	}
	return parseCodesignTeamID(string(output)), nil
}

func parseCodesignTeamID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TeamIdentifier=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "TeamIdentifier="))
		}
	}
	return ""
}

func commandOutputError(action string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if message != "" {
		return fmt.Errorf("%s: %s", action, message)
	}
	return fmt.Errorf("%s: %w", action, err)
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

func safeArchivePath(destDir string, archiveName string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(destDir))
	name := strings.TrimSpace(archiveName)
	if root == "" || name == "" {
		return "", fmt.Errorf("archive path is empty")
	}
	name = filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(name) || name == "." || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive path %q", archiveName)
	}
	target := filepath.Join(root, name)
	if !isWithinDir(target, root) {
		return "", fmt.Errorf("unsafe archive path %q", archiveName)
	}
	return target, nil
}

func validateZipArchivePaths(archivePath string, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if _, err := safeArchivePath(destDir, file.Name); err != nil {
			return err
		}
	}
	return nil
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
	if err := validateZipArchivePaths(archivePath, destDir); err != nil {
		return err
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

func extractZipExecutable(archivePath, destDir, execName string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var candidate string
	for _, file := range reader.File {
		path, err := safeArchivePath(destDir, file.Name)
		if err != nil {
			return "", err
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("archive symlink is not allowed: %s", file.Name)
		}
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

func selectWindowsDownloadAsset(kind installKind, asset softwareupdate.Asset) (softwareupdate.Asset, bool) {
	variantName := ""
	switch kind {
	case installKindInstalled:
		variantName = "installer"
	case installKindPortable:
		variantName = "portable"
	default:
		return softwareupdate.Asset{}, false
	}

	if windowsDownloadAssetKind(asset) == kind && len(asset.DownloadURLs()) > 0 {
		return asset, true
	}

	if variant, ok := asset.Variants[variantName]; ok &&
		windowsDownloadAssetKind(variant) == kind && len(variant.DownloadURLs()) > 0 {
		return variant, true
	}
	return softwareupdate.Asset{}, false
}

func windowsDownloadAssetKind(asset softwareupdate.Asset) installKind {
	switch strings.ToLower(strings.TrimSpace(asset.ArtifactType)) {
	case "exe":
		return installKindInstalled
	case "zip":
		return installKindPortable
	}
	switch strings.ToLower(strings.TrimSpace(asset.InstallStrategy)) {
	case "app-installer", "installer":
		return installKindInstalled
	case "archive", "portable":
		return installKindPortable
	}
	if kind := windowsDownloadNameKind(asset.ArtifactName); kind != installKindUnknown {
		return kind
	}
	for _, source := range asset.SortedSources() {
		if kind := windowsDownloadNameKind(source.URL); kind != installKindUnknown {
			return kind
		}
	}
	return installKindUnknown
}

func windowsDownloadNameKind(raw string) installKind {
	value := strings.ToLower(strings.TrimSpace(raw))
	if index := strings.IndexAny(value, "?#"); index >= 0 {
		value = value[:index]
	}
	switch {
	case strings.HasSuffix(value, "-installer.exe"):
		return installKindInstalled
	case strings.HasSuffix(value, ".zip"):
		return installKindPortable
	default:
		return installKindUnknown
	}
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
EXPECTED_BUNDLE_ID="${9:-}"
EXPECTED_TEAM_ID="${10:-}"
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
    return $?
  fi
  return 1
}

restore_backup_privileged() {
  /usr/bin/osascript - "$TARGET_APP" "$BACKUP_APP" <<'APPLESCRIPT'
on run argv
  set targetApp to item 1 of argv
  set backupApp to item 2 of argv
  set commandText to "set -e; rm -rf " & quoted form of targetApp & "; if [ -d " & quoted form of backupApp & " ]; then mv " & quoted form of backupApp & " " & quoted form of targetApp & "; fi"
  do shell script commandText with administrator privileges
end run
APPLESCRIPT
}

relaunch_fallback() {
  relaunch_app "$FALLBACK_APP" "--skip-prepared-update-once"
  if [ "$FALLBACK_APP" != "$TARGET_APP" ]; then
    relaunch_app "$TARGET_APP" "--skip-prepared-update-once"
  fi
}

bundle_identifier() {
  APP_PATH="$1"
  INFO_PLIST="$APP_PATH/Contents/Info.plist"
  if [ ! -f "$INFO_PLIST" ]; then
    return 1
  fi
  /usr/libexec/PlistBuddy -c "Print :CFBundleIdentifier" "$INFO_PLIST" 2>/dev/null || \
    /usr/bin/plutil -extract CFBundleIdentifier raw -o - "$INFO_PLIST" 2>/dev/null
}

team_identifier() {
  /usr/bin/codesign -dv --verbose=4 "$1" 2>&1 | /usr/bin/sed -n 's/^TeamIdentifier=//p' | /usr/bin/head -n 1
}

validate_app_bundle() {
  APP_PATH="$1"
  if [ ! -d "$APP_PATH" ]; then
    return 1
  fi

  BUNDLE_ID="$(bundle_identifier "$APP_PATH" | /usr/bin/tr -d '\r\n')" || return 1
  if [ -n "$EXPECTED_BUNDLE_ID" ] && [ "$BUNDLE_ID" != "$EXPECTED_BUNDLE_ID" ]; then
    return 1
  fi

  /usr/bin/codesign --verify --deep --strict --verbose=2 "$APP_PATH" >/dev/null 2>&1 || return 1
  TEAM_ID="$(team_identifier "$APP_PATH" | /usr/bin/tr -d '\r\n')" || return 1
  if [ -z "$TEAM_ID" ]; then
    return 1
  fi
  if [ "$TEAM_ID" = "not set" ]; then
    return 1
  fi
  if [ -n "$EXPECTED_TEAM_ID" ] && [ "$TEAM_ID" != "$EXPECTED_TEAM_ID" ]; then
    return 1
  fi

  /usr/sbin/spctl --assess --type execute --verbose=4 "$APP_PATH" >/dev/null 2>&1 || return 1
  return 0
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
    "exit 0; " & ¬
    "else rm -rf " & quoted form of targetApp & "; if [ -d " & quoted form of backupApp & " ]; then mv " & quoted form of backupApp & " " & quoted form of targetApp & "; fi; exit 1; fi"
  do shell script commandText with administrator privileges
end run
APPLESCRIPT
}

if ! validate_app_bundle "$SOURCE_APP"; then
  relaunch_fallback
  exit 1
fi

if ! install_direct; then
  if ! install_privileged; then
    restore_backup || restore_backup_privileged || true
    relaunch_fallback
    exit 1
  fi
fi

if ! validate_app_bundle "$TARGET_APP"; then
  restore_backup || restore_backup_privileged || true
  relaunch_fallback
  exit 1
fi

cp "$PLAN_PATH" "$PENDING_WHATS_NEW_PATH" >/dev/null 2>&1 || true
if ! open "$RELAUNCH_APP"; then
  relaunch_fallback
  exit 1
fi
rm -rf "$BACKUP_APP" >/dev/null 2>&1 || true
rm -f "$PLAN_PATH"
rm -rf "$STAGE_DIR"
`

var _ interface {
	Install(context.Context, string, domainupdate.Info) error
	RestartToApply(context.Context) error
	SelectDownloadAsset(context.Context, softwareupdate.Asset) softwareupdate.Asset
	PreparedUpdate(context.Context) (domainupdate.Info, bool, error)
	ClearPreparedUpdate(context.Context) error
	PendingWhatsNew(context.Context) (domainupdate.WhatsNew, bool, error)
	SeenWhatsNewVersion(context.Context) (string, error)
	MarkWhatsNewSeen(context.Context, string) error
} = (*PlatformInstaller)(nil)
