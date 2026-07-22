package wails

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/settings"
)

func TestInitialWindowOptionsCarryMinimumSizes(t *testing.T) {
	t.Parallel()

	main := buildWindowOptions("main", "XiaDown", "/", settingsdto.WindowBounds{}, settingsdto.Settings{}, false)
	if main.MinWidth != settings.MinMainWindowWidth || main.MinHeight != settings.MinMainWindowHeight {
		t.Fatalf("main minimum size = %dx%d, want %dx%d", main.MinWidth, main.MinHeight, settings.MinMainWindowWidth, settings.MinMainWindowHeight)
	}
	if !main.Hidden {
		t.Fatal("main window must be created hidden until the frontend boot surface is ready")
	}

	settingsWindow := buildWindowOptions("settings", "Settings", "/?window=settings", settingsdto.WindowBounds{}, settingsdto.Settings{}, true)
	if settingsWindow.MinWidth != settings.MinSettingsWindowWidth || settingsWindow.MinHeight != settings.MinSettingsWindowHeight {
		t.Fatalf("settings minimum size = %dx%d, want %dx%d", settingsWindow.MinWidth, settingsWindow.MinHeight, settings.MinSettingsWindowWidth, settings.MinSettingsWindowHeight)
	}
}

func TestMainWindowAlwaysStartsNativeHidden(t *testing.T) {
	for _, launchedByAutoStart := range []bool{false, true} {
		options := buildMainWindowOptions(settingsdto.Settings{}, launchedByAutoStart)
		if !options.Hidden {
			t.Fatalf("main window Hidden = false for autostart=%v", launchedByAutoStart)
		}
	}
}

func TestMainWindowStartupThemeScriptUsesEffectiveAppearance(t *testing.T) {
	for _, test := range []struct {
		appearance string
		want       string
	}{
		{appearance: settings.AppearanceLight.String(), want: `startupTheme = "light"`},
		{appearance: settings.AppearanceDark.String(), want: `startupTheme = "dark"`},
	} {
		options := buildMainWindowOptions(settingsdto.Settings{
			EffectiveAppearance: test.appearance,
		}, false)
		if !strings.Contains(options.JS, test.want) {
			t.Fatalf("startup theme script for %q = %q", test.appearance, options.JS)
		}
	}

	if script := mainWindowStartupThemeScript("system"); script != "" {
		t.Fatalf("unexpected startup theme script for unresolved appearance: %q", script)
	}
}

func TestStartupConstructsOnlyTheMainWebView(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	constructor := windowManagerFunctionSource(t, source, "func NewWindowManager(")
	if count := strings.Count(constructor, ".Window.NewWithOptions("); count != 1 {
		t.Fatalf("startup WebView constructor count = %d, want main only", count)
	}
	for _, forbidden := range []string{
		"buildSettingsWindowOptions(",
		"buildTrayMiniPlayerWindowOptions(",
		"registerSettingsWindowEvents(",
	} {
		if strings.Contains(constructor, forbidden) {
			t.Fatalf("startup constructor eagerly initialises a secondary WebView through %q", forbidden)
		}
	}
}

func TestLazyWebViewsInstallPoliciesBeforeRunning(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, signature := range []string{
		"func (manager *WindowManager) ensureSettingsWindow(",
		"func (manager *WindowManager) ensureTrayMiniPlayer(",
	} {
		body := windowManagerFunctionSource(t, source, signature)
		constructor := strings.Index(body, "application.NewWindow(withRemoteWebViewPermissionPolicy(")
		capabilityPolicy := strings.Index(body, "registerWebViewRemoteCapabilityPolicy(window)")
		add := strings.Index(body, "manager.app.Window.Add(window)")
		run := strings.Index(body, "window.Run()")
		if constructor < 0 || capabilityPolicy < 0 || add < 0 || run < 0 {
			t.Fatalf("%s is missing the prepared lazy-window lifecycle", signature)
		}
		if !(constructor < capabilityPolicy && capabilityPolicy < add && add < run) {
			t.Fatalf("%s must construct, secure, register, then run the WebView", signature)
		}
		if strings.Contains(body, "NewWithOptions(") {
			t.Fatalf("%s must not run before its lifecycle policies are registered", signature)
		}
	}
}

func TestTrayAndUpdateActionsPreserveLazyWindowBehaviour(t *testing.T) {
	t.Parallel()

	trayBytes, err := os.ReadFile("system_tray.go")
	if err != nil {
		t.Fatal(err)
	}
	traySource := string(trayBytes)
	for _, required := range []string{
		"actions.ToggleMiniPlayer()",
		"controller.actions.OpenUpdate()",
		"tray.AttachWindow(window).WindowOffset(10)",
	} {
		if !strings.Contains(traySource, required) {
			t.Fatalf("lazy tray contract is missing %q", required)
		}
	}
	if strings.Contains(
		windowManagerFunctionSource(t, traySource, "func NewSystemTrayController("),
		"miniPlayer application.Window",
	) {
		t.Fatal("system tray construction must not require an eager mini-player WebView")
	}

	windowBytes, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	navigation := windowManagerFunctionSource(
		t,
		string(windowBytes),
		"func (manager *WindowManager) emitNavigateToAbout(",
	)
	for _, required := range []string{
		"manager.ShowSettingsWindow()",
		`localStorage.setItem(key, "about")`,
		`manager.app.Event.Emit("settings:navigate", "about")`,
	} {
		if !strings.Contains(navigation, required) {
			t.Fatalf("lazy settings update navigation is missing %q", required)
		}
	}
}

func windowManagerFunctionSource(t *testing.T, source string, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("could not locate %s", signature)
	}
	rest := source[start+len(signature):]
	end := strings.Index(rest, "\nfunc ")
	if end < 0 {
		return source[start:]
	}
	return source[start : start+len(signature)+end]
}

func TestMainWindowBootStateShowsNativeSurfaceBeforeFrontendReady(t *testing.T) {
	state := newMainWindowBootState(true)

	if state.requestShow() {
		t.Fatal("show request must wait for ApplicationStarted and a safe surface")
	}
	if state.markNativeSurfaceReady() {
		t.Fatal("native surface must wait for ApplicationStarted before revealing the window")
	}
	if state.isReady() {
		t.Fatal("native surface readiness must not impersonate frontend readiness")
	}
	if !state.markApplicationStarted() {
		t.Fatal("ApplicationStarted should reveal the already-installed native surface")
	}
	becameReady, shouldReveal := state.markReady()
	if !becameReady || shouldReveal {
		t.Fatalf("frontend-ready transition = (%v, %v), want (true, false)", becameReady, shouldReveal)
	}
	if becameReady, shouldReveal = state.markReady(); becameReady || shouldReveal {
		t.Fatal("frontend-ready transition must be idempotent")
	}
	if !state.requestShow() {
		t.Fatal("an explicit show must restore and focus an already-visible window")
	}
}

func TestMainWindowBootStateSettlesOnFrontendOrFallback(t *testing.T) {
	frontend := newMainWindowBootState(true)
	if frontend.isSettled() {
		t.Fatal("new boot state must not be settled")
	}
	frontend.markReady()
	if !frontend.isSettled() {
		t.Fatal("frontend-ready boot state must be settled")
	}

	fallback := newMainWindowBootState(true)
	fallback.markFallbackReady()
	if fallback.isReady() {
		t.Fatal("fallback readiness must not impersonate frontend readiness")
	}
	if !fallback.isSettled() {
		t.Fatal("fallback-ready boot state must be settled")
	}
}

func TestMainWindowBootStateExplicitShowRestoresSafeWindow(t *testing.T) {
	state := newMainWindowBootState(true)
	if state.markApplicationStarted() {
		t.Fatal("ApplicationStarted must wait for a safe surface")
	}
	if !state.markNativeSurfaceReady() {
		t.Fatal("native surface should reveal the initial window")
	}
	if !state.requestShow() {
		t.Fatal("tray or Dock show should be applied even after the initial reveal")
	}
}

func TestMainWindowBootStateKeepsAutostartLaunchHidden(t *testing.T) {
	state := newMainWindowBootState(false)

	if state.markApplicationStarted() {
		t.Fatal("ApplicationStarted must not reveal a window whose startup intent is hidden")
	}
	if state.markNativeSurfaceReady() {
		t.Fatal("native surface must not reveal a hidden autostart launch")
	}
	becameReady, shouldReveal := state.markReady()
	if !becameReady || shouldReveal {
		t.Fatalf("hidden frontend-ready transition = (%v, %v), want (true, false)", becameReady, shouldReveal)
	}
	if !state.isReady() {
		t.Fatal("hidden autostart launch should still complete the boot handshake")
	}
	if !state.requestShow() {
		t.Fatal("an explicit later show request should reveal a boot-ready window")
	}
}

func TestMainWindowBootStateRecordsShowIntentBeforeNativeSurface(t *testing.T) {
	state := newMainWindowBootState(false)

	if state.requestShow() {
		t.Fatal("pre-surface show request should only record visibility intent")
	}
	if state.markApplicationStarted() {
		t.Fatal("ApplicationStarted must still wait for a safe startup surface")
	}
	if !state.markNativeSurfaceReady() {
		t.Fatal("recorded visibility intent should be applied as soon as the native surface exists")
	}
}

func TestMainWindowBootStateFallsBackToFrontendWithoutNativeSurface(t *testing.T) {
	state := newMainWindowBootState(true)

	if state.markApplicationStarted() {
		t.Fatal("ApplicationStarted must wait for a safe startup surface")
	}
	becameReady, shouldReveal := state.markReady()
	if !becameReady || !shouldReveal {
		t.Fatalf("frontend fallback transition = (%v, %v), want (true, true)", becameReady, shouldReveal)
	}
}

func TestMainWindowBootStateHideBeforeReadyDoesNotResurface(t *testing.T) {
	state := newMainWindowBootState(true)
	if state.markApplicationStarted() {
		t.Fatal("ApplicationStarted must wait for a safe startup surface")
	}
	state.requestHide()
	if state.markNativeSurfaceReady() {
		t.Fatal("native surface must respect a hide request")
	}
	if becameReady, shouldReveal := state.markReady(); !becameReady || shouldReveal {
		t.Fatalf("hidden frontend-ready transition = (%v, %v), want (true, false)", becameReady, shouldReveal)
	}
}

func TestMainWindowBootStateFallbackDoesNotClaimFrontendReady(t *testing.T) {
	state := newMainWindowBootState(true)
	if state.markApplicationStarted() {
		t.Fatal("ApplicationStarted must wait for a safe startup surface")
	}
	if !state.markFallbackReady() {
		t.Fatal("HTML fallback should reveal a manually launched window")
	}
	if state.isReady() {
		t.Fatal("HTML fallback must not impersonate a stable React frame")
	}
	if becameReady, shouldReveal := state.markReady(); !becameReady || shouldReveal {
		t.Fatalf("late frontend-ready transition = (%v, %v), want (true, false)", becameReady, shouldReveal)
	}
}

func TestMainWindowBootStateInvalidatesClaimedRevealAfterHide(t *testing.T) {
	state := newMainWindowBootState(true)
	if state.markNativeSurfaceReady() {
		t.Fatal("native surface must wait for ApplicationStarted")
	}
	if !state.markApplicationStarted() {
		t.Fatal("manual launch should claim a reveal")
	}
	state.requestHide()
	if state.shouldApplyReveal() {
		t.Fatal("a hide request must invalidate an outstanding reveal claim")
	}
}

func TestAwaitMainWindowBootReadyFallback(t *testing.T) {
	t.Run("pending at deadline", func(t *testing.T) {
		if !awaitMainWindowBootReadyFallback(context.Background(), 0, func() bool { return false }) {
			t.Fatal("pending handshake should trigger the timeout fallback")
		}
	})

	t.Run("already ready", func(t *testing.T) {
		if awaitMainWindowBootReadyFallback(context.Background(), 0, func() bool { return true }) {
			t.Fatal("completed handshake must suppress the timeout fallback")
		}
	})

	t.Run("shutdown", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if awaitMainWindowBootReadyFallback(ctx, time.Hour, func() bool { return false }) {
			t.Fatal("shutdown must cancel the timeout fallback")
		}
	})
}

func TestMainWindowBootFallbackRequestFollowsPlatformHTMLPolicy(t *testing.T) {
	manager := &WindowManager{mainBoot: newMainWindowBootState(false)}

	manager.ReleaseMainWindowBootFallback()
	if !supportsMainWindowStartupOverlay() {
		if !manager.mainBoot.isFallbackReady() {
			t.Fatal("platforms without a native startup overlay should release the HTML fallback immediately")
		}
		return
	}
	if manager.mainBoot.isFallbackReady() {
		t.Fatal("a native startup overlay must remain until WebKit finishes navigation")
	}
	manager.markMainWindowHTMLSurfaceReady()
	if !manager.mainBoot.isFallbackReady() {
		t.Fatal("the pending fallback request must be released when HTML becomes available")
	}
}

func TestMainWindowHTMLReadyBeforeFallbackRequestReleasesImmediately(t *testing.T) {
	manager := &WindowManager{mainBoot: newMainWindowBootState(false)}

	manager.markMainWindowHTMLSurfaceReady()
	manager.ReleaseMainWindowBootFallback()
	if !manager.mainBoot.isFallbackReady() {
		t.Fatal("fallback request should release immediately after HTML is available")
	}
}

func TestMainWindowBootFailureCanReleaseNonNativeHTMLSurface(t *testing.T) {
	if !canReleaseMainWindowBootFallback(false, false) {
		t.Fatal("non-native frontend failure already proves the HTML bootstrap exists")
	}
	if canReleaseMainWindowBootFallback(true, false) {
		t.Fatal("native overlay must remain until WebKit confirms the HTML surface")
	}
	if !canReleaseMainWindowBootFallback(true, true) {
		t.Fatal("native overlay should release after WebKit confirms the HTML surface")
	}
}

func TestMacMainWindowKeepsNativeTrafficLightControls(t *testing.T) {
	t.Parallel()

	options := application.WebviewWindowOptions{
		MinimiseButtonState:   application.ButtonHidden,
		MaximiseButtonState:   application.ButtonHidden,
		FullscreenButtonState: application.ButtonHidden,
		CloseButtonState:      application.ButtonHidden,
	}
	applyMainWindowControlPolicy(&options, "darwin")

	if options.MinimiseButtonState != application.ButtonEnabled ||
		options.MaximiseButtonState != application.ButtonEnabled ||
		options.FullscreenButtonState != application.ButtonEnabled ||
		options.CloseButtonState != application.ButtonEnabled {
		t.Fatalf("macOS main window traffic lights = (%v, %v, %v, %v), want all enabled",
			options.CloseButtonState,
			options.MinimiseButtonState,
			options.MaximiseButtonState,
			options.FullscreenButtonState,
		)
	}
}

func TestInitialPresentationSyncDoesNotDispatchWindowSizing(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("window_manager.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "func (manager *WindowManager) syncWindowPresentation(")
	if start < 0 {
		t.Fatal("could not locate initial presentation sync")
	}
	end := strings.Index(source[start:], "\nfunc (manager *WindowManager) enforceMinimumSize(")
	if end < 0 {
		t.Fatal("could not locate initial presentation sync")
	}
	body := source[start : start+end]
	for _, forbidden := range []string{".SetMinSize(", ".enforceMinimumSize("} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("initial presentation sync must not call %q before Application.Run", forbidden)
		}
	}
}

func TestShouldExposeDeveloperMenu(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"", "dev", "development", "1.2.3-alpha.1"} {
		if !shouldExposeDeveloperMenu(version) {
			t.Fatalf("expected developer menu for %q", version)
		}
	}
	for _, version := range []string{"1.2.3", "v1.2.3"} {
		if shouldExposeDeveloperMenu(version) {
			t.Fatalf("did not expect developer menu for release %q", version)
		}
	}
}

func TestShouldHideNativeMenuBar(t *testing.T) {
	t.Parallel()

	if shouldHideNativeMenuBar("windows", "dev") {
		t.Fatal("Windows dev menu must remain visible for reload and DevTools")
	}
	if !shouldHideNativeMenuBar("windows", "1.2.3") {
		t.Fatal("Windows release menu should retain the product's hidden-menu behaviour")
	}
	if shouldHideNativeMenuBar("darwin", "1.2.3") {
		t.Fatal("non-Windows menu bar must not be hidden through the Windows policy")
	}
}

func TestShouldStartHidden(t *testing.T) {
	tests := []struct {
		name                string
		settings            settingsdto.Settings
		launchedByAutoStart bool
		expected            bool
	}{
		{
			name: "disabled setting",
			settings: settingsdto.Settings{
				MinimizeToTrayOnStart: false,
			},
			launchedByAutoStart: true,
			expected:            false,
		},
		{
			name: "manual launch",
			settings: settingsdto.Settings{
				MinimizeToTrayOnStart: true,
			},
			launchedByAutoStart: false,
			expected:            false,
		},
		{
			name: "autostart launch",
			settings: settingsdto.Settings{
				MinimizeToTrayOnStart: true,
			},
			launchedByAutoStart: true,
			expected:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStartHidden(tt.settings, tt.launchedByAutoStart); got != tt.expected {
				t.Fatalf("shouldStartHidden() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsDialogCancelledError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "windows shell item nil", err: errors.New("shellitem is nil"), want: true},
		{name: "windows shell item spaced", err: errors.New("shell item is nil"), want: true},
		{name: "windows user canceled hresult", err: errors.New("open dialog failed: 0x800704C7"), want: true},
		{name: "user canceled", err: errors.New("operation was canceled by the user"), want: true},
		{name: "real error", err: errors.New("access denied"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDialogCancelledError(tt.err); got != tt.want {
				t.Fatalf("isDialogCancelledError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveExistingDialogDirectory(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "library")
	if err := os.Mkdir(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	filePath := filepath.Join(nestedDir, "video.mp4")
	if err := os.WriteFile(filePath, []byte("media"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: ""},
		{name: "existing directory", path: nestedDir, want: nestedDir},
		{name: "existing file", path: filePath, want: nestedDir},
		{name: "missing child", path: filepath.Join(nestedDir, "missing", "video.mp4"), want: nestedDir},
		{name: "missing relative path", path: filepath.Join("missing", "video.mp4"), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveExistingDialogDirectory(tt.path); got != tt.want {
				t.Fatalf("resolveExistingDialogDirectory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveWindowTitles(t *testing.T) {
	tests := []struct {
		name     string
		language string
		main     string
		settings string
	}{
		{
			name:     "english defaults",
			language: settings.LanguageEnglish.String(),
			main:     "XiaDown",
			settings: "Settings",
		},
		{
			name:     "simplified chinese",
			language: settings.LanguageChineseSimplified.String(),
			main:     "下蛋",
			settings: "设置",
		},
		{
			name:     "invalid language falls back",
			language: "invalid",
			main:     "XiaDown",
			settings: "Settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWindowTitles(settingsdto.Settings{Language: tt.language})
			if got.Main != tt.main {
				t.Fatalf("main title = %q, want %q", got.Main, tt.main)
			}
			if got.Settings != tt.settings {
				t.Fatalf("settings title = %q, want %q", got.Settings, tt.settings)
			}
		})
	}
}

func TestIsWindowRectVisibleOnScreens(t *testing.T) {
	screens := []*application.Screen{
		{
			ID:        "primary",
			IsPrimary: true,
			Bounds: application.Rect{
				X:      0,
				Y:      0,
				Width:  1920,
				Height: 1080,
			},
			WorkArea: application.Rect{
				X:      0,
				Y:      0,
				Width:  1920,
				Height: 1040,
			},
		},
	}

	if !isWindowRectVisibleOnScreens(application.Rect{X: 120, Y: 80, Width: 1280, Height: 800}, screens) {
		t.Fatal("expected window bounds to be visible on primary screen")
	}

	if isWindowRectVisibleOnScreens(application.Rect{X: 2600, Y: 200, Width: 1280, Height: 800}, screens) {
		t.Fatal("expected off-screen window bounds to be treated as invisible")
	}
}

func TestResolveVisibleWindowBoundsRecentersOffscreenWindow(t *testing.T) {
	primary := &application.Screen{
		ID:        "primary",
		IsPrimary: true,
		Bounds: application.Rect{
			X:      0,
			Y:      0,
			Width:  1920,
			Height: 1080,
		},
		WorkArea: application.Rect{
			X:      0,
			Y:      0,
			Width:  1920,
			Height: 1040,
		},
	}
	secondary := &application.Screen{
		ID: "secondary",
		Bounds: application.Rect{
			X:      1920,
			Y:      0,
			Width:  1920,
			Height: 1080,
		},
		WorkArea: application.Rect{
			X:      1920,
			Y:      0,
			Width:  1920,
			Height: 1040,
		},
	}

	bounds := application.Rect{X: 4200, Y: 100, Width: 1280, Height: 800}
	got, changed := resolveVisibleWindowBounds(bounds, []*application.Screen{primary, secondary}, primary)
	if !changed {
		t.Fatal("expected off-screen bounds to be recentered")
	}
	if got.X != 320 || got.Y != 120 {
		t.Fatalf("unexpected recentered bounds: %+v", got)
	}
}

func TestResolveVisibleWindowBoundsLeavesVisibleWindowUntouched(t *testing.T) {
	primary := &application.Screen{
		ID:        "primary",
		IsPrimary: true,
		Bounds: application.Rect{
			X:      0,
			Y:      0,
			Width:  1920,
			Height: 1080,
		},
		WorkArea: application.Rect{
			X:      0,
			Y:      0,
			Width:  1920,
			Height: 1040,
		},
	}

	bounds := application.Rect{X: 200, Y: 120, Width: 1280, Height: 800}
	got, changed := resolveVisibleWindowBounds(bounds, []*application.Screen{primary}, primary)
	if changed {
		t.Fatal("expected visible bounds to remain unchanged")
	}
	if got != bounds {
		t.Fatalf("expected bounds to stay the same, got %+v", got)
	}
}

func TestNormalizeWindowBoundsForPersistenceClampsMinimumSize(t *testing.T) {
	got, ok := normalizeWindowBoundsForPersistence(application.Rect{
		X:      120,
		Y:      80,
		Width:  400,
		Height: 300,
	}, windowTypeMain)
	if !ok {
		t.Fatal("expected bounds to be valid")
	}
	if got.Width != settings.MinMainWindowWidth || got.Height != settings.MinMainWindowHeight {
		t.Fatalf("expected main bounds to clamp to minimum size, got %+v", got)
	}

	settingsBounds, ok := normalizeWindowBoundsForPersistence(application.Rect{
		X:      20,
		Y:      30,
		Width:  320,
		Height: 360,
	}, windowTypeSettings)
	if !ok {
		t.Fatal("expected settings bounds to be valid")
	}
	if settingsBounds.Width != settings.MinSettingsWindowWidth || settingsBounds.Height != settings.MinSettingsWindowHeight {
		t.Fatalf("expected settings bounds to clamp to minimum size, got %+v", settingsBounds)
	}
}

func TestNormalizeWindowBoundsForPersistenceRejectsEmptyBounds(t *testing.T) {
	if _, ok := normalizeWindowBoundsForPersistence(application.Rect{Width: 0, Height: 640}, windowTypeMain); ok {
		t.Fatal("expected zero-width bounds to be rejected")
	}
	if _, ok := normalizeWindowBoundsForPersistence(application.Rect{Width: 960, Height: 0}, windowTypeMain); ok {
		t.Fatal("expected zero-height bounds to be rejected")
	}
}

func TestNormalizeWindowBoundsForLaunchClampsMinimumSize(t *testing.T) {
	got := normalizeWindowBoundsForLaunch(settingsdto.WindowBounds{
		X:      20,
		Y:      30,
		Width:  400,
		Height: 300,
	}, windowTypeMain)
	if got.X != 20 || got.Y != 30 {
		t.Fatalf("expected position to be preserved, got %+v", got)
	}
	if got.Width != settings.MinMainWindowWidth || got.Height != settings.MinMainWindowHeight {
		t.Fatalf("expected main launch bounds to clamp to minimum size, got %+v", got)
	}

	settingsBounds := normalizeWindowBoundsForLaunch(settingsdto.WindowBounds{
		X:      40,
		Y:      50,
		Width:  320,
		Height: 360,
	}, windowTypeSettings)
	if settingsBounds.Width != settings.MinSettingsWindowWidth || settingsBounds.Height != settings.MinSettingsWindowHeight {
		t.Fatalf("expected settings launch bounds to clamp to minimum size, got %+v", settingsBounds)
	}
}

func TestNormalizeWindowBoundsForLaunchPreservesSavedSize(t *testing.T) {
	got := normalizeWindowBoundsForLaunch(settingsdto.WindowBounds{
		X:      132,
		Y:      61,
		Width:  1249,
		Height: 842,
	}, windowTypeMain)
	if got.X != 132 || got.Y != 61 || got.Width != 1249 || got.Height != 842 {
		t.Fatalf("expected saved launch bounds to be preserved, got %+v", got)
	}
}

func TestWindowManagerCachedBoundsUsesLastValidBounds(t *testing.T) {
	manager := &WindowManager{
		lastMainBounds: settingsdto.WindowBounds{
			X:      50,
			Y:      60,
			Width:  1280,
			Height: 720,
		},
	}

	got, ok := manager.cachedBounds(windowTypeMain)
	if !ok {
		t.Fatal("expected cached main bounds")
	}
	if got.X != 50 || got.Y != 60 || got.Width != 1280 || got.Height != settings.MinMainWindowHeight {
		t.Fatalf("unexpected cached bounds: %+v", got)
	}
	if _, ok := manager.cachedBoundsForPersistence(windowTypeMain); ok {
		t.Fatal("expected initial cached bounds to be clean")
	}

	manager.rememberBounds(windowTypeMain, application.Rect{
		X:      80,
		Y:      90,
		Width:  1400,
		Height: 900,
	})
	got, ok = manager.cachedBounds(windowTypeMain)
	if !ok {
		t.Fatal("expected updated cached main bounds")
	}
	if got.X != 80 || got.Y != 90 || got.Width != 1400 || got.Height != 900 {
		t.Fatalf("unexpected updated cached bounds: %+v", got)
	}
	persistable, ok := manager.cachedBoundsForPersistence(windowTypeMain)
	if !ok {
		t.Fatal("expected updated cached bounds to be dirty")
	}
	if persistable != got {
		t.Fatalf("expected persistable cached bounds to match cached bounds, got %+v", persistable)
	}

	manager.markBoundsClean(windowTypeMain)
	if _, ok := manager.cachedBoundsForPersistence(windowTypeMain); ok {
		t.Fatal("expected cached bounds to be clean after markBoundsClean")
	}
}

func TestWindowManagerBoundsTrackingReadyIsPerWindow(t *testing.T) {
	manager := &WindowManager{}

	if manager.boundsTrackingReady(windowTypeMain) {
		t.Fatal("expected main bounds tracking to start disabled")
	}
	if manager.boundsTrackingReady(windowTypeSettings) {
		t.Fatal("expected settings bounds tracking to start disabled")
	}

	manager.markBoundsTrackingReady(windowTypeMain)
	if !manager.boundsTrackingReady(windowTypeMain) {
		t.Fatal("expected main bounds tracking to be enabled")
	}
	if manager.boundsTrackingReady(windowTypeSettings) {
		t.Fatal("expected settings bounds tracking to remain disabled")
	}

	manager.markBoundsTrackingReady(windowTypeSettings)
	if !manager.boundsTrackingReady(windowTypeSettings) {
		t.Fatal("expected settings bounds tracking to be enabled")
	}
}
