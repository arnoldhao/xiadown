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

type appSessionRepoStub struct {
	mu       sync.Mutex
	sessions map[string]appsessions.Session
}

func newAppSessionRepoStub(items ...appsessions.Session) *appSessionRepoStub {
	repo := &appSessionRepoStub{sessions: make(map[string]appsessions.Session)}
	for _, item := range items {
		repo.sessions[item.ID] = item
	}
	return repo
}

func (repo *appSessionRepoStub) List(context.Context) ([]appsessions.Session, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]appsessions.Session, 0, len(repo.sessions))
	for _, item := range repo.sessions {
		result = append(result, item)
	}
	return result, nil
}

func (repo *appSessionRepoStub) Get(_ context.Context, id string) (appsessions.Session, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, ok := repo.sessions[id]
	if !ok {
		return appsessions.Session{}, appsessions.ErrSessionNotFound
	}
	return item, nil
}

func (repo *appSessionRepoStub) GetBySiteKey(_ context.Context, siteKey string) (appsessions.Session, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, item := range repo.sessions {
		if item.SiteKey == siteKey {
			return item, nil
		}
	}
	return appsessions.Session{}, appsessions.ErrSessionNotFound
}

func (repo *appSessionRepoStub) Save(_ context.Context, session appsessions.Session) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.sessions[session.ID] = session
	return nil
}

func (repo *appSessionRepoStub) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.sessions, id)
	return nil
}

type appSessionProviderStub struct {
	saved   bool
	cleared bool
}

func (provider *appSessionProviderStub) AppSessionsSupported() bool {
	return true
}

func (provider *appSessionProviderStub) StartAppSession(context.Context, AppSessionStartRequest) (AppSessionBrowser, error) {
	return nil, appsessions.ErrUnsupported
}

func (provider *appSessionProviderStub) LoadAppSessionCookies(context.Context, string) ([]appcookies.Record, error) {
	return nil, appsessions.ErrNoCookies
}

func (provider *appSessionProviderStub) SaveAppSessionCookies(context.Context, string, []appcookies.Record) error {
	provider.saved = true
	return nil
}

func (provider *appSessionProviderStub) ClearAppSession(context.Context, string, []string) error {
	provider.cleared = true
	return nil
}

func TestFinalizeSessionPublishesChangeEvent(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
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
	var events []AppSessionChangeEvent
	verifyStarted := make(chan struct{})
	releaseVerify := make(chan struct{})
	var verifyStartedOnce sync.Once
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(&appSessionProviderStub{}),
		WithAccountFetcher(func(ctx context.Context, _ string, _ []appcookies.Record) (dto.AppSessionAccount, error) {
			verifyStartedOnce.Do(func() { close(verifyStarted) })
			select {
			case <-releaseVerify:
				return dto.AppSessionAccount{DisplayName: "YouTube User"}, nil
			case <-ctx.Done():
				return dto.AppSessionAccount{}, ctx.Err()
			}
		}),
		WithChangeListener(func(_ context.Context, event AppSessionChangeEvent) {
			events = append(events, event)
		}),
	)
	service.now = func() time.Time { return createdAt.Add(time.Hour) }
	service.putSession(&browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		LastCookies: []appcookies.Record{
			{Name: "SID", Value: "session-value", Domain: ".youtube.com", Path: "/"},
		},
		finalizeDone: make(chan struct{}),
	})

	result, triggered, err := service.finalizeSession(context.Background(), "runtime-session", "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !triggered || !result.Saved {
		t.Fatalf("expected triggered saved finalize, triggered=%v result=%#v", triggered, result)
	}
	if result.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationVerifying) {
		t.Fatalf("expected verifying result, got %#v", result.AppSession)
	}
	select {
	case <-verifyStarted:
	case <-time.After(time.Second):
		t.Fatal("verification did not start")
	}
	if len(events) != 1 {
		t.Fatalf("events count = %d", len(events))
	}
	if events[0].Action != "finish" || !events[0].Saved || events[0].Reason != "browser_closed" {
		t.Fatalf("unexpected event = %#v", events[0])
	}
	if events[0].AppSession.ID != current.ID || events[0].AppSession.SiteKey != "youtube" || events[0].AppSession.Status != string(appsessions.StatusConnected) {
		t.Fatalf("unexpected app session event = %#v", events[0].AppSession)
	}
	close(releaseVerify)
	waitForAppSession(t, service, current.ID, func(saved appsessions.Session) bool {
		return saved.AccountVerificationStatus == appsessions.AccountVerificationVerified &&
			saved.AccountDisplayName == "YouTube User"
	})
}

func TestPerformFinalizeSavesUnverifiedBilibiliSessionAndMarksVerificationFailed(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-bilibili",
		SiteKey:   "bilibili",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	provider := &appSessionProviderStub{}
	verifyStarted := make(chan struct{})
	releaseVerify := make(chan struct{})
	var verifyStartedOnce sync.Once
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(provider),
		WithAccountFetcher(func(ctx context.Context, _ string, _ []appcookies.Record) (dto.AppSessionAccount, error) {
			verifyStartedOnce.Do(func() { close(verifyStarted) })
			select {
			case <-releaseVerify:
				return dto.AppSessionAccount{}, appsessions.ErrNoCookies
			case <-ctx.Done():
				return dto.AppSessionAccount{}, ctx.Err()
			}
		}),
	)
	service.now = func() time.Time { return createdAt.Add(time.Hour) }

	result, err := service.performFinalize(context.Background(), &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		LastCookies: []appcookies.Record{
			{Name: "buvid3", Value: "anonymous", Domain: ".bilibili.com", Path: "/"},
		},
	}, "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !result.Saved {
		t.Fatal("expected bilibili cookies to be saved before account verification")
	}
	if !provider.saved {
		t.Fatal("expected bilibili cookies to be persisted")
	}
	if provider.cleared {
		t.Fatal("did not expect unverified bilibili cookies to be cleared")
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.Status != appsessions.StatusConnected {
		t.Fatalf("expected connected session, got %s", saved.Status)
	}
	if saved.AccountVerificationStatus != appsessions.AccountVerificationVerifying {
		t.Fatalf("expected verifying session, got %s", saved.AccountVerificationStatus)
	}
	if saved.AccountDisplayName != "" || saved.AccountAvatarURL != "" {
		t.Fatalf("expected account fields to stay empty, got %#v", saved)
	}
	if result.AppSession.Status != string(appsessions.StatusConnected) ||
		result.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationVerifying) {
		t.Fatalf("expected connected verifying result, got %#v", result.AppSession)
	}
	select {
	case <-verifyStarted:
	case <-time.After(time.Second):
		t.Fatal("verification did not start")
	}
	close(releaseVerify)
	waitForAppSession(t, service, current.ID, func(saved appsessions.Session) bool {
		return saved.Status == appsessions.StatusConnected &&
			saved.AccountVerificationStatus == appsessions.AccountVerificationUnverified &&
			saved.AccountVerificationError != ""
	})
}

func TestPerformFinalizeVerificationHardTimeoutPublishesUnverified(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
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
	blockVerify := make(chan struct{})
	t.Cleanup(func() { close(blockVerify) })
	finished := make(chan AppSessionChangeEvent, 1)
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(&appSessionProviderStub{}),
		WithAccountFetcher(func(context.Context, string, []appcookies.Record) (dto.AppSessionAccount, error) {
			<-blockVerify
			return dto.AppSessionAccount{DisplayName: "Late User"}, nil
		}),
		WithChangeListener(func(_ context.Context, event AppSessionChangeEvent) {
			if event.Action == "verify-finished" {
				finished <- event
			}
		}),
	)
	service.now = func() time.Time { return createdAt.Add(time.Hour) }
	service.accountVerificationTimeout = 20 * time.Millisecond

	result, err := service.performFinalize(context.Background(), &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		LastCookies: []appcookies.Record{
			{Name: "__Secure-3PAPISID", Value: "session-value", Domain: ".youtube.com", Path: "/"},
		},
	}, "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if result.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationVerifying) {
		t.Fatalf("expected initial verifying result, got %#v", result.AppSession)
	}

	select {
	case event := <-finished:
		if event.Reason != "unverified" || event.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationUnverified) {
			t.Fatalf("unexpected verification event: %#v", event)
		}
		if event.AppSession.AccountVerificationError != "verification timed out" {
			t.Fatalf("expected timeout error, got %#v", event.AppSession)
		}
		if event.AppSession.AccountVerificationStartedAt != "" {
			t.Fatalf("expected verification start timestamp to be cleared, got %#v", event.AppSession)
		}
	case <-time.After(time.Second):
		t.Fatal("verification timeout event was not published")
	}

	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.AccountVerificationStatus != appsessions.AccountVerificationUnverified ||
		saved.AccountVerificationError != "verification timed out" ||
		saved.AccountVerificationStartedAt != nil {
		t.Fatalf("unexpected saved session after timeout: %#v", saved)
	}
}

func TestFinishVerificationTimeoutDoesNotRequireExactStartedAtPrecision(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	expectedStartedAt := createdAt.Add(time.Hour).Add(250 * time.Millisecond)
	storedStartedAt := expectedStartedAt.Truncate(time.Second)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                           "site-app-session-youtube",
		SiteKey:                      "youtube",
		Status:                       string(appsessions.StatusConnected),
		AccountVerificationStatus:    string(appsessions.AccountVerificationVerifying),
		AccountVerificationStartedAt: &storedStartedAt,
		CreatedAt:                    &createdAt,
		UpdatedAt:                    &storedStartedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	finished := make(chan AppSessionChangeEvent, 1)
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithChangeListener(func(_ context.Context, event AppSessionChangeEvent) {
			if event.Action == "verify-finished" {
				finished <- event
			}
		}),
	)
	service.now = func() time.Time { return expectedStartedAt.Add(30 * time.Second) }

	service.finishAppSessionAccountVerification(
		current,
		[]appcookies.Record{{Name: "__Secure-3PAPISID", Value: "session-value", Domain: ".youtube.com", Path: "/"}},
		expectedStartedAt,
		dto.AppSessionAccount{},
		context.DeadlineExceeded,
	)

	select {
	case event := <-finished:
		if event.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationUnverified) ||
			event.AppSession.AccountVerificationError != "verification timed out" {
			t.Fatalf("unexpected verification event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("verification timeout event was not published")
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.AccountVerificationStatus != appsessions.AccountVerificationUnverified ||
		saved.AccountVerificationError != "verification timed out" {
		t.Fatalf("unexpected saved session after timeout: %#v", saved)
	}
}

func TestPerformFinalizeKeepsCookieOnlySessionForUnsupportedAccountSite(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-tiktok",
		SiteKey:   "tiktok",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	provider := &appSessionProviderStub{}
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(provider),
		WithAccountFetcher(func(context.Context, string, []appcookies.Record) (dto.AppSessionAccount, error) {
			return dto.AppSessionAccount{}, appsessions.ErrUnsupported
		}),
	)
	service.now = func() time.Time { return createdAt.Add(time.Hour) }

	result, err := service.performFinalize(context.Background(), &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		LastCookies: []appcookies.Record{
			{Name: "sessionid", Value: "present", Domain: ".tiktok.com", Path: "/"},
		},
	}, "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !result.Saved {
		t.Fatal("expected cookie-only tiktok session to be saved")
	}
	if !provider.saved {
		t.Fatal("expected tiktok cookies to be persisted")
	}
	if provider.cleared {
		t.Fatal("did not expect tiktok cookies to be cleared")
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.Status != appsessions.StatusConnected {
		t.Fatalf("expected connected session, got %s", saved.Status)
	}
	if saved.AccountVerificationStatus != appsessions.AccountVerificationUnsupported {
		t.Fatalf("expected unsupported account verification, got %s", saved.AccountVerificationStatus)
	}
}

func TestCookieExpiresAtPrefersSiteAuthCookies(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	authExpiry := now.Add(365 * 24 * time.Hour)
	deviceExpiry := now.Add(2 * 365 * 24 * time.Hour)

	got := cookieExpiresAt("youtube", []appcookies.Record{
		{Name: "YSC", Expires: now.Add(9 * time.Minute).Unix()},
		{Name: "PREF", Expires: deviceExpiry.Unix()},
		{Name: "__Secure-3PAPISID", Expires: authExpiry.Unix()},
	}, now)
	if got != authExpiry.Format(time.RFC3339) {
		t.Fatalf("youtube expires at = %q, want %q", got, authExpiry.Format(time.RFC3339))
	}

	bilibiliAuthExpiry := now.Add(30 * 24 * time.Hour)
	got = cookieExpiresAt("bilibili", []appcookies.Record{
		{Name: "buvid3", Expires: deviceExpiry.Unix()},
		{Name: "sid", Expires: now.Add(9 * time.Minute).Unix()},
		{Name: "SESSDATA", Expires: bilibiliAuthExpiry.Unix()},
	}, now)
	if got != bilibiliAuthExpiry.Format(time.RFC3339) {
		t.Fatalf("bilibili expires at = %q, want %q", got, bilibiliAuthExpiry.Format(time.RFC3339))
	}
}

func TestCookieExpiresAtFallsBackToLatestFutureCookie(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	latest := now.Add(time.Hour)
	got := cookieExpiresAt("tiktok", []appcookies.Record{
		{Name: "short", Expires: now.Add(9 * time.Minute).Unix()},
		{Name: "latest", Expires: latest.Unix()},
	}, now)
	if got != latest.Format(time.RFC3339) {
		t.Fatalf("fallback expires at = %q, want %q", got, latest.Format(time.RFC3339))
	}
}

func TestAppSessionAccountVerifiedRequiresStableIdentitySignal(t *testing.T) {
	if appSessionAccountVerified(dto.AppSessionAccount{}) {
		t.Fatal("empty account should not verify")
	}
	for name, account := range map[string]dto.AppSessionAccount{
		"display name": {DisplayName: "Arnold"},
		"handle":       {Handle: "arnold"},
		"avatar":       {AvatarURL: "https://example.com/avatar.png"},
	} {
		t.Run(name, func(t *testing.T) {
			if !appSessionAccountVerified(account) {
				t.Fatal("expected account to verify")
			}
		})
	}
}

func TestAppSessionRequiresAccountVerificationOnlyForAccountAPISites(t *testing.T) {
	for _, siteKey := range []string{"youtube", "bilibili"} {
		if !appSessionRequiresAccountVerification(siteKey) {
			t.Fatalf("expected %s to require account verification", siteKey)
		}
	}
	for _, siteKey := range []string{"tiktok", "instagram", "x", "facebook", "vimeo", "twitch", "niconico"} {
		if appSessionRequiresAccountVerification(siteKey) {
			t.Fatalf("did not expect %s to require account verification", siteKey)
		}
	}
}

func waitForAppSession(t *testing.T, service *AppSessionsService, id string, accept func(appsessions.Session) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		saved, err := service.repo.Get(context.Background(), id)
		if err == nil && accept(saved) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	saved, _ := service.repo.Get(context.Background(), id)
	t.Fatalf("condition not met for saved session: %#v", saved)
}
