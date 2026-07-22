package browsercdp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeExecutableIdentityFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestDetectCandidatesCachesScanUntilTTLExpires(t *testing.T) {
	originalNow := detectCandidatesNow
	originalScan := detectCandidatesScan
	originalTTL := detectCandidatesCacheTTL
	current := time.Unix(1_700_000_000, 0)
	scanCalls := 0
	nextPath := "/tmp/chrome"

	detectCandidatesNow = func() time.Time { return current }
	detectCandidatesScan = func() []Candidate {
		scanCalls += 1
		return []Candidate{
			{
				ID:        BrowserChrome,
				Label:     "Chrome",
				ExecPath:  nextPath,
				Available: true,
			},
		}
	}
	detectCandidatesCacheTTL = time.Minute
	resetDetectCandidatesCache()
	t.Cleanup(func() {
		detectCandidatesNow = originalNow
		detectCandidatesScan = originalScan
		detectCandidatesCacheTTL = originalTTL
		resetDetectCandidatesCache()
	})

	first := DetectCandidates()
	if scanCalls != 1 {
		t.Fatalf("expected first detect to scan once, got %d", scanCalls)
	}
	first[0].ExecPath = "/tmp/changed"

	second := DetectCandidates()
	if scanCalls != 1 {
		t.Fatalf("expected second detect to use cache, got %d scans", scanCalls)
	}
	if got := second[0].ExecPath; got != "/tmp/chrome" {
		t.Fatalf("expected cached detect result to be cloned, got %q", got)
	}

	nextPath = "/tmp/chrome-refreshed"
	refreshed := RefreshCandidates()
	if scanCalls != 2 {
		t.Fatalf("expected refresh to scan inside ttl, got %d scans", scanCalls)
	}
	if got := refreshed[0].ExecPath; got != "/tmp/chrome-refreshed" {
		t.Fatalf("expected refresh result to replace cache, got %q", got)
	}

	nextPath = "/tmp/chrome-expired"
	current = current.Add(time.Minute + time.Second)
	_ = DetectCandidates()
	if scanCalls != 3 {
		t.Fatalf("expected detect to rescan after ttl, got %d scans", scanCalls)
	}
}

func TestChooseCandidatePrefersChromeWhenNoPreferredBrowser(t *testing.T) {
	t.Parallel()

	candidate, ok := ChooseCandidate([]Candidate{
		{ID: BrowserBrave, Label: "Brave", ExecPath: "/tmp/brave", Available: true},
		{ID: BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
	}, "")
	if !ok {
		t.Fatal("expected a browser candidate")
	}
	if candidate.ID != BrowserChrome {
		t.Fatalf("expected Chrome to be chosen by default, got %q", candidate.ID)
	}
}

func TestChooseCandidateFallsBackAlphabeticallyWithoutChrome(t *testing.T) {
	t.Parallel()

	candidate, ok := ChooseCandidate([]Candidate{
		{ID: BrowserVivaldi, Label: "Vivaldi", ExecPath: "/tmp/vivaldi", Available: true},
		{ID: BrowserBrave, Label: "Brave", ExecPath: "/tmp/brave", Available: true},
		{ID: BrowserEdge, Label: "Edge", ExecPath: "/tmp/edge", Available: true},
	}, "")
	if !ok {
		t.Fatal("expected a browser candidate")
	}
	if candidate.ID != BrowserBrave {
		t.Fatalf("expected alphabetical fallback to choose Brave, got %q", candidate.ID)
	}
}

func TestChooseCandidateUsesPreferredBrowserWhenAvailable(t *testing.T) {
	t.Parallel()

	candidate, ok := ChooseCandidate([]Candidate{
		{ID: BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
		{ID: BrowserEdge, Label: "Edge", ExecPath: "/tmp/edge", Available: true},
	}, "edge")
	if !ok {
		t.Fatal("expected a browser candidate")
	}
	if candidate.ID != BrowserEdge {
		t.Fatalf("expected preferred browser to be chosen, got %q", candidate.ID)
	}
}

func TestChooseCandidateDoesNotFallbackWhenPreferredBrowserIsUnavailable(t *testing.T) {
	t.Parallel()

	if candidate, ok := ChooseCandidate([]Candidate{
		{ID: BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
		{ID: BrowserEdge, Label: "Edge", Available: false, Error: "browser executable not found"},
	}, "edge"); ok || candidate != (Candidate{}) {
		t.Fatalf("unavailable explicit Edge selection fell back to %#v", candidate)
	}
}

func TestChooseCandidateRejectsSafariInsteadOfFallingBackToChromium(t *testing.T) {
	t.Parallel()

	if candidate, ok := ChooseCandidate([]Candidate{
		{ID: BrowserChrome, Label: "Chrome", ExecPath: "/tmp/chrome", Available: true},
	}, "safari"); ok || candidate != (Candidate{}) {
		t.Fatalf("unsupported explicit Safari selection fell back to %#v", candidate)
	}
}

func TestExecutableIdentityKeepsStableAndBetaExecutablesSeparate(t *testing.T) {
	root := t.TempDir()
	stablePath := filepath.Join(root, "Google Chrome")
	betaPath := filepath.Join(root, "Google Chrome Beta")
	writeExecutableIdentityFixture(t, stablePath)
	writeExecutableIdentityFixture(t, betaPath)
	paths := []string{stablePath, betaPath}

	stableIdentity := detectExecutableIdentity(BrowserChrome, "", paths)
	betaIdentity := detectExecutableIdentity(BrowserChrome, "Beta", paths)
	coarseCandidates := []Candidate{{
		ID:        BrowserChrome,
		Label:     "Chrome",
		ExecPath:  stablePath,
		Available: true,
	}}
	betaCandidate, err := resolveLaunchCandidate(LaunchOptions{
		PreferredBrowser:   "chrome",
		ExecutableIdentity: betaIdentity,
	}, coarseCandidates)
	if err != nil {
		t.Fatal(err)
	}
	if betaCandidate.ExecPath != betaPath {
		t.Fatalf("Beta identity selected %q instead of %q", betaCandidate.ExecPath, betaPath)
	}
	stableCandidate, err := resolveLaunchCandidate(LaunchOptions{
		PreferredBrowser:   "chrome",
		ExecutableIdentity: stableIdentity,
	}, coarseCandidates)
	if err != nil {
		t.Fatal(err)
	}
	if stableCandidate.ExecPath != stablePath {
		t.Fatalf("Stable identity selected %q instead of %q", stableCandidate.ExecPath, stablePath)
	}

	payload, err := json.Marshal(betaIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), betaPath) || string(payload) != "{}" {
		t.Fatalf("opaque executable identity leaked its path: %s", payload)
	}
}

func TestExactExecutableDisappearanceDoesNotFallbackToStable(t *testing.T) {
	root := t.TempDir()
	stablePath := filepath.Join(root, "Google Chrome")
	betaPath := filepath.Join(root, "Google Chrome Beta")
	writeExecutableIdentityFixture(t, stablePath)
	writeExecutableIdentityFixture(t, betaPath)
	betaIdentity := detectExecutableIdentity(
		BrowserChrome,
		"Beta",
		[]string{stablePath, betaPath},
	)
	if err := os.Remove(betaPath); err != nil {
		t.Fatal(err)
	}

	runtime, err := StartLoopbackOnly(context.Background(), LaunchOptions{
		PreferredBrowser:   "chrome",
		ExecutableIdentity: betaIdentity,
	})
	if runtime != nil {
		runtime.Stop()
		t.Fatal("disappeared Beta executable unexpectedly started a runtime")
	}
	if !errors.Is(err, ErrExactExecutableUnavailable) {
		t.Fatalf("expected exact executable failure, got %v", err)
	}
}

func TestStartReturnsNoSupportedBrowserErrorWhenNoCandidateAvailable(t *testing.T) {
	originalScan := detectCandidatesScan
	detectCandidatesScan = func() []Candidate {
		return []Candidate{
			{
				ID:        BrowserChrome,
				Label:     "Chrome",
				Available: false,
				Error:     "browser executable not found",
			},
		}
	}
	resetDetectCandidatesCache()
	t.Cleanup(func() {
		detectCandidatesScan = originalScan
		resetDetectCandidatesCache()
	})

	runtime, err := StartLoopbackOnly(context.Background(), LaunchOptions{})
	if runtime != nil {
		t.Fatal("expected no runtime to be started")
	}
	if !errors.Is(err, ErrNoSupportedBrowser) {
		t.Fatalf("expected ErrNoSupportedBrowser, got %v", err)
	}
}

func TestStartDoesNotLaunchDefaultBrowserForUnavailableExplicitSelection(t *testing.T) {
	originalScan := detectCandidatesScan
	detectCandidatesScan = func() []Candidate {
		return []Candidate{
			{ID: BrowserChrome, Label: "Chrome", ExecPath: "/path/that/must/not/be/launched", Available: true},
			{ID: BrowserEdge, Label: "Edge", Available: false, Error: "browser executable not found"},
		}
	}
	resetDetectCandidatesCache()
	t.Cleanup(func() {
		detectCandidatesScan = originalScan
		resetDetectCandidatesCache()
	})

	runtime, err := StartLoopbackOnly(context.Background(), LaunchOptions{PreferredBrowser: "edge"})
	if runtime != nil {
		t.Fatal("expected no runtime to be started")
	}
	if !errors.Is(err, ErrNoSupportedBrowser) {
		t.Fatalf("expected ErrNoSupportedBrowser, got %v", err)
	}
}

func TestWaitForCDPHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WaitForCDP(ctx, "127.0.0.1", 1, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestCheckCDPReadyRejectsNonLoopbackHost(t *testing.T) {
	t.Parallel()

	if err := CheckCDPReady(context.Background(), "example.com", 9222); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback CDP host error = %v", err)
	}
}
