package settings

import "testing"

func TestParseLanguageAcceptsTraditionalChinese(t *testing.T) {
	language, err := ParseLanguage("zh-TW")
	if err != nil {
		t.Fatalf("parse traditional chinese language: %v", err)
	}
	if language != LanguageChineseTraditional {
		t.Fatalf("expected %q, got %q", LanguageChineseTraditional, language)
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
