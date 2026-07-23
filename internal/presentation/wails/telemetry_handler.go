package wails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	apptelemetry "xiadown/internal/application/telemetry"
)

const telemetrySignalEvent = "telemetry:signal"

const (
	maxTelemetryBodies         = 4
	maxTelemetryPayloadKeys    = 32
	maxTelemetryStringLength   = 256
	maxTelemetryClientIDLength = 512
)

var allowedTelemetrySignalTypes = map[string]struct{}{
	"TelemetryDeck.Session.started":                {},
	"TelemetryDeck.Acquisition.newInstallDetected": {},
	"XiaDown.Station.opened":                       {},
}

var allowedTelemetryBodyKeys = map[string]struct{}{
	"clientUser":             {},
	"sessionID":              {},
	"appID":                  {},
	"type":                   {},
	"telemetryClientVersion": {},
	"isTestMode":             {},
	"payload":                {},
}

var allowedTelemetryPayloadKeys = map[string]struct{}{
	"TelemetryDeck.AppInfo.version":                     {},
	"TelemetryDeck.AppInfo.buildNumber":                 {},
	"TelemetryDeck.AppInfo.versionAndBuildNumber":       {},
	"TelemetryDeck.Device.architecture":                 {},
	"TelemetryDeck.Device.modelName":                    {},
	"TelemetryDeck.Device.operatingSystem":              {},
	"TelemetryDeck.Device.platform":                     {},
	"TelemetryDeck.Device.timeZone":                     {},
	"TelemetryDeck.RunContext.isDebug":                  {},
	"TelemetryDeck.RunContext.targetEnvironment":        {},
	"TelemetryDeck.RunContext.locale":                   {},
	"TelemetryDeck.RunContext.language":                 {},
	"TelemetryDeck.Acquisition.firstSessionDate":        {},
	"TelemetryDeck.Retention.distinctDaysUsed":          {},
	"TelemetryDeck.Retention.distinctDaysUsedLastMonth": {},
	"TelemetryDeck.Retention.totalSessionsCount":        {},
	"TelemetryDeck.UserPreference.language":             {},
	"TelemetryDeck.UserPreference.region":               {},
	"TelemetryDeck.SDK.nameAndVersion":                  {},
	"TelemetryDeck.SDK.name":                            {},
	"TelemetryDeck.SDK.version":                         {},
	"XiaDown.Station.name":                              {},
}

type telemetrySignalEmitter struct {
	app *application.App
}

func NewTelemetrySignalEmitter(app *application.App) apptelemetry.Emitter {
	return &telemetrySignalEmitter{app: app}
}

func (emitter *telemetrySignalEmitter) Emit(signal apptelemetry.Signal) {
	if emitter == nil || emitter.app == nil {
		return
	}
	emitter.app.Event.Emit(telemetrySignalEvent, signal)
}

type TelemetryHandler struct {
	service          *apptelemetry.Service
	windowVisibility telemetryWindowVisibility
	httpClient       telemetryHTTPClientProvider
}

type telemetryWindowVisibility interface {
	MainWindowVisible() bool
}

type telemetryHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type TelemetryPostSignalRequest struct {
	Target string           `json:"target"`
	Body   []map[string]any `json:"body"`
}

func NewTelemetryHandler(service *apptelemetry.Service, windowVisibility telemetryWindowVisibility, httpClient telemetryHTTPClientProvider) *TelemetryHandler {
	return &TelemetryHandler{
		service:          service,
		windowVisibility: windowVisibility,
		httpClient:       httpClient,
	}
}

func (handler *TelemetryHandler) ServiceName() string {
	return "TelemetryHandler"
}

func (handler *TelemetryHandler) Bootstrap(ctx context.Context) (apptelemetry.Bootstrap, error) {
	if handler == nil || handler.service == nil {
		return apptelemetry.Bootstrap{}, nil
	}
	return handler.service.Bootstrap(ctx)
}

func (handler *TelemetryHandler) TrackAppLaunch(ctx context.Context) (int, error) {
	if handler == nil || handler.service == nil {
		return 0, nil
	}
	return handler.service.TrackAppLaunch(ctx)
}

// TrackStationOpened deliberately accepts only a station name. The telemetry
// service reduces it to a fixed low-cardinality enum before constructing the
// signal, so callers cannot attach identifiers, paths, URLs, or other payload.
func (handler *TelemetryHandler) TrackStationOpened(ctx context.Context, station string) bool {
	if handler == nil || handler.service == nil {
		return false
	}
	if handler.windowVisibility != nil && !handler.windowVisibility.MainWindowVisible() {
		return false
	}
	return handler.service.TrackStationOpened(ctx, station)
}

func (handler *TelemetryHandler) PostSignal(ctx context.Context, request TelemetryPostSignalRequest) error {
	if len(request.Body) == 0 {
		return nil
	}
	body, err := sanitizeTelemetryBodies(request.Body)
	if err != nil {
		return err
	}
	return handler.postSignalBody(ctx, request.Target, body)
}

func sanitizeTelemetryBodies(input []map[string]any) ([]map[string]any, error) {
	if len(input) == 0 {
		return nil, nil
	}
	if len(input) > maxTelemetryBodies {
		return nil, fmt.Errorf("telemetry batch exceeds limit")
	}
	result := make([]map[string]any, 0, len(input))
	for _, source := range input {
		if source == nil {
			return nil, fmt.Errorf("telemetry body is invalid")
		}
		for key := range source {
			if _, allowed := allowedTelemetryBodyKeys[key]; !allowed {
				return nil, fmt.Errorf("telemetry body contains an unsupported property")
			}
		}
		signalType, ok := boundedTelemetryString(source["type"], maxTelemetryStringLength)
		if !ok {
			return nil, fmt.Errorf("telemetry signal type is invalid")
		}
		if _, allowed := allowedTelemetrySignalTypes[signalType]; !allowed {
			return nil, fmt.Errorf("telemetry signal type is not allowed")
		}
		body := make(map[string]any, len(source))
		body["type"] = signalType
		for _, key := range []string{"clientUser", "sessionID", "appID", "telemetryClientVersion"} {
			value, valid := boundedTelemetryString(source[key], maxTelemetryClientIDLength)
			if !valid {
				return nil, fmt.Errorf("telemetry identity is invalid")
			}
			body[key] = value
		}
		if testMode, exists := source["isTestMode"]; exists {
			value, valid := testMode.(bool)
			if !valid {
				return nil, fmt.Errorf("telemetry test mode is invalid")
			}
			body["isTestMode"] = value
		}
		payload, err := sanitizeTelemetryPayload(source["payload"], signalType)
		if err != nil {
			return nil, err
		}
		body["payload"] = payload
		result = append(result, body)
	}
	return result, nil
}

func sanitizeTelemetryPayload(value any, signalType string) (map[string]any, error) {
	source, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("telemetry payload is invalid")
	}
	if len(source) > maxTelemetryPayloadKeys {
		return nil, fmt.Errorf("telemetry payload exceeds limit")
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		if _, allowed := allowedTelemetryPayloadKeys[key]; !allowed {
			return nil, fmt.Errorf("telemetry payload contains an unsupported property")
		}
		if key == "XiaDown.Station.name" {
			if signalType != "XiaDown.Station.opened" {
				return nil, fmt.Errorf("telemetry station property is invalid")
			}
			station, valid := boundedTelemetryString(value, 16)
			if !valid {
				return nil, fmt.Errorf("telemetry station is invalid")
			}
			result[key] = normalizeTelemetryStation(station)
			continue
		}
		primitive, valid := boundedTelemetryPrimitive(value)
		if !valid {
			return nil, fmt.Errorf("telemetry payload value is invalid")
		}
		result[key] = primitive
	}
	if signalType == "XiaDown.Station.opened" {
		if _, exists := result["XiaDown.Station.name"]; !exists {
			return nil, fmt.Errorf("telemetry station is missing")
		}
	}
	return result, nil
}

func boundedTelemetryString(value any, maxLength int) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != "" && len(text) <= maxLength
}

func boundedTelemetryPrimitive(value any) (any, bool) {
	switch typed := value.(type) {
	case string:
		if len(typed) > maxTelemetryStringLength {
			return nil, false
		}
		return typed, true
	case bool:
		return typed, true
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		return typed, !math.IsNaN(float64(typed)) && !math.IsInf(float64(typed), 0)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return typed, true
	default:
		return nil, false
	}
}

func normalizeTelemetryStation(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "library", "music", "sniff", "rss", "youtube":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "other"
	}
}

func (handler *TelemetryHandler) postSignalBody(ctx context.Context, rawTarget string, body []map[string]any) error {
	if len(body) == 0 {
		return nil
	}
	target := strings.TrimSpace(rawTarget)
	if !telemetryTargetAllowed(target) {
		return fmt.Errorf("telemetry target is not allowed")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := http.DefaultClient
	if handler != nil && handler.httpClient != nil {
		if provided := handler.httpClient.HTTPClient(); provided != nil {
			client = provided
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telemetry post failed: http %d", resp.StatusCode)
	}
	return nil
}

func telemetryTargetAllowed(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "nom.telemetrydeck.com" || strings.HasSuffix(host, ".telemetrydeck.com")
}
