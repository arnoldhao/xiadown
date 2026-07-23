package main

import (
	"context"
	"embed"
	"log"
	"os"

	"go.uber.org/zap"

	"xiadown/internal/app"
	"xiadown/internal/infrastructure/logging"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	resetApplied, resetErr := app.ApplyPendingApplicationReset(context.Background())
	if resetErr != nil {
		log.Printf("apply pending application reset: %v", resetErr)
		return 1
	}
	if resetApplied {
		log.Printf("pending application reset completed")
	}

	startupLogger, loggerErr := logging.NewStartupLogger()
	if loggerErr != nil {
		log.Printf("initialize startup log: %v", loggerErr)
	} else {
		defer func() { _ = startupLogger.Sync() }()
		zap.L().Info("application launch started", zap.String("version", app.AppVersion))
	}

	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("unhandled panic", zap.Any("error", r), zap.Stack("stack"))
			if startupLogger == nil {
				log.Printf("unhandled panic: %v", r)
			}
			exitCode = 1
		}
	}()

	appliedPreparedUpdate, err := app.TryApplyPreparedUpdateOnLaunch(context.Background(), os.Args[1:])
	if err != nil {
		zap.L().Error("apply prepared update on launch", zap.Error(err))
		if startupLogger == nil {
			log.Printf("apply prepared update on launch: %v", err)
		}
		return 1
	}
	if appliedPreparedUpdate {
		return 0
	}

	application, err := app.CreateApplication(assets)
	if err != nil {
		zap.L().Error("create application", zap.Error(err))
		if startupLogger == nil {
			log.Printf("create application: %v", err)
		}
		return 1
	}

	if err := application.Run(); err != nil {
		zap.L().Error("run application", zap.Error(err))
		if startupLogger == nil {
			log.Printf("run application: %v", err)
		}
		return 1
	}
	return 0
}
