package main

import (
	"context"
	"embed"
	"log"
	"os"

	"go.uber.org/zap"

	"xiadown/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("unhandled panic", zap.Any("error", r), zap.Stack("stack"))
			os.Exit(1)
		}
	}()

	appliedPreparedUpdate, err := app.TryApplyPreparedUpdateOnLaunch(context.Background(), os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if appliedPreparedUpdate {
		return
	}

	application, err := app.CreateApplication(assets)
	if err != nil {
		log.Fatal(err)
	}

	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
