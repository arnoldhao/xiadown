package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestStationBootstrapLogFieldsNeverExposeURLPathTokenOrRawError(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	const canary = "https://PAIR-TOKEN@paired-canary.example/private?sig=QUERY-CANARY /Users/canary/library.key"
	err := errors.New(canary)
	fields := []zap.Field{zap.String("cacheRef", safeStationLogReference("/Users/canary/RSS-cache"))}
	fields = append(fields, safeStationErrorLogFields("paired_api_test_failed", err)...)
	logger := zap.New(core)
	logger.Warn("paired API test", fields...)
	logger.Info("application started", safeApplicationStartedLogFields(
		"/Users/canary/private/logs",
		"debug",
		"zh-CN",
		"system",
		"manual",
		7,
		"https://GATEWAY-TOKEN@network-gateway.example/private?sig=GATEWAY-QUERY",
	)...)
	logger.Error("wails runtime error", safeStationErrorLogFields("wails_runtime_error", err)...)
	logger.Error("wails runtime panic", safeStationTextLogFields("wails_runtime_panic", canary)...)

	var logged strings.Builder
	for _, entry := range observed.All() {
		payload, marshalErr := json.Marshal(entry.ContextMap())
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		logged.WriteString(entry.Message)
		logged.WriteByte(' ')
		logged.Write(payload)
		logged.WriteByte('\n')
	}
	loggedText := logged.String()
	for _, forbidden := range []string{
		"PAIR-TOKEN", "QUERY-CANARY", "paired-canary.example", "/private",
		"/Users/canary", "https://", "library.key", "GATEWAY-TOKEN",
		"GATEWAY-QUERY", "network-gateway.example",
	} {
		if strings.Contains(loggedText, forbidden) {
			t.Fatalf("station bootstrap log leaked %q: %s", forbidden, loggedText)
		}
	}
	if !strings.Contains(loggedText, `"errorCode":"paired_api_test_failed"`) ||
		!strings.Contains(loggedText, `"errorCode":"wails_runtime_error"`) ||
		!strings.Contains(loggedText, `"errorCode":"wails_runtime_panic"`) ||
		!strings.Contains(loggedText, `"errorRef":"`) ||
		!strings.Contains(loggedText, `"cacheRef":"`) ||
		!strings.Contains(loggedText, `"logDirectoryRef":"`) ||
		!strings.Contains(loggedText, `"networkGatewayRef":"`) {
		t.Fatalf("station bootstrap log lost fixed code or opaque references: %s", loggedText)
	}
	startupFields := observed.All()[1].ContextMap()
	for _, removedKey := range []string{"logDir", "networkGateway"} {
		if _, exists := startupFields[removedKey]; exists {
			t.Fatalf("application started retained raw field %q: %#v", removedKey, startupFields)
		}
	}
}
