package settings

import (
	"path/filepath"
	"testing"
)

func TestManagedDownloadDirectoryCreatesOneOwnedChild(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "Downloads")
	expected := filepath.Join(parent, "xiadown")

	if got := ManagedDownloadDirectory(parent); got != expected {
		t.Fatalf("managed download directory = %q, want %q", got, expected)
	}
	if got := ManagedDownloadDirectory(expected); got != expected {
		t.Fatalf("resolved managed directory was nested again: %q", got)
	}
	if got := ManagedDownloadDirectory(filepath.Join(parent, "XiaDown")); got != filepath.Join(parent, "XiaDown") {
		t.Fatalf("case-variant managed directory was nested again: %q", got)
	}
}

func TestDownloadLocationDirectoryHidesOwnedChild(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "Downloads")
	if got := DownloadLocationDirectory(parent); got != parent {
		t.Fatalf("download location = %q, want %q", got, parent)
	}
	if got := DownloadLocationDirectory(filepath.Join(parent, "xiadown")); got != parent {
		t.Fatalf("managed child was exposed as the download location: %q", got)
	}
	if got := DownloadLocationDirectory(filepath.Join(parent, "XiaDown")); got != parent {
		t.Fatalf("case-variant managed child was exposed as the download location: %q", got)
	}
}

func TestNewSettingsNormalizesLegacyManagedDownloadDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "Downloads")
	current, err := NewSettings(SettingsParams{
		Appearance:        AppearanceAuto.String(),
		ColorScheme:       DefaultColorScheme.String(),
		Language:          LanguageEnglish.String(),
		LogLevel:          DefaultLogLevel.String(),
		DownloadDirectory: filepath.Join(parent, "xiadown"),
		MainBounds:        DefaultSettings().MainBounds(),
		SettingsBounds:    DefaultSettings().SettingsBounds(),
		MenuBarVisibility: stringPtr(DefaultMenuBarVisibility.String()),
	})
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}
	if got := current.DownloadDirectory(); got != parent {
		t.Fatalf("download directory = %q, want user-selected parent %q", got, parent)
	}
}

func TestMainWindowUsesFullscreenSurfaceBaseline(t *testing.T) {
	if MinMainWindowWidth != 1024 {
		t.Fatalf("expected main window minimum width 1024, got %d", MinMainWindowWidth)
	}
	if MinMainWindowHeight != 768 {
		t.Fatalf("expected main window minimum height 768, got %d", MinMainWindowHeight)
	}

	bounds := DefaultSettings().MainBounds()
	if bounds.Width() != 1024 || bounds.Height() != 768 {
		t.Fatalf("expected first-launch main window size 1024x768, got %+v", bounds)
	}
}

func TestParseLanguageAcceptsTraditionalChinese(t *testing.T) {
	language, err := ParseLanguage("zh-TW")
	if err != nil {
		t.Fatalf("parse traditional chinese language: %v", err)
	}
	if language != LanguageChineseTraditional {
		t.Fatalf("expected %q, got %q", LanguageChineseTraditional, language)
	}
}

func TestDefaultSettingsWindowUsesSevenTabWidth(t *testing.T) {
	bounds := DefaultSettings().SettingsBounds()
	if bounds.Width() != DefaultSettingsWidth || bounds.Width() != MinSettingsWindowWidth {
		t.Fatalf("expected default settings width %d, got %+v", DefaultSettingsWidth, bounds)
	}
	if bounds.Width() < 640 {
		t.Fatalf("expected settings window to fit seven tabs, got width %d", bounds.Width())
	}
}

func TestParseLanguageAcceptsAdditionalLocales(t *testing.T) {
	tests := []Language{
		LanguageJapanese,
		LanguageKorean,
		LanguageSpanishLatinAmerica,
		LanguagePortugueseBrazil,
		LanguageIndonesian,
		LanguageVietnamese,
	}

	for _, expected := range tests {
		language, err := ParseLanguage(expected.String())
		if err != nil {
			t.Fatalf("parse language %q: %v", expected, err)
		}
		if language != expected {
			t.Fatalf("expected %q, got %q", expected, language)
		}
	}
}

func TestSniffBrowserIsNormalized(t *testing.T) {
	current, err := NewSettings(SettingsParams{
		Appearance:        AppearanceAuto.String(),
		ColorScheme:       DefaultColorScheme.String(),
		Language:          LanguageEnglish.String(),
		LogLevel:          DefaultLogLevel.String(),
		SniffBrowser:      "  Edge  ",
		MainBounds:        DefaultSettings().MainBounds(),
		SettingsBounds:    DefaultSettings().SettingsBounds(),
		MenuBarVisibility: stringPtr(DefaultMenuBarVisibility.String()),
	})
	if err != nil {
		t.Fatalf("NewSettings() error = %v", err)
	}
	if current.SniffBrowser() != "edge" {
		t.Fatalf("expected sniff browser to be normalized, got %q", current.SniffBrowser())
	}
}

func TestPlaybackAudioQualityDefaultsAndValidation(t *testing.T) {
	current := DefaultSettings()
	if current.PlaybackAudioQuality() != DefaultPlaybackAudioQuality {
		t.Fatalf("expected default playback audio quality, got %q", current.PlaybackAudioQuality())
	}

	quality, err := ParsePlaybackAudioQuality("AUDIO_QUALITY_HIGH")
	if err != nil {
		t.Fatalf("ParsePlaybackAudioQuality() error = %v", err)
	}
	if quality != PlaybackAudioQualityHigh {
		t.Fatalf("expected high quality, got %q", quality)
	}

	if _, err := ParsePlaybackAudioQuality("high"); err == nil {
		t.Fatal("expected legacy playback audio quality alias to fail")
	}
	if _, err := ParsePlaybackAudioQuality("lossless"); err == nil {
		t.Fatal("expected invalid playback audio quality to fail")
	}
}

func TestResourceSniffSettingsDefaultsAndValidation(t *testing.T) {
	current := DefaultSettings()
	if current.ResourceSniffScope() != ResourceSniffScopeDefault {
		t.Fatalf("expected default resource sniff scope, got %q", current.ResourceSniffScope())
	}
	if current.ResourceSniffMinBytes() != DefaultResourceSniffMinBytes {
		t.Fatalf("expected default resource sniff minimum bytes, got %d", current.ResourceSniffMinBytes())
	}
	if current.ResourceSniffRetain() != DefaultResourceSniffRetain {
		t.Fatalf("expected default resource sniff retain limit, got %d", current.ResourceSniffRetain())
	}
	if current.YTDLPConcurrentFragments() != DefaultYTDLPConcurrentFragments {
		t.Fatalf("expected default yt-dlp concurrent fragments, got %d", current.YTDLPConcurrentFragments())
	}

	updated, err := NewSettings(SettingsParams{
		Appearance:               AppearanceAuto.String(),
		ColorScheme:              DefaultColorScheme.String(),
		Language:                 LanguageEnglish.String(),
		LogLevel:                 DefaultLogLevel.String(),
		MainBounds:               DefaultSettings().MainBounds(),
		SettingsBounds:           DefaultSettings().SettingsBounds(),
		MenuBarVisibility:        stringPtr(DefaultMenuBarVisibility.String()),
		ResourceSniffScope:       ResourceSniffScopeAll.String(),
		ResourceSniffMinBytes:    64 * 1024,
		ResourceSniffRetain:      2000,
		YTDLPConcurrentFragments: 8,
	})
	if err != nil {
		t.Fatalf("NewSettings() error = %v", err)
	}
	if updated.ResourceSniffScope() != ResourceSniffScopeAll {
		t.Fatalf("expected all resource sniff scope, got %q", updated.ResourceSniffScope())
	}
	if updated.ResourceSniffMinBytes() != 64*1024 {
		t.Fatalf("expected custom resource sniff minimum bytes, got %d", updated.ResourceSniffMinBytes())
	}
	if updated.ResourceSniffRetain() != 2000 {
		t.Fatalf("expected custom resource sniff retain limit, got %d", updated.ResourceSniffRetain())
	}
	if updated.YTDLPConcurrentFragments() != 8 {
		t.Fatalf("expected custom yt-dlp concurrent fragments, got %d", updated.YTDLPConcurrentFragments())
	}

	if _, err := ParseResourceSniffScope("everything"); err == nil {
		t.Fatal("expected invalid resource sniff scope to fail")
	}
	if _, err := NewSettings(SettingsParams{
		Appearance:               AppearanceAuto.String(),
		ColorScheme:              DefaultColorScheme.String(),
		Language:                 LanguageEnglish.String(),
		LogLevel:                 DefaultLogLevel.String(),
		MainBounds:               DefaultSettings().MainBounds(),
		SettingsBounds:           DefaultSettings().SettingsBounds(),
		MenuBarVisibility:        stringPtr(DefaultMenuBarVisibility.String()),
		YTDLPConcurrentFragments: MaxYTDLPConcurrentFragments + 1,
	}); err == nil {
		t.Fatal("expected invalid yt-dlp concurrent fragments to fail")
	}
}

func TestWithAppearanceConfigClonesNestedValues(t *testing.T) {
	source := map[string]any{
		"appearance": map[string]any{
			"themePackId": "citrus",
		},
	}

	current := DefaultSettings().WithAppearanceConfig(source)
	source["appearance"].(map[string]any)["themePackId"] = "nocturne"

	config := current.AppearanceConfig()
	appearance, ok := config["appearance"].(map[string]any)
	if !ok {
		t.Fatalf("expected appearance config map, got %#v", config["appearance"])
	}
	if appearance["themePackId"] != "citrus" {
		t.Fatalf("expected stored theme pack to be isolated from source mutation, got %#v", appearance["themePackId"])
	}

	appearance["themePackId"] = "pixel"
	nextConfig := current.AppearanceConfig()
	nextAppearance, ok := nextConfig["appearance"].(map[string]any)
	if !ok {
		t.Fatalf("expected appearance config map, got %#v", nextConfig["appearance"])
	}
	if nextAppearance["themePackId"] != "citrus" {
		t.Fatalf("expected returned theme pack to be isolated from caller mutation, got %#v", nextAppearance["themePackId"])
	}
}

func stringPtr(value string) *string {
	return &value
}
