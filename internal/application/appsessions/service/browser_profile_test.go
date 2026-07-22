package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

type countingBrowserProfileReader struct {
	mu      sync.Mutex
	calls   int
	records []appcookies.Record
	err     error
}

func (reader *countingBrowserProfileReader) ReadBrowserProfileCookies(
	context.Context,
	string,
	string,
	[]string,
) ([]appcookies.Record, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	reader.calls++
	return append([]appcookies.Record(nil), reader.records...), reader.err
}

func (reader *countingBrowserProfileReader) ReadCurrentBrowserCookies(
	ctx context.Context,
	browserID string,
	domains []string,
) ([]appcookies.Record, error) {
	return reader.ReadBrowserProfileCookies(ctx, browserID, currentBrowserProfileID, domains)
}

func TestBrowserProfileScanDistinguishesProtectedCookiesFromSignedOutProfile(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-youtube",
		SiteKey:   "youtube",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	for _, testCase := range []struct {
		name       string
		readerErr  error
		wantReason string
	}{
		{name: "current browser consent required", readerErr: appsessions.ErrBrowserCookieAccessRequired, wantReason: browserScanReasonAccessRequired},
		{name: "protected browser unsupported", readerErr: appsessions.ErrBrowserCookieProtected, wantReason: browserScanReasonProtected},
		{name: "actually signed out", readerErr: appsessions.ErrNoCookies, wantReason: browserScanReasonNoAuth},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			reader := &countingBrowserProfileReader{err: testCase.readerErr}
			service := NewAppSessionsService(
				newAppSessionRepoStub(current),
				WithBrowserProfileReader(reader),
			)
			scan, scanErr := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
				Mode: browserProfileMode, BrowserID: "chrome", ProfileID: "profile-opaque",
			})
			if scanErr != nil {
				t.Fatalf("scan browser profile: %v", scanErr)
			}
			if !validBrowserScanSnapshotToken(scan.SnapshotToken) {
				t.Fatalf("scan returned invalid snapshot token %q", scan.SnapshotToken)
			}
			if len(scan.Items) == 0 {
				t.Fatal("scan returned no App Session items")
			}
			for _, item := range scan.Items {
				if item.Status != browserScanStatusMissing || item.Selectable || item.Reason != testCase.wantReason {
					t.Fatalf("scan item = %#v, want unavailable reason %q", item, testCase.wantReason)
				}
			}
		})
	}
}

func (reader *countingBrowserProfileReader) callCount() int {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	return reader.calls
}

func TestNormalizeBrowserProfileSelectionSupportsOnlyAdaptedBrowsers(t *testing.T) {
	t.Parallel()

	supported := []string{"chrome", "edge", "brave", "arc", "vivaldi", "opera"}
	if runtime.GOOS == "darwin" {
		supported = append(supported, "safari")
	}
	for _, browserID := range supported {
		browserID := browserID
		t.Run(browserID, func(t *testing.T) {
			t.Parallel()
			gotBrowser, gotProfile, err := normalizeBrowserProfileSelection(
				browserProfileMode,
				"  "+browserID+"  ",
				"profile-opaque-id",
			)
			if err != nil {
				t.Fatalf("expected %s to be supported: %v", browserID, err)
			}
			if gotBrowser != browserID || gotProfile != "profile-opaque-id" {
				t.Fatalf("unexpected normalized selection: browser=%q profile=%q", gotBrowser, gotProfile)
			}
		})
	}

	unsupported := []string{
		"chromium",
		"opera-gx",
		"yandex",
		"helium",
		"firefox",
		"unknown-browser",
	}
	if runtime.GOOS != "darwin" {
		unsupported = append(unsupported, "safari")
	}
	for _, browserID := range unsupported {
		browserID := browserID
		t.Run("reject_"+browserID, func(t *testing.T) {
			t.Parallel()
			_, _, err := normalizeBrowserProfileSelection(
				browserProfileMode,
				browserID,
				"profile-opaque-id",
			)
			if !errors.Is(err, appsessions.ErrUnsupported) {
				t.Fatalf("expected %s to be unsupported, got %v", browserID, err)
			}
		})
	}
}

func TestAppSessionBrowserCookieDomainsReturnsNormalizedCopy(t *testing.T) {
	domains := AppSessionBrowserCookieDomains()
	if len(domains) == 0 ||
		!slices.Contains(domains, "youtube.com") ||
		!slices.Contains(domains, "google.com") ||
		!slices.Contains(domains, "douyin.com") ||
		!slices.Contains(domains, "iesdouyin.com") ||
		!slices.Contains(domains, "xiaohongshu.com") {
		t.Fatalf("App Session browser cookie domains = %#v", domains)
	}
	for index, domain := range domains {
		if domain != strings.ToLower(strings.Trim(strings.TrimSpace(domain), ".")) {
			t.Fatalf("domain %d is not normalized: %q", index, domain)
		}
		if index > 0 && domains[index-1] >= domain {
			t.Fatalf("domains are not strictly sorted: %#v", domains)
		}
	}
	domains[0] = "mutated.invalid"
	if slices.Contains(AppSessionBrowserCookieDomains(), "mutated.invalid") {
		t.Fatal("App Session browser cookie domain result aliases internal state")
	}
}

func TestNormalizeBrowserProfileSelectionSupportsExplicitCurrentChrome(t *testing.T) {
	browserID, profileID, err := normalizeBrowserProfileSelection(currentBrowserMode, " Chrome ", "")
	if err != nil {
		t.Fatalf("normalize current Chrome: %v", err)
	}
	if browserID != "chrome" || profileID != currentBrowserProfileID {
		t.Fatalf("current Chrome selection = %q/%q", browserID, profileID)
	}
	if got := importedBrowserSourceProfile(currentBrowserMode, profileID); got != "" {
		t.Fatalf("current Chrome persisted internal profile marker %q", got)
	}
	for _, selection := range []dto.BrowserProfileSelection{
		{Mode: currentBrowserMode, BrowserID: "edge"},
		{Mode: currentBrowserMode, BrowserID: "chrome", ProfileID: "profile-forged"},
	} {
		if _, _, err := normalizeBrowserProfileSelection(selection.Mode, selection.BrowserID, selection.ProfileID); !errors.Is(err, appsessions.ErrUnsupported) {
			t.Fatalf("invalid current-browser selection %#v error = %v", selection, err)
		}
	}
}

func TestCurrentChromeScanUsesBorrowedReaderWithoutDiskProfileID(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID: "site-app-session-youtube", SiteKey: "youtube",
		Status: string(appsessions.StatusDisconnected), CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := &countingBrowserProfileReader{records: []appcookies.Record{{
		Name: "SAPISID", Value: "live-secret", Domain: ".youtube.com", Path: "/",
	}}}
	service := NewAppSessionsService(newAppSessionRepoStub(current), WithBrowserProfileReader(reader))
	scan, err := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
		Mode: currentBrowserMode, BrowserID: "chrome",
	})
	if err != nil {
		t.Fatalf("scan current Chrome: %v", err)
	}
	if scan.ProfileID != "" || reader.callCount() != 1 {
		t.Fatalf("current Chrome scan = profile %q calls %d", scan.ProfileID, reader.callCount())
	}
	found := false
	for _, item := range scan.Items {
		if item.SiteKey == "youtube" {
			found = item.Selectable && item.Status == browserScanStatusNew && item.Reason == ""
		}
	}
	if !found {
		t.Fatalf("current Chrome scan did not expose selectable YouTube: %#v", scan.Items)
	}
}

func TestCurrentChromeImportConsumesSnapshotWithoutPersistingInternalProfileID(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 10, 30, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID: "site-app-session-youtube", SiteKey: "youtube",
		Status: string(appsessions.StatusDisconnected), CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{}
	reader := &countingBrowserProfileReader{records: []appcookies.Record{{
		Name: "SAPISID", Value: "live-secret", Domain: ".youtube.com", Path: "/",
	}}}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(
		repo,
		WithProvider(provider),
		WithBrowserProfileReader(reader),
		WithImportCommitter(committer),
	)

	scan, err := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
		Mode: currentBrowserMode, BrowserID: "chrome",
	})
	if err != nil {
		t.Fatalf("scan current Chrome: %v", err)
	}
	result, err := service.ImportBrowserProfile(context.Background(), dto.AppSessionBrowserImportRequest{
		Mode:          currentBrowserMode,
		BrowserID:     scan.BrowserID,
		ProfileID:     scan.ProfileID,
		SnapshotToken: scan.SnapshotToken,
		AppSessionIDs: []string{current.ID},
	})
	if err != nil {
		t.Fatalf("import current Chrome: %v", err)
	}
	if len(result.ImportedIDs) != 1 || result.ImportedIDs[0] != current.ID {
		t.Fatalf("current Chrome import result = %#v", result)
	}
	if reader.callCount() != 1 {
		t.Fatalf("current Chrome reader calls = %d, want scan only", reader.callCount())
	}
	if committer.session.SourceBrowser != "chrome" || committer.session.SourceProfile != "" {
		t.Fatalf("persisted current Chrome source = %q/%q", committer.session.SourceBrowser, committer.session.SourceProfile)
	}
}

func TestNormalizeBrowserProfileSelectionRejectsPathsAndUnsupportedModes(t *testing.T) {
	t.Parallel()

	for _, profileID := range []string{
		"Default",
		"/Users/example/Library/Application Support/Google/Chrome/Default",
		`profile-..\\Default`,
		"profile-with/slash",
		"profile-with\x00nul",
	} {
		_, _, err := normalizeBrowserProfileSelection(browserProfileMode, "chrome", profileID)
		if !errors.Is(err, appsessions.ErrInvalidSession) {
			t.Fatalf("expected raw or malformed profile %q to be rejected, got %v", profileID, err)
		}
	}

	_, _, err := normalizeBrowserProfileSelection("external_browser", "chrome", "profile-opaque-id")
	if !errors.Is(err, appsessions.ErrUnsupported) {
		t.Fatalf("expected unsupported import mode to be rejected, got %v", err)
	}
}

func TestBrowserProfileImportAtomicallyConsumesScanSnapshotWithoutRereading(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-youtube",
		SiteKey:   "youtube",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{}
	reader := &countingBrowserProfileReader{records: []appcookies.Record{
		{Name: "SAPISID", Value: "scan-secret", Domain: ".youtube.com", Path: "/"},
		{Name: "unrelated", Value: "must-not-be-retained", Domain: ".example.com", Path: "/"},
	}}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(
		repo,
		WithProvider(provider),
		WithBrowserProfileReader(reader),
		WithImportCommitter(committer),
	)
	service.now = func() time.Time { return createdAt }

	scan, err := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
		Mode:      browserProfileMode,
		BrowserID: "chrome",
		ProfileID: "profile-opaque",
	})
	if err != nil {
		t.Fatalf("scan browser profile: %v", err)
	}
	if !validBrowserScanSnapshotToken(scan.SnapshotToken) {
		t.Fatalf("scan returned invalid opaque token %q", scan.SnapshotToken)
	}
	// Import is intentionally independent from the reader after Scan. Removing
	// it makes any accidental second profile read fail the test.
	service.browserProfileReader = nil

	request := dto.AppSessionBrowserImportRequest{
		Mode:          browserProfileMode,
		BrowserID:     "chrome",
		ProfileID:     "profile-opaque",
		SnapshotToken: scan.SnapshotToken,
		AppSessionIDs: []string{current.ID},
	}
	type outcome struct {
		result dto.AppSessionBrowserImportResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, importErr := service.ImportBrowserProfile(context.Background(), request)
			outcomes <- outcome{result: result, err: importErr}
		}()
	}
	wait.Wait()
	close(outcomes)

	successes := 0
	rejections := 0
	for outcome := range outcomes {
		switch {
		case outcome.err == nil:
			successes++
			if len(outcome.result.ImportedIDs) != 1 || outcome.result.ImportedIDs[0] != current.ID {
				t.Fatalf("successful import result = %#v", outcome.result)
			}
		case errors.Is(outcome.err, appsessions.ErrInvalidSession):
			rejections++
		default:
			t.Fatalf("unexpected import error: %v", outcome.err)
		}
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("snapshot outcomes: successes=%d rejections=%d", successes, rejections)
	}
	if reader.callCount() != 1 {
		t.Fatalf("browser profile read count = %d, want exactly Scan's one read", reader.callCount())
	}
	if committer.calls != 1 || !bytes.Contains(committer.plaintext, []byte("scan-secret")) {
		t.Fatalf("snapshot import did not commit scanned cookies: calls=%d plaintext=%q", committer.calls, committer.plaintext)
	}
	if bytes.Contains(committer.plaintext, []byte("must-not-be-retained")) {
		t.Fatalf("unrelated browser cookie leaked into snapshot: %q", committer.plaintext)
	}
}

func TestBrowserProfileImportRejectsAndConsumesMismatchedSnapshot(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-youtube",
		SiteKey:   "youtube",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	reader := &countingBrowserProfileReader{records: []appcookies.Record{{
		Name: "SAPISID", Value: "scan-secret", Domain: ".youtube.com", Path: "/",
	}}}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(
		repo,
		WithProvider(&appSessionProviderStub{}),
		WithBrowserProfileReader(reader),
		WithImportCommitter(committer),
	)

	scan, err := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
		Mode: browserProfileMode, BrowserID: "chrome", ProfileID: "profile-opaque",
	})
	if err != nil {
		t.Fatalf("scan browser profile: %v", err)
	}
	base := dto.AppSessionBrowserImportRequest{
		Mode: browserProfileMode, BrowserID: "chrome", ProfileID: "profile-other",
		SnapshotToken: scan.SnapshotToken, AppSessionIDs: []string{current.ID},
	}
	if _, err := service.ImportBrowserProfile(context.Background(), base); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("mismatched snapshot error = %v", err)
	}
	base.ProfileID = "profile-opaque"
	if _, err := service.ImportBrowserProfile(context.Background(), base); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("mismatched token was not consumed: %v", err)
	}
	if reader.callCount() != 1 || committer.calls != 0 {
		t.Fatalf("mismatch reread/commit counts = %d/%d", reader.callCount(), committer.calls)
	}
}

func TestBrowserProfileImportRejectsIDsNotSelectableInScan(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-youtube",
		SiteKey:   "youtube",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{}
	reader := &countingBrowserProfileReader{records: []appcookies.Record{{
		Name: "SAPISID", Value: "scan-secret", Domain: ".youtube.com", Path: "/",
	}}}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(
		repo,
		WithProvider(provider),
		WithBrowserProfileReader(reader),
		WithImportCommitter(committer),
	)
	scan, err := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
		Mode: browserProfileMode, BrowserID: "chrome", ProfileID: "profile-opaque",
	})
	if err != nil {
		t.Fatalf("scan browser profile: %v", err)
	}

	tampered := dto.AppSessionBrowserImportRequest{
		Mode:          browserProfileMode,
		BrowserID:     "chrome",
		ProfileID:     "profile-opaque",
		SnapshotToken: scan.SnapshotToken,
		AppSessionIDs: []string{current.ID, siteAppSessionID("tiktok")},
	}
	if _, err := service.ImportBrowserProfile(context.Background(), tampered); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("tampered App Session IDs error = %v", err)
	}
	if committer.calls != 0 || provider.cacheCalls != 0 {
		t.Fatalf("tampered batch partially committed: commits=%d cache=%d", committer.calls, provider.cacheCalls)
	}
	tampered.AppSessionIDs = []string{current.ID}
	if _, err := service.ImportBrowserProfile(context.Background(), tampered); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("tampered request did not consume token: %v", err)
	}
	if reader.callCount() != 1 {
		t.Fatalf("tampered request reread browser profile %d times", reader.callCount())
	}
}

func TestBrowserScanSnapshotExpiresAndDoesNotCrossServiceInstances(t *testing.T) {
	now := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	service := NewAppSessionsService(newAppSessionRepoStub())
	service.now = func() time.Time { return now }
	token, err := service.storeBrowserScanSnapshot(
		"chrome",
		"profile-opaque",
		[]appcookies.Record{{Name: "SAPISID", Value: "secret", Domain: ".youtube.com", Path: "/"}},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("store snapshot: %v", err)
	}
	now = now.Add(browserScanSnapshotTTL)
	if _, err := service.consumeBrowserScanSnapshot(token, "chrome", "profile-opaque"); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("expired snapshot error = %v", err)
	}

	secondToken, err := service.storeBrowserScanSnapshot("chrome", "profile-opaque", nil, nil, nil)
	if err != nil {
		t.Fatalf("store second snapshot: %v", err)
	}
	restarted := NewAppSessionsService(newAppSessionRepoStub())
	if _, err := restarted.consumeBrowserScanSnapshot(secondToken, "chrome", "profile-opaque"); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("snapshot crossed service/process boundary: %v", err)
	}
}

func TestBrowserScanSnapshotStoreIsCapacityAndSizeBounded(t *testing.T) {
	now := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	service := NewAppSessionsService(newAppSessionRepoStub())
	service.now = func() time.Time { return now }
	tokens := make([]string, 0, browserScanSnapshotCapacity+1)
	for index := 0; index <= browserScanSnapshotCapacity; index++ {
		token, err := service.storeBrowserScanSnapshot(
			"chrome",
			"profile-opaque",
			[]appcookies.Record{{Name: "cookie", Value: fmt.Sprintf("value-%d", index)}},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("store snapshot %d: %v", index, err)
		}
		tokens = append(tokens, token)
		now = now.Add(time.Millisecond)
	}
	if len(service.browserScanSnapshots) != browserScanSnapshotCapacity {
		t.Fatalf("snapshot count = %d, want %d", len(service.browserScanSnapshots), browserScanSnapshotCapacity)
	}
	if _, err := service.consumeBrowserScanSnapshot(tokens[0], "chrome", "profile-opaque"); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("oldest capacity-evicted token error = %v", err)
	}
	if _, err := service.consumeBrowserScanSnapshot(tokens[len(tokens)-1], "chrome", "profile-opaque"); err != nil {
		t.Fatalf("newest bounded token: %v", err)
	}

	oversized := appcookies.Record{Value: strings.Repeat("x", int(browserScanSnapshotMaxBytes))}
	if _, err := service.storeBrowserScanSnapshot("chrome", "profile-opaque", []appcookies.Record{oversized}, nil, nil); !errors.Is(err, appsessions.ErrInvalidSession) {
		t.Fatalf("oversized snapshot error = %v", err)
	}
}

func TestAppSessionBrowserLabelIncludesSafari(t *testing.T) {
	if got := appSessionBrowserLabel("safari"); got != "Safari" {
		t.Fatalf("Safari source label = %q", got)
	}
}
