package service

import (
	"context"
	"errors"
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

type blockingFirstGetAppSessionRepo struct {
	*appSessionRepoStub
	firstGetStarted chan struct{}
	releaseFirstGet chan struct{}
	getMu           sync.Mutex
	getCalls        int
}

func (repo *blockingFirstGetAppSessionRepo) Get(ctx context.Context, id string) (appsessions.Session, error) {
	repo.getMu.Lock()
	repo.getCalls++
	first := repo.getCalls == 1
	repo.getMu.Unlock()
	item, err := repo.appSessionRepoStub.Get(ctx, id)
	if first {
		close(repo.firstGetStarted)
		select {
		case <-repo.releaseFirstGet:
		case <-ctx.Done():
			return appsessions.Session{}, ctx.Err()
		}
	}
	return item, err
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
	cacheCalls  int
	cacheSite   string
	cacheRecord []appcookies.Record
}

type blockingAppSessionBrowser struct {
	cookies        []appcookies.Record
	cookiesStarted chan struct{}
	releaseCookies chan struct{}
	done           chan struct{}
	startOnce      sync.Once
	closeOnce      sync.Once
}

type recordingAppSessionImportCommitter struct {
	repo      *appSessionRepoStub
	calls     int
	session   appsessions.Session
	plaintext []byte
	err       error
}

type blockingBrowserProfileReader struct {
	records []appcookies.Record
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (reader *blockingBrowserProfileReader) ReadBrowserProfileCookies(
	ctx context.Context,
	_ string,
	_ string,
	_ []string,
) ([]appcookies.Record, error) {
	reader.once.Do(func() { close(reader.started) })
	select {
	case <-reader.release:
		return append([]appcookies.Record(nil), reader.records...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (committer *recordingAppSessionImportCommitter) CommitImportedAppSession(
	ctx context.Context,
	session appsessions.Session,
	plaintext []byte,
) error {
	committer.calls++
	committer.session = session
	committer.plaintext = append([]byte(nil), plaintext...)
	if committer.err != nil {
		return committer.err
	}
	return committer.repo.Save(ctx, session)
}

func (browser *blockingAppSessionBrowser) Cookies(ctx context.Context) ([]appcookies.Record, error) {
	browser.startOnce.Do(func() { close(browser.cookiesStarted) })
	select {
	case <-browser.releaseCookies:
		return append([]appcookies.Record(nil), browser.cookies...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (browser *blockingAppSessionBrowser) Done() <-chan struct{} {
	return browser.done
}

func (browser *blockingAppSessionBrowser) Close() {
	browser.closeOnce.Do(func() { close(browser.done) })
}

type hydratingAppSessionProviderStub struct {
	*appSessionProviderStub
	hydrateCount   int
	hydrateErr     error
	allowBootstrap bool
}

func TestTargetURLForSiteRequiresCredentialSafeHTTPSOrigin(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "same-site https", rawURL: "https://www.youtube.com/watch?v=abc#player"},
		{name: "plain http", rawURL: "http://www.youtube.com/watch?v=abc", wantErr: true},
		{name: "file URL", rawURL: "file://www.youtube.com/private", wantErr: true},
		{name: "userinfo", rawURL: "https://attacker:secret@www.youtube.com/watch?v=abc", wantErr: true},
		{name: "different https site", rawURL: "https://example.com/watch?v=abc", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := targetURLForSite("youtube", test.rawURL)
			if test.wantErr {
				if !errors.Is(err, appsessions.ErrInvalidSession) {
					t.Fatalf("target error = %v", err)
				}
				return
			}
			if err != nil || got != test.rawURL {
				t.Fatalf("target = %q error=%v", got, err)
			}
		})
	}
}

func (provider *hydratingAppSessionProviderStub) EnsureAppSessionRuntimeCookies(_ context.Context, _ string, allowBootstrap bool) error {
	provider.hydrateCount++
	provider.allowBootstrap = allowBootstrap
	if provider.hydrateErr != nil {
		return provider.hydrateErr
	}
	provider.loadRecords = []appcookies.Record{{
		Name: "LOGIN_INFO", Value: "live", Domain: ".youtube.com", Path: "/",
	}}
	return nil
}

func TestRecordsForSiteKeyFallsBackToCachedCookiesWhenHydrationIOFails(t *testing.T) {
	base := &appSessionProviderStub{loadRecords: []appcookies.Record{{
		Name: "SAPISID", Value: "cached", Domain: ".youtube.com", Path: "/",
	}}}
	provider := &hydratingAppSessionProviderStub{
		appSessionProviderStub: base,
		hydrateErr:             errors.New("transient WebKit read failure"),
	}
	service := NewAppSessionsService(newAppSessionRepoStub(), WithProvider(provider))

	records, err := service.RecordsForSiteKey(context.Background(), "youtube")
	if err != nil {
		t.Fatalf("fallback records after hydration I/O failure: %v", err)
	}
	if len(records) != 1 || records[0].Value != "cached" {
		t.Fatalf("hydration failure discarded cached cookies: %#v", records)
	}
}

func TestRecordsForSiteKeyPropagatesHydrationIOFailureWithoutValidatedCache(t *testing.T) {
	hydrateErr := errors.New("WebKit cookie store unavailable")
	provider := &hydratingAppSessionProviderStub{
		appSessionProviderStub: &appSessionProviderStub{},
		hydrateErr:             hydrateErr,
	}
	service := NewAppSessionsService(newAppSessionRepoStub(), WithProvider(provider))

	if _, err := service.RecordsForSiteKey(context.Background(), "youtube"); !errors.Is(err, hydrateErr) {
		t.Fatalf("missing-cache hydration error = %v, want %v", err, hydrateErr)
	}
}

func TestRecordsForSiteKeyPropagatesHydrationCancellation(t *testing.T) {
	base := &appSessionProviderStub{loadRecords: []appcookies.Record{{
		Name: "SAPISID", Value: "cached", Domain: ".youtube.com", Path: "/",
	}}}
	provider := &hydratingAppSessionProviderStub{
		appSessionProviderStub: base,
		hydrateErr:             context.DeadlineExceeded,
	}
	service := NewAppSessionsService(newAppSessionRepoStub(), WithProvider(provider))

	if _, err := service.RecordsForSiteKey(context.Background(), "youtube"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hydration cancellation error = %v, want deadline exceeded", err)
	}
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

func (provider *appSessionProviderStub) CacheImportedAppSessionCookies(siteKey string, records []appcookies.Record) {
	provider.cacheCalls++
	provider.cacheSite = siteKey
	provider.cacheRecord = append([]appcookies.Record(nil), records...)
}

func TestRecordsForSiteKeyHydratesRuntimeBeforeLoadingCookies(t *testing.T) {
	base := &appSessionProviderStub{}
	provider := &hydratingAppSessionProviderStub{appSessionProviderStub: base}
	service := NewAppSessionsService(newAppSessionRepoStub(), WithProvider(provider))

	records, err := service.RecordsForSiteKey(context.Background(), "youtube")
	if err != nil {
		t.Fatalf("records for hydrated YouTube session: %v", err)
	}
	if provider.hydrateCount != 1 || base.loadCount != 1 {
		t.Fatalf("hydrate/load counts = %d/%d, want 1/1", provider.hydrateCount, base.loadCount)
	}
	if provider.allowBootstrap {
		t.Fatal("disconnected session unexpectedly authorized WebKit bootstrap")
	}
	if len(records) != 1 || records[0].Name != "LOGIN_INFO" {
		t.Fatalf("runtime hydration did not precede cookie load: %#v", records)
	}
}

func TestRecordsForSiteKeyAllowsWebKitBootstrapForConnectedSession(t *testing.T) {
	now := time.Now().UTC()
	connected, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        siteAppSessionID("youtube"),
		SiteKey:   "youtube",
		Status:    string(appsessions.StatusConnected),
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("connected session: %v", err)
	}
	base := &appSessionProviderStub{}
	provider := &hydratingAppSessionProviderStub{appSessionProviderStub: base}
	service := NewAppSessionsService(newAppSessionRepoStub(connected), WithProvider(provider))

	if _, err := service.RecordsForSiteKey(context.Background(), "youtube"); err != nil {
		t.Fatalf("records for connected session: %v", err)
	}
	if !provider.allowBootstrap {
		t.Fatal("connected session did not authorize WebKit bootstrap")
	}
}

func TestYouTubeAuthenticationIgnoresRotatingSecurityCookies(t *testing.T) {
	rotatingOnly := []appcookies.Record{
		{Name: "SIDCC", Value: "cc", Domain: ".youtube.com", Path: "/"},
		{Name: "__Secure-1PSIDTS", Value: "ts", Domain: ".youtube.com", Path: "/"},
		{Name: "__Secure-3PSIDCC", Value: "cc", Domain: ".youtube.com", Path: "/"},
	}
	if appSessionHasAuthenticationCookies("youtube", rotatingOnly) {
		t.Fatal("rotating security cookies must not prove YouTube authentication")
	}
	withStableAuth := append(rotatingOnly, appcookies.Record{
		Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/",
	})
	if !appSessionHasAuthenticationCookies("youtube", withStableAuth) {
		t.Fatal("stable SAPISID should prove YouTube authentication")
	}
	if appSessionHasAuthenticationCookies("youtube", []appcookies.Record{{
		Name: "SAPISID", Domain: ".youtube.com", Path: "/",
	}}) {
		t.Fatal("empty SAPISID must not prove YouTube authentication")
	}
	if appSessionHasAuthenticationCookies("youtube", []appcookies.Record{{
		Name: "SAPISID", Value: "google-only", Domain: ".google.com", Path: "/",
	}}) {
		t.Fatal("Google-domain SAPISID must not prove www.youtube.com authentication")
	}
}

func TestDouyinAuthenticationRequiresAccountSessionCookie(t *testing.T) {
	deviceOnly := []appcookies.Record{
		{Name: "ttwid", Value: "device", Domain: ".douyin.com", Path: "/"},
		{Name: "odin_tt", Value: "device", Domain: ".douyin.com", Path: "/"},
		{Name: "passport_csrf_token", Value: "csrf", Domain: ".douyin.com", Path: "/"},
		{Name: "msToken", Value: "request", Domain: ".douyin.com", Path: "/"},
	}
	if appSessionHasAuthenticationCookies("douyin", deviceOnly) {
		t.Fatal("device and request cookies must not prove Douyin authentication")
	}
	for _, name := range []string{"sessionid", "sessionid_ss", "sid_tt", "sid_guard"} {
		cookies := append(append([]appcookies.Record(nil), deviceOnly...), appcookies.Record{
			Name: name, Value: "account", Domain: ".douyin.com", Path: "/",
		})
		if !appSessionHasAuthenticationCookies("douyin", cookies) {
			t.Fatalf("%s should prove Douyin authentication", name)
		}
	}
}

func TestXiaohongshuAuthenticationRequiresWebSession(t *testing.T) {
	deviceOnly := []appcookies.Record{
		{Name: "a1", Value: "device", Domain: ".xiaohongshu.com", Path: "/"},
		{Name: "webId", Value: "device", Domain: ".xiaohongshu.com", Path: "/"},
		{Name: "xsecappid", Value: "request", Domain: ".xiaohongshu.com", Path: "/"},
	}
	if appSessionHasAuthenticationCookies("xiaohongshu", deviceOnly) {
		t.Fatal("device and request cookies must not prove Xiaohongshu authentication")
	}
	shortLinkSession := append(append([]appcookies.Record(nil), deviceOnly...), appcookies.Record{
		Name: "web_session", Value: "short-link", Domain: ".xhslink.com", Path: "/",
	})
	if appSessionHasAuthenticationCookies("xiaohongshu", shortLinkSession) {
		t.Fatal("a short-link cookie must not prove Xiaohongshu authentication")
	}
	emptySession := append(append([]appcookies.Record(nil), deviceOnly...), appcookies.Record{
		Name: "web_session", Value: "", Domain: ".xiaohongshu.com", Path: "/",
	})
	if appSessionHasAuthenticationCookies("xiaohongshu", emptySession) {
		t.Fatal("an empty web_session must not prove Xiaohongshu authentication")
	}
	withSession := append(append([]appcookies.Record(nil), deviceOnly...), appcookies.Record{
		Name: "web_session", Value: "account", Domain: ".xiaohongshu.com", Path: "/",
	})
	if !appSessionHasAuthenticationCookies("xiaohongshu", withSession) {
		t.Fatal("web_session should identify a Xiaohongshu login candidate")
	}
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
	verificationEpoch := service.nextAccountVerificationEpoch(current.ID)

	service.finishAppSessionAccountVerification(
		current,
		[]appcookies.Record{{Name: "user_session", Value: "session", Domain: ".nicovideo.jp", Path: "/", Expires: expiresAtTime.Unix()}},
		startedAt,
		verificationEpoch,
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
			{Name: "SAPISID", Value: "session-value", Domain: ".youtube.com", Path: "/"},
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

func TestManualFinalizeUsesAtomicSecretAndMetadataCommitter(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-twitch",
		SiteKey:   "twitch",
		Status:    string(appsessions.StatusDisconnected),
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(
		repo,
		WithProvider(provider),
		WithImportCommitter(committer),
	)
	service.now = func() time.Time { return createdAt.Add(time.Hour) }

	result, err := service.performFinalize(context.Background(), &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		LastCookies: []appcookies.Record{{
			Name: "auth-token", Value: "manual-secret", Domain: ".twitch.tv", Path: "/",
		}},
	}, "manual_finish")
	if err != nil {
		t.Fatalf("manual finalize: %v", err)
	}
	if !result.Saved || committer.calls != 1 {
		t.Fatalf("saved=%v committer calls=%d", result.Saved, committer.calls)
	}
	if provider.saved {
		t.Fatal("manual finalize bypassed the atomic committer through provider persistence")
	}
	if committer.session.ID != current.ID || committer.session.Status != appsessions.StatusConnected ||
		committer.session.SourceType != appsessions.SourceTypeXiaDownProfile {
		t.Fatalf("committed session = %#v", committer.session)
	}
	records := appcookies.DecodeJSON(string(committer.plaintext))
	if len(records) != 1 || records[0].Name != "auth-token" || records[0].Value != "manual-secret" {
		t.Fatalf("committed plaintext envelope = %#v", records)
	}
	saved, err := repo.Get(context.Background(), current.ID)
	if err != nil || saved.Status != appsessions.StatusConnected {
		t.Fatalf("saved metadata = %#v err=%v", saved, err)
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
	verificationEpoch := service.nextAccountVerificationEpoch(current.ID)

	service.finishAppSessionAccountVerification(
		current,
		[]appcookies.Record{{Name: "__Secure-3PAPISID", Value: "session-value", Domain: ".youtube.com", Path: "/"}},
		expectedStartedAt,
		verificationEpoch,
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

func TestClearWinsAgainstVerificationCallbackHoldingOlderSnapshot(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(time.Hour)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                           "site-app-session-youtube",
		SiteKey:                      "youtube",
		Status:                       string(appsessions.StatusConnected),
		AccountVerificationStatus:    string(appsessions.AccountVerificationVerifying),
		AccountVerificationStartedAt: &startedAt,
		CreatedAt:                    &createdAt,
		UpdatedAt:                    &startedAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := &blockingFirstGetAppSessionRepo{
		appSessionRepoStub: newAppSessionRepoStub(current),
		firstGetStarted:    make(chan struct{}),
		releaseFirstGet:    make(chan struct{}),
	}
	provider := &appSessionProviderStub{}
	service := NewAppSessionsService(repo, WithProvider(provider))
	service.now = func() time.Time { return startedAt.Add(time.Minute) }
	verificationEpoch := service.nextAccountVerificationEpoch(current.ID)

	verificationDone := make(chan struct{})
	go func() {
		defer close(verificationDone)
		service.finishAppSessionAccountVerification(
			current,
			[]appcookies.Record{{Name: "SAPISID", Value: "secret", Domain: ".youtube.com", Path: "/"}},
			startedAt,
			verificationEpoch,
			dto.AppSessionAccount{DisplayName: "Verified User"},
			nil,
		)
	}()
	select {
	case <-repo.firstGetStarted:
	case <-time.After(time.Second):
		t.Fatal("verification did not read its initial snapshot")
	}

	clearDone := make(chan error, 1)
	go func() {
		clearDone <- service.ClearAppSession(context.Background(), dto.ClearAppSessionRequest{ID: current.ID})
	}()
	// Give Clear a chance to finish if the verification callback is not using
	// the credential mutation gate. Releasing the blocked read afterwards then
	// deterministically reproduces the old connected-metadata resurrection.
	select {
	case err := <-clearDone:
		if err != nil {
			t.Fatalf("clear while verification blocked: %v", err)
		}
		clearDone = nil
	case <-time.After(50 * time.Millisecond):
	}
	close(repo.releaseFirstGet)
	select {
	case <-verificationDone:
	case <-time.After(time.Second):
		t.Fatal("verification callback did not finish")
	}
	if clearDone != nil {
		select {
		case err := <-clearDone:
			if err != nil {
				t.Fatalf("clear after verification: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("clear did not finish")
		}
	}

	saved, err := repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get final session: %v", err)
	}
	if saved.Status != appsessions.StatusDisconnected ||
		saved.AccountVerificationStatus != appsessions.AccountVerificationUnverified ||
		saved.AccountDisplayName != "" {
		t.Fatalf("late verification resurrected cleared Session: %#v", saved)
	}
	if !provider.cleared {
		t.Fatal("clear did not remove the durable secret")
	}
}

func TestClearCancelsLateFinalizeBeforeItCanRestoreCredentials(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-youtube",
		SiteKey:                   "youtube",
		Status:                    string(appsessions.StatusConnected),
		AccountVerificationStatus: string(appsessions.AccountVerificationVerified),
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{loadRecords: []appcookies.Record{{
		Name: "SAPISID", Value: "old-secret", Domain: ".youtube.com", Path: "/",
	}}}
	service := NewAppSessionsService(repo, WithProvider(provider))
	browser := &blockingAppSessionBrowser{
		cookies: []appcookies.Record{{
			Name: "SAPISID", Value: "late-secret", Domain: ".youtube.com", Path: "/",
		}},
		cookiesStarted: make(chan struct{}),
		releaseCookies: make(chan struct{}),
		done:           make(chan struct{}),
	}
	runtimeSession := &browserSession{
		ID:           "runtime-session",
		AppSessionID: current.ID,
		SiteKey:      current.SiteKey,
		Purpose:      browserSessionPurposeConnect,
		Browser:      browser,
		finalizeDone: make(chan struct{}),
	}
	service.putSession(runtimeSession)

	finalizeDone := make(chan error, 1)
	go func() {
		_, err := service.performFinalize(context.Background(), runtimeSession, "browser_closed")
		finalizeDone <- err
	}()
	select {
	case <-browser.cookiesStarted:
	case <-time.After(time.Second):
		t.Fatal("finalize did not begin reading browser credentials")
	}
	if err := service.ClearAppSession(context.Background(), dto.ClearAppSessionRequest{ID: current.ID}); err != nil {
		t.Fatalf("clear while finalize is reading cookies: %v", err)
	}
	close(browser.releaseCookies)
	select {
	case err := <-finalizeDone:
		if !errors.Is(err, appsessions.ErrSessionGone) {
			t.Fatalf("late finalize error = %v, want session gone", err)
		}
	case <-time.After(time.Second):
		t.Fatal("late finalize did not stop")
	}

	saved, err := repo.Get(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("get final session: %v", err)
	}
	if saved.Status != appsessions.StatusDisconnected || saved.AccountVerificationStatus != appsessions.AccountVerificationUnverified {
		t.Fatalf("late finalize resurrected cleared Session: %#v", saved)
	}
	if provider.saved || !provider.cleared {
		t.Fatalf("provider saved=%v cleared=%v", provider.saved, provider.cleared)
	}
}

func TestClearInvalidatesBrowserScanSnapshotThatStartedEarlier(t *testing.T) {
	createdAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	current, err := appsessions.NewSession(appsessions.SessionParams{
		ID:                        "site-app-session-youtube",
		SiteKey:                   "youtube",
		Status:                    string(appsessions.StatusConnected),
		AccountVerificationStatus: string(appsessions.AccountVerificationVerified),
		CreatedAt:                 &createdAt,
		UpdatedAt:                 &createdAt,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	repo := newAppSessionRepoStub(current)
	provider := &appSessionProviderStub{loadRecords: []appcookies.Record{{
		Name: "SAPISID", Value: "old-secret", Domain: ".youtube.com", Path: "/",
	}}}
	reader := &blockingBrowserProfileReader{
		records: []appcookies.Record{{
			Name: "SAPISID", Value: "profile-secret", Domain: ".youtube.com", Path: "/",
		}},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	committer := &recordingAppSessionImportCommitter{repo: repo}
	service := NewAppSessionsService(
		repo,
		WithProvider(provider),
		WithBrowserProfileReader(reader),
		WithImportCommitter(committer),
	)

	scanDone := make(chan struct {
		result dto.AppSessionBrowserScanResult
		err    error
	}, 1)
	go func() {
		result, err := service.ScanBrowserProfile(context.Background(), dto.BrowserProfileSelection{
			Mode:      browserProfileMode,
			BrowserID: "chrome",
			ProfileID: "profile-opaque",
		})
		scanDone <- struct {
			result dto.AppSessionBrowserScanResult
			err    error
		}{result: result, err: err}
	}()
	select {
	case <-reader.started:
	case <-time.After(time.Second):
		t.Fatal("browser scan did not start reading the profile")
	}
	if err := service.ClearAppSession(context.Background(), dto.ClearAppSessionRequest{ID: current.ID}); err != nil {
		t.Fatalf("clear during browser profile read: %v", err)
	}
	close(reader.release)
	var scanResult dto.AppSessionBrowserScanResult
	select {
	case outcome := <-scanDone:
		if outcome.err != nil {
			t.Fatalf("complete stale browser scan: %v", outcome.err)
		}
		scanResult = outcome.result
	case <-time.After(time.Second):
		t.Fatal("stale browser scan did not stop")
	}
	result, err := service.ImportBrowserProfile(context.Background(), dto.AppSessionBrowserImportRequest{
		Mode:          browserProfileMode,
		BrowserID:     "chrome",
		ProfileID:     "profile-opaque",
		SnapshotToken: scanResult.SnapshotToken,
		AppSessionIDs: []string{current.ID},
	})
	if err != nil {
		t.Fatalf("consume invalidated browser scan snapshot: %v", err)
	}
	if len(result.ImportedIDs) != 0 ||
		len(result.SkippedIDs) != 1 || result.SkippedIDs[0] != current.ID {
		t.Fatalf("invalidated browser import result = %#v", result)
	}
	if committer.calls != 0 || provider.cacheCalls != 0 {
		t.Fatalf("late browser import committed=%d cached=%d", committer.calls, provider.cacheCalls)
	}
	saved, err := repo.Get(context.Background(), current.ID)
	if err != nil || saved.Status != appsessions.StatusDisconnected {
		t.Fatalf("late browser import resurrected cleared Session: %#v err=%v", saved, err)
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
		{siteKey: "douyin", authCookie: "sessionid_ss"},
		{siteKey: "xiaohongshu", authCookie: "web_session"},
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

	xiaohongshuExpiry := now.Add(90 * 24 * time.Hour)
	got = cookieExpiresAt("xiaohongshu", []appcookies.Record{
		{Name: "web_session", Expires: xiaohongshuExpiry.Unix()},
		{Name: "web_session_sec", Expires: now.Add(9 * time.Minute).Unix()},
	}, now)
	if got != xiaohongshuExpiry.Format(time.RFC3339) {
		t.Fatalf("xiaohongshu expires at = %q, want %q", got, xiaohongshuExpiry.Format(time.RFC3339))
	}
}

func TestXiaohongshuAppSessionCookieDomainsExcludeShortLinks(t *testing.T) {
	domains := appSessionCookieDomains("xiaohongshu")
	if len(domains) != 1 || domains[0] != "xiaohongshu.com" {
		t.Fatalf("xiaohongshu cookie domains = %#v", domains)
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
	for _, siteKey := range []string{"youtube", "bilibili", "tiktok", "douyin", "xiaohongshu", "instagram", "x", "facebook", "vimeo", "twitch", "niconico"} {
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
