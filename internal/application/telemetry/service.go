package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	settingsdto "xiadown/internal/application/settings/dto"

	"github.com/google/uuid"
)

type State struct {
	InstallID                 string
	InstallCreatedAt          time.Time
	LaunchCount               int
	DistinctDaysUsed          int
	DistinctDaysUsedLastMonth int
}

type StateRepository interface {
	Ensure(ctx context.Context) (State, error)
	IncrementLaunchCount(ctx context.Context, at time.Time) (State, error)
}

type Signal struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Bootstrap struct {
	Enabled    bool   `json:"enabled"`
	AppID      string `json:"appId,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
	InstallID  string `json:"installId,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
	TestMode   bool   `json:"testMode"`
}

type Emitter interface {
	Emit(signal Signal)
}

type SettingsReader interface {
	GetSettings(ctx context.Context) (settingsdto.Settings, error)
}

type Service struct {
	repo       StateRepository
	emitter    Emitter
	settings   SettingsReader
	appID      string
	appVersion string
	sessionID  string
	now        func() time.Time

	mu                  sync.Mutex
	launched            bool
	launching           bool
	launchAttemptDone   chan struct{}
	state               *State
	language            string
	pendingStationOpens []string
	openedStations      map[string]struct{}
}

const maxPendingStationOpens = 16

func NewService(repo StateRepository, emitter Emitter, settings SettingsReader, appID string, appVersion string) *Service {
	return &Service{
		repo:           repo,
		emitter:        emitter,
		settings:       settings,
		appID:          strings.TrimSpace(appID),
		appVersion:     strings.TrimSpace(appVersion),
		sessionID:      uuid.NewString(),
		now:            time.Now,
		openedStations: make(map[string]struct{}),
	}
}

func (service *Service) Enabled() bool {
	return service != nil && service.repo != nil && strings.TrimSpace(service.appID) != ""
}

func (service *Service) Bootstrap(ctx context.Context) (Bootstrap, error) {
	if !service.Enabled() {
		return Bootstrap{
			Enabled:    false,
			AppVersion: normalizeVersion(service.appVersion),
			TestMode:   releaseChannel(service.appVersion) == "dev",
		}, nil
	}
	state, err := service.resolveState(ctx)
	if err != nil {
		return Bootstrap{}, err
	}
	return Bootstrap{
		Enabled:    true,
		AppID:      service.appID,
		AppVersion: normalizeVersion(service.appVersion),
		InstallID:  state.InstallID,
		SessionID:  service.sessionID,
		TestMode:   releaseChannel(service.appVersion) == "dev",
	}, nil
}

func (service *Service) TrackAppLaunch(ctx context.Context) (int, error) {
	if !service.Enabled() {
		return 0, nil
	}
	for {
		service.mu.Lock()
		if service.launched {
			service.mu.Unlock()
			return 0, nil
		}
		if !service.launching {
			service.launching = true
			service.launchAttemptDone = make(chan struct{})
			service.mu.Unlock()
			break
		}
		done := service.launchAttemptDone
		service.mu.Unlock()
		select {
		case <-done:
			continue
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	state, err := service.repo.IncrementLaunchCount(ctx, service.now())
	if err != nil {
		service.abortLaunchAttempt()
		return 0, err
	}
	service.cacheState(state)

	payload := service.buildPayload(ctx, state)
	signals := []Signal{{
		Type:    "TelemetryDeck.Session.started",
		Payload: payload,
	}}
	if state.LaunchCount == 1 {
		signals = append(signals, Signal{
			Type:    "TelemetryDeck.Acquisition.newInstallDetected",
			Payload: payload,
		})
	}
	for _, signal := range signals {
		service.emit(signal)
	}
	service.flushPendingStationOpens(ctx, state)
	return len(signals), nil
}

// TrackStationOpened records only the coarse product station. Calls that arrive
// before the frontend telemetry listener has subscribed are retained in a
// small bounded queue and flushed after TrackAppLaunch emits the launch signal.
func (service *Service) TrackStationOpened(ctx context.Context, station string) bool {
	if !service.Enabled() {
		return false
	}
	normalizedStation := normalizeStationName(station)
	if normalizedStation == "" {
		return false
	}

	service.mu.Lock()
	if _, alreadyOpened := service.openedStations[normalizedStation]; alreadyOpened {
		service.mu.Unlock()
		return true
	}
	service.openedStations[normalizedStation] = struct{}{}
	if !service.launched {
		service.enqueuePendingStationOpenLocked(normalizedStation)
		service.mu.Unlock()
		return true
	}
	service.mu.Unlock()

	state, err := service.resolveState(ctx)
	if err != nil {
		service.mu.Lock()
		delete(service.openedStations, normalizedStation)
		service.mu.Unlock()
		return false
	}
	service.emit(service.stationOpenedSignal(ctx, state, normalizedStation))
	return true
}

func (service *Service) flushPendingStationOpens(ctx context.Context, state State) {
	for {
		service.mu.Lock()
		if len(service.pendingStationOpens) == 0 {
			service.launched = true
			service.finishLaunchAttemptLocked()
			service.mu.Unlock()
			return
		}
		pending := append([]string(nil), service.pendingStationOpens...)
		service.pendingStationOpens = service.pendingStationOpens[:0]
		service.mu.Unlock()

		for _, station := range pending {
			service.emit(service.stationOpenedSignal(ctx, state, station))
		}
	}
}

func (service *Service) stationOpenedSignal(ctx context.Context, state State, station string) Signal {
	payload := service.buildPayload(ctx, state)
	payload["XiaDown.Station.name"] = station
	return Signal{Type: "XiaDown.Station.opened", Payload: payload}
}

func (service *Service) enqueuePendingStationOpenLocked(station string) {
	if len(service.pendingStationOpens) >= maxPendingStationOpens {
		dropped := service.pendingStationOpens[0]
		copy(service.pendingStationOpens, service.pendingStationOpens[1:])
		service.pendingStationOpens[len(service.pendingStationOpens)-1] = station
		delete(service.openedStations, dropped)
		return
	}
	service.pendingStationOpens = append(service.pendingStationOpens, station)
}

func (service *Service) abortLaunchAttempt() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.finishLaunchAttemptLocked()
}

func (service *Service) finishLaunchAttemptLocked() {
	service.launching = false
	if service.launchAttemptDone != nil {
		close(service.launchAttemptDone)
		service.launchAttemptDone = nil
	}
}

func normalizeStationName(station string) string {
	switch normalized := strings.ToLower(strings.TrimSpace(station)); normalized {
	case "library", "music", "sniff", "rss", "youtube":
		return normalized
	case "":
		return ""
	default:
		return "other"
	}
}

func (service *Service) buildPayload(ctx context.Context, state State) map[string]any {
	appVersion := normalizeVersion(service.appVersion)
	buildNumber := buildNumberFromVersion(service.appVersion)
	platform := normalizedPlatform(runtime.GOOS)
	now := service.now()
	timeZoneOffset := utcOffsetName(now)
	isDebugBuild := releaseChannel(service.appVersion) == "dev"
	distinctDaysUsed := state.DistinctDaysUsed
	if distinctDaysUsed <= 0 && state.LaunchCount > 0 {
		distinctDaysUsed = 1
	}
	distinctDaysUsedLastMonth := state.DistinctDaysUsedLastMonth
	if distinctDaysUsedLastMonth <= 0 && state.LaunchCount > 0 {
		distinctDaysUsedLastMonth = 1
	}

	payload := map[string]any{
		"TelemetryDeck.AppInfo.version":                     appVersion,
		"TelemetryDeck.Device.architecture":                 runtime.GOARCH,
		"TelemetryDeck.Device.modelName":                    desktopModelName(runtime.GOOS),
		"TelemetryDeck.Device.operatingSystem":              platform,
		"TelemetryDeck.Device.platform":                     platform,
		"TelemetryDeck.Device.timeZone":                     timeZoneOffset,
		"TelemetryDeck.RunContext.isDebug":                  isDebugBuild,
		"TelemetryDeck.RunContext.targetEnvironment":        "desktop",
		"TelemetryDeck.Acquisition.firstSessionDate":        dateInLocation(state.InstallCreatedAt, now.Location()),
		"TelemetryDeck.Retention.distinctDaysUsed":          distinctDaysUsed,
		"TelemetryDeck.Retention.distinctDaysUsedLastMonth": distinctDaysUsedLastMonth,
		"TelemetryDeck.Retention.totalSessionsCount":        state.LaunchCount,
	}
	if buildNumber != "" {
		payload["TelemetryDeck.AppInfo.buildNumber"] = buildNumber
		payload["TelemetryDeck.AppInfo.versionAndBuildNumber"] = appVersion + " " + buildNumber
	}
	if locale := normalizeLocale(service.currentLanguage(ctx)); locale != "" {
		payload["TelemetryDeck.RunContext.locale"] = locale
		if language := primaryLanguage(locale); language != "" {
			payload["TelemetryDeck.RunContext.language"] = language
			payload["TelemetryDeck.UserPreference.language"] = language
		}
		if region := regionFromLocale(locale); region != "" {
			payload["TelemetryDeck.UserPreference.region"] = region
		}
	}
	return payload
}

func (service *Service) currentLanguage(ctx context.Context) string {
	if service == nil {
		return ""
	}
	service.mu.Lock()
	cachedLanguage := service.language
	service.mu.Unlock()
	if cachedLanguage != "" {
		return cachedLanguage
	}
	if service.settings != nil {
		settings, err := service.settings.GetSettings(ctx)
		if err == nil {
			language := strings.TrimSpace(settings.Language)
			if language != "" {
				service.mu.Lock()
				service.language = language
				service.mu.Unlock()
				return language
			}
		}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.language
}

func (service *Service) emit(signal Signal) {
	if !service.Enabled() || service.emitter == nil {
		return
	}
	service.emitter.Emit(signal)
}

func (service *Service) resolveState(ctx context.Context) (State, error) {
	if service == nil {
		return State{}, fmt.Errorf("telemetry service is nil")
	}
	service.mu.Lock()
	if service.state != nil {
		cached := *service.state
		service.mu.Unlock()
		return cached, nil
	}
	service.mu.Unlock()
	state, err := service.repo.Ensure(ctx)
	if err != nil {
		return State{}, err
	}
	service.cacheState(state)
	return state, nil
}

func (service *Service) cacheState(state State) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	cached := state
	service.state = &cached
}

func buildNumberFromVersion(version string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	_, buildNumber, found := strings.Cut(trimmed, "+")
	if !found {
		return ""
	}
	return strings.TrimSpace(buildNumber)
}

func normalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return "dev"
	}
	if baseVersion, _, found := strings.Cut(strings.TrimPrefix(trimmed, "v"), "+"); found {
		return strings.TrimSpace(baseVersion)
	}
	return strings.TrimPrefix(trimmed, "v")
}

func normalizeLocale(locale string) string {
	return strings.ReplaceAll(strings.TrimSpace(locale), "_", "-")
}

func primaryLanguage(locale string) string {
	normalized := normalizeLocale(locale)
	if normalized == "" {
		return ""
	}
	language, _, found := strings.Cut(normalized, "-")
	if !found {
		return normalized
	}
	return strings.TrimSpace(language)
}

func regionFromLocale(locale string) string {
	normalized := normalizeLocale(locale)
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "-")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
}

func dateInLocation(value time.Time, location *time.Location) string {
	if value.IsZero() {
		return ""
	}
	if location == nil {
		location = time.UTC
	}
	return value.In(location).Format("2006-01-02")
}

func utcOffsetName(value time.Time) string {
	if value.IsZero() {
		value = time.Now()
	}
	_, offsetSeconds := value.Zone()
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	if minutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, hours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, hours, minutes)
}

func normalizedPlatform(goos string) string {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return goos
	}
}

func desktopModelName(goos string) string {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		return "Mac"
	case "windows":
		return "Windows PC"
	case "linux":
		return "Linux PC"
	default:
		return strings.TrimSpace(goos)
	}
}

func releaseChannel(version string) string {
	normalized := strings.ToLower(normalizeVersion(version))
	switch {
	case normalized == "dev":
		return "dev"
	case strings.Contains(normalized, "alpha"):
		return "alpha"
	case strings.Contains(normalized, "beta"):
		return "beta"
	case strings.Contains(normalized, "rc"):
		return "rc"
	default:
		return "stable"
	}
}
