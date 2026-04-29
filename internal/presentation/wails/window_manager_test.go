package wails

import (
	"errors"
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
