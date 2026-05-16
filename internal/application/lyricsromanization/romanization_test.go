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
