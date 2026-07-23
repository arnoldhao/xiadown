package browsercdp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	targetpkg "github.com/chromedp/cdproto/target"
)

const currentBrowserTestID = "598cf21d-ec63-45f3-abba-698f26a88807"

func TestInspectCurrentChromeAtAcceptsTrustedWebSocketOnlyEndpoint(t *testing.T) {
	root := trustedCurrentBrowserTestRoot(t)
	const port = 43123
	writeCurrentBrowserTestFile(t, root, "DevToolsActivePort", fmt.Sprintf("%d\n/devtools/browser/%s\n", port, currentBrowserTestID))
	writeCurrentBrowserTestFile(t, root, "Local State", `{
  "profile": {
    "last_used": "Profile 2",
    "info_cache": {"Profile 2": {"name": "Work"}}
  }
}`)

	inspection := inspectCurrentChromeAt(context.Background(), Candidate{
		ID: BrowserChrome, Available: true,
	}, root, true)
	status := inspection.status
	if status.State != CurrentBrowserStateReady || !status.Ready || !status.Running || !status.Supported {
		t.Fatalf("status = %#v", status)
	}
	if status.Version != "" || status.ProfileName != "Work" {
		t.Fatalf("version/profile = %q/%q", status.Version, status.ProfileName)
	}
	wantEndpoint := fmt.Sprintf("ws://127.0.0.1:%d/devtools/browser/%s", port, currentBrowserTestID)
	if inspection.websocketURL != wantEndpoint {
		t.Fatalf("websocket URL = %q, want %q", inspection.websocketURL, wantEndpoint)
	}
}

func TestValidateCurrentChromeProductRejectsUnsupportedVersion(t *testing.T) {
	version, err := validateCurrentChromeProduct("Chrome/143.0.0.0")
	if version != "143.0.0.0" || CurrentBrowserErrorState(err) != CurrentBrowserStateUnsupportedVersion {
		t.Fatalf("version/state/error = %q/%q/%v", version, CurrentBrowserErrorState(err), err)
	}
}

func TestValidateCurrentChromeProductRequiresChrome(t *testing.T) {
	if _, err := validateCurrentChromeProduct("Chromium/149.0.0.0"); CurrentBrowserErrorState(err) != CurrentBrowserStateUnsupportedBrowser {
		t.Fatalf("state/error = %q/%v", CurrentBrowserErrorState(err), err)
	}
	version, err := validateCurrentChromeProduct("Chrome/149.0.7827.201")
	if err != nil || version != "149.0.7827.201" {
		t.Fatalf("version/error = %q/%v", version, err)
	}
}

func TestInspectCurrentChromeAtDistinguishesStoppedAndDebuggingDisabled(t *testing.T) {
	root := trustedCurrentBrowserTestRoot(t)
	candidate := Candidate{ID: BrowserChrome, Available: true}

	notRunning := inspectCurrentChromeAt(context.Background(), candidate, root, false).status
	if notRunning.State != CurrentBrowserStateNotRunning || notRunning.Running {
		t.Fatalf("not-running status = %#v", notRunning)
	}
	debuggingDisabled := inspectCurrentChromeAt(context.Background(), candidate, root, true).status
	if debuggingDisabled.State != CurrentBrowserStateRemoteDebuggingDisabled || !debuggingDisabled.Running {
		t.Fatalf("debugging-disabled status = %#v", debuggingDisabled)
	}
}

func TestInspectCurrentChromeAtIgnoresStaleEndpointWhenChromeIsStopped(t *testing.T) {
	root := trustedCurrentBrowserTestRoot(t)
	writeCurrentBrowserTestFile(t, root, "DevToolsActivePort", fmt.Sprintf("43123\n/devtools/browser/%s\n", currentBrowserTestID))

	status := inspectCurrentChromeAt(context.Background(), Candidate{
		ID: BrowserChrome, Available: true,
	}, root, false).status
	if status.State != CurrentBrowserStateNotRunning || status.Ready || status.Running {
		t.Fatalf("stale endpoint status = %#v", status)
	}
}

func TestReadTrustedCurrentBrowserFileRejectsSymbolicLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires optional Windows privileges")
	}
	root := trustedCurrentBrowserTestRoot(t)
	realFile := filepath.Join(root, "real-port")
	if err := os.WriteFile(realFile, []byte("9222\n/devtools/browser/test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "DevToolsActivePort")
	if err := os.Symlink(realFile, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readTrustedCurrentBrowserFile(root, link, currentBrowserEndpointReadLimit); err == nil {
		t.Fatal("trusted reader accepted a symbolic link")
	}
}

func TestReadTrustedCurrentBrowserFileRejectsWritableMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX write bits do not describe Windows ACLs")
	}
	root := trustedCurrentBrowserTestRoot(t)
	path := filepath.Join(root, "DevToolsActivePort")
	if err := os.WriteFile(path, []byte("9222\n/devtools/browser/test\n"), 0o622); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o622); err != nil {
		t.Fatal(err)
	}
	if _, err := readTrustedCurrentBrowserFile(root, path, currentBrowserEndpointReadLimit); err == nil {
		t.Fatal("trusted reader accepted group/other-writable metadata")
	}
}

func TestNormalizeCurrentBrowserWebSocketURLRequiresLoopbackAndExactPort(t *testing.T) {
	for _, value := range []string{
		"ws://192.0.2.1:9222/devtools/browser/598cf21d-ec63-45f3-abba-698f26a88807",
		"ws://127.0.0.1:9333/devtools/browser/598cf21d-ec63-45f3-abba-698f26a88807",
		"wss://127.0.0.1:9222/devtools/browser/598cf21d-ec63-45f3-abba-698f26a88807",
		"ws://127.0.0.1:9222/devtools/page/598cf21d-ec63-45f3-abba-698f26a88807",
		"ws://127.0.0.1:9222/devtools/browser/598cf21d-ec63-45f3-abba-698f26a88807?token=secret",
		"ws://[::1]:9222/devtools/browser/598cf21d-ec63-45f3-abba-698f26a88807",
	} {
		if _, err := normalizeCurrentBrowserWebSocketURL(value, 9222); err == nil {
			t.Fatalf("unsafe endpoint %q was accepted", value)
		}
	}
}

func TestBorrowedRuntimeClosePathsOnlyCancelBorrowedContexts(t *testing.T) {
	var browserCancels atomic.Int32
	var allocatorCancels atomic.Int32
	runtimeBrowser := &Runtime{
		ownership: RuntimeOwnershipBorrowed,
		browserCancel: func() {
			browserCancels.Add(1)
		},
		allocCancel: func() {
			allocatorCancels.Add(1)
		},
		stopped: make(chan struct{}),
		status:  Status{Ready: true},
	}

	runtimeBrowser.RequestGracefulClose(time.Millisecond)
	if !runtimeBrowser.WaitStopped(2 * time.Second) {
		t.Fatal("borrowed graceful close did not finish")
	}
	runtimeBrowser.Stop()
	runtimeBrowser.ForceTerminate(time.Millisecond)
	if browserCancels.Load() != 1 || allocatorCancels.Load() != 1 {
		t.Fatalf("cancel counts = browser %d, allocator %d", browserCancels.Load(), allocatorCancels.Load())
	}
	if !runtimeBrowser.Stopped() || runtimeBrowser.Status().Ready {
		t.Fatalf("borrowed runtime did not stop: %#v", runtimeBrowser.Status())
	}
	if runtimeBrowser.cmd != nil || runtimeBrowser.registryID != "" || runtimeBrowser.rootPID != 0 || runtimeBrowser.processGroupID != 0 {
		t.Fatal("borrowed runtime unexpectedly acquired process ownership")
	}
}

func TestBorrowedRuntimeSkipsManagedRoute(t *testing.T) {
	runtimeBrowser := &Runtime{
		ownership: RuntimeOwnershipBorrowed,
		stopped:   make(chan struct{}),
		status:    Status{Ready: true},
	}
	if err := runtimeBrowser.VerifyNetworkRoute(context.Background()); err != nil {
		t.Fatalf("borrowed network route = %v", err)
	}
}

func TestBorrowedPageTargetInScopeRequiresPageInSelectedBrowserContext(t *testing.T) {
	t.Parallel()

	const anchorContextID = cdp.BrowserContextID("regular-profile")
	borrowed := &Runtime{
		ownership:          RuntimeOwnershipBorrowed,
		borrowedContextID:  anchorContextID,
		borrowedContextSet: true,
	}
	tests := []struct {
		name string
		info *targetpkg.Info
		want bool
	}{
		{name: "nil target", info: nil, want: false},
		{
			name: "same context page",
			info: &targetpkg.Info{Type: "page", BrowserContextID: anchorContextID},
			want: true,
		},
		{
			name: "different context page",
			info: &targetpkg.Info{Type: "page", BrowserContextID: "incognito-profile"},
			want: false,
		},
		{
			name: "same context worker",
			info: &targetpkg.Info{Type: "service_worker", BrowserContextID: anchorContextID},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := borrowed.BorrowedPageTargetInScope(test.info); got != test.want {
				t.Fatalf("BorrowedPageTargetInScope(%#v) = %t, want %t", test.info, got, test.want)
			}
		})
	}

	owned := &Runtime{
		ownership:          RuntimeOwnershipOwned,
		borrowedContextID:  anchorContextID,
		borrowedContextSet: true,
	}
	if owned.BorrowedPageTargetInScope(&targetpkg.Info{Type: "page", BrowserContextID: anchorContextID}) {
		t.Fatal("owned runtime accepted a borrowed page target")
	}
}

func TestBorrowedPageTargetInScopeRequiresEstablishedProfileContext(t *testing.T) {
	t.Parallel()

	borrowed := &Runtime{ownership: RuntimeOwnershipBorrowed}
	if borrowed.BorrowedPageTargetInScope(&targetpkg.Info{Type: "page", BrowserContextID: "regular-profile"}) {
		t.Fatal("borrowed runtime accepted a page before establishing its selected profile context")
	}
}

func TestCurrentBrowserProfileContextSelectsExposedPageContext(t *testing.T) {
	t.Parallel()

	targets := []*targetpkg.Info{
		{TargetID: "worker", Type: "service_worker", BrowserContextID: "worker-context"},
		{TargetID: "empty-context", Type: "page"},
		{TargetID: "selected", Type: "page", BrowserContextID: "selected-profile"},
		{TargetID: "selected-two", Type: "page", BrowserContextID: "selected-profile"},
	}
	contextID, ok := currentBrowserProfileContext(targets)
	if !ok || contextID != "selected-profile" {
		t.Fatalf("currentBrowserProfileContext() = %q/%t", contextID, ok)
	}
	if contextID, ok := currentBrowserProfileContext([]*targetpkg.Info{{TargetID: "empty-context", Type: "page"}}); ok || contextID != "" {
		t.Fatalf("empty page context was trusted: %q/%t", contextID, ok)
	}
	if contextID, ok := currentBrowserProfileContext([]*targetpkg.Info{
		{TargetID: "first", Type: "page", BrowserContextID: "first-profile"},
		{TargetID: "second", Type: "page", BrowserContextID: "second-profile"},
	}); ok || contextID != "" {
		t.Fatalf("multiple page contexts were trusted: %q/%t", contextID, ok)
	}
}

func trustedCurrentBrowserTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := secureTrustedCurrentBrowserTestPath(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeCurrentBrowserTestFile(t *testing.T, root string, name string, value string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secureTrustedCurrentBrowserTestPath(path); err != nil {
		t.Fatal(err)
	}
}
