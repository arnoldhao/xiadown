package browsercdp

import (
	"testing"

	targetpkg "github.com/chromedp/cdproto/target"
)

func TestManagedNetworkProbeTargetRequestBootstrapsWithSafeOrdinaryTarget(t *testing.T) {
	t.Parallel()

	request := newManagedNetworkProbeTargetRequest(nil)
	if !isManagedNetworkProbeURL(request.URL) {
		t.Fatalf("bootstrap probe URL = %q, want App-owned data URL", request.URL)
	}
	if !request.Background {
		t.Fatal("bootstrap probe must remain in the background")
	}
	if request.Hidden {
		t.Fatal("first probe target cannot be hidden before a page target exists")
	}

	params := request.protocolParams()
	if got := params["url"]; got != request.URL {
		t.Fatalf("protocol URL = %#v, want %q", got, request.URL)
	}
	if got := params["background"]; got != true {
		t.Fatalf("protocol background = %#v, want true", got)
	}
	for _, forbidden := range []string{"hidden", "newWindow", "forTab"} {
		if _, exists := params[forbidden]; exists {
			t.Fatalf("ordinary bootstrap target unexpectedly sets %q", forbidden)
		}
	}
}

func TestManagedNetworkProbeTargetRequestUsesHiddenTargetWhenPageExists(t *testing.T) {
	t.Parallel()

	targets := []*targetpkg.Info{
		nil,
		{Type: "service_worker", URL: "https://example.invalid/worker.js"},
		{Type: " PAGE ", URL: "https://example.invalid/"},
	}
	request := newManagedNetworkProbeTargetRequest(targets)
	if !request.Hidden {
		t.Fatal("probe should be hidden once a page target exists")
	}
	if !isManagedNetworkProbeURL(request.URL) {
		t.Fatalf("hidden probe URL = %q, want App-owned data URL", request.URL)
	}
	if got := request.protocolParams()["hidden"]; got != true {
		t.Fatalf("protocol hidden = %#v, want true", got)
	}
}

func TestManagedNetworkProbeTargetRequestNeverDerivesURLFromExistingTargets(t *testing.T) {
	t.Parallel()

	const externalURL = "https://attacker.example/probe"
	request := newManagedNetworkProbeTargetRequest([]*targetpkg.Info{{
		Type: "page",
		URL:  externalURL,
	}})
	if request.URL == externalURL || !isManagedNetworkProbeURL(request.URL) {
		t.Fatalf("probe URL = %q, must not derive from existing target %q", request.URL, externalURL)
	}
}
