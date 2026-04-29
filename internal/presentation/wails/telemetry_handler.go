package wails

import (
	"bytes"
	"context"
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

func (handler *TelemetryHandler) PostSignal(ctx context.Context, request TelemetryPostSignalRequest) error {
	if len(request.Body) == 0 {
		return nil
	}
	target := strings.TrimSpace(request.Target)
	if !telemetryTargetAllowed(target) {
		return fmt.Errorf("telemetry target is not allowed")
	}
	payload, err := json.Marshal(request.Body)
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
