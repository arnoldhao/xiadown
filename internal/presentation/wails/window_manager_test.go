package wails

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/settings"
)

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
	if got.X != 50 || got.Y != 60 || got.Width != 1280 || got.Height != 720 {
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
