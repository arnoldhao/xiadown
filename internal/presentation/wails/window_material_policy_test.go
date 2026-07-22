package wails

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	settingsdto "xiadown/internal/application/settings/dto"
)

func TestResolveMainWindowMaterialMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos string
		want mainWindowMaterialMode
	}{
		{goos: "darwin", want: mainWindowMaterialLiquidGlass},
		{goos: "windows", want: mainWindowMaterialAcrylic},
		{goos: "linux", want: mainWindowMaterialSolid},
		{goos: "freebsd", want: mainWindowMaterialSolid},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()
			if got := resolveMainWindowMaterialMode(tt.goos); got != tt.want {
				t.Fatalf("resolveMainWindowMaterialMode(%q) = %v, want %v", tt.goos, got, tt.want)
			}
		})
	}
}

func TestApplyMainWindowMaterialPolicyUsesNeutralLiquidGlassOnMacOS(t *testing.T) {
	t.Parallel()

	options := baselineWindowMaterialOptions()
	applyMainWindowMaterialPolicy(&options, "darwin")

	if options.Mac.Backdrop != application.MacBackdropLiquidGlass {
		t.Fatalf("Mac.Backdrop = %v, want MacBackdropLiquidGlass", options.Mac.Backdrop)
	}
	if options.Mac.LiquidGlass.Style != application.LiquidGlassStyleAutomatic {
		t.Fatalf("LiquidGlass.Style = %v, want LiquidGlassStyleAutomatic", options.Mac.LiquidGlass.Style)
	}
	if options.Mac.LiquidGlass.Material != application.NSVisualEffectMaterialAuto {
		t.Fatalf("LiquidGlass.Material = %v, want NSVisualEffectMaterialAuto", options.Mac.LiquidGlass.Material)
	}
	if options.Mac.LiquidGlass.TintColor != nil {
		t.Fatalf("LiquidGlass.TintColor = %+v, want nil neutral tint", options.Mac.LiquidGlass.TintColor)
	}
	if options.BackgroundType != application.BackgroundTypeSolid {
		t.Fatalf("BackgroundType = %v, want unchanged solid background", options.BackgroundType)
	}
	if options.BackgroundColour != (application.RGBA{Alpha: 0}) {
		t.Fatalf("BackgroundColour = %+v, want transparent WebKit background", options.BackgroundColour)
	}
	if options.Windows.BackdropType != application.None {
		t.Fatalf("Windows.BackdropType = %v, want unchanged None", options.Windows.BackdropType)
	}
}

func TestApplyMainWindowMaterialPolicyUsesAcrylicOnWindows(t *testing.T) {
	t.Parallel()

	options := baselineWindowMaterialOptions()
	applyMainWindowMaterialPolicy(&options, "windows")

	if options.BackgroundType != application.BackgroundTypeTranslucent {
		t.Fatalf("BackgroundType = %v, want BackgroundTypeTranslucent", options.BackgroundType)
	}
	if options.Windows.BackdropType != application.Acrylic {
		t.Fatalf("Windows.BackdropType = %v, want Acrylic", options.Windows.BackdropType)
	}
	if options.Mac.Backdrop != application.MacBackdropNormal {
		t.Fatalf("Mac.Backdrop = %v, want unchanged MacBackdropNormal", options.Mac.Backdrop)
	}
	if options.BackgroundColour != (application.RGBA{Alpha: 0}) {
		t.Fatalf("BackgroundColour = %+v, want transparent WebView2 background", options.BackgroundColour)
	}
}

func TestApplyMainWindowCompositionPolicyIsScopedToWindowsMain(t *testing.T) {
	t.Parallel()

	options := baselineWindowMaterialOptions()
	applyMainWindowCompositionPolicy(&options, "windows")
	if !options.Windows.WebView2CompositionHosting {
		t.Fatal("Windows main window must use WebView2 composition hosting")
	}

	other := baselineWindowMaterialOptions()
	applyMainWindowCompositionPolicy(&other, "darwin")
	if other.Windows.WebView2CompositionHosting {
		t.Fatal("non-Windows window unexpectedly enabled WebView2 composition hosting")
	}
	applyMainWindowCompositionPolicy(nil, "windows")
}

func TestApplyMainWindowMaterialPolicyPreservesOtherPlatforms(t *testing.T) {
	t.Parallel()

	options := baselineWindowMaterialOptions()
	want := options
	applyMainWindowMaterialPolicy(&options, "linux")

	if !reflect.DeepEqual(options, want) {
		t.Fatalf("unsupported platform options changed:\n got: %+v\nwant: %+v", options, want)
	}
}

func TestApplyMainWindowMaterialPolicyAcceptsNilOptions(t *testing.T) {
	t.Parallel()
	applyMainWindowMaterialPolicy(nil, "darwin")
}

func TestResolveSettingsWindowSurfaceStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]any
		want   settingsWindowSurfaceStyle
	}{
		{name: "missing defaults to Glass", want: settingsWindowSurfaceGlass},
		{
			name: "reads Surface Style",
			config: map[string]any{
				"appearance": map[string]any{"surfaceStyle": "contrast"},
			},
			want: settingsWindowSurfaceContrast,
		},
		{
			name: "migrates legacy Sidebar Style",
			config: map[string]any{
				"appearance": map[string]any{"sidebarStyle": "contrast"},
			},
			want: settingsWindowSurfaceContrast,
		},
		{
			name: "ignores unsupported styles",
			config: map[string]any{
				"appearance": map[string]any{"surfaceStyle": "pixel"},
			},
			want: settingsWindowSurfaceGlass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveSettingsWindowSurfaceStyle(tt.config); got != tt.want {
				t.Fatalf("resolveSettingsWindowSurfaceStyle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSettingsWindowBackgroundFollowsSurfaceStyle(t *testing.T) {
	t.Parallel()

	theme := application.RGBA{Red: 18, Green: 18, Blue: 20, Alpha: 255}
	transparent := application.RGBA{Alpha: 0}

	for _, goos := range []string{"darwin", "windows"} {
		if got := resolveSettingsWindowBackground(goos, settingsWindowSurfaceGlass, theme); got != transparent {
			t.Fatalf("%s Glass settings background = %+v, want transparent", goos, got)
		}
		if got := resolveSettingsWindowBackground(goos, settingsWindowSurfaceContrast, theme); got != theme {
			t.Fatalf("%s Contrast settings background = %+v, want themed opaque colour", goos, got)
		}
	}
	if got := resolveSettingsWindowBackground("linux", settingsWindowSurfaceGlass, theme); got != theme {
		t.Fatalf("Linux Glass settings background = %+v, want themed opaque colour", got)
	}
}

func TestApplySettingsWindowMaterialPolicyKeepsContrastOpaque(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"darwin", "windows"} {
		options := baselineWindowMaterialOptions()
		wantBackground := options.BackgroundColour
		applySettingsWindowMaterialPolicy(
			&options,
			goos,
			settingsWindowSurfaceContrast,
		)
		if options.BackgroundColour != wantBackground {
			t.Fatalf("%s Contrast background = %+v, want %+v", goos, options.BackgroundColour, wantBackground)
		}
		if goos == "darwin" && options.Mac.Backdrop != application.MacBackdropLiquidGlass {
			t.Fatalf("macOS settings backdrop = %v, want Liquid Glass capability", options.Mac.Backdrop)
		}
		if goos == "windows" && options.Windows.BackdropType != application.Acrylic {
			t.Fatalf("Windows settings backdrop = %v, want Acrylic capability", options.Windows.BackdropType)
		}
	}
}

func TestBuildSettingsWindowOptionsPropagatesContrastSurface(t *testing.T) {
	t.Parallel()

	current := settingsdto.Settings{
		EffectiveAppearance: "dark",
		AppearanceConfig: map[string]any{
			"appearance": map[string]any{"surfaceStyle": "contrast"},
		},
	}
	options := buildSettingsWindowOptions(current, false)
	if options.URL != "/?window=settings&surfaceStyle=contrast" {
		t.Fatalf("Contrast settings URL = %q, want startup Surface Style hint", options.URL)
	}
	wantBackground := backgroundColour(current)
	if options.BackgroundColour != wantBackground {
		t.Fatalf(
			"Contrast settings background = %+v, want canonical opaque theme colour %+v",
			options.BackgroundColour,
			wantBackground,
		)
	}

	switch runtime.GOOS {
	case "darwin":
		if options.Mac.Backdrop != application.MacBackdropLiquidGlass {
			t.Fatalf("macOS settings backdrop = %v, want preinstalled Liquid Glass", options.Mac.Backdrop)
		}
	case "windows":
		if options.Windows.BackdropType != application.Acrylic {
			t.Fatalf("Windows settings backdrop = %v, want preinstalled Acrylic", options.Windows.BackdropType)
		}
	}
}

func TestResolveWindowRuntimeBackgrounds(t *testing.T) {
	t.Parallel()

	light := application.RGBA{Red: 245, Green: 245, Blue: 247, Alpha: 255}
	dark := application.RGBA{Red: 18, Green: 18, Blue: 20, Alpha: 255}
	transparent := application.RGBA{Alpha: 0}

	tests := []struct {
		name     string
		goos     string
		theme    application.RGBA
		wantMain application.RGBA
	}{
		{name: "macOS light update stays transparent", goos: "darwin", theme: light, wantMain: transparent},
		{name: "macOS dark update stays transparent", goos: "darwin", theme: dark, wantMain: transparent},
		{name: "Windows light update stays transparent", goos: "windows", theme: light, wantMain: transparent},
		{name: "Windows dark update stays transparent", goos: "windows", theme: dark, wantMain: transparent},
		{name: "Linux keeps themed background", goos: "linux", theme: dark, wantMain: dark},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveWindowRuntimeBackgrounds(tt.goos, tt.theme)
			if got.main != tt.wantMain {
				t.Fatalf("resolveWindowRuntimeBackgrounds(%q, %+v).main = %+v, want %+v", tt.goos, tt.theme, got.main, tt.wantMain)
			}
			if got.content != tt.theme {
				t.Fatalf("resolveWindowRuntimeBackgrounds(%q, %+v).content = %+v, want original theme colour", tt.goos, tt.theme, got.content)
			}
		})
	}
}

func TestNativeWindowMaterialIncludesGlassSettingsButNotUtilityWindows(t *testing.T) {
	t.Parallel()

	current := settingsdto.Settings{}
	mainOptions := buildMainWindowOptions(current, false)
	settingsOptions := buildSettingsWindowOptions(current, false)
	trayOptions := buildTrayMiniPlayerWindowOptions(current)

	switch runtime.GOOS {
	case "darwin":
		if mainOptions.Mac.Backdrop != application.MacBackdropLiquidGlass {
			t.Fatalf("main Mac.Backdrop = %v, want MacBackdropLiquidGlass", mainOptions.Mac.Backdrop)
		}
		if settingsOptions.Mac.Backdrop != application.MacBackdropLiquidGlass ||
			settingsOptions.BackgroundColour != (application.RGBA{Alpha: 0}) {
			t.Fatalf(
				"settings macOS material = (%v, %+v), want Liquid Glass with transparent WebView",
				settingsOptions.Mac.Backdrop,
				settingsOptions.BackgroundColour,
			)
		}
	case "windows":
		if mainOptions.BackgroundType != application.BackgroundTypeTranslucent ||
			mainOptions.Windows.BackdropType != application.Acrylic ||
			!mainOptions.Windows.WebView2CompositionHosting {
			t.Fatalf(
				"main Windows composition = (%v, %v, %v), want translucent Acrylic with visual hosting",
				mainOptions.BackgroundType,
				mainOptions.Windows.BackdropType,
				mainOptions.Windows.WebView2CompositionHosting,
			)
		}
		if settingsOptions.BackgroundType != application.BackgroundTypeTranslucent ||
			settingsOptions.Windows.BackdropType != application.Acrylic ||
			settingsOptions.BackgroundColour != (application.RGBA{Alpha: 0}) {
			t.Fatalf(
				"settings Windows material = (%v, %v, %+v), want translucent Acrylic with transparent WebView",
				settingsOptions.BackgroundType,
				settingsOptions.Windows.BackdropType,
				settingsOptions.BackgroundColour,
			)
		}
		if settingsOptions.Windows.WebView2CompositionHosting {
			t.Fatal("settings window unexpectedly enabled main-window composition hosting")
		}
	default:
		if mainOptions.BackgroundType != application.BackgroundTypeSolid {
			t.Fatalf("main BackgroundType = %v, want BackgroundTypeSolid", mainOptions.BackgroundType)
		}
		if settingsOptions.BackgroundType != application.BackgroundTypeSolid {
			t.Fatalf("settings BackgroundType = %v, want BackgroundTypeSolid", settingsOptions.BackgroundType)
		}
	}
	if trayOptions.Mac.Backdrop != application.MacBackdropTransparent ||
		trayOptions.Windows.BackdropType != application.None {
		t.Fatalf(
			"tray mini player material changed: mac=%v windows=%v",
			trayOptions.Mac.Backdrop,
			trayOptions.Windows.BackdropType,
		)
	}
}

func baselineWindowMaterialOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.RGBA{Red: 18, Green: 18, Blue: 20, Alpha: 255},
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropNormal,
		},
		Windows: application.WindowsWindow{
			BackdropType: application.None,
		},
	}
}
