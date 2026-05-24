package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	cdptarget "github.com/chromedp/cdproto/target"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/connectors"
)

type memoryConnectorRepo struct {
	mu    sync.Mutex
	items map[string]connectors.Connector
}

func newMemoryConnectorRepo(items ...connectors.Connector) *memoryConnectorRepo {
	repo := &memoryConnectorRepo{items: make(map[string]connectors.Connector, len(items))}
	for _, item := range items {
		repo.items[item.ID] = item
	}
	return repo
}

func (repo *memoryConnectorRepo) List(context.Context) ([]connectors.Connector, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	result := make([]connectors.Connector, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}

func (repo *memoryConnectorRepo) Get(_ context.Context, id string) (connectors.Connector, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	item, ok := repo.items[id]
	if !ok {
		return connectors.Connector{}, connectors.ErrConnectorNotFound
	}
	return item, nil
}

func (repo *memoryConnectorRepo) Save(_ context.Context, connector connectors.Connector) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.items[connector.ID] = connector
	return nil
}

func (repo *memoryConnectorRepo) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	delete(repo.items, id)
	return nil
}

func TestEnsureDefaultsCreatesChinaPrivateProfileConnector(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cookiesJSON, err := encodeCookies([]appcookies.Record{
		{Name: "sessionid", Value: "legacy", Domain: ".douyin.com", Path: "/"},
	})
	if err != nil {
		t.Fatalf("encode cookies: %v", err)
	}
	repo := newMemoryConnectorRepo(connectors.Connector{
		ID:             "connector-china-private",
		Type:           connectors.ConnectorChinaPrivate,
		Status:         connectors.StatusConnected,
		CookiesJSON:    cookiesJSON,
		LastVerifiedAt: &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	service := NewConnectorsService(repo)

	if err := service.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("ensure defaults: %v", err)
	}
	saved, err := repo.Get(context.Background(), "connector-china-private")
	if err != nil {
		t.Fatalf("get saved connector: %v", err)
	}
	if saved.CredentialMode != connectors.CredentialModeProfile {
		t.Fatalf("expected profile credential mode, got %q", saved.CredentialMode)
	}
	if saved.Status != connectors.StatusDisconnected {
		t.Fatalf("expected migrated profile connector to be disconnected, got %q", saved.Status)
	}
	if saved.CookiesJSON != "" || saved.LastVerifiedAt != nil {
		t.Fatalf("expected migrated profile connector cookies and verification to be cleared")
	}
	if saved.ProfileKey == "" {
		t.Fatalf("expected profile key, got empty value")
	}
	if saved.ProfilePath != "" || saved.ProfileBrowser != "" {
		t.Fatalf("expected default profile connector to start without browser binding, got path=%q browser=%q", saved.ProfilePath, saved.ProfileBrowser)
	}
}

func TestEnsureProfileConnectorCreatesProfileDirectory(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	profileBrowser := NewConnectorsService(newMemoryConnectorRepo()).resolveProfileBrowser(ctx)
	profilePath := filepath.Join(t.TempDir(), "connector-china-private", profileBrowser)
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     "connector-china-private",
		ProfilePath:    profilePath,
		ProfileBrowser: profileBrowser,
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(newMemoryConnectorRepo(connector))

	got, err := service.EnsureProfileConnector(ctx, string(connectors.ConnectorChinaPrivate))
	if err != nil {
		t.Fatalf("ensure profile connector: %v", err)
	}
	if got.ProfilePath != profilePath {
		t.Fatalf("expected profile path %q, got %q", profilePath, got.ProfilePath)
	}
	if got.CredentialState != "profile" {
		t.Fatalf("expected initialized profile state, got %q", got.CredentialState)
	}
	if stat, err := os.Stat(profilePath); err != nil || !stat.IsDir() {
		t.Fatalf("expected profile directory to exist, stat=%#v err=%v", stat, err)
	}
}

func TestProfileConnectorFinalizeSavesProfileWithoutCookies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	profilePath := filepath.Join(t.TempDir(), "china-private-profile")
	legacyCookiesJSON, err := encodeCookies([]appcookies.Record{
		{Name: "sessionid", Value: "legacy", Domain: ".douyin.com", Path: "/"},
	})
	if err != nil {
		t.Fatalf("encode legacy cookies: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		CookiesJSON:    legacyCookiesJSON,
		ProfileKey:     "connector-china-private",
		ProfilePath:    profilePath,
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	repo := newMemoryConnectorRepo(connector)
	service := NewConnectorsService(repo)
	service.now = func() time.Time { return now.Add(time.Minute) }
	service.removeAll = func(path string) error {
		t.Fatalf("profile connector cleanup should not remove profile path %q", path)
		return nil
	}

	session := &connectorSession{
		ID:                "session-profile",
		ConnectorID:       connector.ID,
		ConnectorType:     connector.Type,
		CredentialMode:    connectors.CredentialModeProfile,
		UserDataDir:       profilePath,
		State:             connectorSessionStateRunning,
		ConnectorSnapshot: mapConnectorDTO(connector),
		finalizeDone:      make(chan struct{}),
	}
	service.putSession(session)

	result, triggered, err := service.finalizeConnectSession(context.Background(), session.ID, "manual_finish")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !triggered {
		t.Fatalf("expected finalize to run")
	}
	if !result.Saved {
		t.Fatalf("expected profile connector to be saved")
	}
	saved, err := repo.Get(context.Background(), connector.ID)
	if err != nil {
		t.Fatalf("get saved connector: %v", err)
	}
	if saved.Status != connectors.StatusConnected {
		t.Fatalf("expected connected status, got %q", saved.Status)
	}
	if saved.CredentialMode != connectors.CredentialModeProfile {
		t.Fatalf("expected profile credential mode, got %q", saved.CredentialMode)
	}
	if saved.ProfilePath != profilePath {
		t.Fatalf("expected profile path %q, got %q", profilePath, saved.ProfilePath)
	}
	if saved.ProfileBrowser != connectorDefaultProfileBrowser {
		t.Fatalf("expected default profile browser from legacy session, got %q", saved.ProfileBrowser)
	}
	if saved.CookiesJSON != "" {
		t.Fatalf("expected profile connector cookies to be cleared")
	}
}

func TestProfileConnectorBindingUsesBrowserScopedPaths(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     "connector-china-private",
		ProfileBrowser: "chrome",
		ProfilePath:    filepath.Join(t.TempDir(), "chrome-profile"),
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}

	edge, changed, err := bindConnectorProfileToBrowser(connector, "edge")
	if err != nil {
		t.Fatalf("bind edge profile: %v", err)
	}
	if !changed {
		t.Fatalf("expected browser switch to change profile binding")
	}
	if edge.ProfileBrowser != "edge" {
		t.Fatalf("expected edge browser binding, got %q", edge.ProfileBrowser)
	}
	if filepath.Base(edge.ProfilePath) != "edge" {
		t.Fatalf("expected edge scoped profile path, got %q", edge.ProfilePath)
	}

	chrome, changed, err := bindConnectorProfileToBrowser(edge, "chrome")
	if err != nil {
		t.Fatalf("bind chrome profile: %v", err)
	}
	if !changed {
		t.Fatalf("expected switching back to chrome to change profile binding")
	}
	if chrome.ProfileBrowser != "chrome" {
		t.Fatalf("expected chrome browser binding, got %q", chrome.ProfileBrowser)
	}
	if filepath.Base(chrome.ProfilePath) != "chrome" {
		t.Fatalf("expected chrome scoped profile path, got %q", chrome.ProfilePath)
	}
}

func TestStartConnectorConnectLaunchesProfileAsPersistent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	profilePath := filepath.Join(t.TempDir(), "connector-china-private", "test-browser")
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     "connector-china-private",
		ProfilePath:    profilePath,
		ProfileBrowser: "test-browser",
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(
		newMemoryConnectorRepo(connector),
		WithSettingsReader(connectorSettingsReaderStub{settings: settingsdto.Settings{DefaultBrowser: "test-browser"}}),
	)
	expectedProfilePath, err := defaultConnectorProfilePath("connector-china-private", service.resolveProfileBrowser(context.Background()))
	if err != nil {
		t.Fatalf("resolve expected profile path: %v", err)
	}
	expectedErr := errors.New("launch failed")
	var capturedUserDataDir string
	var capturedPersistentProfile bool
	service.startBrowser = func(_ string, _ bool, userDataDir string, persistentProfile bool) (*browsercdp.Runtime, context.Context, context.CancelFunc, cdptarget.ID, error) {
		capturedUserDataDir = userDataDir
		capturedPersistentProfile = persistentProfile
		return nil, nil, nil, "", expectedErr
	}

	_, err = service.StartConnectorConnect(context.Background(), dto.StartConnectorConnectRequest{ID: connector.ID})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected launch error, got %v", err)
	}
	if capturedUserDataDir != expectedProfilePath {
		t.Fatalf("expected profile path %q, got %q", expectedProfilePath, capturedUserDataDir)
	}
	if !capturedPersistentProfile {
		t.Fatalf("expected profile connector launch to be marked persistent")
	}
}

func TestOpenConnectorSiteLaunchesProfileAsPersistent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	profilePath := filepath.Join(t.TempDir(), "connector-china-private", "test-browser")
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     "connector-china-private",
		ProfilePath:    profilePath,
		ProfileBrowser: "test-browser",
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(
		newMemoryConnectorRepo(connector),
		WithSettingsReader(connectorSettingsReaderStub{settings: settingsdto.Settings{DefaultBrowser: "test-browser"}}),
	)
	expectedProfilePath, err := defaultConnectorProfilePath("connector-china-private", service.resolveProfileBrowser(context.Background()))
	if err != nil {
		t.Fatalf("resolve expected profile path: %v", err)
	}
	expectedErr := errors.New("launch failed")
	var capturedUserDataDir string
	var capturedPersistentProfile bool
	service.startBrowser = func(_ string, _ bool, userDataDir string, persistentProfile bool) (*browsercdp.Runtime, context.Context, context.CancelFunc, cdptarget.ID, error) {
		capturedUserDataDir = userDataDir
		capturedPersistentProfile = persistentProfile
		return nil, nil, nil, "", expectedErr
	}

	_, err = service.OpenConnectorSite(context.Background(), dto.OpenConnectorSiteRequest{ID: connector.ID, TargetURL: "https://www.douyin.com/"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected launch error, got %v", err)
	}
	if capturedUserDataDir != expectedProfilePath {
		t.Fatalf("expected profile path %q, got %q", expectedProfilePath, capturedUserDataDir)
	}
	if !capturedPersistentProfile {
		t.Fatalf("expected profile connector launch to be marked persistent")
	}
}

func TestClearProfileConnectorRemovesProfileDirectoryAndClearsBrowserBinding(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	profileKey := fmt.Sprintf("connector-clear-%d", time.Now().UnixNano())
	profilePath := filepath.Join(t.TempDir(), profileKey, "chrome")
	if err := os.MkdirAll(profilePath, 0o755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     profileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: "chrome",
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	repo := newMemoryConnectorRepo(connector)
	service := NewConnectorsService(repo)

	if err := service.ClearConnector(context.Background(), dto.ClearConnectorRequest{ID: connector.ID}); err != nil {
		t.Fatalf("clear connector: %v", err)
	}
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Fatalf("expected profile dir to be removed, stat err=%v", err)
	}
	saved, err := repo.Get(context.Background(), connector.ID)
	if err != nil {
		t.Fatalf("get saved connector: %v", err)
	}
	if saved.Status != connectors.StatusDisconnected {
		t.Fatalf("expected disconnected status, got %q", saved.Status)
	}
	if saved.ProfileKey != profileKey || saved.ProfilePath != "" || saved.ProfileBrowser != "" {
		t.Fatalf("expected profile key to remain without browser binding, got key=%q path=%q browser=%q", saved.ProfileKey, saved.ProfilePath, saved.ProfileBrowser)
	}
	items, err := service.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("list connectors after clear: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one connector, got %d", len(items))
	}
	if items[0].ProfilePath != "" || items[0].ProfileBrowser != "" {
		t.Fatalf("expected cleared connector list item without browser binding, got path=%q browser=%q", items[0].ProfilePath, items[0].ProfileBrowser)
	}
	if items[0].ProfileInfo == nil {
		t.Fatalf("expected profile info for profile connector")
	}
	if len(items[0].ProfileInfo.Bindings) != 0 {
		t.Fatalf("expected no browser profile bindings after clear, got %#v", items[0].ProfileInfo.Bindings)
	}
}

func TestListConnectorsClearsMissingDisconnectedProfileBinding(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	profileKey := fmt.Sprintf("connector-missing-%d", time.Now().UnixNano())
	profilePath := filepath.Join(t.TempDir(), profileKey, "chrome")
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     profileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: "chrome",
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	repo := newMemoryConnectorRepo(connector)
	service := NewConnectorsService(repo)

	items, err := service.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("list connectors: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one connector, got %d", len(items))
	}
	if items[0].ProfilePath != "" || items[0].ProfileBrowser != "" {
		t.Fatalf("expected missing profile binding to be cleared in DTO, got path=%q browser=%q", items[0].ProfilePath, items[0].ProfileBrowser)
	}
	saved, err := repo.Get(context.Background(), connector.ID)
	if err != nil {
		t.Fatalf("get saved connector: %v", err)
	}
	if saved.ProfilePath != "" || saved.ProfileBrowser != "" {
		t.Fatalf("expected missing profile binding to be cleared in repo, got path=%q browser=%q", saved.ProfilePath, saved.ProfileBrowser)
	}
}

func TestConnectorProfileClearPathPrefersProfileRootForBrowserScopedPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	profileKey := fmt.Sprintf("connector-clear-root-%d", time.Now().UnixNano())
	profilePath, err := defaultConnectorProfilePath(profileKey, "chrome")
	if err != nil {
		t.Fatalf("resolve profile path: %v", err)
	}
	rootPath, err := defaultConnectorProfileRootPath(profileKey)
	if err != nil {
		t.Fatalf("resolve profile root: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     profileKey,
		ProfilePath:    profilePath,
		ProfileBrowser: "chrome",
		CreatedAt:      &now,
		UpdatedAt:      &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}

	if got := connectorProfileClearPath(connector); got != rootPath {
		t.Fatalf("expected profile root clear path %q, got %q", rootPath, got)
	}
}

func TestFinalizeUsesCachedCookiesWhenCDPUnavailable(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:        "connector-youtube",
		Type:      string(connectors.ConnectorYouTube),
		Status:    string(connectors.StatusDisconnected),
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	repo := newMemoryConnectorRepo(connector)
	service := NewConnectorsService(repo)
	service.now = func() time.Time { return now.Add(time.Minute) }
	service.removeAll = nil

	session := &connectorSession{
		ID:                "session-1",
		ConnectorID:       connector.ID,
		ConnectorType:     connector.Type,
		State:             connectorSessionStateRunning,
		LastCookies:       []appcookies.Record{{Name: "SID", Value: "1", Domain: ".youtube.com", Path: "/"}, {Name: "HSID", Value: "2", Domain: ".youtube.com", Path: "/"}},
		LastCookiesAt:     now,
		ConnectorSnapshot: mapConnectorDTO(connector),
		finalizeDone:      make(chan struct{}),
	}
	service.putSession(session)

	result, triggered, err := service.finalizeConnectSession(context.Background(), session.ID, "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !triggered {
		t.Fatalf("expected finalize to run")
	}
	if !result.Saved {
		t.Fatalf("expected cached cookies to be saved")
	}
	if result.RawCookiesCount != 2 || result.FilteredCookiesCount != 2 {
		t.Fatalf("expected 2 cached cookies, got raw=%d filtered=%d", result.RawCookiesCount, result.FilteredCookiesCount)
	}
	if result.Reason != "browser_closed" {
		t.Fatalf("expected browser_closed reason, got %q", result.Reason)
	}

	saved, err := repo.Get(context.Background(), connector.ID)
	if err != nil {
		t.Fatalf("get saved connector: %v", err)
	}
	if saved.Status != connectors.StatusConnected {
		t.Fatalf("expected connected status, got %q", saved.Status)
	}
	if stored := decodeCookies(saved.CookiesJSON); len(stored) != 2 {
		t.Fatalf("expected 2 stored cookies, got %d", len(stored))
	}
}

func TestPreferredBrowserReadsSettings(t *testing.T) {
	service := NewConnectorsService(
		newMemoryConnectorRepo(),
		WithSettingsReader(connectorSettingsReaderStub{settings: settingsdto.Settings{DefaultBrowser: "edge"}}),
	)

	if got := service.preferredBrowser(context.Background()); got != "edge" {
		t.Fatalf("expected preferred browser from settings, got %q", got)
	}
}

func TestConnectorTargetURLAllowsChinaPrivateProfileTargets(t *testing.T) {
	for _, rawURL := range []string{
		"https://www.douyin.com/",
		"https://www.iesdouyin.com/share/video/123/",
		"https://www.xiaohongshu.com/explore",
		"https://www.rednote.com/explore/123",
		"https://xhslink.com/a/example",
	} {
		got, err := connectorTargetURL(connectors.ConnectorChinaPrivate, rawURL)
		if err != nil {
			t.Fatalf("expected %s to be allowed: %v", rawURL, err)
		}
		if got != rawURL {
			t.Fatalf("expected target URL %q, got %q", rawURL, got)
		}
	}
}

func TestConnectorTargetURLRejectsOutOfScopeProfileSite(t *testing.T) {
	if _, err := connectorTargetURL(connectors.ConnectorChinaPrivate, "https://example.com/"); err == nil {
		t.Fatalf("expected out-of-scope target URL to be rejected")
	}
}

type connectorSettingsReaderStub struct {
	settings settingsdto.Settings
	err      error
}

func (stub connectorSettingsReaderStub) GetSettings(context.Context) (settingsdto.Settings, error) {
	if stub.err != nil {
		return settingsdto.Settings{}, stub.err
	}
	return stub.settings, nil
}
