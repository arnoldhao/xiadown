package telemetry

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	settingsdto "xiadown/internal/application/settings/dto"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type State struct {
	InstallID                 string
	InstallCreatedAt          time.Time
	LaunchCount               int
	DistinctDaysUsed          int
	DistinctDaysUsedLastMonth int
	CompletedSessionCount     int
	TotalSessionSeconds       float64
	PreviousSessionSeconds    *float64
	FirstLibraryCompletedAt   *time.Time
}

type StateRepository interface {
	Ensure(ctx context.Context) (State, error)
	IncrementLaunchCount(ctx context.Context, at time.Time) (State, error)
	RecordSessionSummary(ctx context.Context, endedAt time.Time, durationSeconds float64) (State, error)
	MarkFirstLibraryCompleted(ctx context.Context, at time.Time) (State, bool, error)
}

type Signal struct {
	Type       string         `json:"type"`
	FloatValue *float64       `json:"floatValue,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
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

type AppLaunchContext struct {
	LaunchedByAutoStart bool
}

type Service struct {
	repo       StateRepository
	emitter    Emitter
	settings   SettingsReader
	appID      string
	appVersion string
	sessionID  string
	startedAt  time.Time
	now        func() time.Time

	mu       sync.Mutex
	launched bool
	flushed  bool
	session  sessionMetrics
	state    *State
	language string
}

type sessionMetrics struct {
	libraryCompleted     int
	connectorConnected   int
	dependencyInstalled  int
	updateReadyToRestart int
	operationIDs         map[string]struct{}
}

func NewService(repo StateRepository, emitter Emitter, settings SettingsReader, appID string, appVersion string) *Service {
	return &Service{
		repo:       repo,
		emitter:    emitter,
		settings:   settings,
		appID:      strings.TrimSpace(appID),
		appVersion: strings.TrimSpace(appVersion),
		sessionID:  uuid.NewString(),
		startedAt:  time.Now(),
		now:        time.Now,
		session: sessionMetrics{
			operationIDs: make(map[string]struct{}),
		},
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

func (service *Service) TrackAppLaunch(ctx context.Context, launch AppLaunchContext) (int, error) {
	if !service.Enabled() {
		return 0, nil
	}
	service.mu.Lock()
	if service.launched {
		service.mu.Unlock()
		return 0, nil
	}
	service.launched = true
	service.mu.Unlock()

	state, err := service.repo.IncrementLaunchCount(ctx, service.now())
	if err != nil {
		return 0, err
	}
	service.cacheState(state)

	payload := service.buildPayload(ctx, state)
	payload["XiaDown.App.launchCount"] = state.LaunchCount
	payload["XiaDown.App.launchOrdinalBucket"] = bucketLaunchOrdinal(state.LaunchCount)
	payload["XiaDown.App.launchedByAutoStart"] = launch.LaunchedByAutoStart
	payload["XiaDown.App.startMode"] = startMode(launch)
	payload["XiaDown.Install.firstLaunch"] = state.LaunchCount == 1

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
	return len(signals), nil
}

func (service *Service) TrackConnectorConnected(ctx context.Context, connectorType string) {
	if !service.Enabled() {
		return
	}
	normalizedConnectorType := strings.TrimSpace(connectorType)
	if normalizedConnectorType == "" {
		return
	}
	service.incrementCounter(func(metrics *sessionMetrics) {
		metrics.connectorConnected++
	})
	state, err := service.repo.Ensure(ctx)
	if err != nil {
		zap.L().Debug("telemetry: connector state ensure failed", zap.Error(err))
		return
	}
	service.cacheState(state)
	payload := service.buildPayload(ctx, state)
	payload["XiaDown.Setup.connectorType"] = normalizedConnectorType
	service.emitAsync(Signal{Type: "XiaDown.Setup.connectorConnected", Payload: payload})
}

func (service *Service) TrackDependencyInstalled(ctx context.Context, dependencyName string) {
	if !service.Enabled() {
		return
	}
	normalizedDependencyName := strings.TrimSpace(dependencyName)
	if normalizedDependencyName == "" {
		return
	}
	service.incrementCounter(func(metrics *sessionMetrics) {
		metrics.dependencyInstalled++
	})
	state, err := service.repo.Ensure(ctx)
	if err != nil {
		zap.L().Debug("telemetry: dependency state ensure failed", zap.Error(err))
		return
	}
	service.cacheState(state)
	payload := service.buildPayload(ctx, state)
	payload["XiaDown.Setup.dependency"] = normalizedDependencyName
	service.emitAsync(Signal{Type: "XiaDown.Setup.dependencyInstalled", Payload: payload})
}

func (service *Service) TrackLibraryOperationCompleted(ctx context.Context, operationID string, kind string) {
	if !service.Enabled() {
		return
	}
	normalizedOperationID := strings.TrimSpace(operationID)
	if normalizedOperationID == "" {
		return
	}
	if !service.markLibraryOperationCompleted(normalizedOperationID) {
		return
	}

	state, first, err := service.repo.MarkFirstLibraryCompleted(ctx, service.now())
	if err != nil {
		zap.L().Debug("telemetry: library completion state update failed", zap.Error(err))
		return
	}
	service.cacheState(state)
	if !first {
		return
	}
	payload := service.buildPayload(ctx, state)
	if normalizedKind := strings.TrimSpace(kind); normalizedKind != "" {
		payload["XiaDown.Library.operationKind"] = normalizedKind
	}
	service.emitAsync(Signal{Type: "XiaDown.Activation.firstLibraryCompleted", Payload: payload})
}

func (service *Service) TrackUpdateReadyToRestart(ctx context.Context, latestVersion string) {
	if !service.Enabled() {
		return
	}
	normalizedVersion := strings.TrimSpace(latestVersion)
	if normalizedVersion == "" {
		return
	}
	service.incrementCounter(func(metrics *sessionMetrics) {
		metrics.updateReadyToRestart++
	})
	state, err := service.repo.Ensure(ctx)
	if err != nil {
		zap.L().Debug("telemetry: update state ensure failed", zap.Error(err))
		return
	}
	service.cacheState(state)
	payload := service.buildPayload(ctx, state)
	payload["XiaDown.App.targetVersion"] = normalizedVersion
	service.emitAsync(Signal{Type: "XiaDown.App.updateReadyToRestart", Payload: payload})
}

func (service *Service) FlushSessionSummary(ctx context.Context) error {
	signal, ok, err := service.FlushSessionSummarySignal(ctx)
	if err != nil || !ok {
		return err
	}
	service.emit(signal)
	return nil
}

func (service *Service) FlushSessionSummarySignal(ctx context.Context) (Signal, bool, error) {
	if !service.Enabled() {
		return Signal{}, false, nil
	}

	service.mu.Lock()
	if service.flushed || !service.launched {
		service.mu.Unlock()
		return Signal{}, false, nil
	}
	service.flushed = true
	snapshot := service.session
	startedAt := service.startedAt
	service.mu.Unlock()

	durationSeconds := service.now().Sub(startedAt).Seconds()
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	durationSeconds = roundSeconds(durationSeconds)

	state, err := service.repo.RecordSessionSummary(ctx, service.now(), durationSeconds)
	if err != nil {
		return Signal{}, false, err
	}
	service.cacheState(state)

	payload := service.buildPayload(ctx, state)
	payload["TelemetryDeck.Signal.durationInSeconds"] = durationSeconds
	payload["XiaDown.Session.durationBucket"] = bucketSessionDuration(time.Duration(durationSeconds * float64(time.Second)))
	payload["XiaDown.Session.libraryCompletedBucket"] = bucketCount(snapshot.libraryCompleted)
	payload["XiaDown.Session.connectorConnectedBucket"] = bucketCount(snapshot.connectorConnected)
	payload["XiaDown.Session.dependencyInstalledBucket"] = bucketCount(snapshot.dependencyInstalled)
	payload["XiaDown.Session.updateReadyToRestartBucket"] = bucketCount(snapshot.updateReadyToRestart)
	return Signal{
		Type:       "XiaDown.Session.summaryRecorded",
		FloatValue: float64Ptr(durationSeconds),
		Payload:    payload,
	}, true, nil
}

func (service *Service) markLibraryOperationCompleted(operationID string) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, exists := service.session.operationIDs[operationID]; exists {
		return false
	}
	service.session.operationIDs[operationID] = struct{}{}
	service.session.libraryCompleted++
	return true
}

func (service *Service) incrementCounter(update func(metrics *sessionMetrics)) {
	if service == nil || update == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	update(&service.session)
}

func (service *Service) buildPayload(ctx context.Context, state State) map[string]any {
	appVersion := normalizeVersion(service.appVersion)
	buildNumber := buildNumberFromVersion(service.appVersion)
	platform := normalizedPlatform(runtime.GOOS)
	now := service.now()
	timeZone := service.timeZoneName(now)
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
		"XiaDown.App.version":                               appVersion,
		"XiaDown.App.channel":                               releaseChannel(service.appVersion),
		"XiaDown.App.isDebugBuild":                          isDebugBuild,
		"XiaDown.Platform.os":                               platform,
		"XiaDown.Platform.arch":                             runtime.GOARCH,
		"XiaDown.Locale.timeZone":                           timeZone,
		"XiaDown.Install.ageBucket":                         bucketInstallAge(now.Sub(state.InstallCreatedAt)),
	}
	for key, value := range calendarPayload(now) {
		payload[key] = value
	}
	if buildNumber != "" {
		payload["TelemetryDeck.AppInfo.buildNumber"] = buildNumber
		payload["TelemetryDeck.AppInfo.versionAndBuildNumber"] = appVersion + " " + buildNumber
		payload["XiaDown.App.buildNumber"] = buildNumber
		payload["XiaDown.App.versionAndBuildNumber"] = appVersion + " " + buildNumber
	}
	if state.CompletedSessionCount > 0 {
		payload["TelemetryDeck.Retention.averageSessionSeconds"] = roundSeconds(state.TotalSessionSeconds / float64(state.CompletedSessionCount))
	}
	if state.PreviousSessionSeconds != nil {
		payload["TelemetryDeck.Retention.previousSessionSeconds"] = roundSeconds(*state.PreviousSessionSeconds)
	}
	if locale := normalizeLocale(service.currentLanguage(ctx)); locale != "" {
		payload["TelemetryDeck.RunContext.locale"] = locale
		payload["XiaDown.Locale.language"] = locale
		if language := primaryLanguage(locale); language != "" {
			payload["TelemetryDeck.RunContext.language"] = language
			payload["TelemetryDeck.UserPreference.language"] = language
			payload["XiaDown.Locale.primaryLanguage"] = language
		}
		if region := regionFromLocale(locale); region != "" {
			payload["TelemetryDeck.UserPreference.region"] = region
			payload["XiaDown.Locale.region"] = region
		}
	}
	return payload
}

func (service *Service) currentLanguage(ctx context.Context) string {
	if service == nil {
		return ""
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

func (service *Service) timeZoneName(at time.Time) string {
	if service == nil {
		return ""
	}
	location := at.Location()
	if location == nil {
		return ""
	}
	return location.String()
}

func startMode(launch AppLaunchContext) string {
	if launch.LaunchedByAutoStart {
		return "autostart"
	}
	return "manual"
}

func (service *Service) emit(signal Signal) {
	if !service.Enabled() || service.emitter == nil {
		return
	}
	service.emitter.Emit(signal)
}

func (service *Service) emitAsync(signal Signal) {
	go func() {
		service.emit(signal)
	}()
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

func calendarPayload(value time.Time) map[string]any {
	if value.IsZero() {
		value = time.Now()
	}
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	_, week := value.ISOWeek()
	return map[string]any{
		"TelemetryDeck.Calendar.dayOfMonth":    value.Day(),
		"TelemetryDeck.Calendar.dayOfWeek":     weekday,
		"TelemetryDeck.Calendar.dayOfYear":     value.YearDay(),
		"TelemetryDeck.Calendar.weekOfYear":    week,
		"TelemetryDeck.Calendar.isWeekend":     value.Weekday() == time.Saturday || value.Weekday() == time.Sunday,
		"TelemetryDeck.Calendar.monthOfYear":   int(value.Month()),
		"TelemetryDeck.Calendar.quarterOfYear": ((int(value.Month()) - 1) / 3) + 1,
		"TelemetryDeck.Calendar.hourOfDay":     value.Hour() + 1,
	}
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

func bucketInstallAge(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	days := int(duration.Hours() / 24)
	switch {
	case days <= 0:
		return "day0"
	case days < 7:
		return "day1-6"
	case days < 30:
		return "day7-29"
	case days < 90:
		return "day30-89"
	default:
		return "day90+"
	}
}

func bucketLaunchOrdinal(launchCount int) string {
	switch {
	case launchCount <= 1:
		return "1"
	case launchCount <= 3:
		return "2-3"
	case launchCount <= 9:
		return "4-9"
	case launchCount <= 29:
		return "10-29"
	default:
		return "30+"
	}
}

func bucketSessionDuration(duration time.Duration) string {
	switch {
	case duration < time.Minute:
		return "lt1m"
	case duration < 5*time.Minute:
		return "1m-5m"
	case duration < 15*time.Minute:
		return "5m-15m"
	case duration < time.Hour:
		return "15m-60m"
	default:
		return "60m+"
	}
}

func bucketCount(value int) string {
	switch {
	case value <= 0:
		return "0"
	case value == 1:
		return "1"
	case value <= 3:
		return "2-3"
	case value <= 9:
		return "4-9"
	default:
		return "10+"
	}
}

func roundSeconds(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func float64Ptr(value float64) *float64 {
	return &value
}
