package lyricsromanization

import (
	"regexp"
	"strings"
)

type script int

const (
	scriptUnknown script = iota
	scriptLatin
	scriptJapanese
	scriptChinese
)

type Kind string

const (
	KindRomanized Kind = "romanized"
	KindPinyin    Kind = "pinyin"
)

type Result struct {
	Text string
	Kind Kind
}

var whitespacePattern = regexp.MustCompile(`\s+`)

// Romanize returns a Latin transcription for supported non-Latin lyrics text.
// On macOS it uses the same system tokenizer approach as Kaset, so Japanese
// kanji can be converted from their reading rather than by a static kana map.
func Romanize(text string) string {
	return Transcribe(text).Text
}

func Available() bool {
	return systemRomanizationAvailable()
}

func Transcribe(text string) Result {
	source := strings.TrimSpace(text)
	if source == "" || isLatinOnly(source) || !Available() {
		return Result{}
	}

	var result string
	var kind Kind
	switch dominantScript(source) {
	case scriptJapanese:
		result = romanizeWithLocale(source, "ja")
		kind = KindRomanized
	case scriptChinese:
		result = romanizeWithLocale(normalizeChineseForTokenizer(source), "zh")
		kind = KindPinyin
	default:
		return Result{}
	}

	result = canonicalize(result)
	if result == "" || result == source || containsSourceScript(result) {
		return Result{}
	}
	return Result{Text: result, Kind: kind}
}

func dominantScript(text string) script {
	hasKana := false
	hasCJK := false
	for _, r := range text {
		switch {
		case isKana(r):
			hasKana = true
		case isCJK(r):
			hasCJK = true
		}
	}
	if hasKana {
		return scriptJapanese
	}
	if !hasCJK {
		if isLatinOnly(text) {
			return scriptLatin
		}
		return scriptUnknown
	}
	switch language := strings.ToLower(strings.TrimSpace(dominantLanguage(text))); {
	case strings.HasPrefix(language, "ja"):
		return scriptJapanese
	case strings.HasPrefix(language, "zh"):
		return scriptChinese
	default:
		return scriptChinese
	}
}

func canonicalize(text string) string {
	result := strings.NewReplacer(
		"\u00a0", " ",
		"\u2009", " ",
		"\u200a", " ",
		"\u202f", " ",
	).Replace(text)
	result = whitespacePattern.ReplaceAllString(result, " ")
	result = strings.ReplaceAll(result, " ' ", "'")
	result = strings.ReplaceAll(result, " ,", ",")
	result = strings.ReplaceAll(result, " .", ".")
	return strings.TrimSpace(result)
}

func normalizeChineseForTokenizer(text string) string {
	return strings.NewReplacer(
		"妳", "你",
	).Replace(text)
}

func isKana(r rune) bool {
	return (r >= 0x3040 && r <= 0x309f) ||
		(r >= 0x30a0 && r <= 0x30ff) ||
		(r >= 0x31f0 && r <= 0x31ff)
}

func isCJK(r rune) bool {
	return (r >= 0x3400 && r <= 0x4dbf) ||
		(r >= 0x4e00 && r <= 0x9fff) ||
		(r >= 0xf900 && r <= 0xfaff)
}

func containsSourceScript(text string) bool {
	for _, r := range text {
		if isKana(r) || isCJK(r) {
			return true
		}
	}
	return false
}

func isLatinOnly(text string) bool {
	for _, r := range text {
		if r <= 0x024f {
			continue
		}
		if r >= 0x2000 && r <= 0x206f {
			continue
		}
		if r >= 0x20a0 && r <= 0x20cf {
			continue
		}
		if r >= 0xfe00 && r <= 0xfe0f {
			continue
		}
		if r >= 0xfff0 && r <= 0xffff {
			continue
		}
		return false
	}
	return true
}
