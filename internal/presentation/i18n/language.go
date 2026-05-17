package i18n

import (
	"os"
	"strings"

	"golang.org/x/text/language"

	"xiadown/internal/domain/settings"
)

var SupportedLanguages = []settings.Language{
	settings.LanguageEnglish,
	settings.LanguageChineseSimplified,
	settings.LanguageChineseTraditional,
	settings.LanguageJapanese,
	settings.LanguageKorean,
	settings.LanguageSpanishLatinAmerica,
	settings.LanguagePortugueseBrazil,
	settings.LanguageIndonesian,
	settings.LanguageVietnamese,
}

// DetectSystemLanguage tries to derive the OS language from common environment variables.
// If the detected language is not supported, it falls back to English.
func DetectSystemLanguage() settings.Language {
	candidates := []string{
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANG"),
	}

	for _, candidate := range candidates {
		tag := parseLanguageTag(candidate)
		if tag == "" {
			continue
		}

		if lang, ok := normalizeLanguage(tag); ok {
			return lang
		}
	}

	return settings.LanguageEnglish
}

func normalizeLanguage(tag string) (settings.Language, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	switch {
	case normalized == "en" || strings.HasPrefix(normalized, "en-"):
		return settings.LanguageEnglish, true
	case normalized == "zh" ||
		strings.HasPrefix(normalized, "zh-cn") ||
		strings.HasPrefix(normalized, "zh-hans") ||
		strings.HasPrefix(normalized, "zh-sg"):
		return settings.LanguageChineseSimplified, true
	case strings.HasPrefix(normalized, "zh-tw") ||
		strings.HasPrefix(normalized, "zh-hant") ||
		strings.HasPrefix(normalized, "zh-hk") ||
		strings.HasPrefix(normalized, "zh-mo"):
		return settings.LanguageChineseTraditional, true
	case normalized == "ja" || strings.HasPrefix(normalized, "ja-"):
		return settings.LanguageJapanese, true
	case normalized == "ko" || strings.HasPrefix(normalized, "ko-"):
		return settings.LanguageKorean, true
	case normalized == "es" || strings.HasPrefix(normalized, "es-"):
		return settings.LanguageSpanishLatinAmerica, true
	case normalized == "pt" || strings.HasPrefix(normalized, "pt-"):
		return settings.LanguagePortugueseBrazil, true
	case normalized == "id" || strings.HasPrefix(normalized, "id-"):
		return settings.LanguageIndonesian, true
	case normalized == "vi" || strings.HasPrefix(normalized, "vi-"):
		return settings.LanguageVietnamese, true
	default:
		return "", false
	}
}

func parseLanguageTag(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	parsed, err := language.Parse(value)
	if err != nil {
		// Some values may include encoding (e.g. zh_CN.UTF-8); strip encoding manually.
		value = strings.Split(value, ".")[0]
		value = strings.ReplaceAll(value, "_", "-")
		parsed, err = language.Parse(value)
		if err != nil {
			return ""
		}
	}

	normalized := strings.ToLower(parsed.String())
	if strings.HasPrefix(normalized, "zh-hant") || strings.HasPrefix(normalized, "zh-hans") {
		return normalized
	}

	base, _ := parsed.Base()
	region, _ := parsed.Region()
	if region.String() != "ZZ" {
		return strings.ToLower(base.String() + "-" + region.String())
	}

	return strings.ToLower(base.String())
}
