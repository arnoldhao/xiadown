package service

import (
	"context"
	"errors"
	"testing"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/library/dto"
)

func TestResourceSniffLaunchFingerprintSeparatesOwnershipModes(t *testing.T) {
	managed := resourceSniffLaunchFingerprint("managed_profile", "chrome", "/profile", "http://127.0.0.1:1234", true)
	borrowed := resourceSniffLaunchFingerprint("current_browser", "chrome", "/profile", "http://127.0.0.1:1234", true)
	if managed == borrowed {
		t.Fatal("managed and current browser modes shared a launch fingerprint")
	}
}

func TestStartResourceSniffEmptyURLReachesModeRouting(t *testing.T) {
	service := &LibraryService{}
	_, err := service.StartResourceSniff(context.Background(), dto.StartResourceSniffRequest{
		Mode: "unsupported",
	})
	if code := apperrors.CodeOf(err); code != apperrors.CodeResourceBrowserLaunchFailed {
		t.Fatalf("empty URL mode-routing code = %q, want %q (err=%v)", code, apperrors.CodeResourceBrowserLaunchFailed, err)
	}
}

func TestStartResourceSniffStillRejectsNonEmptyInvalidURL(t *testing.T) {
	service := &LibraryService{}
	_, err := service.StartResourceSniff(context.Background(), dto.StartResourceSniffRequest{
		URL:  "file:///etc/passwd",
		Mode: "unsupported",
	})
	if code := apperrors.CodeOf(err); code != apperrors.CodeDownloadURLInvalid {
		t.Fatalf("invalid URL code = %q, want %q (err=%v)", code, apperrors.CodeDownloadURLInvalid, err)
	}
}

func TestCurrentBrowserSessionIsNeverReusable(t *testing.T) {
	session := &resourceSniffSession{
		Mode:    "current_browser",
		Runtime: &browsercdp.Runtime{},
		State:   resourceSniffStateRunning,
	}
	if resourceSniffSessionReusableForLaunch(session, "fingerprint") {
		t.Fatal("current browser mode reused a borrowed session")
	}
}

func TestResourceSniffTargetDiscoveryMatchesModeOwnership(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		ownership browsercdp.RuntimeOwnership
		want      bool
	}{
		{
			name:      "current browser borrows Chrome targets",
			mode:      "current_browser",
			ownership: browsercdp.RuntimeOwnershipBorrowed,
			want:      true,
		},
		{
			name:      "current browser rejects owned runtime",
			mode:      "current_browser",
			ownership: browsercdp.RuntimeOwnershipOwned,
			want:      false,
		},
		{
			name:      "managed profile discovers owned targets",
			mode:      "managed_profile",
			ownership: browsercdp.RuntimeOwnershipOwned,
			want:      true,
		},
		{
			name:      "managed profile rejects borrowed runtime",
			mode:      "managed_profile",
			ownership: browsercdp.RuntimeOwnershipBorrowed,
			want:      false,
		},
		{
			name:      "unknown mode rejects every runtime",
			mode:      "unsupported",
			ownership: browsercdp.RuntimeOwnershipOwned,
			want:      false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resourceSniffTargetDiscoveryAllowed(test.mode, test.ownership); got != test.want {
				t.Fatalf("resourceSniffTargetDiscoveryAllowed(%q, %q) = %t, want %t", test.mode, test.ownership, got, test.want)
			}
		})
	}
}

func TestCurrentBrowserErrorsKeepStableAppCodes(t *testing.T) {
	tests := []struct {
		state string
		code  apperrors.Code
	}{
		{browsercdp.CurrentBrowserStateUnsupportedVersion, apperrors.CodeResourceCurrentBrowserUnsupported},
		{browsercdp.CurrentBrowserStateNotRunning, apperrors.CodeResourceCurrentBrowserNotRunning},
		{browsercdp.CurrentBrowserStateRemoteDebuggingDisabled, apperrors.CodeResourceCurrentBrowserRemoteDebugging},
		{browsercdp.CurrentBrowserStatePermissionDenied, apperrors.CodeResourceCurrentBrowserPermission},
		{browsercdp.CurrentBrowserStateEndpointUnavailable, apperrors.CodeResourceCurrentBrowserEndpointUnavailable},
	}
	for _, test := range tests {
		t.Run(test.state, func(t *testing.T) {
			err := currentBrowserResourceSniffError(&browsercdp.CurrentBrowserError{
				State: test.state,
				Err:   errors.New("test"),
			})
			if code := apperrors.CodeOf(err); code != test.code {
				t.Fatalf("code = %q, want %q (err=%v)", code, test.code, err)
			}
		})
	}
}

func TestNormalizeResourceSniffModeAcceptsCurrentBrowser(t *testing.T) {
	if got := normalizeResourceSniffMode(" current_browser "); got != "current_browser" {
		t.Fatalf("mode = %q", got)
	}
}

func TestMapResourceSniffSessionPreservesBrowserSource(t *testing.T) {
	service := &LibraryService{}
	mapped := service.mapResourceSniffSession(&resourceSniffSession{
		ID:        "sniff-1",
		Mode:      "current_browser",
		BrowserID: "chrome",
		ProfileID: "",
		State:     resourceSniffStateRunning,
	})
	if mapped.Mode != "current_browser" || mapped.BrowserID != "chrome" || mapped.ProfileID != "" {
		t.Fatalf("mapped browser source = mode %q, browser %q, profile %q", mapped.Mode, mapped.BrowserID, mapped.ProfileID)
	}
}

func TestResourceSniffTabMatchesOnlyItsOwnDetachSession(t *testing.T) {
	tab := &resourceSniffTab{
		TargetID:        "target-1",
		TargetSessionID: "session-xiadown",
	}
	if !resourceSniffTabMatchesDetach(tab, "target-1", "session-xiadown") {
		t.Fatal("expected XiaDown's own detach session to match")
	}
	if resourceSniffTabMatchesDetach(tab, "target-1", "session-devtools") {
		t.Fatal("another CDP client's detach session must not match")
	}
	if resourceSniffTabMatchesDetach(tab, "target-2", "session-xiadown") {
		t.Fatal("a different target must not match")
	}
}

func TestRememberResourceSniffTargetSessionDoesNotReplaceOwner(t *testing.T) {
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"sniff-1": {
				Tabs: map[string]*resourceSniffTab{
					"target-1": {
						TargetID:        "target-1",
						TargetSessionID: "session-xiadown",
					},
				},
			},
		},
	}

	service.rememberResourceSniffTargetSessionID("sniff-1", "target-1", "session-devtools")
	if got := service.resourceSniffs["sniff-1"].Tabs["target-1"].TargetSessionID; got != "session-xiadown" {
		t.Fatalf("target session = %q, want XiaDown owner", got)
	}
}

func TestRememberResourceSniffTargetSessionRejectsUnverifiedEmptyOwner(t *testing.T) {
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"sniff-1": {
				Tabs: map[string]*resourceSniffTab{
					"target-1": {TargetID: "target-1"},
				},
			},
		},
	}

	service.rememberResourceSniffTargetSessionID("sniff-1", "target-1", "session-devtools")
	if got := service.resourceSniffs["sniff-1"].Tabs["target-1"].TargetSessionID; got != "" {
		t.Fatalf("unverified target session was accepted: %q", got)
	}
}

func TestHandleResourceSniffTargetDetachedRemovesOnlyMatchingRunningOwner(t *testing.T) {
	tests := []struct {
		name            string
		state           string
		storedSessionID string
		detachedSession string
		wantRemoved     bool
	}{
		{
			name:            "own session detach",
			state:           resourceSniffStateRunning,
			storedSessionID: "session-xiadown",
			detachedSession: "session-xiadown",
			wantRemoved:     true,
		},
		{
			name:            "other client detach",
			state:           resourceSniffStateRunning,
			storedSessionID: "session-xiadown",
			detachedSession: "session-devtools",
		},
		{
			name:            "closing session ignores detach",
			state:           resourceSniffStateClosing,
			storedSessionID: "session-xiadown",
			detachedSession: "session-xiadown",
		},
		{
			name:            "late old detach does not remove replacement",
			state:           resourceSniffStateRunning,
			storedSessionID: "session-new",
			detachedSession: "session-old",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cancelCount := 0
			service := &LibraryService{
				resourceSniffs: map[string]*resourceSniffSession{
					"sniff-1": {
						State: test.state,
						Tabs: map[string]*resourceSniffTab{
							"target-1": {
								TargetID:        "target-1",
								TargetSessionID: test.storedSessionID,
								Cancel: func() {
									cancelCount++
								},
							},
						},
					},
				},
			}

			service.handleResourceSniffTargetDetached(
				"sniff-1",
				"target-1",
				test.detachedSession,
			)
			_, exists := service.resourceSniffs["sniff-1"].Tabs["target-1"]
			removed := !exists
			if removed != test.wantRemoved {
				t.Fatalf("tab removed = %t, want %t", removed, test.wantRemoved)
			}
			wantCancelCount := 0
			if test.wantRemoved {
				wantCancelCount = 1
			}
			if cancelCount != wantCancelCount {
				t.Fatalf("cancel count = %d, want %d", cancelCount, wantCancelCount)
			}
		})
	}
}
