package update

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiadown/internal/application/softwareupdate"
)

func TestSelectWindowsDownloadAssetKeepsURLAndChecksumTogether(t *testing.T) {
	t.Parallel()

	installerAsset := windowsTestDownloadAsset(
		"xiadown-windows-x64-1.0.0-installer.exe",
		"exe",
		"installer-sha256",
	)
	portableAsset := windowsTestDownloadAsset(
		"xiadown-windows-x64-1.0.0.zip",
		"zip",
		"portable-sha256",
	)

	tests := []struct {
		name    string
		kind    installKind
		primary softwareupdate.Asset
		variant softwareupdate.Asset
		want    softwareupdate.Asset
	}{
		{
			name:    "installed selects installer variant",
			kind:    installKindInstalled,
			primary: portableAsset,
			variant: installerAsset,
			want:    installerAsset,
		},
		{
			name:    "portable selects portable variant",
			kind:    installKindPortable,
			primary: installerAsset,
			variant: portableAsset,
			want:    portableAsset,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			variantName := "portable"
			if test.kind == installKindInstalled {
				variantName = "installer"
			}
			primary := test.primary
			primary.Variants = map[string]softwareupdate.Asset{variantName: test.variant}

			selected, ok := selectWindowsDownloadAsset(test.kind, primary)
			if !ok {
				t.Fatal("expected a compatible Windows update asset")
			}
			if selected.ArtifactName != test.want.ArtifactName || selected.SHA256 != test.want.SHA256 {
				t.Fatalf("selected asset = %#v, want URL and checksum from %#v", selected, test.want)
			}
			if selected.PrimaryDownloadURL() != test.want.PrimaryDownloadURL() {
				t.Fatalf("selected URL = %q, want %q", selected.PrimaryDownloadURL(), test.want.PrimaryDownloadURL())
			}
		})
	}
}

func TestSelectWindowsDownloadAssetRejectsMissingVariant(t *testing.T) {
	t.Parallel()

	primary := windowsTestDownloadAsset(
		"xiadown-windows-x64-1.0.0-installer.exe",
		"exe",
		"installer-sha256",
	)
	selected, ok := selectWindowsDownloadAsset(installKindPortable, primary)

	if ok || len(selected.DownloadURLs()) > 0 {
		t.Fatalf("missing portable variant selected an incompatible asset: %#v", selected)
	}
}

func TestSelectWindowsDownloadAssetKeepsMatchingPrimary(t *testing.T) {
	t.Parallel()

	primary := windowsTestDownloadAsset(
		"xiadown-windows-x64-1.0.0-installer.exe",
		"exe",
		"installer-sha256",
	)
	selected, ok := selectWindowsDownloadAsset(installKindInstalled, primary)
	if !ok || selected.PrimaryDownloadURL() != primary.PrimaryDownloadURL() || selected.SHA256 != primary.SHA256 {
		t.Fatalf("matching installer primary was not preserved: %#v", selected)
	}
}

func TestSelectWindowsDownloadAssetRejectsUnknownInstallKind(t *testing.T) {
	t.Parallel()

	primary := windowsTestDownloadAsset(
		"xiadown-windows-x64-1.0.0-installer.exe",
		"exe",
		"installer-sha256",
	)
	if selected, ok := selectWindowsDownloadAsset(installKindUnknown, primary); ok || len(selected.DownloadURLs()) > 0 {
		t.Fatalf("unknown Windows install kind selected an asset: %#v", selected)
	}
}

func windowsTestDownloadAsset(name string, artifactType string, checksum string) softwareupdate.Asset {
	return softwareupdate.Asset{
		ArtifactName: name,
		ArtifactType: artifactType,
		SHA256:       checksum,
		Sources: []softwareupdate.DownloadSource{{
			Name:     "github",
			URL:      "https://example.com/" + name,
			Priority: 10,
			Enabled:  true,
		}},
	}
}

func TestResolveMacTargetBundleKeepsApplicationsInstall(t *testing.T) {
	t.Parallel()

	currentBundle := "/Applications/xiadown.app"
	targetBundle := resolveMacTargetBundle(currentBundle)
	if targetBundle != currentBundle {
		t.Fatalf("expected applications bundle to stay in place, got %q", targetBundle)
	}
}

func TestResolveMacTargetBundleMovesNonApplicationsInstall(t *testing.T) {
	t.Parallel()

	currentBundle := "/Users/test/Downloads/xiadown.app"
	targetBundle := resolveMacTargetBundle(currentBundle)
	if targetBundle != "/Applications/xiadown.app" {
		t.Fatalf("expected non-applications bundle to move to /Applications, got %q", targetBundle)
	}
}

func TestExtractZipExecutableRejectsTraversal(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "xiadown.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../xiadown.exe")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("payload")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	_, err = extractZipExecutable(archivePath, t.TempDir(), "xiadown.exe")
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestLoadPlanAcceptsValidWindowsPortablePlan(t *testing.T) {
	t.Parallel()

	installer, plan, _ := validWindowsRestartFixture(t)
	if err := installer.savePlan(plan); err != nil {
		t.Fatalf("save valid plan: %v", err)
	}

	loaded, err := installer.loadPlan()
	if err != nil {
		t.Fatalf("load valid plan: %v", err)
	}
	if loaded.StageDir != plan.StageDir || loaded.SourcePath != plan.SourcePath || loaded.TargetPath != plan.TargetPath {
		t.Fatalf("valid plan changed unexpectedly: %#v", loaded)
	}
}

func TestLoadPlanAcceptsValidWindowsInstallerPlan(t *testing.T) {
	t.Parallel()

	installer, plan, _ := validWindowsRestartFixture(t)
	installerSource := filepath.Join(plan.StageDir, "xiadown-installer.exe")
	if err := os.WriteFile(installerSource, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write staged installer: %v", err)
	}
	plan.Mode = "installer"
	plan.SourcePath = installerSource
	if err := installer.savePlan(plan); err != nil {
		t.Fatalf("save installer plan: %v", err)
	}

	loaded, err := installer.loadPlan()
	if err != nil {
		t.Fatalf("load valid installer plan: %v", err)
	}
	if loaded.Mode != "installer" || loaded.SourcePath != installerSource {
		t.Fatalf("unexpected installer plan: %#v", loaded)
	}
}

func TestLoadPlanRejectsUnsafeStagedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(t *testing.T, installer *PlatformInstaller, plan *stagedPlan)
		wantErrorText string
	}{
		{
			name: "stage path prefix collision",
			mutate: func(t *testing.T, installer *PlatformInstaller, plan *stagedPlan) {
				t.Helper()
				stageDir := filepath.Join(installer.stateDir, "update-stage-escape", "prepared-evil")
				sourcePath := filepath.Join(stageDir, "portable", "xiadown.exe")
				if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
					t.Fatalf("create prefix-collision stage: %v", err)
				}
				if err := os.WriteFile(sourcePath, []byte("evil"), 0o755); err != nil {
					t.Fatalf("write prefix-collision source: %v", err)
				}
				plan.StageDir = stageDir
				plan.SourcePath = sourcePath
			},
			wantErrorText: "outside update stage root",
		},
		{
			name: "external source",
			mutate: func(t *testing.T, _ *PlatformInstaller, plan *stagedPlan) {
				t.Helper()
				externalSource := filepath.Join(t.TempDir(), "external.exe")
				if err := os.WriteFile(externalSource, []byte("evil"), 0o755); err != nil {
					t.Fatalf("write external source: %v", err)
				}
				plan.SourcePath = externalSource
			},
			wantErrorText: "outside stage directory",
		},
		{
			name: "source symlink escape",
			mutate: func(t *testing.T, _ *PlatformInstaller, plan *stagedPlan) {
				t.Helper()
				externalSource := filepath.Join(t.TempDir(), "external.exe")
				if err := os.WriteFile(externalSource, []byte("evil"), 0o755); err != nil {
					t.Fatalf("write external source: %v", err)
				}
				symlinkPath := filepath.Join(plan.StageDir, "portable", "linked.exe")
				if err := os.Symlink(externalSource, symlinkPath); err != nil {
					t.Skipf("symlinks unavailable on this platform: %v", err)
				}
				plan.SourcePath = symlinkPath
			},
			wantErrorText: "symbolic link is not allowed",
		},
		{
			name: "stage symlink escape",
			mutate: func(t *testing.T, installer *PlatformInstaller, plan *stagedPlan) {
				t.Helper()
				externalStage := filepath.Join(t.TempDir(), "external-stage")
				externalSource := filepath.Join(externalStage, "portable", "xiadown.exe")
				if err := os.MkdirAll(filepath.Dir(externalSource), 0o755); err != nil {
					t.Fatalf("create external stage: %v", err)
				}
				if err := os.WriteFile(externalSource, []byte("evil"), 0o755); err != nil {
					t.Fatalf("write external staged source: %v", err)
				}
				stageLink := filepath.Join(installer.stateDir, "update-stage", "prepared-link")
				if err := os.Symlink(externalStage, stageLink); err != nil {
					t.Skipf("symlinks unavailable on this platform: %v", err)
				}
				plan.StageDir = stageLink
				plan.SourcePath = filepath.Join(stageLink, "portable", "xiadown.exe")
			},
			wantErrorText: "symbolic link is not allowed",
		},
		{
			name: "stage root symlink escape",
			mutate: func(t *testing.T, installer *PlatformInstaller, plan *stagedPlan) {
				t.Helper()
				stageRoot := filepath.Join(installer.stateDir, "update-stage")
				if err := os.RemoveAll(stageRoot); err != nil {
					t.Fatalf("remove original stage root: %v", err)
				}
				externalRoot := t.TempDir()
				externalStage := filepath.Join(externalRoot, "prepared-evil")
				externalSource := filepath.Join(externalStage, "portable", "xiadown.exe")
				if err := os.MkdirAll(filepath.Dir(externalSource), 0o755); err != nil {
					t.Fatalf("create external stage: %v", err)
				}
				if err := os.WriteFile(externalSource, []byte("evil"), 0o755); err != nil {
					t.Fatalf("write external staged source: %v", err)
				}
				if err := os.Symlink(externalRoot, stageRoot); err != nil {
					t.Skipf("symlinks unavailable on this platform: %v", err)
				}
				plan.StageDir = filepath.Join(stageRoot, "prepared-evil")
				plan.SourcePath = filepath.Join(plan.StageDir, "portable", "xiadown.exe")
			},
			wantErrorText: "symbolic link is not allowed",
		},
		{
			name: "wrong Windows artifact shape",
			mutate: func(t *testing.T, _ *PlatformInstaller, plan *stagedPlan) {
				t.Helper()
				sourcePath := filepath.Join(plan.StageDir, "portable", "payload.txt")
				if err := os.WriteFile(sourcePath, []byte("evil"), 0o644); err != nil {
					t.Fatalf("write invalid source: %v", err)
				}
				plan.SourcePath = sourcePath
			},
			wantErrorText: "regular .exe",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			installer, plan, _ := validWindowsRestartFixture(t)
			test.mutate(t, installer, &plan)
			if err := installer.savePlan(plan); err != nil {
				t.Fatalf("save unsafe plan: %v", err)
			}

			_, err := installer.loadPlan()
			if err == nil || !strings.Contains(err.Error(), test.wantErrorText) {
				t.Fatalf("expected %q rejection, got %v", test.wantErrorText, err)
			}
		})
	}
}

func TestLoadPlanRejectsCorruptAndUnexpectedJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{name: "malformed", data: `{"platform":"windows"`},
		{name: "unknown field", data: `{"platform":"windows","unexpected":true}`},
		{name: "trailing object", data: `{}` + "\n" + `{}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			stateDir := t.TempDir()
			installer := &PlatformInstaller{
				stateDir: stateDir,
				planPath: filepath.Join(stateDir, "update_install_plan.json"),
			}
			if err := os.WriteFile(installer.planPath, []byte(test.data), 0o600); err != nil {
				t.Fatalf("write corrupt plan: %v", err)
			}
			if _, err := installer.loadPlan(); err == nil {
				t.Fatal("expected corrupt plan rejection")
			}
		})
	}
}

func TestCleanupStagedUpdateNeverRemovesUnsafeStage(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	externalStage := filepath.Join(t.TempDir(), "must-survive")
	marker := filepath.Join(externalStage, "marker.txt")
	if err := os.MkdirAll(externalStage, 0o755); err != nil {
		t.Fatalf("create external stage: %v", err)
	}
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write external marker: %v", err)
	}
	installer := &PlatformInstaller{
		stateDir: stateDir,
		planPath: filepath.Join(stateDir, "update_install_plan.json"),
	}
	if err := installer.savePlan(stagedPlan{
		Platform:   "windows",
		Mode:       "portable",
		StageDir:   externalStage,
		SourcePath: filepath.Join(externalStage, "external.exe"),
		TargetPath: filepath.Join(stateDir, "xiadown.exe"),
	}); err != nil {
		t.Fatalf("save malicious plan: %v", err)
	}

	if err := installer.cleanupStagedUpdate(); err != nil {
		t.Fatalf("clear malicious plan: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("unsafe external stage was touched: %v", err)
	}
	if _, err := os.Stat(installer.planPath); !os.IsNotExist(err) {
		t.Fatalf("invalid plan should be discarded safely, got %v", err)
	}
}

func TestRemoveStageDirRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	stageRoot := filepath.Join(stateDir, "update-stage")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		t.Fatalf("create stage root: %v", err)
	}
	externalStage := filepath.Join(t.TempDir(), "must-survive")
	if err := os.MkdirAll(externalStage, 0o755); err != nil {
		t.Fatalf("create external stage: %v", err)
	}
	marker := filepath.Join(externalStage, "marker.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	stageLink := filepath.Join(stageRoot, "prepared-link")
	if err := os.Symlink(externalStage, stageLink); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	installer := &PlatformInstaller{stateDir: stateDir}

	err := installer.removeStageDir(stageLink)
	if err == nil || !strings.Contains(err.Error(), "symbolic link is not allowed") {
		t.Fatalf("expected symlink cleanup rejection, got %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("symlink target was touched: %v", err)
	}
}

func TestCleanupStagedUpdateDiscardsMalformedPlanSafely(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	planPath := filepath.Join(stateDir, "update_install_plan.json")
	if err := os.WriteFile(planPath, []byte(`{"stageDir":"/","sourcePath":`), 0o600); err != nil {
		t.Fatalf("write malformed plan: %v", err)
	}
	marker := filepath.Join(stateDir, "marker.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	installer := &PlatformInstaller{stateDir: stateDir, planPath: planPath}

	if err := installer.cleanupStagedUpdate(); err != nil {
		t.Fatalf("discard malformed plan: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("unrelated file was touched: %v", err)
	}
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Fatalf("malformed plan should be removed, got %v", err)
	}
}

func TestCleanupStagedUpdateRemovesValidStage(t *testing.T) {
	t.Parallel()

	installer, plan, _ := validWindowsRestartFixture(t)
	if err := installer.savePlan(plan); err != nil {
		t.Fatalf("save valid plan: %v", err)
	}
	if err := installer.cleanupStagedUpdate(); err != nil {
		t.Fatalf("cleanup valid plan: %v", err)
	}
	if _, err := os.Stat(plan.StageDir); !os.IsNotExist(err) {
		t.Fatalf("expected valid stage directory to be removed, got %v", err)
	}
	if _, err := os.Stat(installer.planPath); !os.IsNotExist(err) {
		t.Fatalf("expected valid plan to be removed, got %v", err)
	}
}

func TestCleanupStagedUpdateIsIdempotentWhenStageAlreadyMissing(t *testing.T) {
	t.Parallel()

	installer, plan, _ := validWindowsRestartFixture(t)
	if err := installer.savePlan(plan); err != nil {
		t.Fatalf("save valid plan: %v", err)
	}
	if err := os.RemoveAll(plan.StageDir); err != nil {
		t.Fatalf("simulate externally removed stage: %v", err)
	}

	if err := installer.cleanupStagedUpdate(); err != nil {
		t.Fatalf("cleanup missing stage: %v", err)
	}
	if _, err := os.Stat(installer.planPath); !os.IsNotExist(err) {
		t.Fatalf("stale plan should be removed, got %v", err)
	}
	if err := installer.cleanupStagedUpdate(); err != nil {
		t.Fatalf("repeated cleanup should remain idempotent: %v", err)
	}
}

func TestValidateMacUpdateBundleAcceptsMatchingSignedBundle(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	currentBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Current.app"), "com.dreamapp.xiadown")
	stagedBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Staged.app"), "com.dreamapp.xiadown")
	installer := &PlatformInstaller{
		commandOutput: fakeMacValidationCommands(map[string]string{
			currentBundle: "ABCDE12345",
			stagedBundle:  "ABCDE12345",
		}),
	}

	validation, err := installer.validateMacUpdateBundle(context.Background(), currentBundle, stagedBundle)
	if err != nil {
		t.Fatalf("validateMacUpdateBundle failed: %v", err)
	}
	if validation.BundleID != "com.dreamapp.xiadown" {
		t.Fatalf("unexpected bundle id %q", validation.BundleID)
	}
	if validation.TeamID != "ABCDE12345" {
		t.Fatalf("unexpected team id %q", validation.TeamID)
	}
}

func TestValidateMacUpdateBundleRejectsBundleIdentifierMismatch(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	currentBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Current.app"), "com.dreamapp.xiadown")
	stagedBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Staged.app"), "com.example.other")
	installer := &PlatformInstaller{
		commandOutput: fakeMacValidationCommands(map[string]string{
			currentBundle: "ABCDE12345",
			stagedBundle:  "ABCDE12345",
		}),
	}

	_, err := installer.validateMacUpdateBundle(context.Background(), currentBundle, stagedBundle)
	if err == nil || !strings.Contains(err.Error(), "bundle identifier mismatch") {
		t.Fatalf("expected bundle identifier mismatch, got %v", err)
	}
}

func TestValidateMacUpdateBundleRejectsSigningTeamMismatch(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	currentBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Current.app"), "com.dreamapp.xiadown")
	stagedBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Staged.app"), "com.dreamapp.xiadown")
	installer := &PlatformInstaller{
		commandOutput: fakeMacValidationCommands(map[string]string{
			currentBundle: "ABCDE12345",
			stagedBundle:  "VWXYZ67890",
		}),
	}

	_, err := installer.validateMacUpdateBundle(context.Background(), currentBundle, stagedBundle)
	if err == nil || !strings.Contains(err.Error(), "signing team mismatch") {
		t.Fatalf("expected signing team mismatch, got %v", err)
	}
}

func TestValidateMacUpdateBundleAllowsUnsignedCurrentBundleMigration(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	currentBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Current.app"), "com.dreamapp.xiadown")
	stagedBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Staged.app"), "com.dreamapp.xiadown")
	installer := &PlatformInstaller{
		commandOutput: fakeMacValidationCommands(map[string]string{
			currentBundle: "not set",
			stagedBundle:  "ABCDE12345",
		}),
	}

	validation, err := installer.validateMacUpdateBundle(context.Background(), currentBundle, stagedBundle)
	if err != nil {
		t.Fatalf("validateMacUpdateBundle failed: %v", err)
	}
	if validation.TeamID != "ABCDE12345" {
		t.Fatalf("unexpected team id %q", validation.TeamID)
	}
}

func TestRestartDarwinDerivesRelaunchAndFallbackFromCurrentExecutable(t *testing.T) {
	t.Parallel()

	var (
		capturedName string
		capturedArgs []string
	)
	installer, plan, currentBundle, targetBundle := validMacRestartFixture(t)
	plan.RelaunchPath = "/tmp/untrusted-relaunch.app"
	plan.FallbackPath = "/tmp/untrusted-fallback.app"
	installer.startDetached = func(name string, args []string) error {
		capturedName = name
		capturedArgs = append([]string(nil), args...)
		return nil
	}

	if err := installer.restartDarwin(context.Background(), plan); err != nil {
		t.Fatalf("restartDarwin failed: %v", err)
	}

	if capturedName != "/bin/sh" {
		t.Fatalf("unexpected restart helper: %q", capturedName)
	}
	if len(capturedArgs) != 11 {
		t.Fatalf("unexpected helper args: %#v", capturedArgs)
	}
	if capturedArgs[4] != targetBundle {
		t.Fatalf("expected derived relaunch path %q, got %q", targetBundle, capturedArgs[4])
	}
	if capturedArgs[5] != currentBundle {
		t.Fatalf("expected derived fallback path %q, got %q", currentBundle, capturedArgs[5])
	}
	if capturedArgs[8] != installer.whatsNewPendingPath {
		t.Fatalf("expected pending what's new path %q, got %q", installer.whatsNewPendingPath, capturedArgs[8])
	}
	if capturedArgs[9] != "com.dreamapp.xiadown" {
		t.Fatalf("expected revalidated bundle id, got %q", capturedArgs[9])
	}
	if capturedArgs[10] != "ABCDE12345" {
		t.Fatalf("expected revalidated team id, got %q", capturedArgs[10])
	}
}

func TestRestartToApplyAcceptsLegitimateLegacyMacPlan(t *testing.T) {
	t.Parallel()

	installer, plan, currentBundle, targetBundle := validMacRestartFixture(t)
	plan.RelaunchPath = ""
	plan.FallbackPath = ""
	plan.BundleID = ""
	plan.TeamID = ""
	if err := installer.savePlan(plan); err != nil {
		t.Fatalf("save legacy plan: %v", err)
	}
	var capturedArgs []string
	installer.startDetached = func(_ string, args []string) error {
		capturedArgs = append([]string(nil), args...)
		return nil
	}

	if err := installer.RestartToApply(context.Background()); err != nil {
		t.Fatalf("restart legitimate legacy plan: %v", err)
	}
	if len(capturedArgs) != 11 {
		t.Fatalf("unexpected helper args: %#v", capturedArgs)
	}
	if capturedArgs[4] != targetBundle || capturedArgs[5] != currentBundle {
		t.Fatalf("legacy paths were not safely re-derived: %#v", capturedArgs)
	}
	if capturedArgs[9] != "com.dreamapp.xiadown" || capturedArgs[10] != "ABCDE12345" {
		t.Fatalf("legacy signing metadata was not safely re-derived: %#v", capturedArgs)
	}
}

func writeTestMacBundle(t *testing.T, path string, bundleID string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "Contents"), 0o755); err != nil {
		t.Fatalf("create test bundle: %v", err)
	}
	infoPlist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>` + bundleID + `</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(path, "Contents", "Info.plist"), []byte(infoPlist), 0o644); err != nil {
		t.Fatalf("write test Info.plist: %v", err)
	}
	return path
}

func fakeMacValidationCommands(teamByBundle map[string]string) func(context.Context, string, ...string) ([]byte, error) {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 0 {
			return nil, os.ErrInvalid
		}
		bundlePath := args[len(args)-1]
		switch {
		case name == "/usr/bin/codesign" && args[0] == "--verify":
			return []byte("valid"), nil
		case name == "/usr/bin/codesign" && args[0] == "-dv":
			teamID := teamByBundle[bundlePath]
			return []byte("Executable=/Contents/MacOS/xiadown\nTeamIdentifier=" + teamID + "\n"), nil
		case name == "/usr/sbin/spctl":
			return []byte(bundlePath + ": accepted"), nil
		case name == "/usr/libexec/PlistBuddy":
			return nil, os.ErrInvalid
		default:
			return nil, os.ErrInvalid
		}
	}
}

func validMacRestartFixture(t *testing.T) (*PlatformInstaller, stagedPlan, string, string) {
	t.Helper()
	stateDir := t.TempDir()
	stageDir := filepath.Join(stateDir, "update-stage", "prepared-test")
	stagedBundle := writeTestMacBundle(t, filepath.Join(stageDir, "bundle", "xiadown.app"), "com.dreamapp.xiadown")
	currentBundle := writeTestMacBundle(t, filepath.Join(stateDir, "Current.app"), "com.dreamapp.xiadown")
	currentExe := filepath.Join(currentBundle, "Contents", "MacOS", "xiadown")
	if err := os.MkdirAll(filepath.Dir(currentExe), 0o755); err != nil {
		t.Fatalf("create current executable directory: %v", err)
	}
	if err := os.WriteFile(currentExe, []byte("current"), 0o755); err != nil {
		t.Fatalf("write current executable: %v", err)
	}
	resolvedExe, err := filepath.EvalSymlinks(currentExe)
	if err != nil {
		t.Fatalf("resolve current executable: %v", err)
	}
	resolvedCurrentBundle, err := resolveAppBundle(resolvedExe)
	if err != nil {
		t.Fatalf("resolve current app bundle: %v", err)
	}
	targetBundle := resolveMacTargetBundle(resolvedCurrentBundle)
	installer := &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
		goos:                "darwin",
		executablePath: func() (string, error) {
			return currentExe, nil
		},
		commandOutput: fakeMacValidationCommands(map[string]string{
			resolvedCurrentBundle: "ABCDE12345",
			stagedBundle:          "ABCDE12345",
		}),
	}
	return installer, stagedPlan{
		Platform:   "darwin",
		Mode:       "bundle",
		StageDir:   stageDir,
		SourcePath: stagedBundle,
		TargetPath: targetBundle,
		BundleID:   "untrusted.bundle.id",
		TeamID:     "UNTRUSTED",
	}, resolvedCurrentBundle, targetBundle
}

func validWindowsRestartFixture(t *testing.T) (*PlatformInstaller, stagedPlan, string) {
	t.Helper()
	stateDir := t.TempDir()
	stageDir := filepath.Join(stateDir, "update-stage", "prepared-test")
	stagedExe := filepath.Join(stageDir, "portable", "xiadown.exe")
	if err := os.MkdirAll(filepath.Dir(stagedExe), 0o755); err != nil {
		t.Fatalf("create Windows stage: %v", err)
	}
	if err := os.WriteFile(stagedExe, []byte("update"), 0o755); err != nil {
		t.Fatalf("write staged executable: %v", err)
	}
	currentExe := filepath.Join(stateDir, "current", "xiadown.exe")
	if err := os.MkdirAll(filepath.Dir(currentExe), 0o755); err != nil {
		t.Fatalf("create current executable directory: %v", err)
	}
	if err := os.WriteFile(currentExe, []byte("current"), 0o755); err != nil {
		t.Fatalf("write current executable: %v", err)
	}
	resolvedCurrentExe, err := filepath.EvalSymlinks(currentExe)
	if err != nil {
		t.Fatalf("resolve current executable: %v", err)
	}
	installer := &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
		goos:                "windows",
		executablePath: func() (string, error) {
			return currentExe, nil
		},
	}
	return installer, stagedPlan{
		Platform:     "windows",
		Mode:         "portable",
		StageDir:     stageDir,
		SourcePath:   stagedExe,
		TargetPath:   resolvedCurrentExe,
		RelaunchPath: filepath.Join(stateDir, "untrusted-relaunch.exe"),
		InstallDir:   filepath.Join(stateDir, "untrusted-install-dir"),
	}, resolvedCurrentExe
}

func TestRestartWindowsDerivesTargetAndInstallDirectory(t *testing.T) {
	t.Parallel()

	installer, plan, currentExe := validWindowsRestartFixture(t)
	var capturedArgs []string
	installer.startDetached = func(name string, args []string) error {
		if name != "powershell.exe" {
			t.Fatalf("unexpected Windows restart helper %q", name)
		}
		capturedArgs = append([]string(nil), args...)
		return nil
	}

	if err := installer.restartWindows(context.Background(), plan); err != nil {
		t.Fatalf("restartWindows failed: %v", err)
	}
	if len(capturedArgs) != 13 {
		t.Fatalf("unexpected helper args: %#v", capturedArgs)
	}
	if capturedArgs[8] != currentExe {
		t.Fatalf("expected derived target %q, got %q", currentExe, capturedArgs[8])
	}
	if capturedArgs[9] != filepath.Dir(currentExe) {
		t.Fatalf("expected derived install directory %q, got %q", filepath.Dir(currentExe), capturedArgs[9])
	}
}

func TestRestartDarwinRejectsUntrustedExternalTarget(t *testing.T) {
	t.Parallel()

	installer, plan, _, _ := validMacRestartFixture(t)
	plan.TargetPath = filepath.Join(t.TempDir(), "Victim.app")
	started := false
	installer.startDetached = func(string, []string) error {
		started = true
		return nil
	}

	err := installer.restartDarwin(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "does not match current app target") {
		t.Fatalf("expected untrusted target rejection, got %v", err)
	}
	if started {
		t.Fatal("restart helper must not start for an untrusted target")
	}
	if _, statErr := os.Stat(filepath.Join(installer.stateDir, "apply_update.sh")); !os.IsNotExist(statErr) {
		t.Fatalf("helper script must not be written for an untrusted target, got %v", statErr)
	}
}

func TestPendingWhatsNewReadsCopiedPlan(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	installer := &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
	}
	if err := installer.savePlan(stagedPlan{
		Version:   "2.0.7",
		Changelog: "## Updated",
	}); err != nil {
		t.Fatalf("savePlan failed: %v", err)
	}
	data, err := os.ReadFile(installer.planPath)
	if err != nil {
		t.Fatalf("read plan failed: %v", err)
	}
	if err := os.WriteFile(installer.whatsNewPendingPath, data, 0o600); err != nil {
		t.Fatalf("write pending file failed: %v", err)
	}

	notice, found, err := installer.PendingWhatsNew(context.Background())
	if err != nil {
		t.Fatalf("PendingWhatsNew failed: %v", err)
	}
	if !found {
		t.Fatal("expected pending what's new notice")
	}
	if notice.Version != "2.0.7" {
		t.Fatalf("expected version 2.0.7, got %q", notice.Version)
	}
	if notice.Changelog != "## Updated" {
		t.Fatalf("expected changelog to be preserved, got %q", notice.Changelog)
	}
}

func TestMarkWhatsNewSeenPersistsVersionAndClearsCoveredPendingNotice(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	installer := &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
	}
	if err := os.WriteFile(installer.whatsNewPendingPath, []byte(`{"version":"2.0.7","changelog":"hi"}`), 0o600); err != nil {
		t.Fatalf("seed pending file failed: %v", err)
	}

	if err := installer.MarkWhatsNewSeen(context.Background(), "2.0.7"); err != nil {
		t.Fatalf("MarkWhatsNewSeen failed: %v", err)
	}

	seenVersion, err := installer.SeenWhatsNewVersion(context.Background())
	if err != nil {
		t.Fatalf("SeenWhatsNewVersion failed: %v", err)
	}
	if seenVersion != "2.0.7" {
		t.Fatalf("expected seen version 2.0.7, got %q", seenVersion)
	}
	if _, err := os.Stat(installer.whatsNewPendingPath); !os.IsNotExist(err) {
		t.Fatalf("expected pending file to be removed, got err=%v", err)
	}
}
