package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestResourceSniffLogsNeverExposeURLPathTokenTitleOrRawError(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	const source = "https://viewer:LOG-TOKEN-CANARY@media-canary.example/private/watch.m3u8?sig=QUERY-CANARY"
	const executable = "/Applications/CANARY Browser.app/Contents/MacOS/CANARY"
	err := errors.New("open /Users/canary/Cookies while navigating " + source)
	fields := []zap.Field{
		zap.String("sourceRef", resourceSniffLogReference(source)),
		zap.String("executableRef", resourceSniffLogReference(executable)),
		zap.Bool("hadTitle", true),
	}
	fields = append(fields, resourceSniffErrorLogFields("resource_sniff_test_failed", err)...)
	zap.L().Warn("resource sniff canary", fields...)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("observed %d log entries, want 1", len(entries))
	}
	payload, marshalErr := json.Marshal(entries[0].ContextMap())
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	logged := entries[0].Message + " " + string(payload)
	for _, forbidden := range []string{
		"LOG-TOKEN-CANARY", "QUERY-CANARY", "media-canary.example", "/private/watch.m3u8",
		"/Users/canary/Cookies", "/Applications/CANARY Browser.app", "https://", "viewer:",
	} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("resource-sniff log leaked %q: %s", forbidden, logged)
		}
	}
	if !strings.Contains(logged, `"errorCode":"resource_sniff_test_failed"`) ||
		!strings.Contains(logged, `"errorRef":"`) ||
		!strings.Contains(logged, `"sourceRef":"`) ||
		!strings.Contains(logged, `"executableRef":"`) {
		t.Fatalf("resource-sniff log lost fixed codes or opaque references: %s", logged)
	}
}

func TestResourceSniffPanicLogFieldsHashRecoveredText(t *testing.T) {
	fields := resourceSniffPanicLogFields(
		"resource_sniff_cleanup_panic",
		"panic at https://panic-canary.example/private?token=PANIC-CANARY",
	)
	core, observed := observer.New(zapcore.DebugLevel)
	zap.New(core).Warn("resource sniff cleanup", fields...)
	payload, err := json.Marshal(observed.All()[0].ContextMap())
	if err != nil {
		t.Fatal(err)
	}
	logged := string(payload)
	for _, forbidden := range []string{"panic-canary.example", "/private", "PANIC-CANARY", "https://"} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("resource-sniff panic log leaked %q: %s", forbidden, logged)
		}
	}
}
