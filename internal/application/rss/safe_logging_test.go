package rss

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRSSLogFieldsNeverExposeURLPathTokenOrRawError(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	const source = "https://reader:LOG-TOKEN-CANARY@source-canary.example/private/feed.xml?sig=QUERY-CANARY"
	err := errors.New("open /Users/canary/private.db while fetching " + source)
	zap.L().Debug("refresh RSS subscriptions", rssSafeLogErrorFields(err)...)

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
		"LOG-TOKEN-CANARY", "QUERY-CANARY", "source-canary.example", "/private/feed.xml",
		"/Users/canary/private.db", "https://", "reader:",
	} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("RSS log leaked %q: %s", forbidden, logged)
		}
	}
	if !strings.Contains(logged, `"errorCode":"rss_operation_failed"`) ||
		!strings.Contains(logged, `"errorRef":"`) {
		t.Fatalf("RSS log lost fixed code or opaque reference: %s", logged)
	}

	feedReference := redactedFeedURL(source)
	if !strings.HasPrefix(feedReference, "<feed-ref:") {
		t.Fatalf("feed reference = %q", feedReference)
	}
	for _, forbidden := range []string{"source-canary.example", "/private/feed.xml", "LOG-TOKEN-CANARY"} {
		if strings.Contains(feedReference, forbidden) {
			t.Fatalf("feed reference leaked %q: %q", forbidden, feedReference)
		}
	}
}
