package logging

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"xiadown/internal/domain/settings"
)

const StartupLogFilename = "startup.log"

// NewStartupLogger installs a persistent logger before any application state is
// opened. This is especially important for Windows GUI builds, where stderr is
// not visible and WebView2 may terminate the process before the regular logger
// has been configured from user settings.
func NewStartupLogger() (*Logger, error) {
	directories := make([]string, 0, 3)
	var candidateErrors []error
	if directory, err := DefaultLogDir(); err == nil {
		directories = append(directories, directory)
	} else {
		candidateErrors = append(candidateErrors, err)
	}
	if cacheDirectory, err := os.UserCacheDir(); err == nil {
		directories = append(directories, filepath.Join(cacheDirectory, "xiadown", "logs"))
	} else {
		candidateErrors = append(candidateErrors, fmt.Errorf("get user cache directory: %w", err))
	}
	if temporaryDirectory := os.TempDir(); temporaryDirectory != "" {
		directories = append(directories, filepath.Join(temporaryDirectory, "xiadown", "logs"))
	}

	logger, openErrors := newStartupLoggerInDirectories(directories)
	candidateErrors = append(candidateErrors, openErrors...)
	if logger == nil {
		candidateErrors = append(candidateErrors, errors.New("no writable startup log directory"))
		return nil, errors.Join(candidateErrors...)
	}

	// Wails' embedded WebView2 implementation uses the standard library logger.
	// Route it through the current zap global so it follows the later transition
	// from startup.log to the user-configured app.log.
	log.SetFlags(0)
	log.SetOutput(standardLogWriter{})
	if len(candidateErrors) > 0 {
		zap.L().Warn(
			"startup log is using a fallback directory",
			zap.String("logFile", logger.LogFilePath()),
			zap.Errors("failures", candidateErrors),
		)
	}
	return logger, nil
}

func newStartupLoggerInDirectories(directories []string) (*Logger, []error) {
	seen := make(map[string]struct{}, len(directories))
	var failures []error
	for _, directory := range directories {
		directory = strings.TrimSpace(directory)
		if directory == "" {
			continue
		}
		directory = filepath.Clean(directory)
		if _, exists := seen[directory]; exists {
			continue
		}
		seen[directory] = struct{}{}

		logger, err := NewLogger(Config{
			Directory:  directory,
			Filename:   StartupLogFilename,
			Level:      settings.LogLevelInfo,
			MaxSizeMB:  5,
			MaxBackups: 2,
			MaxAgeDays: 7,
			Compress:   true,
		})
		if err == nil {
			return logger, failures
		}
		failures = append(failures, fmt.Errorf("open startup log in %q: %w", directory, err))
	}
	return nil, failures
}

type standardLogWriter struct{}

func (standardLogWriter) Write(payload []byte) (int, error) {
	message := strings.TrimSpace(string(payload))
	if message == "" {
		return len(payload), nil
	}

	fields := []zap.Field{zap.String("message", message)}
	switch {
	case isBenignReverseProxyCancellation(message):
		// WebKit cancels superseded Vite requests while dependencies are being
		// optimised or a page is hot-reloaded. net/http's ReverseProxy reports those
		// expected cancellations through the process-wide standard logger.
		// Treating every one as an error adds an error-level stack trace and can
		// turn a normal dev reload into megabytes of misleading diagnostics.
		zap.L().Debug("standard library log", fields...)
	case isStandardLibraryError(message):
		zap.L().Error("standard library log", fields...)
	default:
		// The standard logger has no severity metadata. Unknown messages are
		// still persisted, but warning is a safer default than manufacturing an
		// error (and its stack trace) for informational net/http diagnostics.
		zap.L().Warn("standard library log", fields...)
	}
	return len(payload), nil
}

func isBenignReverseProxyCancellation(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if !strings.Contains(lower, "suppressing panic for copyresponse error in test; copy error:") {
		return false
	}
	for _, marker := range []string{
		"request has been stopped",
		"context canceled",
		"context cancelled",
		"operation canceled",
		"operation cancelled",
		"broken pipe",
		"connection reset by peer",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isStandardLibraryError(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "panic") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "error")
}

// NewSlogLogger bridges Wails' slog output to the current zap global. Looking
// up zap.L for every record keeps Wails on the startup logger during bootstrap
// and automatically moves it to app.log once settings have been loaded. Wails
// debug records include every asset request and binding arguments/results, so
// they are kept out of the user-controlled application debug log by default.
func NewSlogLogger() *slog.Logger {
	return slog.New(&dynamicZapHandler{minimumLevel: slog.LevelInfo})
}

type dynamicZapHandler struct {
	fields       []zap.Field
	group        string
	minimumLevel slog.Level
}

func (handler *dynamicZapHandler) Enabled(_ context.Context, level slog.Level) bool {
	if level < handler.minimumLevel {
		return false
	}
	return zap.L().Core().Enabled(zapLevel(level))
}

func (handler *dynamicZapHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Level < handler.minimumLevel {
		return nil
	}
	if isBenignWailsRequestCancellation(record) {
		return nil
	}
	fields := append([]zap.Field(nil), handler.fields...)
	record.Attrs(func(attr slog.Attr) bool {
		fields = appendSlogAttr(fields, handler.group, attr)
		return true
	})
	zap.L().Log(zapLevel(record.Level), record.Message, fields...)
	return nil
}

func isBenignWailsRequestCancellation(record slog.Record) bool {
	if record.Message != "Unable to write json payload. Please report this to the Wails team!" {
		return false
	}

	cancelled := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "error" {
			return true
		}
		value := attr.Value.Resolve()
		var message string
		if value.Kind() == slog.KindString {
			message = value.String()
		} else {
			message = fmt.Sprint(value.Any())
		}
		cancelled = strings.EqualFold(strings.TrimSpace(message), "request has been stopped")
		return !cancelled
	})
	return cancelled
}

func (handler *dynamicZapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &dynamicZapHandler{
		fields:       append([]zap.Field(nil), handler.fields...),
		group:        handler.group,
		minimumLevel: handler.minimumLevel,
	}
	for _, attr := range attrs {
		next.fields = appendSlogAttr(next.fields, next.group, attr)
	}
	return next
}

func (handler *dynamicZapHandler) WithGroup(name string) slog.Handler {
	next := &dynamicZapHandler{
		fields:       append([]zap.Field(nil), handler.fields...),
		group:        handler.group,
		minimumLevel: handler.minimumLevel,
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return next
	}
	if next.group == "" {
		next.group = name
	} else {
		next.group += "." + name
	}
	return next
}

func appendSlogAttr(fields []zap.Field, group string, attr slog.Attr) []zap.Field {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		nestedGroup := group
		if attr.Key != "" {
			if nestedGroup == "" {
				nestedGroup = attr.Key
			} else {
				nestedGroup += "." + attr.Key
			}
		}
		for _, nested := range attr.Value.Group() {
			fields = appendSlogAttr(fields, nestedGroup, nested)
		}
		return fields
	}
	key := attr.Key
	if group != "" {
		if key == "" {
			key = group
		} else {
			key = group + "." + key
		}
	}
	if key == "" {
		return fields
	}
	return append(fields, zap.Any(key, attr.Value.Any()))
}

func zapLevel(level slog.Level) zapcore.Level {
	switch {
	case level >= slog.LevelError:
		return zapcore.ErrorLevel
	case level >= slog.LevelWarn:
		return zapcore.WarnLevel
	case level <= slog.LevelDebug:
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}
