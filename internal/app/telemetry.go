package app

import (
	"strings"
)

var (
	// TelemetryDeckAppID is injected at build time via ldflags when telemetry is enabled.
	TelemetryDeckAppID = ""
)

type telemetryConfig struct {
	AppID string
}

func resolveTelemetryConfig() telemetryConfig {
	return telemetryConfig{
		AppID: strings.TrimSpace(TelemetryDeckAppID),
	}
}
