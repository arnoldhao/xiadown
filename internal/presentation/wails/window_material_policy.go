package wails

import "github.com/wailsapp/wails/v3/pkg/application"

// mainWindowMaterialMode is the product-level native material requested for
// the main window. It deliberately describes configuration, not runtime
// availability: Wails owns the OS-version checks and platform fallbacks.
type mainWindowMaterialMode uint8

const (
	mainWindowMaterialSolid mainWindowMaterialMode = iota
	mainWindowMaterialLiquidGlass
	mainWindowMaterialAcrylic
)

func resolveMainWindowMaterialMode(goos string) mainWindowMaterialMode {
	switch goos {
	case "darwin":
		return mainWindowMaterialLiquidGlass
	case "windows":
		return mainWindowMaterialAcrylic
	default:
		return mainWindowMaterialSolid
	}
}

type windowRuntimeBackgrounds struct {
	main    application.RGBA
	content application.RGBA
}

// Windows hosts embedded media as a child HWND below the main React WebView.
// Composition hosting puts that WebView in DirectComposition's topmost visual
// plane, so transparent pixels reveal the child while React content continues
// to compose above it. This is the Windows equivalent of placing the player
// WKWebView below the main WKWebView on macOS.
func applyMainWindowCompositionPolicy(options *application.WebviewWindowOptions, goos string) {
	if options == nil || goos != "windows" {
		return
	}
	options.Windows.WebView2CompositionHosting = true
}

// resolveWindowRuntimeBackgrounds preserves the theme colour for opaque
// content planes and keeps only the main webview transparent wherever a native
// backdrop (including Wails' native fallback) provides the base material. It is
// shared by initial option construction and later theme updates so
// SetBackgroundColour cannot accidentally cover the backdrop.
func resolveWindowRuntimeBackgrounds(goos string, themeColour application.RGBA) windowRuntimeBackgrounds {
	result := windowRuntimeBackgrounds{
		main:    themeColour,
		content: themeColour,
	}
	if resolveMainWindowMaterialMode(goos) != mainWindowMaterialSolid {
		result.main = application.RGBA{Alpha: 0}
	}
	return result
}

// applyMainWindowMaterialPolicy owns the canonical desktop backdrop recipe.
// The main window applies it directly; Settings wraps it so Surface Style can
// decide whether the WebView reveals or masks the same native underlay.
//
// Wails falls MacBackdropLiquidGlass back to a translucent window when
// NSGlassEffectView is unavailable. On Windows releases older than the
// system-backdrop API, its Acrylic path falls back to blur-behind.
func applyMainWindowMaterialPolicy(options *application.WebviewWindowOptions, goos string) {
	if options == nil {
		return
	}
	options.BackgroundColour = resolveWindowRuntimeBackgrounds(goos, options.BackgroundColour).main

	switch resolveMainWindowMaterialMode(goos) {
	case mainWindowMaterialLiquidGlass:
		options.Mac.Backdrop = application.MacBackdropLiquidGlass
		options.Mac.LiquidGlass = application.MacLiquidGlass{
			Style:    application.LiquidGlassStyleAutomatic,
			Material: application.NSVisualEffectMaterialAuto,
			// TintColor intentionally remains nil. Theme tint belongs to the
			// web ambient layer, while the native window material stays neutral.
		}
	case mainWindowMaterialAcrylic:
		options.BackgroundType = application.BackgroundTypeTranslucent
		options.Windows.BackdropType = application.Acrylic
	case mainWindowMaterialSolid:
		// Preserve the platform-neutral options assembled by buildWindowOptions.
	}
}

type settingsWindowSurfaceStyle string

const (
	settingsWindowSurfaceGlass    settingsWindowSurfaceStyle = "glass"
	settingsWindowSurfaceContrast settingsWindowSurfaceStyle = "contrast"
)

func resolveSettingsWindowSurfaceStyle(appearanceConfig map[string]any) settingsWindowSurfaceStyle {
	appearance, ok := appearanceConfig["appearance"].(map[string]any)
	if !ok {
		return settingsWindowSurfaceGlass
	}
	for _, key := range []string{"surfaceStyle", "sidebarStyle"} {
		value, ok := appearance[key].(string)
		if ok && value == string(settingsWindowSurfaceContrast) {
			return settingsWindowSurfaceContrast
		}
		if ok && value == string(settingsWindowSurfaceGlass) {
			return settingsWindowSurfaceGlass
		}
	}
	return settingsWindowSurfaceGlass
}

func resolveSettingsWindowBackground(
	goos string,
	surfaceStyle settingsWindowSurfaceStyle,
	themeColour application.RGBA,
) application.RGBA {
	if surfaceStyle != settingsWindowSurfaceContrast &&
		resolveMainWindowMaterialMode(goos) != mainWindowMaterialSolid {
		return application.RGBA{Alpha: 0}
	}
	return themeColour
}

// Settings installs the same native underlay capability as the main window so
// switching Surface Style at runtime never requires recreating the WebView.
// Contrast's canonical frontend canvas remains the cross-platform mask; the
// themed native colour is retained as an additional opaque host fallback.
func applySettingsWindowMaterialPolicy(
	options *application.WebviewWindowOptions,
	goos string,
	surfaceStyle settingsWindowSurfaceStyle,
) {
	if options == nil {
		return
	}
	themeColour := options.BackgroundColour
	applyMainWindowMaterialPolicy(options, goos)
	options.BackgroundColour = resolveSettingsWindowBackground(
		goos,
		surfaceStyle,
		themeColour,
	)
}
