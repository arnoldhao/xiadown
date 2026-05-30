package update

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestRestartDarwinUsesExplicitRelaunchAndFallbackPaths(t *testing.T) {
	t.Parallel()

	var (
		capturedName string
		capturedArgs []string
	)
	stateDir := t.TempDir()
	installer := &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
		startDetached: func(name string, args []string) error {
			capturedName = name
			capturedArgs = append([]string(nil), args...)
			return nil
		},
	}

	plan := stagedPlan{
		SourcePath:   "/tmp/source.app",
		TargetPath:   "/Applications/xiadown.app",
		RelaunchPath: "/Applications/xiadown.app",
		FallbackPath: "/Users/test/bin/xiadown.app",
		StageDir:     "/tmp/stage",
		BundleID:     "com.dreamapp.xiadown",
		TeamID:       "ABCDE12345",
	}
	if err := installer.restartDarwin(plan); err != nil {
		t.Fatalf("restartDarwin failed: %v", err)
	}

	if capturedName != "/bin/sh" {
		t.Fatalf("unexpected restart helper: %q", capturedName)
	}
	if len(capturedArgs) != 11 {
		t.Fatalf("unexpected helper args: %#v", capturedArgs)
	}
	if capturedArgs[4] != plan.RelaunchPath {
		t.Fatalf("expected relaunch path %q, got %q", plan.RelaunchPath, capturedArgs[4])
	}
	if capturedArgs[5] != plan.FallbackPath {
		t.Fatalf("expected fallback path %q, got %q", plan.FallbackPath, capturedArgs[5])
	}
	if capturedArgs[8] != installer.whatsNewPendingPath {
		t.Fatalf("expected pending what's new path %q, got %q", installer.whatsNewPendingPath, capturedArgs[8])
	}
	if capturedArgs[9] != plan.BundleID {
		t.Fatalf("expected bundle id %q, got %q", plan.BundleID, capturedArgs[9])
	}
	if capturedArgs[10] != plan.TeamID {
		t.Fatalf("expected team id %q, got %q", plan.TeamID, capturedArgs[10])
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

func TestRestartDarwinDefaultsRelaunchAndFallbackToTarget(t *testing.T) {
	t.Parallel()

	var capturedArgs []string
	stateDir := t.TempDir()
	installer := &PlatformInstaller{
		stateDir:            stateDir,
		planPath:            filepath.Join(stateDir, "update_install_plan.json"),
		whatsNewPendingPath: filepath.Join(stateDir, "pending_whats_new.json"),
		whatsNewSeenPath:    filepath.Join(stateDir, "whats_new_seen.json"),
		startDetached: func(_ string, args []string) error {
			capturedArgs = append([]string(nil), args...)
			return nil
		},
	}

	plan := stagedPlan{
		SourcePath: "/tmp/source.app",
		TargetPath: "/Applications/xiadown.app",
		StageDir:   "/tmp/stage",
	}
	if err := installer.restartDarwin(plan); err != nil {
		t.Fatalf("restartDarwin failed: %v", err)
	}

	if len(capturedArgs) != 11 {
		t.Fatalf("unexpected helper args: %#v", capturedArgs)
	}
	if capturedArgs[4] != plan.TargetPath {
		t.Fatalf("expected default relaunch path %q, got %q", plan.TargetPath, capturedArgs[4])
	}
	if capturedArgs[5] != plan.TargetPath {
		t.Fatalf("expected default fallback path %q, got %q", plan.TargetPath, capturedArgs[5])
	}
	if capturedArgs[8] != installer.whatsNewPendingPath {
		t.Fatalf("expected pending what's new path %q, got %q", installer.whatsNewPendingPath, capturedArgs[8])
	}
	if capturedArgs[9] != "" || capturedArgs[10] != "" {
		t.Fatalf("expected empty signing validation args, got bundle=%q team=%q", capturedArgs[9], capturedArgs[10])
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
