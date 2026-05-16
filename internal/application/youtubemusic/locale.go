package youtubemusic

import (
	"context"
	"strings"
)

type localeContextKey struct{}

func WithLocale(ctx context.Context, locale string) context.Context {
	normalized := NormalizeLocale(locale)
	if normalized == "" {
		return ctx
	}
	return context.WithValue(ctx, localeContextKey{}, normalized)
}

func NormalizeLocale(locale string) string {
	value := strings.TrimSpace(locale)
	if value == "" {
		return ""
	}
	normalized := strings.ReplaceAll(value, "_", "-")
	lower := strings.ToLower(normalized)
	switch {
	case strings.HasPrefix(lower, "zh-tw") ||
		strings.HasPrefix(lower, "zh-hant") ||
		strings.HasPrefix(lower, "zh-hk") ||
		strings.HasPrefix(lower, "zh-mo"):
		return "zh-TW"
	case strings.HasPrefix(lower, "zh-cn") ||
		strings.HasPrefix(lower, "zh-hans") ||
		strings.HasPrefix(lower, "zh-sg") ||
		lower == "zh" ||
		strings.HasPrefix(lower, "zh-"):
		return "zh-CN"
	default:
		return "en"
	}
}

func localeFromContext(ctx context.Context) string {
	if ctx == nil {
		return "en"
	}
	if locale, ok := ctx.Value(localeContextKey{}).(string); ok {
		if normalized := NormalizeLocale(locale); normalized != "" {
			return normalized
		}
	}
	return "en"
}

func acceptLanguageForLocale(locale string) string {
	switch NormalizeLocale(locale) {
	case "zh-CN":
		return "zh-CN,zh;q=0.9,en;q=0.7"
	case "zh-TW":
		return "zh-TW,zh-Hant;q=0.9,zh;q=0.8,en;q=0.7"
	default:
		return "en-US,en;q=0.9"
	}
}
