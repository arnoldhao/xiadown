package browsercdp

import (
	"context"
	"testing"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func TestBorrowedTargetDetachOnlyCancelClearsCloseIdentityBeforeCancel(t *testing.T) {
	t.Parallel()

	tabCtx, cleanup := chromedp.NewContext(context.Background())
	defer cleanup()
	chromeCtx := chromedp.FromContext(tabCtx)
	if chromeCtx == nil {
		t.Fatal("chromedp context is unavailable")
	}
	chromeCtx.Target = &chromedp.Target{
		TargetID:  "user-owned-target",
		SessionID: "borrowed-session",
	}

	cancelCalls := 0
	var targetIDAtCancel targetpkg.ID
	var sessionIDAtCancel targetpkg.SessionID
	detachOnlyCancel := borrowedTargetDetachOnlyCancel(tabCtx, func() {
		cancelCalls++
		targetIDAtCancel = chromeCtx.Target.TargetID
		sessionIDAtCancel = chromeCtx.Target.SessionID
	})
	detachOnlyCancel()
	detachOnlyCancel()

	if cancelCalls != 1 {
		t.Fatalf("raw cancel calls = %d, want 1", cancelCalls)
	}
	if targetIDAtCancel != "" || chromeCtx.Target.TargetID != "" {
		t.Fatalf("target close identity survived detach-only cancel: atCancel=%q current=%q", targetIDAtCancel, chromeCtx.Target.TargetID)
	}
	if sessionIDAtCancel != "borrowed-session" || chromeCtx.Target.SessionID != "borrowed-session" {
		t.Fatalf("detach session was cleared: atCancel=%q current=%q", sessionIDAtCancel, chromeCtx.Target.SessionID)
	}
}

func TestPickReusableTargetID_FallsBackToAttachedBlankPage(t *testing.T) {
	t.Parallel()

	infos := []*targetpkg.Info{
		{
			TargetID: "attached-blank",
			Type:     "page",
			URL:      "about:blank",
			Attached: true,
		},
	}

	if got := pickReusableTargetID(infos); got != "attached-blank" {
		t.Fatalf("expected attached blank target to be reusable fallback, got %q", got)
	}
}

func TestPickReusableTargetID_IgnoresVivaldiWelcomePage(t *testing.T) {
	t.Parallel()

	infos := []*targetpkg.Info{
		{
			TargetID: "vivaldi-welcome",
			Type:     "page",
			URL:      "chrome-extension://mpognobbkildjkofajifpdfhcoklimli/components/welcome/welcome.html",
		},
	}

	if got := pickReusableTargetID(infos); got != "" {
		t.Fatalf("expected Vivaldi welcome target to be ignored, got %q", got)
	}
}

func TestManagedNetworkProbeIsNotAReusableOrVisiblePageTarget(t *testing.T) {
	t.Parallel()

	probeURL := newManagedNetworkProbePageURL()
	info := &targetpkg.Info{
		TargetID: "probe-target",
		Type:     "page",
		URL:      probeURL,
	}
	if isPageTargetInfo(info) {
		t.Fatal("managed network probe must not be exposed as a page target")
	}
	if isReusablePageTargetInfo(info) {
		t.Fatal("managed network probe must not be reusable")
	}
	if hasPageTargetInfo([]*targetpkg.Info{info}) {
		t.Fatal("managed network probe must not count as a user page")
	}
	if got := pickReusableTargetID([]*targetpkg.Info{info}); got != "" {
		t.Fatalf("managed network probe was selected as %q", got)
	}
}

func TestWaitForReusablePageTarget_ReturnsEmptyForNonReusablePage(t *testing.T) {
	t.Parallel()

	manager := &PageTargetManager{
		targets: map[string]*targetpkg.Info{
			"real-page": {
				TargetID: "real-page",
				Type:     "page",
				URL:      "https://example.test/",
			},
		},
	}
	runtime := &Runtime{targetManager: manager}

	got, err := waitForReusablePageTarget(runtime, time.Second)
	if err != nil {
		t.Fatalf("waitForReusablePageTarget() error = %v", err)
	}
	if got != "" {
		t.Fatalf("expected non-reusable page to force a new target, got %q", got)
	}
}

func TestPickReusableTargetID_PrefersUnattachedBlankPage(t *testing.T) {
	t.Parallel()

	infos := []*targetpkg.Info{
		{
			TargetID: "attached-blank",
			Type:     "page",
			URL:      "about:blank",
			Attached: true,
		},
		{
			TargetID: "unattached-blank",
			Type:     "page",
			URL:      "about:blank",
			Attached: false,
		},
	}

	if got := pickReusableTargetID(infos); got != "unattached-blank" {
		t.Fatalf("expected unattached blank target first, got %q", got)
	}
}
