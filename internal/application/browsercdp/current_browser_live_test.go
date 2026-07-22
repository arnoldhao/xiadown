package browsercdp

import (
	"context"
	"os"
	"testing"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func TestInspectCurrentChromeLive(t *testing.T) {
	if os.Getenv("XIADOWN_CURRENT_CHROME_LIVE_TEST") != "1" {
		t.Skip("set XIADOWN_CURRENT_CHROME_LIVE_TEST=1 to inspect the local Chrome bridge")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	status := InspectCurrentBrowser(ctx, string(BrowserChrome))
	t.Logf("current Chrome status: state=%s installed=%t running=%t supported=%t ready=%t version=%s profile=%s",
		status.State,
		status.Installed,
		status.Running,
		status.Supported,
		status.Ready,
		status.Version,
		status.ProfileName,
	)
	if status.BrowserID != string(BrowserChrome) || status.State == "" {
		t.Fatalf("invalid current Chrome status: %#v", status)
	}
}

func TestStartBorrowedCurrentChromeLiveKeepsBrowserConnectionAlive(t *testing.T) {
	if os.Getenv("XIADOWN_CURRENT_CHROME_LIVE_TEST") != "1" {
		t.Skip("set XIADOWN_CURRENT_CHROME_LIVE_TEST=1 to attach to the local Chrome bridge")
	}
	runtimeBrowser, err := StartBorrowedCurrentBrowser(context.Background(), string(BrowserChrome))
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeBrowser.Stop()

	browserCtx := runtimeBrowser.BrowserContext()
	if browserCtx == nil {
		t.Fatal("borrowed Chrome returned no browser context")
	}
	if runtimeBrowser.TargetManager() == nil {
		t.Fatal("borrowed Chrome returned no page target manager")
	}

	// Regresses the former temporary first-Run context: Start returned, its
	// deferred cancel closed the websocket immediately, and the browser-level
	// connection was already done before target discovery could run.
	time.Sleep(250 * time.Millisecond)
	select {
	case <-browserCtx.Done():
		t.Fatalf("borrowed browser connection ended after Start returned: %v", browserCtx.Err())
	default:
	}

	commandCtx, cancel, err := RuntimeBrowserExecutorContext(runtimeBrowser, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := targetpkg.GetTargets().Do(commandCtx)
	cancel()
	if err != nil {
		t.Fatalf("list targets after Start returned: %v", err)
	}
	if len(targets) == 0 || !runtimeBrowser.Status().Ready {
		t.Fatalf("borrowed browser targets/status = %d/%#v", len(targets), runtimeBrowser.Status())
	}
	if info := currentChromeLiveScopedPageTarget(t, runtimeBrowser); info == nil {
		t.Fatal("borrowed target manager exposed no page in the selected profile")
	}
}

func TestAttachBorrowedCurrentChromeLiveCancelLeavesTargetOpen(t *testing.T) {
	if os.Getenv("XIADOWN_CURRENT_CHROME_LIVE_TEST") != "1" {
		t.Skip("set XIADOWN_CURRENT_CHROME_LIVE_TEST=1 to attach to the local Chrome bridge")
	}
	runtimeBrowser, err := StartBorrowedCurrentBrowser(context.Background(), string(BrowserChrome))
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeBrowser.Stop()

	infoBefore := currentChromeLiveScopedPageTarget(t, runtimeBrowser)
	targetID := string(infoBefore.TargetID)

	attachedCtx, detachOnlyCancel, attachedTargetID, err := AttachBorrowedPageTarget(runtimeBrowser, targetID, 3*time.Second)
	if err != nil {
		t.Fatalf("attach borrowed target: %v", err)
	}
	if attachedTargetID != targetID {
		detachOnlyCancel()
		t.Fatalf("attached target = %q, want %q", attachedTargetID, targetID)
	}
	var href string
	if err := chromedp.Run(attachedCtx, chromedp.Evaluate("location.href", &href)); err != nil {
		detachOnlyCancel()
		t.Fatalf("run command through borrowed target attachment: %v", err)
	}
	detachOnlyCancel()
	select {
	case <-attachedCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("detach-only cancel did not end the borrowed attachment context")
	}

	// The borrowed session is gone, but the user-owned target and browser-level
	// connection must both remain alive. A normal chromedp cancel would send
	// Target.closeTarget and this lookup would fail.
	info := currentChromeLiveTargetInfo(t, runtimeBrowser, targetID)
	if string(info.TargetID) != targetID {
		t.Fatalf("target after detach = %q, want %q", info.TargetID, targetID)
	}
	select {
	case <-runtimeBrowser.BrowserContext().Done():
		t.Fatalf("detach-only cancel ended the browser context: %v", runtimeBrowser.BrowserContext().Err())
	default:
	}
	if !runtimeBrowser.Status().Ready {
		t.Fatalf("borrowed runtime stopped after sibling detach: %#v", runtimeBrowser.Status())
	}
}

func currentChromeLiveScopedPageTarget(t *testing.T, runtimeBrowser *Runtime) *targetpkg.Info {
	t.Helper()
	manager := runtimeBrowser.TargetManager()
	if manager == nil {
		t.Fatal("borrowed Chrome target manager is unavailable")
	}
	for _, info := range manager.ListPageTargets() {
		if runtimeBrowser.BorrowedPageTargetInScope(info) {
			return info
		}
	}
	t.Fatal("borrowed Chrome exposed no page in the selected profile")
	return nil
}

func currentChromeLiveTargetInfo(t *testing.T, runtimeBrowser *Runtime, targetID string) *targetpkg.Info {
	t.Helper()
	ctx, cancel, err := RuntimeBrowserExecutorContext(runtimeBrowser, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	info, err := targetpkg.GetTargetInfo().WithTargetID(targetpkg.ID(targetID)).Do(ctx)
	if err != nil {
		t.Fatalf("get borrowed target info: %v", err)
	}
	if info == nil {
		t.Fatal("Chrome returned no borrowed target info")
	}
	return info
}
