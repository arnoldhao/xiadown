package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPendingApplicationResetRunsBeforeStartupLogging(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mainSource := string(source)
	reset := strings.Index(mainSource, "app.ApplyPendingApplicationReset(context.Background())")
	startupLog := strings.Index(mainSource, "logging.NewStartupLogger()")
	preparedUpdate := strings.Index(mainSource, "app.TryApplyPreparedUpdateOnLaunch(")
	if reset < 0 || startupLog < 0 || preparedUpdate < 0 || !(reset < startupLog && reset < preparedUpdate) {
		t.Fatalf("unsafe startup reset order: reset=%d log=%d update=%d", reset, startupLog, preparedUpdate)
	}

	resetSource, err := os.ReadFile("application_reset.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resetSource), "appsessionvault.DeleteApplicationResetSecrets") {
		t.Fatal("whole-application reset does not clear current and legacy Session Vault secrets")
	}
}
