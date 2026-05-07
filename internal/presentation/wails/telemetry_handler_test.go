package wails

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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

func (stub *telemetryHandlerRepoStub) IncrementLaunchCount(_ context.Context, _ time.Time) (apptelemetry.State, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.state.LaunchCount++
	stub.state.DistinctDaysUsed = 1
	stub.state.DistinctDaysUsedLastMonth = 1
	return stub.state, nil
}

func (stub *telemetryHandlerRepoStub) RecordSessionSummary(_ context.Context, _ time.Time, durationSeconds float64) (apptelemetry.State, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.state.CompletedSessionCount++
	stub.state.TotalSessionSeconds += durationSeconds
	duration := durationSeconds
	stub.state.PreviousSessionSeconds = &duration
	return stub.state, nil
}

func (stub *telemetryHandlerRepoStub) MarkFirstLibraryCompleted(_ context.Context, at time.Time) (apptelemetry.State, bool, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.state.FirstLibraryCompletedAt != nil {
		return stub.state, false, nil
	}
	timestamp := at.UTC()
	stub.state.FirstLibraryCompletedAt = &timestamp
	return stub.state, true, nil
}

type telemetryHandlerSettingsStub struct{}

func (telemetryHandlerSettingsStub) GetSettings(context.Context) (settingsdto.Settings, error) {
	return settingsdto.Settings{Language: "en"}, nil
}

type telemetryHTTPClientStub struct {
	client *http.Client
}

func (stub telemetryHTTPClientStub) HTTPClient() *http.Client {
	return stub.client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestTelemetryHandlerFlushSessionSummaryForShutdownPostsDirectSignal(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != telemetryDefaultTarget {
			t.Fatalf("unexpected telemetry target: %s", request.URL.String())
		}
		if contentType := request.Header.Get("Content-Type"); contentType != "application/json; charset=utf-8" {
			t.Fatalf("unexpected content type: %s", contentType)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read telemetry body: %v", err)
		}
		requestBody = body
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}
	repo := &telemetryHandlerRepoStub{state: apptelemetry.State{
		InstallID:        "install-1",
		InstallCreatedAt: time.Now().Add(-time.Hour),
	}}
	service := apptelemetry.NewService(repo, nil, telemetryHandlerSettingsStub{}, "app-123", "1.2.3")
	if _, err := service.TrackAppLaunch(context.Background(), apptelemetry.AppLaunchContext{}); err != nil {
		t.Fatalf("track launch: %v", err)
	}
	handler := NewTelemetryHandler(service, apptelemetry.AppLaunchContext{}, telemetryHTTPClientStub{client: client})

	if err := handler.FlushSessionSummaryForShutdown(context.Background()); err != nil {
		t.Fatalf("flush shutdown telemetry: %v", err)
	}

	var posted []map[string]any
	if err := json.Unmarshal(requestBody, &posted); err != nil {
		t.Fatalf("decode telemetry body: %v", err)
	}
	if len(posted) != 1 {
		t.Fatalf("expected 1 telemetry body, got %d", len(posted))
	}
	body := posted[0]
	if body["type"] != "XiaDown.Session.summaryRecorded" {
		t.Fatalf("unexpected signal type: %#v", body["type"])
	}
	if body["appID"] != "app-123" {
		t.Fatalf("unexpected app id: %#v", body["appID"])
	}
	if body["clientUser"] != sha256Hex("install-1") {
		t.Fatalf("unexpected client user hash: %#v", body["clientUser"])
	}
	if _, ok := body["floatValue"].(float64); !ok {
		t.Fatalf("expected top-level floatValue, got %#v", body["floatValue"])
	}
	payload, ok := body["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload object, got %#v", body["payload"])
	}
	if payload["TelemetryDeck.SDK.name"] != telemetryNativeClientName {
		t.Fatalf("unexpected SDK name: %#v", payload["TelemetryDeck.SDK.name"])
	}
	if payload["TelemetryDeck.SDK.version"] != "1.2.3" {
		t.Fatalf("unexpected SDK version: %#v", payload["TelemetryDeck.SDK.version"])
	}
}
