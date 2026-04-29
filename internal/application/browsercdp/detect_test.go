package browsercdp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDetectCandidatesCachesScanUntilTTLExpires(t *testing.T) {
	t.Parallel()

	originalNow := detectCandidatesNow
	originalScan := detectCandidatesScan
	originalTTL := detectCandidatesCacheTTL
	current := time.Unix(1_700_000_000, 0)
	scanCalls := 0

	detectCandidatesNow = func() time.Time { return current }
	detectCandidatesScan = func() []Candidate {
		scanCalls += 1
		return []Candidate{
			{
				ID:        BrowserChrome,
				Label:     "Chrome",
				ExecPath:  "/tmp/chrome",
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

	current = current.Add(time.Minute + time.Second)
	_ = DetectCandidates()
	if scanCalls != 2 {
		t.Fatalf("expected detect to rescan after ttl, got %d scans", scanCalls)
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
