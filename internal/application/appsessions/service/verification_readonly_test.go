package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

type readTrackingAppSessionBrowser struct {
	mu           sync.Mutex
	cookies      []appcookies.Record
	cookiesCalls int
	done         chan struct{}
	doneOnce     sync.Once
}

type runtimeHydratingVerificationProvider struct {
	*appSessionProviderStub
	runtimeRecords []appcookies.Record
	hydrationCalls int
}

func (provider *runtimeHydratingVerificationProvider) EnsureAppSessionRuntimeCookies(
	context.Context,
	string,
	bool,
) error {
	provider.hydrationCalls++
	provider.loadRecords = append([]appcookies.Record(nil), provider.runtimeRecords...)
	return nil
}

func newReadTrackingAppSessionBrowser(records ...appcookies.Record) *readTrackingAppSessionBrowser {
	return &readTrackingAppSessionBrowser{
		cookies: append([]appcookies.Record(nil), records...),
		done:    make(chan struct{}),
	}
}

func (browser *readTrackingAppSessionBrowser) Cookies(context.Context) ([]appcookies.Record, error) {
	browser.mu.Lock()
	defer browser.mu.Unlock()
	browser.cookiesCalls++
	return append([]appcookies.Record(nil), browser.cookies...), nil
}

func (browser *readTrackingAppSessionBrowser) Done() <-chan struct{} {
	return browser.done
}

func (browser *readTrackingAppSessionBrowser) Close() {
	browser.doneOnce.Do(func() { close(browser.done) })
}

func (browser *readTrackingAppSessionBrowser) cookieReadCount() int {
	browser.mu.Lock()
	defer browser.mu.Unlock()
	return browser.cookiesCalls
}

func TestVerifyAppSessionUsesReadOnlyRuntimeSnapshotAndPreservesBrowserSource(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	lastSyncedAt := createdAt.Add(20 * time.Minute)
	previousVerifiedAt := createdAt.Add(30 * time.Minute)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-youtube",
		SiteKey:                   "youtube",
		Status:                    string(appsessions.StatusConnected),
		AccountDisplayName:        "Previous account",
		AccountVerificationStatus: string(appsessions.AccountVerificationVerified),
		LastVerifiedAt:            &previousVerifiedAt,
		SourceType:                string(appsessions.SourceTypeBrowserProfile),
		SourceBrowser:             "safari",
		SourceProfile:             "profile-safari-default",
		LastSyncedAt:              &lastSyncedAt,
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &previousVerifiedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	provider := &runtimeHydratingVerificationProvider{
		appSessionProviderStub: &appSessionProviderStub{loadRecords: []appcookies.Record{{
			Name: "SAPISID", Value: "stable-backup-only", Domain: ".youtube.com", Path: "/",
		}}},
		runtimeRecords: []appcookies.Record{
			{Name: "SAPISID", Value: "runtime-secret", Domain: ".youtube.com", Path: "/"},
			{Name: "LOGIN_INFO", Value: "runtime-login-info", Domain: ".youtube.com", Path: "/"},
		},
	}
	fetchStarted := make(chan []appcookies.Record, 1)
	releaseFetch := make(chan struct{})
	startedEvent := make(chan AppSessionChangeEvent, 1)
	finishedEvent := make(chan AppSessionChangeEvent, 1)
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(provider),
		WithAccountFetcher(func(ctx context.Context, _ string, records []appcookies.Record) (dto.AppSessionAccount, error) {
			fetchStarted <- append([]appcookies.Record(nil), records...)
			select {
			case <-releaseFetch:
				return dto.AppSessionAccount{DisplayName: "Verified account", Handle: "@verified"}, nil
			case <-ctx.Done():
				return dto.AppSessionAccount{}, ctx.Err()
			}
		}),
		WithChangeListener(func(_ context.Context, event AppSessionChangeEvent) {
			if event.Action == "verify-started" {
				startedEvent <- event
			}
			if event.Action == "verify-finished" {
				finishedEvent <- event
			}
		}),
	)
	verificationTime := createdAt.Add(time.Hour)
	service.now = func() time.Time { return verificationTime }

	result, err := service.VerifyAppSession(context.Background(), dto.VerifyAppSessionRequest{ID: current.ID})
	if err != nil {
		t.Fatalf("verify session: %v", err)
	}
	if result.AccountVerificationStatus != string(appsessions.AccountVerificationVerifying) {
		t.Fatalf("verification result = %#v", result)
	}
	if result.SourceType != string(appsessions.SourceTypeBrowserProfile) ||
		result.SourceBrowser != "safari" || result.SourceProfile != "profile-safari-default" ||
		result.LastSyncedAt != lastSyncedAt.Format(time.RFC3339) {
		t.Fatalf("verification changed source metadata: %#v", result)
	}
	if provider.hydrationCalls != 1 || provider.loadCount != 1 ||
		provider.saved || provider.cleared || provider.cacheCalls != 0 {
		t.Fatalf(
			"provider hydrate/load/save/clear/cache = %d/%d/%v/%v/%d",
			provider.hydrationCalls,
			provider.loadCount,
			provider.saved,
			provider.cleared,
			provider.cacheCalls,
		)
	}

	select {
	case records := <-fetchStarted:
		values := make(map[string]string, len(records))
		for _, record := range records {
			values[record.Name] = record.Value
		}
		if len(records) != 2 ||
			values["SAPISID"] != "runtime-secret" ||
			values["LOGIN_INFO"] != "runtime-login-info" {
			t.Fatalf("account fetcher records = %#v", records)
		}
	case <-time.After(time.Second):
		t.Fatal("account verification did not start")
	}
	select {
	case event := <-startedEvent:
		if event.Saved || event.Reason != "read_only" || event.AppSession.SourceBrowser != "safari" {
			t.Fatalf("verify-started event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("verify-started event was not published")
	}

	started, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get verifying session: %v", err)
	}
	if started.SourceType != current.SourceType || started.SourceBrowser != current.SourceBrowser ||
		started.SourceProfile != current.SourceProfile || !started.LastSyncedAt.Equal(*current.LastSyncedAt) ||
		started.AccountDisplayName != "Previous account" || started.LastVerifiedAt == nil ||
		!started.LastVerifiedAt.Equal(previousVerifiedAt) {
		t.Fatalf("verifying write changed immutable session state: %#v", started)
	}

	close(releaseFetch)
	waitForAppSession(t, service, current.ID, func(saved appsessions.Session) bool {
		return saved.AccountVerificationStatus == appsessions.AccountVerificationVerified &&
			saved.AccountDisplayName == "Verified account"
	})
	select {
	case event := <-finishedEvent:
		if event.Saved || event.Reason != "verified" || event.AppSession.SourceBrowser != "safari" {
			t.Fatalf("verify-finished event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("verify-finished event was not published")
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get verified session: %v", err)
	}
	if saved.SourceType != current.SourceType || saved.SourceBrowser != current.SourceBrowser ||
		saved.SourceProfile != current.SourceProfile || saved.LastSyncedAt == nil ||
		!saved.LastSyncedAt.Equal(lastSyncedAt) {
		t.Fatalf("verification result changed credential provenance: %#v", saved)
	}
	if provider.saved || provider.cleared || provider.cacheCalls != 0 {
		t.Fatalf("verification persisted credential data: %#v", provider)
	}
}

func TestOpenBrowserCloseNeverReadsOrCommitsLiveCookies(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	lastSyncedAt := createdAt.Add(20 * time.Minute)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:            "site-app-session-youtube",
		SiteKey:       "youtube",
		Status:        string(appsessions.StatusConnected),
		SourceType:    string(appsessions.SourceTypeBrowserProfile),
		SourceBrowser: "safari",
		SourceProfile: "profile-safari-default",
		LastSyncedAt:  &lastSyncedAt,
		CreatedAt:     &createdAt,
		UpdatedAt:     &lastSyncedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{loadRecords: []appcookies.Record{{
		Name: "SAPISID", Value: "stored-secret", Domain: ".youtube.com", Path: "/",
	}}}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(repo, WithProvider(provider), WithImportCommitter(committer))
	browser := newReadTrackingAppSessionBrowser(appcookies.Record{
		Name: "SAPISID", Value: "mutated-webview-secret", Domain: ".youtube.com", Path: "/",
	})
	runtimeSession := &browserSession{
		ID:           "open-runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeOpen,
		Browser:      browser,
		State:        browserSessionStateRunning,
		finalizeDone: make(chan struct{}),
	}
	service.putSession(runtimeSession)
	service.startBrowserMonitor(runtimeSession.ID)
	browser.Close()

	deadline := time.Now().Add(time.Second)
	for {
		service.mu.Lock()
		session, ok := service.sessions[runtimeSession.ID]
		state := ""
		if session != nil {
			state = session.State
		}
		service.mu.Unlock()
		if ok && state == browserSessionStateCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("open browser session did not complete: state=%q", state)
		}
		time.Sleep(time.Millisecond)
	}

	if browser.cookieReadCount() != 0 {
		t.Fatalf("open browser cookies read count = %d", browser.cookieReadCount())
	}
	if provider.saved || provider.cleared || provider.cacheCalls != 0 || committer.calls != 0 {
		t.Fatalf("open close committed credentials: provider=%#v committer=%#v", provider, committer)
	}
	saved, err := repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get session after open close: %v", err)
	}
	if saved.SourceType != appsessions.SourceTypeBrowserProfile || saved.SourceBrowser != "safari" ||
		saved.SourceProfile != "profile-safari-default" || saved.LastSyncedAt == nil ||
		!saved.LastSyncedAt.Equal(lastSyncedAt) || !saved.UpdatedAt.Equal(lastSyncedAt) {
		t.Fatalf("open close changed stored session: %#v", saved)
	}
}

func TestStaleVerificationCannotOverwriteNewCredentialGenerationAtRoundedTimestamp(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	oldStartedAt := createdAt.Add(time.Hour).Add(250 * time.Millisecond)
	newStoredStartedAt := oldStartedAt.Truncate(time.Second)
	oldSession, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                           "site-app-session-youtube",
		SiteKey:                      "youtube",
		Status:                       string(appsessions.StatusConnected),
		AccountVerificationStatus:    string(appsessions.AccountVerificationVerifying),
		AccountVerificationStartedAt: &oldStartedAt,
		SourceType:                   string(appsessions.SourceTypeBrowserProfile),
		SourceBrowser:                "safari",
		SourceProfile:                "profile-safari-default",
		CreatedAt:                    &createdAt,
		UpdatedAt:                    &oldStartedAt,
	})
	if err != nil {
		t.Fatalf("create old session: %v", err)
	}
	repo := newAppSessionRepoStub(oldSession)
	service := NewAppSessionsService(repo)
	oldEpoch := service.nextAccountVerificationEpoch(oldSession.ID)

	newSession, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                           oldSession.ID,
		SiteKey:                      oldSession.SiteKey,
		Status:                       string(appsessions.StatusConnected),
		AccountVerificationStatus:    string(appsessions.AccountVerificationVerifying),
		AccountVerificationStartedAt: &newStoredStartedAt,
		SourceType:                   string(appsessions.SourceTypeBrowserProfile),
		SourceBrowser:                "chrome",
		SourceProfile:                "profile-chrome-work",
		CreatedAt:                    &createdAt,
		UpdatedAt:                    &newStoredStartedAt,
	})
	if err != nil {
		t.Fatalf("create new session: %v", err)
	}
	if err := repo.Save(context.Background(), newSession); err != nil {
		t.Fatalf("save new session: %v", err)
	}
	newEpoch := service.nextAccountVerificationEpoch(newSession.ID)

	service.finishAppSessionAccountVerification(
		oldSession,
		[]appcookies.Record{{Name: "SAPISID", Value: "old-secret", Domain: ".youtube.com", Path: "/"}},
		oldStartedAt,
		oldEpoch,
		dto.AppSessionAccount{DisplayName: "Old account"},
		nil,
	)

	afterOld, err := repo.Get(context.Background(), newSession.ID)
	if err != nil {
		t.Fatalf("get after stale callback: %v", err)
	}
	if afterOld.AccountDisplayName != "" || afterOld.AccountVerificationStatus != appsessions.AccountVerificationVerifying ||
		afterOld.SourceBrowser != "chrome" || afterOld.SourceProfile != "profile-chrome-work" {
		t.Fatalf("stale callback changed new credential generation: %#v", afterOld)
	}

	service.finishAppSessionAccountVerification(
		newSession,
		[]appcookies.Record{{Name: "SAPISID", Value: "new-secret", Domain: ".youtube.com", Path: "/"}},
		newStoredStartedAt,
		newEpoch,
		dto.AppSessionAccount{DisplayName: "New account"},
		nil,
	)
	afterNew, err := repo.Get(context.Background(), newSession.ID)
	if err != nil {
		t.Fatalf("get after current callback: %v", err)
	}
	if afterNew.AccountDisplayName != "New account" || afterNew.AccountVerificationStatus != appsessions.AccountVerificationVerified {
		t.Fatalf("current callback did not finish: %#v", afterNew)
	}
}
