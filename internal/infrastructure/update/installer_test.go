package update

import (
	"context"
	"os"
	"path/filepath"
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
	}
	if err := installer.restartDarwin(plan); err != nil {
		t.Fatalf("restartDarwin failed: %v", err)
	}

	if capturedName != "/bin/sh" {
		t.Fatalf("unexpected restart helper: %q", capturedName)
	}
	if len(capturedArgs) != 9 {
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

	if len(capturedArgs) != 9 {
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
