package browsercdp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeShouldSuppressTransientCDPDocumentUpdatedError(t *testing.T) {
	t.Parallel()

	if !runtimeShouldSuppressCDPError("received DOM.documentUpdated when there's no top-level frame") {
		t.Fatal("expected transient DOM.documentUpdated frame error to be suppressed")
	}
	if runtimeShouldSuppressCDPError("could not attach target") {
		t.Fatal("expected unrelated CDP errors to be preserved")
	}
}

func TestBuildLaunchArgs_HeadfulAllowsStartupPage(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if strings.Contains(joined, " --no-startup-window ") {
		t.Fatalf("expected headful launch to allow a startup page, args=%v", args)
	}
}

func TestBuildLaunchArgsBindsRemoteDebuggingToLoopback(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --remote-debugging-address=127.0.0.1 ") {
		t.Fatalf("expected cdp to bind loopback, args=%v", args)
	}
}

func TestBuildLaunchArgs_TransientProfileUsesMockKeychain(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{})
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --password-store=basic ") {
		t.Fatalf("expected transient profile to use basic password store, args=%v", args)
	}
	if !strings.Contains(joined, " --use-mock-keychain ") {
		t.Fatalf("expected transient profile to use mock keychain, args=%v", args)
	}
}

func TestBuildLaunchArgs_PersistentProfileUsesRealKeychain(t *testing.T) {
	t.Parallel()

	args := buildLaunchArgs(9222, "/tmp/xiadown-profile", LaunchOptions{PersistentProfile: true})
	joined := " " + strings.Join(args, " ") + " "

	if strings.Contains(joined, " --use-mock-keychain ") {
		t.Fatalf("expected persistent profile to avoid mock keychain, args=%v", args)
	}
	if strings.Contains(joined, " --password-store=basic ") {
		t.Fatalf("expected persistent profile to avoid basic password store, args=%v", args)
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

func TestAppendBrowserLaunchArgs_DisablesVivaldiWelcome(t *testing.T) {
	t.Parallel()

	args := appendBrowserLaunchArgs([]string{"--custom-flag"}, BrowserVivaldi)
	joined := " " + strings.Join(args, " ") + " "

	if !strings.Contains(joined, " --disable-vivaldi ") {
		t.Fatalf("expected Vivaldi launch to disable Vivaldi welcome, args=%v", args)
	}
}

func TestAppendBrowserLaunchArgs_LeavesOtherBrowsersUnchanged(t *testing.T) {
	t.Parallel()

	args := appendBrowserLaunchArgs([]string{"--custom-flag"}, BrowserChrome)

	if len(args) != 1 || args[0] != "--custom-flag" {
		t.Fatalf("expected Chrome launch args unchanged, got %v", args)
	}
}

func TestAppendStartupPageArg_HeadlessSkipsPageArg(t *testing.T) {
	t.Parallel()

	args := appendStartupPageArg([]string{"--custom-flag"}, LaunchOptions{Headless: true})

	if len(args) != 1 || args[0] != "--custom-flag" {
		t.Fatalf("expected headless launch args unchanged, got %v", args)
	}
}

func TestRuntimeGracefulCloseTimeout_ExtendsForPersistentProfile(t *testing.T) {
	t.Parallel()

	if got := runtimeGracefulCloseTimeout(LaunchOptions{}); got != 1500*time.Millisecond {
		t.Fatalf("expected transient close timeout 1.5s, got %s", got)
	}
	if got := runtimeGracefulCloseTimeout(LaunchOptions{PersistentProfile: true}); got != 3*time.Second {
		t.Fatalf("expected persistent close timeout 3s, got %s", got)
	}
}

func TestRuntimeTerminateGraceTimeout_ExtendsForPersistentProfile(t *testing.T) {
	t.Parallel()

	if got := runtimeTerminateGraceTimeout(LaunchOptions{}); got != 300*time.Millisecond {
		t.Fatalf("expected transient terminate grace 300ms, got %s", got)
	}
	if got := runtimeTerminateGraceTimeout(LaunchOptions{PersistentProfile: true}); got != 1*time.Millisecond {
		t.Fatalf("expected persistent terminate grace 1ms, got %s", got)
	}
}

func TestReadDevToolsActivePortBuildsWebSocketURL(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "DevToolsActivePort")
	if err := os.WriteFile(path, []byte("43123\n/devtools/browser/session-id\n"), 0o644); err != nil {
		t.Fatalf("write devtools active port: %v", err)
	}

	port, wsURL, err := readDevToolsActivePort(path)
	if err != nil {
		t.Fatalf("read devtools active port: %v", err)
	}
	if port != 43123 {
		t.Fatalf("expected port 43123, got %d", port)
	}
	if wsURL != "ws://127.0.0.1:43123/devtools/browser/session-id" {
		t.Fatalf("unexpected websocket url: %q", wsURL)
	}
}

func TestRuntimeRegistryCleanupRemovesStaleRecord(t *testing.T) {
	tempDir := t.TempDir()
	previousRegistryDir := runtimeRegistryDir
	runtimeRegistryDir = func() string { return tempDir }
	t.Cleanup(func() { runtimeRegistryDir = previousRegistryDir })

	id, err := registerRuntimeProcess(runtimeProcessRecord{
		PID:            0,
		ProcessGroupID: 0,
		ExecutablePath: "/tmp/browser",
		UserDataDir:    "/tmp/profile",
		CDPPort:        9222,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("register runtime process: %v", err)
	}
	if id != "" {
		t.Fatalf("expected invalid pid to skip registration, got %q", id)
	}

	record := runtimeProcessRecord{
		ID:             "stale",
		PID:            -1,
		ProcessGroupID: 0,
		ExecutablePath: "/tmp/browser",
		UserDataDir:    "/tmp/profile",
		CDPPort:        9223,
		CreatedAt:      time.Now(),
	}
	payload := []byte(`{"id":"stale","pid":-1,"executablePath":"/tmp/browser","cdpPort":9223}`)
	if err := os.WriteFile(filepath.Join(tempDir, record.ID+".json"), payload, 0o644); err != nil {
		t.Fatalf("write stale record: %v", err)
	}

	if err := CleanupStaleRuntimes(context.Background()); err != nil {
		t.Fatalf("cleanup stale runtimes: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, record.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected stale record to be removed, err=%v", err)
	}
}

func TestRegisterRuntimeProcessUsesPrivatePermissions(t *testing.T) {
	tempDir := filepath.Join(t.TempDir(), "registry")
	previousRegistryDir := runtimeRegistryDir
	runtimeRegistryDir = func() string { return tempDir }
	t.Cleanup(func() { runtimeRegistryDir = previousRegistryDir })

	id, err := registerRuntimeProcess(runtimeProcessRecord{
		PID:            12345,
		ProcessGroupID: 12345,
		ExecutablePath: "/tmp/browser",
		UserDataDir:    "/tmp/profile",
		CDPPort:        9222,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("register runtime process: %v", err)
	}
	if id == "" {
		t.Fatal("expected runtime process record id")
	}
	dirInfo, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("stat registry dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("expected registry dir mode 0700, got %o", got)
	}
	fileInfo, err := os.Stat(filepath.Join(tempDir, id+".json"))
	if err != nil {
		t.Fatalf("stat registry record: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected registry record mode 0600, got %o", got)
	}
}
