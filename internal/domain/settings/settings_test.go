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

func TestDefaultBrowserIsNormalized(t *testing.T) {
	current, err := NewSettings(SettingsParams{
		Appearance:        AppearanceAuto.String(),
		ColorScheme:       DefaultColorScheme.String(),
		Language:          LanguageEnglish.String(),
		LogLevel:          DefaultLogLevel.String(),
		DefaultBrowser:    "  Edge  ",
		MainBounds:        DefaultSettings().MainBounds(),
		SettingsBounds:    DefaultSettings().SettingsBounds(),
		MenuBarVisibility: stringPtr(DefaultMenuBarVisibility.String()),
	})
	if err != nil {
		t.Fatalf("NewSettings() error = %v", err)
	}
	if current.DefaultBrowser() != "edge" {
		t.Fatalf("expected default browser to be normalized, got %q", current.DefaultBrowser())
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
