package telemetry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	settingsdto "xiadown/internal/application/settings/dto"
)

type telemetryStateRepoStub struct {
	mu              sync.Mutex
	state           State
	incrementErrors []error
	incrementCalls  int
}

func (stub *telemetryStateRepoStub) Ensure(context.Context) (State, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.state, nil
}

func (stub *telemetryStateRepoStub) IncrementLaunchCount(context.Context, time.Time) (State, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.incrementCalls++
	if len(stub.incrementErrors) > 0 {
		err := stub.incrementErrors[0]
		stub.incrementErrors = stub.incrementErrors[1:]
		if err != nil {
			return State{}, err
		}
	}
	stub.state.LaunchCount++
	if stub.state.DistinctDaysUsed == 0 {
		stub.state.DistinctDaysUsed = 1
	}
	if stub.state.DistinctDaysUsedLastMonth == 0 {
		stub.state.DistinctDaysUsedLastMonth = 1
	}
	return stub.state, nil
}

type telemetryEmitterStub struct {
	mu      sync.Mutex
	signals []Signal
}

func (stub *telemetryEmitterStub) Emit(signal Signal) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	payload := make(map[string]any, len(signal.Payload))
	for key, value := range signal.Payload {
		payload[key] = value
	}
	stub.signals = append(stub.signals, Signal{Type: signal.Type, Payload: payload})
}

func (stub *telemetryEmitterStub) signalsFor(signalType string) []Signal {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	result := make([]Signal, 0)
	for _, signal := range stub.signals {
		if signal.Type == signalType {
			result = append(result, signal)
		}
	}
	return result
}

type telemetrySettingsStub struct{ language string }

func (stub telemetrySettingsStub) GetSettings(context.Context) (settingsdto.Settings, error) {
	return settingsdto.Settings{Language: stub.language}, nil
}

func newTelemetryTestService(repo *telemetryStateRepoStub, emitter *telemetryEmitterStub) *Service {
	return NewService(repo, emitter, telemetrySettingsStub{language: "zh-CN"}, "app-123", "1.2.3+45")
}

func TestServiceQueuesAndNormalizesStationOpenedUntilLaunch(t *testing.T) {
	repo := &telemetryStateRepoStub{state: State{
		InstallID:        "install-1",
		InstallCreatedAt: time.Now().Add(-24 * time.Hour),
	}}
	emitter := &telemetryEmitterStub{}
	service := newTelemetryTestService(repo, emitter)

	service.TrackStationOpened(context.Background(), " Library ")
	service.TrackStationOpened(context.Background(), "custom/private/station")
	service.TrackStationOpened(context.Background(), "  ")
	if got := len(emitter.signalsFor("XiaDown.Station.opened")); got != 0 {
		t.Fatalf("station signals emitted before app launch: %d", got)
	}

	if _, err := service.TrackAppLaunch(context.Background()); err != nil {
		t.Fatalf("track app launch: %v", err)
	}
	stationSignals := emitter.signalsFor("XiaDown.Station.opened")
	if len(stationSignals) != 2 {
		t.Fatalf("station signal count = %d, want 2", len(stationSignals))
	}
	if got := stationSignals[0].Payload["XiaDown.Station.name"]; got != "library" {
		t.Fatalf("first station = %#v, want library", got)
	}
	if got := stationSignals[1].Payload["XiaDown.Station.name"]; got != "other" {
		t.Fatalf("second station = %#v, want other", got)
	}
}

func TestServiceBoundsAndDeduplicatesPendingStationOpens(t *testing.T) {
	repo := &telemetryStateRepoStub{state: State{InstallID: "install-1", InstallCreatedAt: time.Now()}}
	emitter := &telemetryEmitterStub{}
	service := newTelemetryTestService(repo, emitter)

	for _, station := range []string{
		"library", "music", "sniff", "rss", "youtube", "custom",
		"library", "music", "another-custom",
	} {
		service.TrackStationOpened(context.Background(), station)
	}
	if got := len(service.pendingStationOpens); got != 6 || got > maxPendingStationOpens {
		t.Fatalf("pending station count = %d, want 6 within bound %d", got, maxPendingStationOpens)
	}
	if _, err := service.TrackAppLaunch(context.Background()); err != nil {
		t.Fatalf("track app launch: %v", err)
	}
	if got := len(emitter.signalsFor("XiaDown.Station.opened")); got != 6 {
		t.Fatalf("station signal count = %d, want 6", got)
	}
}

func TestServiceRetriesAppLaunchAfterRepositoryFailure(t *testing.T) {
	retryErr := errors.New("temporary telemetry state failure")
	repo := &telemetryStateRepoStub{
		state:           State{InstallID: "install-1", InstallCreatedAt: time.Now()},
		incrementErrors: []error{retryErr},
	}
	emitter := &telemetryEmitterStub{}
	service := newTelemetryTestService(repo, emitter)
	service.TrackStationOpened(context.Background(), "music")

	if _, err := service.TrackAppLaunch(context.Background()); !errors.Is(err, retryErr) {
		t.Fatalf("first launch error = %v, want %v", err, retryErr)
	}
	if _, err := service.TrackAppLaunch(context.Background()); err != nil {
		t.Fatalf("retry app launch: %v", err)
	}
	if repo.incrementCalls != 2 {
		t.Fatalf("repository increment calls = %d, want 2", repo.incrementCalls)
	}
	if got := len(emitter.signalsFor("TelemetryDeck.Session.started")); got != 1 {
		t.Fatalf("launch signal count = %d, want 1", got)
	}
	stationSignals := emitter.signalsFor("XiaDown.Station.opened")
	if len(stationSignals) != 1 || stationSignals[0].Payload["XiaDown.Station.name"] != "music" {
		t.Fatalf("queued station was not flushed after retry: %#v", stationSignals)
	}
}

func TestServiceRecordsEachStationOnceAfterLaunch(t *testing.T) {
	repo := &telemetryStateRepoStub{state: State{InstallID: "install-1", InstallCreatedAt: time.Now()}}
	emitter := &telemetryEmitterStub{}
	service := newTelemetryTestService(repo, emitter)
	if _, err := service.TrackAppLaunch(context.Background()); err != nil {
		t.Fatalf("track app launch: %v", err)
	}

	if !service.TrackStationOpened(context.Background(), "rss") {
		t.Fatal("first station should be accepted")
	}
	if !service.TrackStationOpened(context.Background(), "RSS") {
		t.Fatal("duplicate station should be treated as already accepted")
	}
	if got := len(emitter.signalsFor("XiaDown.Station.opened")); got != 1 {
		t.Fatalf("station signal count = %d, want 1", got)
	}
}

func TestServicePayloadUsesStandardPropertiesAndOnlyStationBusinessField(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	repo := &telemetryStateRepoStub{state: State{
		InstallID:                 "install-1",
		InstallCreatedAt:          base.Add(-48 * time.Hour),
		DistinctDaysUsed:          3,
		DistinctDaysUsedLastMonth: 2,
	}}
	emitter := &telemetryEmitterStub{}
	service := newTelemetryTestService(repo, emitter)
	service.now = func() time.Time { return base }

	if got, err := service.TrackAppLaunch(context.Background()); err != nil || got != 2 {
		t.Fatalf("track first launch = (%d, %v), want (2, nil)", got, err)
	}
	service.TrackStationOpened(context.Background(), "images/custom")

	launchPayload := emitter.signalsFor("TelemetryDeck.Session.started")[0].Payload
	if launchPayload["TelemetryDeck.AppInfo.version"] != "1.2.3" {
		t.Fatalf("unexpected app version: %#v", launchPayload)
	}
	if launchPayload["TelemetryDeck.AppInfo.buildNumber"] != "45" {
		t.Fatalf("unexpected build number: %#v", launchPayload)
	}
	if launchPayload["TelemetryDeck.RunContext.locale"] != "zh-CN" {
		t.Fatalf("unexpected locale: %#v", launchPayload)
	}
	for key := range launchPayload {
		if len(key) >= len("XiaDown.") && key[:len("XiaDown.")] == "XiaDown." {
			t.Fatalf("duplicate custom launch property remains: %s", key)
		}
		if len(key) >= len("TelemetryDeck.Calendar.") && key[:len("TelemetryDeck.Calendar.")] == "TelemetryDeck.Calendar." {
			t.Fatalf("calendar detail remains: %s", key)
		}
	}

	stationPayload := emitter.signalsFor("XiaDown.Station.opened")[0].Payload
	if stationPayload["XiaDown.Station.name"] != "other" {
		t.Fatalf("unexpected station: %#v", stationPayload)
	}
	for key := range stationPayload {
		if len(key) >= len("XiaDown.") && key[:len("XiaDown.")] == "XiaDown." && key != "XiaDown.Station.name" {
			t.Fatalf("unexpected station business property: %s", key)
		}
	}
}

func TestNormalizeStationNameUsesFixedLowCardinalityValues(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{input: "library", want: "library"},
		{input: " MUSIC ", want: "music"},
		{input: "sniff", want: "sniff"},
		{input: "rss", want: "rss"},
		{input: "YouTube", want: "youtube"},
		{input: "", want: ""},
		{input: "https://example.com/private", want: "other"},
	} {
		if got := normalizeStationName(test.input); got != test.want {
			t.Errorf("normalizeStationName(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestBootstrapMarksDevBuildsAsTestMode(t *testing.T) {
	repo := &telemetryStateRepoStub{state: State{InstallID: "install-1"}}
	service := NewService(repo, nil, telemetrySettingsStub{}, "app-123", "dev")
	bootstrap, err := service.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !bootstrap.TestMode {
		t.Fatal("dev build should use TelemetryDeck test mode")
	}
}
