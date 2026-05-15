//go:build !darwin || !cgo

package lyricsromanization

func romanizeWithLocale(string, string) string {
	return ""
}

func dominantLanguage(string) string {
	return ""
}

func systemRomanizationAvailable() bool {
	return false
}
