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
	switch strings.ToLower(tag) {
	case "en", "en-us", "en-gb":
		return settings.LanguageEnglish, true
	case "zh", "zh-cn", "zh-hans", "zh_sg":
		return settings.LanguageChineseSimplified, true
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

	base, _ := parsed.Base()
	region, _ := parsed.Region()
	if region.IsCountry() {
		return strings.ToLower(base.String() + "-" + region.String())
	}

	return strings.ToLower(base.String())
}
