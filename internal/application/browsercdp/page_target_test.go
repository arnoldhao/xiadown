package browsercdp

import (
	"testing"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
)

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
