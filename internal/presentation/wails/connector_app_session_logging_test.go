package wails

import (
	"os"
	"strings"
	"testing"
)

func TestAppSessionWindowCloseDoesNotLogCookieInventory(t *testing.T) {
	sourceBytes, err := os.ReadFile("connector_app_session.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "func (session *connectorAppSessionWindow) captureCookiesAndClose()")
	end := strings.Index(source, "func (session *connectorAppSessionWindow) completeClose(")
	if start < 0 || end <= start {
		t.Fatalf("App Session close capture function is missing: start=%d end=%d", start, end)
	}
	captureSource := source[start:end]
	for _, forbidden := range []string{
		"log.Printf",
		"captured cookies before native close",
		"count=%d",
		`zap.Int("cookieCount"`,
	} {
		if strings.Contains(captureSource, forbidden) {
			t.Fatalf("App Session close capture logs cookie inventory through %q", forbidden)
		}
	}
	if !strings.Contains(captureSource, "zap.L().Debug") ||
		!strings.Contains(captureSource, "zap.Error(err)") {
		t.Fatal("App Session close capture failure is not a structured debug event")
	}
	if strings.Contains(source, `"log"`) {
		t.Fatal("App Session connector still routes messages through the standard-library warning adapter")
	}
}
