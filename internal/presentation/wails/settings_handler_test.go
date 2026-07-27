package wails

import (
	"context"
	"errors"
	"testing"
	"time"

	"xiadown/internal/application/settings/dto"
	settingsservice "xiadown/internal/application/settings/service"
	"xiadown/internal/domain/settings"
	"xiadown/internal/infrastructure/proxy"
)

func TestProxyNetworkConfigChangedIgnoresTestMetadata(t *testing.T) {
	previous := dto.Proxy{
		Mode:           "manual",
		Scheme:         "http",
		Host:           "127.0.0.1",
		Port:           7890,
		Username:       "user",
		Password:       "pass",
		NoProxy:        []string{"localhost"},
		TimeoutSeconds: 10,
		TestedAt:       "2026-04-27T00:00:00Z",
		TestSuccess:    false,
		TestMessage:    "timeout",
	}
	current := previous
	current.TestedAt = "2026-04-27T00:01:00Z"
	current.TestSuccess = true
	current.TestMessage = "status 204"

	if proxyNetworkConfigChanged(previous, current) {
		t.Fatal("expected proxy test metadata changes to be ignored")
	}

	current.Host = "127.0.0.2"
	if !proxyNetworkConfigChanged(previous, current) {
		t.Fatal("expected proxy host change to be detected")
	}
}

func TestSettingsHandlerMarksMainWindowBootReady(t *testing.T) {
	windows := &WindowManager{mainBoot: newMainWindowBootState(false)}
	handler := &SettingsHandler{windows: windows}

	handler.MarkMainWindowBootReady()
	handler.MarkMainWindowBootReady()

	if !windows.MainBootReady() {
		t.Fatal("settings handler should complete the main-window boot handshake")
	}
}

func TestSettingsHandlerReleasesMainWindowBootFailureFallback(t *testing.T) {
	windows := &WindowManager{mainBoot: newMainWindowBootState(false)}
	windows.mainHTMLSurfaceReady.Store(true)
	handler := &SettingsHandler{windows: windows}

	handler.MarkMainWindowBootFailed()
	handler.MarkMainWindowBootFailed()

	if !windows.mainBoot.isFallbackReady() {
		t.Fatal("settings handler should expose the HTML recovery surface")
	}
}

func TestUpdateSettingsAppliesAutostartAndPersistsSetting(t *testing.T) {
	repo := &settingsMemoryRepository{}
	handler := &SettingsHandler{
		service:   settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings()),
		autostart: &recordingAutoStartManager{},
	}

	enabled := true
	updated, err := handler.UpdateSettings(context.Background(), dto.UpdateSettingsRequest{
		AutoStart: &enabled,
	})
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if !updated.AutoStart {
		t.Fatal("expected returned settings to enable autostart")
	}
	stored, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !stored.AutoStart() {
		t.Fatal("expected stored settings to enable autostart")
	}
	calls := handler.autostart.(*recordingAutoStartManager).calls
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("expected autostart manager to be called with true, got %#v", calls)
	}
}

func TestUpdateSettingsSynchronizesDownloadRootAndRollsBackBothSidesOnFailure(t *testing.T) {
	ctx := context.Background()
	repo := &settingsMemoryRepository{}
	settingsService := settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings())
	previous, err := settingsService.GetSettings(ctx)
	if err != nil {
		t.Fatalf("get default settings: %v", err)
	}
	syncer := &recordingDownloadRootSyncer{failuresRemaining: 1}
	handler := &SettingsHandler{
		service:            settingsService,
		downloadRootSyncer: syncer,
	}
	nextPath := t.TempDir()
	if _, err := handler.UpdateSettings(ctx, dto.UpdateSettingsRequest{
		DownloadDirectory: &nextPath,
	}); err == nil {
		t.Fatal("expected download root synchronization failure")
	}
	current, err := settingsService.GetSettings(ctx)
	if err != nil {
		t.Fatalf("get rolled-back settings: %v", err)
	}
	if current.DownloadDirectory != previous.DownloadDirectory {
		t.Fatalf(
			"download setting was not rolled back: got %q want %q",
			current.DownloadDirectory,
			previous.DownloadDirectory,
		)
	}
	if len(syncer.paths) != 2 ||
		syncer.paths[0] != nextPath ||
		syncer.paths[1] != previous.DownloadDirectory {
		t.Fatalf("root synchronization calls = %#v", syncer.paths)
	}
}

func TestUpdateSettingsRollsBackWhenAutostartUnavailable(t *testing.T) {
	repo := &settingsMemoryRepository{}
	handler := &SettingsHandler{
		service: settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings()),
	}

	enabled := true
	_, err := handler.UpdateSettings(context.Background(), dto.UpdateSettingsRequest{
		AutoStart: &enabled,
	})
	if err == nil {
		t.Fatal("expected autostart unavailable error")
	}
	stored, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.AutoStart() {
		t.Fatal("expected stored settings to roll back autostart")
	}
}

func TestUpdateSettingsRollsBackWhenAutostartApplyFails(t *testing.T) {
	repo := &settingsMemoryRepository{}
	handler := &SettingsHandler{
		service: settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings()),
		autostart: &recordingAutoStartManager{
			err: errors.New("launch item failed"),
		},
	}

	enabled := true
	_, err := handler.UpdateSettings(context.Background(), dto.UpdateSettingsRequest{
		AutoStart: &enabled,
	})
	if err == nil {
		t.Fatal("expected autostart apply error")
	}
	stored, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.AutoStart() {
		t.Fatal("expected stored settings to roll back autostart")
	}
	calls := handler.autostart.(*recordingAutoStartManager).calls
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("expected autostart manager to be called with true, got %#v", calls)
	}
}

func TestUpdateSettingsDoesNotRepublishNetworkPolicyForUnrelatedChanges(t *testing.T) {
	repo := &settingsMemoryRepository{}
	manager, err := proxy.NewManager(proxy.Config{
		Mode: settings.ProxyModeNone, Scheme: settings.ProxySchemeHTTP,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	handler := &SettingsHandler{
		service: settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings()),
		proxy:   manager,
	}

	language := "en"
	if _, err := handler.UpdateSettings(context.Background(), dto.UpdateSettingsRequest{
		Language: &language,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if manager.Generation() != 1 {
		t.Fatalf("unrelated settings change published generation %d", manager.Generation())
	}
}

func TestRefreshSystemProxyDoesNotRepublishANonSystemPolicy(t *testing.T) {
	repo := &settingsMemoryRepository{}
	manager, err := proxy.NewManager(proxy.Config{
		Mode: settings.ProxyModeNone, Scheme: settings.ProxySchemeHTTP,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	handler := &SettingsHandler{
		service: settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings()),
		proxy:   manager,
	}

	if _, err := handler.RefreshSystemProxy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if manager.Generation() != 1 {
		t.Fatalf("system proxy discovery republished non-system generation %d", manager.Generation())
	}
}

func TestUpdateSettingsDoesNotPublishProxyBeforeLaterValidationSucceeds(t *testing.T) {
	repo := &settingsMemoryRepository{}
	manager, err := proxy.NewManager(proxy.Config{
		Mode: settings.ProxyModeNone, Scheme: settings.ProxySchemeHTTP,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	player := &failingSettingsPlayer{err: errors.New("player rejected quality")}
	handler := &SettingsHandler{
		service: settingsservice.NewSettingsService(repo, nil, settings.DefaultSettings()),
		proxy:   manager,
		players: []settingsOnlinePlayerResetter{player},
	}

	quality := "high"
	_, err = handler.UpdateSettings(context.Background(), dto.UpdateSettingsRequest{
		PlaybackAudioQuality: &quality,
		Proxy: &dto.Proxy{
			Mode: "manual", Scheme: "http", Host: "127.0.0.1", Port: 7890,
			TimeoutSeconds: 5,
		},
	})
	if err == nil {
		t.Fatal("expected playback quality synchronization failure")
	}
	if manager.Generation() != 1 || manager.CurrentConfig().Mode != settings.ProxyModeNone {
		t.Fatalf("failed settings transaction changed live network policy: generation=%d mode=%s", manager.Generation(), manager.CurrentConfig().Mode.String())
	}
	stored, getErr := repo.Get(context.Background())
	if getErr != nil {
		t.Fatal(getErr)
	}
	if stored.Proxy().Mode() != settings.ProxyModeNone {
		t.Fatalf("failed settings transaction left persisted proxy mode %s", stored.Proxy().Mode().String())
	}
}

type failingSettingsPlayer struct {
	err error
}

func (player *failingSettingsPlayer) Reset() error { return nil }

func (player *failingSettingsPlayer) SetPlaybackAudioQuality(string) error {
	return player.err
}

type recordingAutoStartManager struct {
	calls []bool
	err   error
}

func (manager *recordingAutoStartManager) SetEnabled(enabled bool) error {
	manager.calls = append(manager.calls, enabled)
	return manager.err
}

type recordingDownloadRootSyncer struct {
	paths             []string
	failuresRemaining int
}

func (syncer *recordingDownloadRootSyncer) SyncDefaultDownloadStorageRoot(
	_ context.Context,
	path string,
) error {
	syncer.paths = append(syncer.paths, path)
	if syncer.failuresRemaining > 0 {
		syncer.failuresRemaining--
		return errors.New("root unavailable")
	}
	return nil
}

type settingsMemoryRepository struct {
	current settings.Settings
	found   bool
}

func (repo *settingsMemoryRepository) Get(context.Context) (settings.Settings, error) {
	if !repo.found {
		return settings.Settings{}, settings.ErrSettingsNotFound
	}
	return repo.current, nil
}

func (repo *settingsMemoryRepository) Save(_ context.Context, current settings.Settings) error {
	repo.current = current
	repo.found = true
	return nil
}
