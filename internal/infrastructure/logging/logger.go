package logging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"xiadown/internal/domain/settings"
)

type Config struct {
	Directory  string
	Filename   string
	Level      settings.LogLevel
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	Encoding   string
}

type Logger struct {
	logger   *zap.Logger
	level    zap.AtomicLevel
	logDir   string
	filename string
}

func NewLogger(config Config) (*Logger, error) {
	if config.Directory == "" {
		return nil, errors.New("log directory is required")
	}

	if err := os.MkdirAll(config.Directory, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	filename := config.Filename
	if filename == "" {
		filename = "app.log"
	}

	maxSize := config.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 50
	}
	maxBackups := config.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 5
	}
	maxAge := config.MaxAgeDays
	if maxAge <= 0 {
		maxAge = 7
	}
	if config.MaxSizeMB > 0 {
		maxSize = config.MaxSizeMB
	}
	if config.MaxBackups > 0 {
		maxBackups = config.MaxBackups
	}
	if config.MaxAgeDays > 0 {
		maxAge = config.MaxAgeDays
	}
	compress := config.Compress
	if !config.Compress && config.MaxBackups == 0 && config.MaxAgeDays == 0 && config.MaxSizeMB == 0 {
		compress = true
	}

	level := zap.NewAtomicLevel()
	if err := setLevel(&level, config.Level); err != nil {
		return nil, err
	}

	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(config.Directory, filename),
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	})

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	var encoder zapcore.Encoder
	if config.Encoding == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(writer, zapcore.AddSync(os.Stdout)),
		level,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	zap.ReplaceGlobals(logger)

	return &Logger{
		logger:   logger,
		level:    level,
		logDir:   config.Directory,
		filename: filename,
	}, nil
}

func (l *Logger) Sync() error {
	if l == nil || l.logger == nil {
		return nil
	}
	return l.logger.Sync()
}

func (l *Logger) LogDir() string {
	return l.logDir
}

func (l *Logger) LogFilePath() string {
	if l == nil {
		return ""
	}
	if l.logDir == "" || l.filename == "" {
		return ""
	}
	return filepath.Join(l.logDir, l.filename)
}

func (l *Logger) SetLevel(level settings.LogLevel) error {
	return setLevel(&l.level, level)
}

func setLevel(target *zap.AtomicLevel, level settings.LogLevel) error {
	if target == nil {
		return errors.New("nil log level target")
	}
	zapLevel, err := parseLevel(level)
	if err != nil {
		return err
	}
	target.SetLevel(zapLevel)
	return nil
}

func parseLevel(level settings.LogLevel) (zapcore.Level, error) {
	switch level {
	case settings.LogLevelDebug:
		return zapcore.DebugLevel, nil
	case settings.LogLevelInfo:
		return zapcore.InfoLevel, nil
	case settings.LogLevelWarn:
		return zapcore.WarnLevel, nil
	case settings.LogLevelError:
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}
