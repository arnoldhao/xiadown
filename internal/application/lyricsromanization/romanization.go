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
	scriptKorean
	scriptChinese
	scriptThai
	scriptBengali
	scriptHindi
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
	if source == "" || isLatinOnly(source) {
		return Result{}
	}

	var result string
	var kind Kind
	switch dominantScript(source) {
	case scriptJapanese:
		if !Available() {
			return Result{}
		}
		result = romanizeWithLocale(source, "ja")
		kind = KindRomanized
	case scriptKorean:
		result = romanizeKorean(source)
		kind = KindRomanized
	case scriptChinese:
		if !Available() {
			return Result{}
		}
		result = romanizeWithLocale(normalizeChineseForTokenizer(source), "zh")
		kind = KindPinyin
	case scriptThai:
		if !Available() {
			return Result{}
		}
		result = romanizeWithLocale(source, "th")
		kind = KindRomanized
	case scriptBengali:
		if !Available() {
			return Result{}
		}
		result = romanizeWithLocale(source, "bn")
		kind = KindRomanized
	case scriptHindi:
		if !Available() {
			return Result{}
		}
		result = romanizeWithLocale(source, "hi")
		kind = KindRomanized
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
	if hasJapanese(text) {
		return scriptJapanese
	}
	if hasKorean(text) {
		return scriptKorean
	}
	if hasChinese(text) {
		return scriptChinese
	}
	if hasThai(text) {
		return scriptThai
	}
	if hasBengali(text) {
		return scriptBengali
	}
	if hasHindi(text) {
		return scriptHindi
	}
	if isLatinOnly(text) {
		return scriptLatin
	}
	return scriptUnknown
}

func hasJapanese(text string) bool {
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
		return true
	}
	if !hasCJK {
		return false
	}
	return isJapaneseCJKText(text)
}

func hasKorean(text string) bool {
	for _, r := range text {
		if isHangul(r) {
			return true
		}
	}
	return false
}

func hasChinese(text string) bool {
	hasCJK := false
	for _, r := range text {
		if isKana(r) {
			return false
		}
		if isCJK(r) {
			hasCJK = true
		}
	}
	if !hasCJK {
		return false
	}
	return !isJapaneseCJKText(text)
}

func hasThai(text string) bool {
	for _, r := range text {
		if r >= 0x0e00 && r <= 0x0e7f {
			return true
		}
	}
	return false
}

func hasBengali(text string) bool {
	for _, r := range text {
		if r >= 0x0980 && r <= 0x09ff {
			return true
		}
	}
	return false
}

func hasHindi(text string) bool {
	for _, r := range text {
		if r >= 0x0900 && r <= 0x097f {
			return true
		}
	}
	return false
}

func isJapaneseCJKText(text string) bool {
	language := strings.ToLower(strings.TrimSpace(dominantLanguage(text)))
	return strings.HasPrefix(language, "ja")
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

func isHangul(r rune) bool {
	return (r >= 0x1100 && r <= 0x11ff) ||
		(r >= 0x3130 && r <= 0x318f) ||
		(r >= 0xac00 && r <= 0xd7af)
}

func containsSourceScript(text string) bool {
	for _, r := range text {
		if isKana(r) || isCJK(r) || isHangul(r) ||
			(r >= 0x0e00 && r <= 0x0e7f) ||
			(r >= 0x0980 && r <= 0x09ff) ||
			(r >= 0x0900 && r <= 0x097f) {
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
