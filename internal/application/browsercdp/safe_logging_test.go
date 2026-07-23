package browsercdp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestBrowserRuntimeLogFieldsNeverExposeURLPathTokenOrRawError(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	const canary = "https://BROWSER-TOKEN@browser-canary.example/private?sig=QUERY-CANARY /Users/canary/Chrome"
	zap.New(core).Warn(
		"browser runtime test",
		zap.String("sourceRef", browserLogReference(canary)),
		browserErrorLogField(errors.New(canary)),
		browserRecoveredLogField("panic "+canary),
	)

	payload, err := json.Marshal(observed.All()[0].ContextMap())
	if err != nil {
		t.Fatal(err)
	}
	logged := observed.All()[0].Message + " " + string(payload)
	for _, forbidden := range []string{
		"BROWSER-TOKEN", "QUERY-CANARY", "browser-canary.example", "/private",
		"/Users/canary", "https://", "Chrome",
	} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("browser runtime log leaked %q: %s", forbidden, logged)
		}
	}
	if !strings.Contains(logged, `"errorRef":"`) || !strings.Contains(logged, `"sourceRef":"`) {
		t.Fatalf("browser runtime log lost opaque references: %s", logged)
	}
}
