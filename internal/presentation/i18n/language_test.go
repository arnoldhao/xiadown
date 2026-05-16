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
