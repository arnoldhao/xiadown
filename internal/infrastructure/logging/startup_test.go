package logging

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewStartupLoggerPersistsEarlyMessages(t *testing.T) {
	previousZap := zap.L()
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	t.Cleanup(func() {
		zap.ReplaceGlobals(previousZap)
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_DATA_HOME", "")

	logger, err := NewStartupLogger()
	if err != nil {
		t.Fatal(err)
	}
	zap.L().Info("startup-test-message")
	log.Print("standard-log-test-message")
	if err := logger.Sync(); err != nil {
		// Syncing stdout can return EINVAL on some platforms; the file sink is
		// synchronous and can still be verified below.
		t.Logf("logger sync: %v", err)
	}

	contents, err := os.ReadFile(logger.LogFilePath())
	if err != nil {
		t.Fatal(err)
	}
	logText := string(contents)
	for _, expected := range []string{"startup-test-message", "standard-log-test-message"} {
		if !strings.Contains(logText, expected) {
			t.Fatalf("startup log does not contain %q: %s", expected, logText)
		}
	}
}

func TestSlogBridgeFollowsZapGlobalReplacement(t *testing.T) {
	previous := zap.L()
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	firstCore, firstLogs := observer.New(zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(firstCore))
	bridge := NewSlogLogger()

	secondCore, secondLogs := observer.New(zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(secondCore))

	// The bridge must resolve zap.L at record time; this exercises the call
	// after the global has been replaced by the settings-backed app logger.
	bridge.Info("dynamic-slog-bridge")
	if firstLogs.Len() != 0 {
		t.Fatalf("bridge wrote to stale zap logger: %+v", firstLogs.All())
	}
	if secondLogs.FilterMessage("dynamic-slog-bridge").Len() != 1 {
		t.Fatalf("bridge did not write to current zap logger: %+v", secondLogs.All())
	}
}

func TestSlogBridgeKeepsWailsDebugOutOfApplicationDebugLog(t *testing.T) {
	previous := zap.L()
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	core, logs := observer.New(zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
	bridge := NewSlogLogger()

	bridge.Debug("binding payload that may contain user data", "args", `{"password":"secret"}`)
	bridge.Info("wails lifecycle message")

	if logs.FilterMessage("binding payload that may contain user data").Len() != 0 {
		t.Fatalf("Wails debug payload leaked into application log: %+v", logs.All())
	}
	if logs.FilterMessage("wails lifecycle message").Len() != 1 {
		t.Fatalf("Wails info message was not retained: %+v", logs.All())
	}
}

func TestSlogBridgeIgnoresOnlyKnownWailsRequestCancellation(t *testing.T) {
	previous := zap.L()
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	core, logs := observer.New(zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
	bridge := NewSlogLogger()
	const writePayloadMessage = "Unable to write json payload. Please report this to the Wails team!"

	bridge.Error(writePayloadMessage, "error", errors.New("request has been stopped"))
	if logs.Len() != 0 {
		t.Fatalf("known Wails request cancellation was logged as an error: %+v", logs.All())
	}

	bridge.Error(writePayloadMessage, "error", "disk write failed")
	if logs.FilterMessage(writePayloadMessage).FilterLevelExact(zapcore.ErrorLevel).Len() != 1 {
		t.Fatalf("real Wails payload error was suppressed: %+v", logs.All())
	}

	logs.TakeAll()
	bridge.Error("different Wails failure", "error", "request has been stopped")
	if logs.FilterMessage("different Wails failure").FilterLevelExact(zapcore.ErrorLevel).Len() != 1 {
		t.Fatalf("unrelated Wails error was suppressed: %+v", logs.All())
	}
}

func TestStandardLogWriterClassifiesMessages(t *testing.T) {
	previous := zap.L()
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	core, logs := observer.New(zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
	writer := standardLogWriter{}

	tests := []struct {
		name    string
		message string
		level   zapcore.Level
	}{
		{
			name:    "expected Vite proxy cancellation",
			message: "suppressing panic for copyResponse error in test; copy error: request has been stopped",
			level:   zapcore.DebugLevel,
		},
		{
			name:    "generic standard library diagnostic",
			message: `Unsolicited response received on idle HTTP channel starting with "HTTP/1.1 400 Bad Request"; err=<nil>`,
			level:   zapcore.WarnLevel,
		},
		{
			name:    "real standard library error",
			message: "WebView2 fatal error: runtime initialization failed",
			level:   zapcore.ErrorLevel,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logs.TakeAll()
			if _, err := writer.Write([]byte(test.message)); err != nil {
				t.Fatal(err)
			}
			entries := logs.All()
			if len(entries) != 1 {
				t.Fatalf("log entries = %d, want 1: %+v", len(entries), entries)
			}
			if entries[0].Level != test.level {
				t.Fatalf("log level = %s, want %s", entries[0].Level, test.level)
			}
			if got := entries[0].ContextMap()["message"]; got != test.message {
				t.Fatalf("logged message = %v, want %q", got, test.message)
			}
		})
	}
}

func TestStartupLoggerFallsBackToNextWritableDirectory(t *testing.T) {
	previous := zap.L()
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(root, "fallback")
	logger, failures := newStartupLoggerInDirectories([]string{
		filepath.Join(blocked, "logs"),
		fallback,
	})
	if logger == nil {
		t.Fatalf("fallback startup logger was not created: %v", failures)
	}
	if len(failures) != 1 {
		t.Fatalf("startup log failures = %d, want 1: %v", len(failures), failures)
	}
	want := filepath.Join(fallback, StartupLogFilename)
	if got := logger.LogFilePath(); got != want {
		t.Fatalf("startup log path = %q, want %q", got, want)
	}
}
