//go:build windows

package browsercdp

import (
	"context"
	"os"
	"testing"
	"time"

	"xiadown/internal/application/youtubecookies"
)

// This opt-in test exercises Chrome's consent-gated current-session bridge
// without printing cookie names or values. It is intentionally skipped in
// ordinary test runs because Chrome must be running and a person must approve
// the connection prompt.
func TestLiveBorrowedCurrentChromeCookieAudit(t *testing.T) {
	if os.Getenv("XIADOWN_LIVE_CURRENT_CHROME_COOKIE_AUDIT") != "1" {
		t.Skip("set XIADOWN_LIVE_CURRENT_CHROME_COOKIE_AUDIT=1 for an approved local audit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	status := InspectCurrentBrowser(ctx, string(BrowserChrome))
	t.Logf("status state=%s running=%t supported=%t ready=%t version=%s", status.State, status.Running, status.Supported, status.Ready, status.Version)
	if !status.Ready {
		t.Fatalf("current Chrome bridge is not ready: state=%s", status.State)
	}

	runtimeBrowser, err := StartBorrowedCurrentBrowser(ctx, string(BrowserChrome))
	if err != nil {
		t.Fatalf("start approved current Chrome: state=%s error=%v", CurrentBrowserErrorState(err), err)
	}
	defer runtimeBrowser.Stop()

	allTargets := 0
	inScopeTargets := 0
	if manager := runtimeBrowser.TargetManager(); manager != nil {
		for _, info := range manager.ListPageTargets() {
			allTargets++
			if runtimeBrowser.BorrowedPageTargetInScope(info) {
				inScopeTargets++
			}
		}
	}
	t.Logf("approved context set=%t context_length=%d page_targets=%d in_scope_targets=%d", runtimeBrowser.borrowedContextSet, len(runtimeBrowser.borrowedContextID), allTargets, inScopeTargets)

	records, err := SnapshotBorrowedCookiesForDomains(ctx, runtimeBrowser, []string{
		"youtube.com",
		"google.com",
	})
	if err != nil {
		t.Fatalf("snapshot approved current Chrome cookies: %v", err)
	}
	hasYouTubeAuth := youtubecookies.HasAuthForURL(records, "https://www.youtube.com/", time.Now())
	t.Logf("allowlisted_records=%d youtube_authenticated=%t", len(records), hasYouTubeAuth)
	if !hasYouTubeAuth {
		t.Fatal("approved current Chrome snapshot did not contain a usable YouTube authentication indicator")
	}
}
