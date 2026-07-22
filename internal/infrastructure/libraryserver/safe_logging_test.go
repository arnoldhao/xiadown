package libraryserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestPairedAPIServerErrorLogNeverExposesURLPathTokenOrRawError(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	server, err := New(Config{Handler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	server.newHTTPServer().ErrorLog.Print(
		"panic serving https://PAIR-TOKEN@paired-canary.example/private/item?sig=QUERY-CANARY " +
			"from /Users/canary/secret.db",
	)

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
		"PAIR-TOKEN", "QUERY-CANARY", "paired-canary.example", "/private/item",
		"/Users/canary/secret.db", "https://", "panic serving",
	} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("paired API log leaked %q: %s", forbidden, logged)
		}
	}
	if !strings.Contains(logged, `"errorCode":"paired_api_server_error"`) ||
		!strings.Contains(logged, `"errorRef":"`) {
		t.Fatalf("paired API log lost fixed code or opaque reference: %s", logged)
	}
}
