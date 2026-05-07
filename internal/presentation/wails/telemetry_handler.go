package wails

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	apptelemetry "xiadown/internal/application/telemetry"
)

const telemetrySignalEvent = "telemetry:signal"
const telemetryDefaultTarget = "https://nom.telemetrydeck.com/v2/"
const telemetryNativeClientName = "XiaDownNativeTelemetry"

var forbiddenTelemetryPayloadKeys = map[string]struct{}{
	"count":      {},
	"type":       {},
	"appID":      {},
	"clientUser": {},
	"__time":     {},
	"payload":    {},
	"platform":   {},
	"receivedAt": {},
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
	service       *apptelemetry.Service
	launchContext apptelemetry.AppLaunchContext
	httpClient    telemetryHTTPClientProvider
}

type telemetryHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type TelemetryPostSignalRequest struct {
	Target    string           `json:"target"`
	Body      []map[string]any `json:"body"`
	Keepalive bool             `json:"keepalive,omitempty"`
}

func NewTelemetryHandler(service *apptelemetry.Service, launchContext apptelemetry.AppLaunchContext, httpClient telemetryHTTPClientProvider) *TelemetryHandler {
	return &TelemetryHandler{
		service:       service,
		launchContext: launchContext,
		httpClient:    httpClient,
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
	return handler.service.TrackAppLaunch(ctx, handler.launchContext)
}

func (handler *TelemetryHandler) FlushSessionSummary(ctx context.Context) error {
	if handler == nil || handler.service == nil {
		return nil
	}
	return handler.service.FlushSessionSummary(ctx)
}

func (handler *TelemetryHandler) FlushSessionSummaryForShutdown(ctx context.Context) error {
	if handler == nil || handler.service == nil {
		return nil
	}
	bootstrap, err := handler.service.Bootstrap(ctx)
	if err != nil {
		return err
	}
	signal, ok, err := handler.service.FlushSessionSummarySignal(ctx)
	if err != nil || !ok {
		return err
	}
	body, err := telemetrySignalBody(bootstrap, signal)
	if err != nil {
		return err
	}
	return handler.postSignalBody(ctx, telemetryDefaultTarget, []map[string]any{body})
}

func (handler *TelemetryHandler) PostSignal(ctx context.Context, request TelemetryPostSignalRequest) error {
	if len(request.Body) == 0 {
		return nil
	}
	return handler.postSignalBody(ctx, request.Target, request.Body)
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

func telemetrySignalBody(bootstrap apptelemetry.Bootstrap, signal apptelemetry.Signal) (map[string]any, error) {
	if !bootstrap.Enabled || strings.TrimSpace(bootstrap.AppID) == "" || strings.TrimSpace(bootstrap.InstallID) == "" {
		return nil, fmt.Errorf("telemetry bootstrap is not enabled")
	}
	signalType := strings.TrimSpace(signal.Type)
	if signalType == "" {
		return nil, fmt.Errorf("telemetry signal type is empty")
	}
	appVersion := strings.TrimSpace(bootstrap.AppVersion)
	nameAndVersion := telemetryNativeClientName
	if appVersion != "" {
		nameAndVersion += " " + appVersion
	}
	body := map[string]any{
		"clientUser":             sha256Hex(strings.TrimSpace(bootstrap.InstallID)),
		"sessionID":              strings.TrimSpace(bootstrap.SessionID),
		"appID":                  strings.TrimSpace(bootstrap.AppID),
		"type":                   signalType,
		"telemetryClientVersion": nameAndVersion,
	}
	if bootstrap.TestMode {
		body["isTestMode"] = true
	}
	if signal.FloatValue != nil {
		body["floatValue"] = *signal.FloatValue
	}
	payload := sanitizedTelemetryPayload(signal.Payload)
	payload["TelemetryDeck.SDK.nameAndVersion"] = nameAndVersion
	payload["TelemetryDeck.SDK.name"] = telemetryNativeClientName
	if appVersion != "" {
		payload["TelemetryDeck.SDK.version"] = appVersion
	}
	if len(payload) > 0 {
		body["payload"] = payload
	}
	return body, nil
}

func sanitizedTelemetryPayload(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, forbidden := forbiddenTelemetryPayloadKeys[trimmedKey]; forbidden {
			continue
		}
		result[trimmedKey] = value
	}
	return result
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
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
