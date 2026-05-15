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
