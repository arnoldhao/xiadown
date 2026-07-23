package service

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"

	"xiadown/internal/application/library/dto"
)

const resourceCookieCanary = "xiadown_cookie_canary=late-extra-info"

func TestResourceCaptureBackfillsCookieWhenExtraInfoArrivesBeforeRequest(t *testing.T) {
	state := newResourceCaptureState()
	defer state.clear()
	requestID := network.RequestID("extra-before-request")
	mediaURL := "https://media.example/video.mp4"

	state.recordRequestHeaders(requestID, network.Headers{"cookie": resourceCookieCanary})
	state.recordRequest(requestID, mediaURL, "https://example/", network.Headers{"User-Agent": "XiaDown-Test"})
	state.recordResponse(
		requestID,
		mediaURL,
		200,
		"video/mp4",
		network.Headers{"Content-Type": "video/mp4", "Content-Length": "1048576"},
		network.ResourceTypeMedia,
		true,
	)

	assertObservedCookieCanary(t, state, mediaURL)
}

func TestResourceCaptureBackfillsAuthoritativeCookieAfterResponseAndFinish(t *testing.T) {
	state := newResourceCaptureState()
	defer state.clear()
	requestID := network.RequestID("extra-after-response")
	mediaURL := "https://media.example/video.mp4?token=one"

	state.recordRequest(requestID, mediaURL, "https://example/", network.Headers{"Cookie": "stale_cookie=old"})
	state.recordResponse(
		requestID,
		mediaURL,
		200,
		"video/mp4",
		network.Headers{"Content-Type": "video/mp4", "Content-Length": "1048576"},
		network.ResourceTypeMedia,
		true,
	)
	state.markRequestFinished(requestID)
	state.recordRequestHeaders(requestID, network.Headers{"cookie": resourceCookieCanary})

	assertObservedCookieCanary(t, state, mediaURL)
}

func TestResourceCaptureKeepsRedirectCookieStagesSeparate(t *testing.T) {
	state := newResourceCaptureState()
	defer state.clear()
	requestID := network.RequestID("redirect-chain")
	redirectURL := "https://media.example/redirect"
	mediaURL := "https://cdn.example/final.mp4"

	state.recordRequest(requestID, redirectURL, "https://example/", network.Headers{"Cookie": "redirect_cookie=old"})
	state.recordResponse(
		requestID,
		redirectURL,
		302,
		"text/html",
		network.Headers{"Location": mediaURL},
		network.ResourceTypeMedia,
		false,
	)
	// CDP can queue the next hop's request extra-info before the corresponding
	// request/response pair has been fully synchronized.
	state.recordRequestHeaders(requestID, network.Headers{"cookie": resourceCookieCanary})
	state.recordRequest(requestID, mediaURL, "https://example/", network.Headers{"User-Agent": "XiaDown-Test"})
	state.recordResponse(
		requestID,
		mediaURL,
		200,
		"video/mp4",
		network.Headers{"Content-Type": "video/mp4", "Content-Length": "1048576"},
		network.ResourceTypeMedia,
		true,
	)

	assertObservedCookieCanary(t, state, mediaURL)
	for _, observed := range state.observedSnapshot() {
		if resourceComparableURL(observed.url, false) != resourceComparableURL(redirectURL, false) {
			continue
		}
		if value, ok := findHeader(observed.headers, "Cookie"); !ok || value != "redirect_cookie=old" {
			t.Fatalf("redirect hop Cookie changed: %#v", observed.headers)
		}
	}
}

func TestResourceCapturePrunesFinishedRequestChainAfterLateExtraGrace(t *testing.T) {
	state := newResourceCaptureState()
	defer state.clear()
	requestID := network.RequestID("finished-request")
	mediaURL := "https://media.example/video.mp4"
	state.recordRequest(requestID, mediaURL, "https://example/", nil)
	state.recordResponse(requestID, mediaURL, 200, "video/mp4", nil, network.ResourceTypeMedia, true)
	state.markRequestFinished(requestID)

	state.mu.Lock()
	state.requestChains[requestID].finishedAt = time.Now().Add(-resourceSniffRequestLateExtraGrace - time.Millisecond)
	state.pruneRequestChainsLocked(time.Now())
	_, chainExists := state.requestChains[requestID]
	_, requestExists := state.requests[requestID]
	state.mu.Unlock()
	if chainExists || requestExists {
		t.Fatalf("finished request retained after grace: chain=%v request=%v", chainExists, requestExists)
	}
}

func TestMergeHeadersTreatsCookieNamesCaseInsensitively(t *testing.T) {
	merged := mergeHeaders(
		map[string]string{"Cookie": "stale_cookie=old", "User-Agent": "XiaDown-Test"},
		map[string]string{"cookie": resourceCookieCanary},
	)
	if value, ok := findHeader(merged, "Cookie"); !ok || value != resourceCookieCanary {
		t.Fatalf("authoritative Cookie missing: %#v", merged)
	}
	count := 0
	for key := range merged {
		if strings.EqualFold(key, "Cookie") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one case-insensitive Cookie header, got %d in %#v", count, merged)
	}
}

func TestResourceMediaPrepareSnapshotIsSessionOwnedAndClaimedOneShot(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	session := &resourceSniffSession{ID: "session-1"}
	service := &LibraryService{
		resourceSniffs:    map[string]*resourceSniffSession{session.ID: session},
		resourceMedia:     make(map[string]resourceMedia),
		resourceMediaMeta: make(map[string]resourceMediaSnapshotMetadata),
		nowFunc:           func() time.Time { return now },
	}
	media := resourceMedia{
		URL:            "https://media.example/video.mp4",
		RequestHeaders: map[string]string{"Cookie": resourceCookieCanary},
	}
	originalID := service.putResourceMediaSnapshotForSession(media, session.ID)
	if originalID == "" || len(session.LastMediaIDs) != 1 || session.LastMediaIDs[0] != originalID {
		t.Fatalf("prepare snapshot was not attached to session: id=%q ids=%#v", originalID, session.LastMediaIDs)
	}

	claimedRequest, claimedID, err := service.claimResourceMediaForQueuedOperation(dto.CreateYTDLPJobRequest{
		ResourceSessionID: session.ID,
		ResourceMediaID:   originalID,
		FormatID:          originalID,
	})
	if err != nil {
		t.Fatalf("claim media snapshot: %v", err)
	}
	if claimedID == "" || claimedID == originalID || claimedRequest.ResourceMediaID != claimedID || claimedRequest.FormatID != claimedID {
		t.Fatalf("unexpected claim: original=%q claimed=%q request=%#v", originalID, claimedID, claimedRequest)
	}
	if _, ok := service.peekResourceMediaSnapshot(originalID); ok {
		t.Fatal("prepare snapshot remained readable after one-shot claim")
	}
	if len(session.LastMediaIDs) != 0 {
		t.Fatalf("claimed prepare snapshot remained session-owned: %#v", session.LastMediaIDs)
	}
	claimed, ok := service.peekResourceMediaSnapshot(claimedID)
	if !ok {
		t.Fatal("claimed operation snapshot is unavailable")
	}
	if value, ok := findHeader(claimed.RequestHeaders, "Cookie"); !ok || value != resourceCookieCanary {
		t.Fatalf("claimed snapshot lost Cookie: %#v", claimed.RequestHeaders)
	}
}

func TestResourceMediaPrepareSnapshotExpiresAndLeavesSessionCleanupList(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	session := &resourceSniffSession{ID: "session-1"}
	service := &LibraryService{
		resourceSniffs:    map[string]*resourceSniffSession{session.ID: session},
		resourceMedia:     make(map[string]resourceMedia),
		resourceMediaMeta: make(map[string]resourceMediaSnapshotMetadata),
		nowFunc:           func() time.Time { return now },
	}
	mediaID := service.putResourceMediaSnapshotForSession(resourceMedia{URL: "https://media.example/video.mp4"}, session.ID)
	now = now.Add(resourceMediaSnapshotTTL + time.Second)
	if _, ok := service.peekResourceMediaSnapshot(mediaID); ok {
		t.Fatal("expired prepare snapshot remained readable")
	}
	if len(session.LastMediaIDs) != 0 {
		t.Fatalf("expired snapshot remained in session cleanup list: %#v", session.LastMediaIDs)
	}
}

func assertObservedCookieCanary(t *testing.T, state *resourceCaptureState, rawURL string) {
	t.Helper()
	for _, observed := range state.observedSnapshot() {
		if resourceComparableURL(observed.url, false) != resourceComparableURL(rawURL, false) {
			continue
		}
		value, ok := findHeader(observed.headers, "Cookie")
		if !ok || value != resourceCookieCanary {
			t.Fatalf("Cookie canary missing from %q: %#v", rawURL, observed.headers)
		}
		count := 0
		for key := range observed.headers {
			if strings.EqualFold(key, "Cookie") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected one Cookie header for %q, got %d in %#v", rawURL, count, observed.headers)
		}
		return
	}
	t.Fatalf("observed resource not found: %q", rawURL)
}
