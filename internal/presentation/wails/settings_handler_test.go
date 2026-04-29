package wails

import (
	"context"
	"errors"
	"testing"

	"xiadown/internal/application/settings/dto"
	settingsservice "xiadown/internal/application/settings/service"
	"xiadown/internal/domain/settings"
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

type recordingAutoStartManager struct {
	calls []bool
	err   error
}

func (manager *recordingAutoStartManager) SetEnabled(enabled bool) error {
	manager.calls = append(manager.calls, enabled)
	return manager.err
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
