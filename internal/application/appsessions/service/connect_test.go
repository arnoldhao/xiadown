package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

type appSessionRepoStub struct {
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
	result := make([]appsessions.Session, 0, len(repo.sessions))
	for _, item := range repo.sessions {
		result = append(result, item)
	}
	return result, nil
}

func (repo *appSessionRepoStub) Get(_ context.Context, id string) (appsessions.Session, error) {
	item, ok := repo.sessions[id]
	if !ok {
		return appsessions.Session{}, appsessions.ErrSessionNotFound
	}
	return item, nil
}

func (repo *appSessionRepoStub) GetBySiteKey(_ context.Context, siteKey string) (appsessions.Session, error) {
	for _, item := range repo.sessions {
		if item.SiteKey == siteKey {
			return item, nil
		}
	}
	return appsessions.Session{}, appsessions.ErrSessionNotFound
}

func (repo *appSessionRepoStub) Save(_ context.Context, session appsessions.Session) error {
	repo.sessions[session.ID] = session
	return nil
}

func (repo *appSessionRepoStub) Delete(_ context.Context, id string) error {
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
	service := NewAppSessionsService(
		newAppSessionRepoStub(current),
		WithProvider(&appSessionProviderStub{}),
		WithAccountFetcher(func(context.Context, string, []appcookies.Record) (dto.AppSessionAccount, error) {
			return dto.AppSessionAccount{DisplayName: "YouTube User"}, nil
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
	if len(events) != 1 {
		t.Fatalf("events count = %d", len(events))
	}
	if events[0].Action != "finish" || !events[0].Saved || events[0].Reason != "browser_closed" {
		t.Fatalf("unexpected event = %#v", events[0])
	}
	if events[0].AppSession.ID != current.ID || events[0].AppSession.SiteKey != "youtube" || events[0].AppSession.Status != string(appsessions.StatusConnected) {
		t.Fatalf("unexpected app session event = %#v", events[0].AppSession)
	}
}

func TestPerformFinalizeRejectsUnverifiedBilibiliSession(t *testing.T) {
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
			return dto.AppSessionAccount{}, appsessions.ErrNoCookies
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
		t.Fatal("expected unverified bilibili session not to be saved")
	}
	if provider.saved {
		t.Fatal("expected unverified bilibili cookies not to be persisted")
	}
	if !provider.cleared {
		t.Fatal("expected unverified bilibili cookies to be cleared")
	}
	saved, err := service.repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get saved session: %v", err)
	}
	if saved.Status != appsessions.StatusDisconnected {
		t.Fatalf("expected disconnected session, got %s", saved.Status)
	}
	if saved.AccountDisplayName != "" || saved.AccountAvatarURL != "" {
		t.Fatalf("expected account fields to stay empty, got %#v", saved)
	}
	if result.AppSession.Status != string(appsessions.StatusDisconnected) {
		t.Fatalf("expected disconnected result, got %#v", result.AppSession)
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

func TestRejectUnverifiedAppSessionIgnoresClearNoCookies(t *testing.T) {
	provider := &appSessionProviderStub{}
	clearErr := appsessions.ErrNoCookies
	service := NewAppSessionsService(newAppSessionRepoStub(), WithProvider(appSessionProviderFunc{
		clear: func(context.Context, string, []string) error {
			provider.cleared = true
			return clearErr
		},
	}))
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:      "site-app-session-bilibili",
		SiteKey: "bilibili",
		Status:  string(appsessions.StatusDisconnected),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	service.repo.Save(context.Background(), current)
	_, err = service.rejectUnverifiedAppSession(context.Background(), &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
	}, current, dto.FinishAppSessionConnectResult{}, appsessions.ErrNoCookies)
	if err != nil {
		t.Fatalf("reject unverified: %v", err)
	}
	if !provider.cleared {
		t.Fatal("expected clear attempt")
	}
}

type appSessionProviderFunc struct {
	clear func(context.Context, string, []string) error
}

func (provider appSessionProviderFunc) AppSessionsSupported() bool {
	return true
}

func (provider appSessionProviderFunc) StartAppSession(context.Context, AppSessionStartRequest) (AppSessionBrowser, error) {
	return nil, appsessions.ErrUnsupported
}

func (provider appSessionProviderFunc) LoadAppSessionCookies(context.Context, string) ([]appcookies.Record, error) {
	return nil, appsessions.ErrNoCookies
}

func (provider appSessionProviderFunc) SaveAppSessionCookies(context.Context, string, []appcookies.Record) error {
	return errors.New("unexpected save")
}

func (provider appSessionProviderFunc) ClearAppSession(ctx context.Context, siteKey string, domains []string) error {
	if provider.clear == nil {
		return nil
	}
	return provider.clear(ctx, siteKey, domains)
}
