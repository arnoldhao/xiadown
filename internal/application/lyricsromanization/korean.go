package lyricsromanization

import "strings"

const (
	hangulBase  rune = 0xac00
	hangulEnd   rune = 0xd7a3
	medialCount      = 21
	finalCount       = 28
)

var (
	koreanInitials = []string{
		"g", "kk", "n", "d", "tt", "r", "m", "b", "pp",
		"s", "ss", "", "j", "jj", "ch", "k", "t", "p", "h",
	}
	koreanMedials = []string{
		"a", "ae", "ya", "yae", "eo", "e", "yeo", "ye", "o",
		"wa", "wae", "oe", "yo", "u", "wo", "we", "wi", "yu",
		"eu", "ui", "i",
	}
	koreanFinals = []string{
		"", "k", "k", "k", "n", "n", "n", "t",
		"l", "l", "l", "l", "l", "l", "l", "l",
		"m", "p", "p", "t", "t", "ng", "t", "t",
		"k", "t", "p", "t",
	}
)

func romanizeKorean(text string) string {
	var builder strings.Builder
	for _, r := range text {
		if r < hangulBase || r > hangulEnd {
			builder.WriteRune(r)
			continue
		}

		syllableIndex := int(r - hangulBase)
		initialIndex := syllableIndex / (medialCount * finalCount)
		medialIndex := (syllableIndex % (medialCount * finalCount)) / finalCount
		finalIndex := syllableIndex % finalCount

		builder.WriteString(koreanInitials[initialIndex])
		builder.WriteString(koreanMedials[medialIndex])
		builder.WriteString(koreanFinals[finalIndex])
	}
	return strings.TrimSpace(builder.String())
}
