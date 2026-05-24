//go:build windows

package wails

import "github.com/wailsapp/wails/v3/pkg/application"

func applyTrayMiniPlayerWindowShape(application.Window) {
	// Windows uses the default rectangular tray window. Rounded native shaping
	// caused positioning and layered-window artifacts with Wails v3/WebView2.
}
