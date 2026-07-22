package wails

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	settingsdto "xiadown/internal/application/settings/dto"
	apptelemetry "xiadown/internal/application/telemetry"
)

type telemetryHandlerRepoStub struct {
	mu    sync.Mutex
	state apptelemetry.State
}

func (stub *telemetryHandlerRepoStub) Ensure(context.Context) (apptelemetry.State, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.state, nil
}

func (stub *telemetryHandlerRepoStub) IncrementLaunchCount(context.Context, time.Time) (apptelemetry.State, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.state.LaunchCount++
	stub.state.DistinctDaysUsed = 1
	stub.state.DistinctDaysUsedLastMonth = 1
	return stub.state, nil
}

type telemetryHandlerSettingsStub struct{}

func (telemetryHandlerSettingsStub) GetSettings(context.Context) (settingsdto.Settings, error) {
	return settingsdto.Settings{Language: "en"}, nil
}

type telemetryHandlerEmitterStub struct{ signals chan apptelemetry.Signal }

func (stub telemetryHandlerEmitterStub) Emit(signal apptelemetry.Signal) {
	stub.signals <- signal
}

type telemetryWindowVisibilityStub bool

func (stub telemetryWindowVisibilityStub) MainWindowVisible() bool { return bool(stub) }

func TestTelemetryHandlerTrackStationOpenedUsesStrictStationPayload(t *testing.T) {
	repo := &telemetryHandlerRepoStub{state: apptelemetry.State{
		InstallID:        "install-1",
		InstallCreatedAt: time.Now().Add(-time.Hour),
	}}
	emitter := telemetryHandlerEmitterStub{signals: make(chan apptelemetry.Signal, 4)}
	service := apptelemetry.NewService(repo, emitter, telemetryHandlerSettingsStub{}, "app-123", "1.2.3")
	handler := NewTelemetryHandler(service, telemetryWindowVisibilityStub(true), nil)

	if !handler.TrackStationOpened(context.Background(), "private://resource/identifier") {
		t.Fatal("visible station should be accepted")
	}
	select {
	case signal := <-emitter.signals:
		t.Fatalf("station emitted before launch listener handshake: %#v", signal)
	default:
	}
	if _, err := handler.TrackAppLaunch(context.Background()); err != nil {
		t.Fatalf("track launch: %v", err)
	}

	for {
		select {
		case signal := <-emitter.signals:
			if signal.Type != "XiaDown.Station.opened" {
				continue
			}
			if got := signal.Payload["XiaDown.Station.name"]; got != "other" {
				t.Fatalf("station name = %#v, want other", got)
			}
			for key, value := range signal.Payload {
				if strings.Contains(key, "private://") || strings.Contains(fmt.Sprint(value), "private://") {
					t.Fatalf("raw station data escaped strict payload: %s=%v", key, value)
				}
			}
			return
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for station telemetry")
		}
	}
}

func TestTelemetryHandlerIgnoresStationWhileMainWindowIsHidden(t *testing.T) {
	repo := &telemetryHandlerRepoStub{state: apptelemetry.State{
		InstallID:        "install-1",
		InstallCreatedAt: time.Now().Add(-time.Hour),
	}}
	emitter := telemetryHandlerEmitterStub{signals: make(chan apptelemetry.Signal, 4)}
	service := apptelemetry.NewService(repo, emitter, telemetryHandlerSettingsStub{}, "app-123", "1.2.3")
	handler := NewTelemetryHandler(service, telemetryWindowVisibilityStub(false), nil)

	if handler.TrackStationOpened(context.Background(), "library") {
		t.Fatal("hidden main window station should be rejected")
	}
	if _, err := handler.TrackAppLaunch(context.Background()); err != nil {
		t.Fatalf("track launch: %v", err)
	}
	for len(emitter.signals) > 0 {
		if signal := <-emitter.signals; signal.Type == "XiaDown.Station.opened" {
			t.Fatalf("hidden main window emitted station telemetry: %#v", signal)
		}
	}
}

func TestSanitizeTelemetryBodiesAllowsOnlyAnonymousCommonAndStationProperties(t *testing.T) {
	body, err := sanitizeTelemetryBodies([]map[string]any{{
		"clientUser":             strings.Repeat("a", 64),
		"sessionID":              "session-1",
		"appID":                  "app-123",
		"type":                   "XiaDown.Station.opened",
		"telemetryClientVersion": "JavaScriptSDK 2.0.4",
		"payload": map[string]any{
			"TelemetryDeck.AppInfo.version": "1.2.3",
			"XiaDown.Station.name":          "custom",
		},
	}})
	if err != nil {
		t.Fatalf("sanitize station body: %v", err)
	}
	payload := body[0]["payload"].(map[string]any)
	if payload["XiaDown.Station.name"] != "other" {
		t.Fatalf("station = %#v, want other", payload["XiaDown.Station.name"])
	}
	if payload["TelemetryDeck.AppInfo.version"] != "1.2.3" {
		t.Fatalf("standard payload was not preserved: %#v", payload)
	}
}

func TestSanitizeTelemetryBodiesRejectsDetailedOrIdentifyingPayload(t *testing.T) {
	base := map[string]any{
		"clientUser":             strings.Repeat("a", 64),
		"sessionID":              "session-1",
		"appID":                  "app-123",
		"type":                   "XiaDown.Station.opened",
		"telemetryClientVersion": "JavaScriptSDK 2.0.4",
		"payload": map[string]any{
			"XiaDown.Station.name": "library",
			"XiaDown.Library.path": "/Users/private/video.mp4",
		},
	}
	if _, err := sanitizeTelemetryBodies([]map[string]any{base}); err == nil {
		t.Fatal("identifying library detail should be rejected")
	}

	base["payload"] = map[string]any{"XiaDown.Station.name": "library"}
	base["title"] = "private title"
	if _, err := sanitizeTelemetryBodies([]map[string]any{base}); err == nil {
		t.Fatal("unknown top-level property should be rejected")
	}
}
