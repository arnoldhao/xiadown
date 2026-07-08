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
	saved       bool
	cleared     bool
	loadCount   int
	loadRecords []appcookies.Record
	loadErr     error
}

func (provider *appSessionProviderStub) AppSessionsSupported() bool {
	return true
}

func (provider *appSessionProviderStub) StartAppSession(context.Context, AppSessionStartRequest) (AppSessionBrowser, error) {
	return nil, appsessions.ErrUnsupported
}

func (provider *appSessionProviderStub) LoadAppSessionCookies(context.Context, string) ([]appcookies.Record, error) {
	provider.loadCount++
	if provider.loadErr != nil {
		return nil, provider.loadErr
	}
	if len(provider.loadRecords) > 0 {
		return append([]appcookies.Record(nil), provider.loadRecords...), nil
	}
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

func TestListAppSessionsDoesNotReadStoredCookies(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	verifiedAt := createdAt.Add(time.Hour)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-twitch",
		SiteKey:                   "twitch",
		Status:                    string(appsessions.StatusConnected),
		AccountDisplayName:        "Twitch User",
		AccountHandle:             "twitch_user",
		AccountVerificationStatus: string(appsessions.AccountVerificationVerified),
		LastVerifiedAt:            &verifiedAt,
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &verifiedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	provider := &appSessionProviderStub{
		loadRecords: []appcookies.Record{{Name: "auth-token", Value: "token", Domain: ".twitch.tv", Path: "/", Expires: verifiedAt.Add(24 * time.Hour).Unix()}},
	}
	service := NewAppSessionsService(newAppSessionRepoStub(current), WithProvider(provider))

	items, err := service.ListAppSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if provider.loadCount != 0 {
		t.Fatalf("expected list to avoid stored cookie reads, got %d", provider.loadCount)
	}
	var twitch dto.AppSession
	for _, item := range items {
		if item.SiteKey == "twitch" {
			twitch = item
			break
		}
	}
	if twitch.ID == "" {
		t.Fatal("missing twitch session")
	}
	if twitch.Status != string(appsessions.StatusConnected) ||
		twitch.CredentialState != "app_session" ||
		twitch.AccountVerificationStatus != string(appsessions.AccountVerificationVerified) {
		t.Fatalf("unexpected twitch session summary: %#v", twitch)
	}
	if twitch.CookiesCount != 0 || len(twitch.Cookies) != 0 {
		t.Fatalf("list should not expose cookies without reading keychain: %#v", twitch.Cookies)
	}
	if twitch.Account == nil || twitch.Account.DisplayName != "Twitch User" || twitch.Account.ExpiresAt != "" {
		t.Fatalf("unexpected account summary: %#v", twitch.Account)
	}
}

func TestListAppSessionsUsesPersistedAccountExpiresAtWithoutReadingCookies(t *testing.T) {
	verifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	createdAt := verifiedAt.Add(-time.Hour)
	expiresAtTime := verifiedAt.Add(30 * 24 * time.Hour)
	expiresAt := expiresAtTime.UTC().Format(time.RFC3339)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-vimeo",
		SiteKey:                   "vimeo",
		Status:                    string(appsessions.StatusConnected),
		AccountDisplayName:        "Vimeo User",
		AccountMetadataJSON:       encodeMetadata(map[string]any{"userID": "111", appSessionAccountExpiresAtMetadataKey: expiresAt}),
		AccountVerificationStatus: string(appsessions.AccountVerificationVerified),
		LastVerifiedAt:            &verifiedAt,
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &verifiedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	provider := &appSessionProviderStub{
		loadRecords: []appcookies.Record{{Name: "vimeo", Value: "token", Domain: ".vimeo.com", Path: "/", Expires: expiresAtTime.Unix()}},
	}
	service := NewAppSessionsService(newAppSessionRepoStub(current), WithProvider(provider))

	items, err := service.ListAppSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if provider.loadCount != 0 {
		t.Fatalf("expected list to avoid stored cookie reads, got %d", provider.loadCount)
	}
	var vimeo dto.AppSession
	for _, item := range items {
		if item.SiteKey == "vimeo" {
			vimeo = item
			break
		}
	}
	if vimeo.Account == nil {
		t.Fatalf("missing vimeo account: %#v", vimeo)
	}
	if vimeo.Account.ExpiresAt != expiresAt {
		t.Fatalf("expires at = %q, want %q", vimeo.Account.ExpiresAt, expiresAt)
	}
	if _, ok := vimeo.Account.Metadata[appSessionAccountExpiresAtMetadataKey]; ok {
		t.Fatalf("internal expires metadata leaked: %#v", vimeo.Account.Metadata)
	}
	if vimeo.Account.Metadata["userID"] != "111" {
		t.Fatalf("public metadata lost: %#v", vimeo.Account.Metadata)
	}
}

func TestFinishVerificationPersistsAccountExpiresAt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	startedAt := now.Add(-time.Minute)
	createdAt := startedAt.Add(-time.Hour)
	expiresAtTime := now.Add(30 * 24 * time.Hour)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                           "site-app-session-niconico",
		SiteKey:                      "niconico",
		Status:                       string(appsessions.StatusConnected),
		AccountVerificationStatus:    string(appsessions.AccountVerificationVerifying),
		AccountVerificationStartedAt: &startedAt,
		CreatedAt:                    &createdAt,
		UpdatedAt:                    &startedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	service := NewAppSessionsService(repo)
	service.now = func() time.Time { return now }

	service.finishAppSessionAccountVerification(
		current,
		[]appcookies.Record{{Name: "user_session", Value: "session", Domain: ".nicovideo.jp", Path: "/", Expires: expiresAtTime.Unix()}},
		startedAt,
		dto.AppSessionAccount{DisplayName: "Nico User", Metadata: map[string]any{"userID": "80003"}},
		nil,
	)

	saved, err := repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved: %v", err)
	}
	metadata := decodeMetadata(saved.AccountMetadataJSON)
	expiresAt := expiresAtTime.UTC().Format(time.RFC3339)
	if metadata[appSessionAccountExpiresAtMetadataKey] != expiresAt {
		t.Fatalf("saved metadata = %#v, want expires %q", metadata, expiresAt)
	}
	summary := service.mapSessionDTO(context.Background(), saved)
	if summary.Account == nil || summary.Account.ExpiresAt != expiresAt {
		t.Fatalf("summary account = %#v, want expires %q", summary.Account, expiresAt)
	}
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

func TestPerformFinalizeDoesNotSaveBilibiliSessionWithoutAuthCookie(t *testing.T) {
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
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(provider),
		WithAccountFetcher(func(context.Context, string, []appcookies.Record) (dto.AppSessionAccount, error) {
			t.Fatal("verification should not start without auth cookies")
			return dto.AppSessionAccount{}, nil
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
	if result.Saved {
		t.Fatalf("expected non-auth bilibili cookies not to be saved, got %#v", result)
	}
	if provider.saved {
		t.Fatal("did not expect non-auth bilibili cookies to be persisted")
	}
	if provider.cleared {
		t.Fatal("did not expect provider clear during finalize")
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.Status != appsessions.StatusDisconnected {
		t.Fatalf("expected disconnected session, got %s", saved.Status)
	}
	if result.AppSession.Status != string(appsessions.StatusDisconnected) ||
		result.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationUnverified) {
		t.Fatalf("expected disconnected unverified result, got %#v", result.AppSession)
	}
}

func TestPerformFinalizeDoesNotSaveInstagramSessionWithoutSessionID(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-instagram",
		SiteKey:   "instagram",
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
			t.Fatal("verification should not start without instagram sessionid")
			return dto.AppSessionAccount{}, nil
		}),
	)
	service.now = func() time.Time { return createdAt.Add(time.Hour) }

	deviceExpiry := createdAt.Add(365 * 24 * time.Hour)
	result, err := service.performFinalize(context.Background(), &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		LastCookies: []appcookies.Record{
			{Name: "ig_did", Value: "device", Domain: ".instagram.com", Path: "/", Expires: deviceExpiry.Unix()},
		},
	}, "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if result.Saved {
		t.Fatalf("expected non-auth instagram cookies not to be saved, got %#v", result)
	}
	if provider.saved {
		t.Fatal("did not expect non-auth instagram cookies to be persisted")
	}
	if result.AppSession.Status != string(appsessions.StatusDisconnected) ||
		result.AppSession.CredentialState != string(appsessions.StatusDisconnected) ||
		result.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationUnverified) {
		t.Fatalf("expected disconnected unverified result, got %#v", result.AppSession)
	}
	if result.AppSession.Account != nil && result.AppSession.Account.ExpiresAt != "" {
		t.Fatalf("did not expect non-auth cookie expiry to be exposed, got %#v", result.AppSession.Account)
	}
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

func TestPerformFinalizeStartsVerificationForCookieAccountSite(t *testing.T) {
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
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(provider),
		WithAccountFetcher(func(context.Context, string, []appcookies.Record) (dto.AppSessionAccount, error) {
			close(fetchStarted)
			<-releaseFetch
			return dto.AppSessionAccount{}, appsessions.ErrUnsupported
		}),
	)
	defer close(releaseFetch)
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
	if result.AppSession.AccountVerificationStatus != string(appsessions.AccountVerificationVerifying) {
		t.Fatalf("expected result verification to start, got %#v", result.AppSession)
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.Status != appsessions.StatusConnected {
		t.Fatalf("expected connected session, got %s", saved.Status)
	}
	if saved.AccountVerificationStatus != appsessions.AccountVerificationVerifying {
		t.Fatalf("expected verifying account verification, got %s", saved.AccountVerificationStatus)
	}
	select {
	case <-fetchStarted:
	case <-time.After(time.Second):
		t.Fatal("verification fetcher was not started")
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

	for _, test := range []struct {
		siteKey    string
		authCookie string
	}{
		{siteKey: "tiktok", authCookie: "sessionid"},
		{siteKey: "instagram", authCookie: "sessionid"},
		{siteKey: "x", authCookie: "auth_token"},
		{siteKey: "facebook", authCookie: "xs"},
		{siteKey: "vimeo", authCookie: "vimeo"},
		{siteKey: "twitch", authCookie: "auth-token"},
		{siteKey: "niconico", authCookie: "user_session"},
	} {
		test := test
		t.Run(test.siteKey, func(t *testing.T) {
			siteAuthExpiry := now.Add(45 * 24 * time.Hour)
			got := cookieExpiresAt(test.siteKey, []appcookies.Record{
				{Name: "device", Expires: deviceExpiry.Unix()},
				{Name: test.authCookie, Expires: siteAuthExpiry.Unix()},
			}, now)
			if got != siteAuthExpiry.Format(time.RFC3339) {
				t.Fatalf("%s expires at = %q, want %q", test.siteKey, got, siteAuthExpiry.Format(time.RFC3339))
			}
		})
	}
}

func TestCookieExpiresAtDoesNotFallbackToNonAuthCookie(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	latest := now.Add(time.Hour)
	got := cookieExpiresAt("tiktok", []appcookies.Record{
		{Name: "short", Expires: now.Add(9 * time.Minute).Unix()},
		{Name: "latest", Expires: latest.Unix()},
	}, now)
	if got != "" {
		t.Fatalf("non-auth expires at = %q, want empty", got)
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
	for _, siteKey := range []string{"youtube", "bilibili", "tiktok", "instagram", "x", "facebook", "vimeo", "twitch", "niconico"} {
		if !appSessionRequiresAccountVerification(siteKey) {
			t.Fatalf("expected %s to require account verification", siteKey)
		}
	}
	for _, siteKey := range []string{"china_private", "unknown"} {
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
