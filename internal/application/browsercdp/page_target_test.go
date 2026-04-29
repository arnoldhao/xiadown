package browsercdp

import (
	"testing"

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
