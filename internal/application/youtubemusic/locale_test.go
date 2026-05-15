package youtubemusic

import "testing"

func TestNormalizeLocaleSupportsChineseVariants(t *testing.T) {
	tests := map[string]string{
		"zh-TW":      "zh-TW",
		"zh-Hant":    "zh-TW",
		"zh-Hant-TW": "zh-TW",
		"zh_HK":      "zh-TW",
		"zh-CN":      "zh-CN",
		"zh-Hans-CN": "zh-CN",
		"zh_SG":      "zh-CN",
	}

	for value, expected := range tests {
		if got := NormalizeLocale(value); got != expected {
			t.Fatalf("expected %q for %q, got %q", expected, value, got)
		}
	}
}

func TestAcceptLanguageForTraditionalChinese(t *testing.T) {
	if got := acceptLanguageForLocale("zh-Hant-TW"); got != "zh-TW,zh-Hant;q=0.9,zh;q=0.8,en;q=0.7" {
		t.Fatalf("unexpected accept-language: %q", got)
	}
}
