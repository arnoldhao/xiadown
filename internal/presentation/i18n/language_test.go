package i18n

import (
	"testing"

	"xiadown/internal/domain/settings"
)

func TestNormalizeLanguageSupportsTraditionalChineseTags(t *testing.T) {
	tests := []string{
		"zh-TW",
		"zh-Hant",
		"zh-Hant-TW",
		"zh_HK",
	}

	for _, value := range tests {
		language, ok := normalizeLanguage(value)
		if !ok {
			t.Fatalf("expected %q to normalize", value)
		}
		if language != settings.LanguageChineseTraditional {
			t.Fatalf("expected %q for %q, got %q", settings.LanguageChineseTraditional, value, language)
		}
	}
}

func TestParseLanguageTagPreservesChineseScript(t *testing.T) {
	if got := parseLanguageTag("zh_Hant_TW.UTF-8"); got != "zh-hant-tw" {
		t.Fatalf("expected zh-hant-tw, got %q", got)
	}
}

func TestNormalizeLanguageSupportsAdditionalLocales(t *testing.T) {
	tests := map[string]settings.Language{
		"ja":     settings.LanguageJapanese,
		"ja-JP":  settings.LanguageJapanese,
		"ko_KR":  settings.LanguageKorean,
		"es-419": settings.LanguageSpanishLatinAmerica,
		"es-MX":  settings.LanguageSpanishLatinAmerica,
		"pt-BR":  settings.LanguagePortugueseBrazil,
		"id-ID":  settings.LanguageIndonesian,
		"vi-VN":  settings.LanguageVietnamese,
	}

	for value, expected := range tests {
		language, ok := normalizeLanguage(value)
		if !ok {
			t.Fatalf("expected %q to normalize", value)
		}
		if language != expected {
			t.Fatalf("expected %q for %q, got %q", expected, value, language)
		}
	}
}

func TestParseLanguageTagPreservesM49Region(t *testing.T) {
	if got := parseLanguageTag("es_419.UTF-8"); got != "es-419" {
		t.Fatalf("expected es-419, got %q", got)
	}
}
