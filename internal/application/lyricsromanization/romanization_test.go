package lyricsromanization

import (
	"runtime"
	"strings"
	"testing"
)

func TestRomanizeSkipsLatinOnlyText(t *testing.T) {
	if got := Romanize("Hello world"); got != "" {
		t.Fatalf("expected latin-only text to be skipped, got %q", got)
	}
}

func TestDominantScriptDetectsAdditionalScripts(t *testing.T) {
	tests := []struct {
		name string
		text string
		want script
	}{
		{name: "korean", text: "감사합니다", want: scriptKorean},
		{name: "thai", text: "ขอบคุณ", want: scriptThai},
		{name: "bengali", text: "নমস্কার", want: scriptBengali},
		{name: "hindi", text: "नमस्ते", want: scriptHindi},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dominantScript(test.text); got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestRomanizeKoreanKnownSyllables(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{text: "가", want: "ga"},
		{text: "한", want: "han"},
		{text: "방", want: "bang"},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			if got := romanizeKorean(test.text); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestRomanizeKoreanLatinPassthrough(t *testing.T) {
	result := Transcribe("hey 안녕")
	if result.Kind != KindRomanized {
		t.Fatalf("expected romanized kind, got %+v", result)
	}
	if !strings.Contains(result.Text, "hey") {
		t.Fatalf("expected latin token to be preserved, got %+v", result)
	}
	if strings.Contains(result.Text, "안") || strings.Contains(result.Text, "녕") {
		t.Fatalf("expected Hangul to be romanized, got %+v", result)
	}
}

func TestRomanizeJapaneseKanaPreservesLatinTokens(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	got := Romanize("Kissして")
	if got == "" {
		t.Fatalf("expected romanized text")
	}
	if !strings.Contains(got, "Kiss") {
		t.Fatalf("expected latin token to be preserved, got %q", got)
	}
	if strings.Contains(got, "して") {
		t.Fatalf("expected kana to be romanized, got %q", got)
	}
}

func TestRomanizeJapaneseKanji(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := Transcribe("東京")
	got := result.Text
	if got == "" || got == "東京" {
		t.Fatalf("expected kanji romanization, got %q", got)
	}
	if result.Kind != KindRomanized {
		t.Fatalf("expected romanized kind, got %+v", result)
	}
	if strings.Contains(got, "東") || strings.Contains(got, "京") {
		t.Fatalf("expected kanji to be transcribed, got %q", got)
	}
}

func TestRomanizeChinesePinyinKind(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := Transcribe("你好")
	if result.Text == "" || strings.Contains(result.Text, "你") || strings.Contains(result.Text, "好") {
		t.Fatalf("expected pinyin transcription, got %+v", result)
	}
	if result.Kind != KindPinyin {
		t.Fatalf("expected pinyin kind, got %+v", result)
	}
}

func TestRomanizeChinesePinyinKeepsToneMarks(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := Transcribe("谢谢")
	if result.Kind != KindPinyin {
		t.Fatalf("expected pinyin kind, got %+v", result)
	}
	if !strings.Contains(result.Text, "è") {
		t.Fatalf("expected pinyin tone marks to be preserved, got %+v", result)
	}
	if containsSourceScript(result.Text) {
		t.Fatalf("expected no Chinese characters in pinyin transcription, got %+v", result)
	}
}

func TestRomanizeChinesePolyphoneUsesTokenizerReading(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := Transcribe("重写")
	if result.Kind != KindPinyin {
		t.Fatalf("expected pinyin kind, got %+v", result)
	}
	if !strings.Contains(result.Text, "chóng") {
		t.Fatalf("expected 重写 to keep tokenizer reading, got %+v", result)
	}
	if strings.Contains(result.Text, "zhong") || strings.Contains(result.Text, "zhòng") {
		t.Fatalf("expected 重写 not to use generic transform reading, got %+v", result)
	}
}

func TestRomanizeChineseVariantNi(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := Transcribe("妳")
	if result.Kind != KindPinyin {
		t.Fatalf("expected pinyin kind, got %+v", result)
	}
	if result.Text != "nǐ" {
		t.Fatalf("expected 妳 to normalize to nǐ, got %+v", result)
	}
}

func TestRomanizeChineseMixedWithLatinDoesNotLeakCharacters(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := Transcribe("我 love 你")
	if result.Text == "" {
		t.Fatalf("expected pinyin transcription")
	}
	if result.Kind != KindPinyin {
		t.Fatalf("expected pinyin kind, got %+v", result)
	}
	if containsSourceScript(result.Text) {
		t.Fatalf("expected no Chinese characters in pinyin transcription, got %+v", result)
	}
	if !strings.Contains(strings.ToLower(result.Text), "love") {
		t.Fatalf("expected latin token to be preserved, got %+v", result)
	}
}

func TestRomanizeThai(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	const text = "สวัสดีครับ"
	raw := romanizeWithLocale(text, "th")
	if raw == "" {
		t.Fatalf("expected Thai tokenizer to return text")
	}

	result := Transcribe(text)
	if canonicalize(raw) == text || containsSourceScript(raw) {
		if result.Text != "" {
			t.Fatalf("expected unchanged Thai tokenizer result to be skipped, got %+v", result)
		}
		return
	}
	if result.Kind != KindRomanized || result.Text == "" {
		t.Fatalf("expected Thai romanized result, got %+v", result)
	}
}

func TestRomanizeBengali(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	assertSystemRomanized(t, "নমস্কার")
}

func TestRomanizeHindi(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	assertSystemRomanized(t, "नमस्ते")
}

func assertSystemRomanized(t *testing.T, text string) {
	t.Helper()
	result := Transcribe(text)
	if result.Kind != KindRomanized {
		t.Fatalf("expected romanized kind, got %+v", result)
	}
	if result.Text == "" {
		t.Fatalf("expected romanized text")
	}
	if containsSourceScript(result.Text) {
		t.Fatalf("expected source script to be romanized, got %+v", result)
	}
}
