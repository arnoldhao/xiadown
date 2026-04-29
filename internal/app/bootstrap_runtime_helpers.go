package app

import (
	"io/fs"
	"runtime"

	"go.uber.org/zap"
)

func loadAppIcon(assets fs.FS) []byte {
	data, err := fs.ReadFile(assets, appIconAssetPath(runtime.GOOS))
	if err != nil {
		zap.L().Debug("app icon not found, fallback to default icon", zap.Error(err))
		return nil
	}
	return data
}

func appIconAssetPath(goos string) string {
	if goos == "windows" {
		return "frontend/dist/appicon_windows.png"
	}
	return "frontend/dist/appicon.png"
}

func loadTrayIcon(assets fs.FS) []byte {
	data, err := fs.ReadFile(assets, trayIconAssetPath(runtime.GOOS))
	if err != nil {
		zap.L().Debug("tray icon not found, fallback to default icon", zap.Error(err))
		return nil
	}
	return data
}

func trayIconAssetPath(goos string) string {
	if goos == "windows" {
		return "frontend/dist/tray_windows.ico"
	}
	return "frontend/dist/tray.png"
}
