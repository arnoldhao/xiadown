package locallyrics

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseLRCProvidesExplicitEndsAndMetadata(t *testing.T) {
	result, err := ParseContent([]byte("[ar:Artist]\n[ti:Title]\n[00:01.00]Hello\n[00:03.500]World"), FormatLRC, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatLRC || result.TimingQuality != TimingQualityLine {
		t.Fatalf("unexpected LRC classification: %#v", result)
	}
	if result.Title != "Title" || result.Artist != "Artist" {
		t.Fatalf("metadata was not retained: %#v", result)
	}
	if len(result.Lines) != 2 {
		t.Fatalf("expected two lines, got %#v", result.Lines)
	}
	if result.Lines[0].Start != time.Second || result.Lines[0].End != 3500*time.Millisecond || !result.Lines[0].EndEstimated {
		t.Fatalf("expected next line to provide first end: %#v", result.Lines[0])
	}
	if result.Lines[1].End != 8500*time.Millisecond || !result.Lines[1].EndEstimated {
		t.Fatalf("expected bounded fallback end: %#v", result.Lines[1])
	}
}

func TestParseEnhancedLRCPreservesWordEndsAndSpacing(t *testing.T) {
	content := "[00:01.00]<00:01.00>Hello <00:01.50>world<00:02.00>"
	result, err := ParseContent([]byte(content), FormatLRC, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatEnhancedLRC || result.TimingQuality != TimingQualityWord {
		t.Fatalf("expected enhanced LRC word timing, got %#v", result)
	}
	if len(result.Lines) != 1 || len(result.Lines[0].Words) != 2 {
		t.Fatalf("unexpected enhanced lines: %#v", result.Lines)
	}
	first := result.Lines[0].Words[0]
	second := result.Lines[0].Words[1]
	if first.Text != "Hello" || !first.EndsWithSpace || first.Start != time.Second || first.End != 1500*time.Millisecond {
		t.Fatalf("first word lost timing or space semantics: %#v", first)
	}
	if second.Text != "world" || second.Start != 1500*time.Millisecond || second.End != 2*time.Second {
		t.Fatalf("second word lost exact timing: %#v", second)
	}
	if result.Lines[0].End != 2*time.Second || result.Lines[0].EndEstimated {
		t.Fatalf("expected closing marker to define exact line end: %#v", result.Lines[0])
	}
}

func TestParseBracketEnhancedLRC(t *testing.T) {
	result, err := ParseContent([]byte("[00:01.00]你[00:01.40]好[00:02.00]"), FormatLRC, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.TimingQuality != TimingQualityWord || len(result.Lines) != 1 || len(result.Lines[0].Words) != 2 {
		t.Fatalf("expected bracket enhanced LRC, got %#v", result)
	}
	if result.Lines[0].Words[1].End != 2*time.Second {
		t.Fatalf("expected exact bracket word end, got %#v", result.Lines[0].Words[1])
	}
}

func TestLRCPositiveOffsetAdvancesAndClampsAtTimelineStart(t *testing.T) {
	result, err := ParseContent([]byte("[offset:2000]\n[00:01.00]Starts before zero"), FormatLRC, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 1 || result.Lines[0].Start != 0 || result.Lines[0].End <= result.Lines[0].Start {
		t.Fatalf("expected a safe non-negative timeline: %#v", result.Lines)
	}
}

func TestLRCNegativeOffsetDelaysTheTimeline(t *testing.T) {
	result, err := ParseContent([]byte("[offset:-250]\n[00:01.00]Delayed"), FormatLRC, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 1 || result.Lines[0].Start != 1250*time.Millisecond {
		t.Fatalf("expected negative LRC offset to delay the timeline: %#v", result.Lines)
	}
}

func TestParseVTTExactCueAndInlineWordTiming(t *testing.T) {
	content := "WEBVTT\n\n1\n00:00:01.000 --> 00:00:03.000\n<00:00:01.000>Hello <00:00:02.000><b>world</b>"
	result, err := ParseContent([]byte(content), FormatVTT, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatVTT || result.TimingQuality != TimingQualityWord || len(result.Lines) != 1 {
		t.Fatalf("unexpected VTT result: %#v", result)
	}
	line := result.Lines[0]
	if line.End != 3*time.Second || line.EndEstimated || line.Text != "Hello world" {
		t.Fatalf("cue timing/text was not preserved: %#v", line)
	}
	if len(line.Words) != 2 || line.Words[0].End != 2*time.Second || line.Words[1].End != 3*time.Second {
		t.Fatalf("inline VTT timing was not retained: %#v", line.Words)
	}
}

func TestParseTTMLPreservesSyllablesTranslationAndAlternates(t *testing.T) {
	content := `<?xml version="1.0"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata">
  <body><div>
    <p begin="1s" end="3s"><span begin="1s" end="2s">Hello </span><span begin="2s" end="3s">world</span><span ttm:role="x-translation" xml:lang="zh">你好世界</span></p>
    <p begin="4s" end="5s"><span begin="4s" end="5s"><span begin="4s" end="4.4s">Sing</span><span begin="4.4s" end="5s">ing</span></span><span ttm:role="x-romanization" xml:lang="en">sing ing</span></p>
  </div></body>
</tt>`
	result, err := ParseContent([]byte(content), FormatTTML, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatTTML || result.TimingQuality != TimingQualitySyllable || len(result.Lines) != 2 {
		t.Fatalf("unexpected TTML result: %#v", result)
	}
	if result.Lines[0].Translation != "你好世界" || len(result.Lines[0].AlternateTexts) != 1 {
		t.Fatalf("translation alternate missing: %#v", result.Lines[0])
	}
	if len(result.Lines[1].Words) != 1 || len(result.Lines[1].Words[0].Syllables) != 2 {
		t.Fatalf("syllable timing missing: %#v", result.Lines[1].Words)
	}
	if result.Lines[1].AlternateTexts[0].Role != "romanization" {
		t.Fatalf("romanization alternate missing: %#v", result.Lines[1].AlternateTexts)
	}
}

func TestParseTTMLAcceptsAMLLBareSecondsAndGroupsFlatSyllables(t *testing.T) {
	content := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata">
  <body dur="4.000"><div begin="1.000" end="4.000">
    <p begin="1.000" end="3.000"><span begin="1.000" end="1.300">sto</span><span begin="1.300" end="1.700">ry</span> <span begin="1.900" end="2.500">time</span><span ttm:role="x-translation" xml:lang="zh">故事时间</span></p>
  </div></body>
</tt>`
	result, err := ParseContent([]byte(content), FormatTTML, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.TimingQuality != TimingQualitySyllable || len(result.Lines) != 1 {
		t.Fatalf("unexpected AMLL TTML result: %#v", result)
	}
	line := result.Lines[0]
	if line.Start != time.Second || line.End != 3*time.Second || line.Text != "story time" || line.Translation != "故事时间" {
		t.Fatalf("bare-second line was not preserved: %#v", line)
	}
	if len(line.Words) != 2 || line.Words[0].Text != "story" || len(line.Words[0].Syllables) != 2 {
		t.Fatalf("flat AMLL syllables were not grouped: %#v", line.Words)
	}
	if !line.Words[0].EndsWithSpace || line.Words[1].EndsWithSpace {
		t.Fatalf("inter-span whitespace was not preserved: %#v", line.Words)
	}
}

func TestParseTTMLMergesITunesMetadataSidecarAlternates(t *testing.T) {
	content := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:itunes="http://music.apple.com/lyric-ttml-internal">
  <head><metadata>
    <iTunesMetadata xmlns="http://music.apple.com/lyric-ttml-internal">
      <translations><translation type="subtitle" xml:lang="zh-Hans">
        <text for="L1">好想见你 <span xmlns="http://www.w3.org/ns/ttml" ttm:role="x-bg">（好想见你）</span></text>
        <text for="L2">第二行</text>
        <text for="L3">第三行</text>
      </translation></translations>
      <transliterations><transliteration xml:lang="ja-Latn">
        <text for="L1"><span xmlns="http://www.w3.org/ns/ttml" begin="1.000" end="1.200">a</span> <span xmlns="http://www.w3.org/ns/ttml" begin="1.200" end="1.400">i</span> <span xmlns="http://www.w3.org/ns/ttml" begin="1.400" end="1.800">tai</span></text>
      </transliteration></transliterations>
    </iTunesMetadata>
  </metadata></head>
  <body><div>
    <p begin="1.000" end="2.000" itunes:key="L1">会いたい<span ttm:role="x-translation" xml:lang="zh-Hans">好想见你</span></p>
    <p begin="2.000" end="3.000" xml:id="L2">二番</p>
    <p begin="3.000" end="4.000" id="L3">三番</p>
  </div></body>
</tt>`
	result, err := ParseContent([]byte(content), FormatTTML, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.TimingQuality != TimingQualityLine || len(result.Lines) != 3 {
		t.Fatalf("unexpected sidecar TTML result: %#v", result)
	}
	first := result.Lines[0]
	if first.Text != "会いたい" || first.Translation != "好想见你" || len(first.AlternateTexts) != 2 {
		t.Fatalf("inline/sidecar translation was not merged and deduplicated: %#v", first)
	}
	if romanization := first.AlternateTexts[1]; romanization.Role != "romanization" || romanization.Language != "ja-Latn" || romanization.Text != "a i tai" {
		t.Fatalf("sidecar transliteration lost role/language/text: %#v", romanization)
	}
	if strings.Contains(first.Translation, "（好想见你）") {
		t.Fatalf("sidecar background vocal leaked into primary translation: %#v", first)
	}
	if result.Lines[1].Translation != "第二行" || result.Lines[2].Translation != "第三行" {
		t.Fatalf("xml:id/id sidecar fallback failed: %#v", result.Lines)
	}
}

func TestParseTTMLIgnoresPrettyPrintWhitespaceBetweenTimedSpans(t *testing.T) {
	content := `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
    <p begin="1.000" end="3.000"><span begin="1.000" end="1.400">May</span>
      <span begin="1.400" end="1.800">be</span> <span begin="2.000" end="2.600">now</span></p>
    <p begin="4.000" end="6.000"><span begin="4.000" end="4.400">あ</span>
      <span begin="4.400" end="4.800">い</span>
      <span begin="4.800" end="5.200">う</span></p>
  </div></body></tt>`
	result, err := ParseContent([]byte(content), FormatTTML, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.TimingQuality != TimingQualitySyllable || len(result.Lines) != 2 {
		t.Fatalf("unexpected pretty-printed TTML result: %#v", result)
	}
	english := result.Lines[0]
	if english.Text != "Maybe now" || len(english.Words) != 2 || english.Words[0].Text != "Maybe" || len(english.Words[0].Syllables) != 2 || !english.Words[0].EndsWithSpace {
		t.Fatalf("layout newline changed English word boundaries: %#v", english)
	}
	japanese := result.Lines[1]
	if japanese.Text != "あいう" || len(japanese.Words) != 1 || len(japanese.Words[0].Syllables) != 3 {
		t.Fatalf("layout newline inserted spaces into Japanese syllables: %#v", japanese)
	}
}

func TestParseTTMLDropsPartialWordTimelineWhenZeroDurationSpanIsMissing(t *testing.T) {
	content := `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
    <p begin="1.000" end="3.000"><span begin="1.000" end="2.000">Keep</span> <span begin="2.000" end="2.000">me</span></p>
    <p begin="4.000" end="6.000"><span begin="4.000" end="5.000">All</span> <span begin="5.000" end="6.000">good</span></p>
  </div></body></tt>`
	result, err := ParseContent([]byte(content), FormatTTML, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.TimingQuality != TimingQualityLine || len(result.Lines) != 2 {
		t.Fatalf("partial TTML timing must downgrade honestly: %#v", result)
	}
	if result.Lines[0].Text != "Keep me" || len(result.Lines[0].Words) != 0 {
		t.Fatalf("partially timed line should retain text without phantom words: %#v", result.Lines[0])
	}
	if result.Lines[1].Text != "All good" || len(result.Lines[1].Words) != 2 {
		t.Fatalf("complete word timeline should remain available: %#v", result.Lines[1])
	}
}

func TestTranslationAlignmentUsesNearestTimestampWithinTolerance(t *testing.T) {
	main := []byte("[00:01.00]Hello\n[00:04.00]World")
	translation := []byte("[00:01.20]你好\n[00:04.70]世界")
	result, err := ParseWithTranslation(main, FormatLRC, translation, FormatLRC, Options{TranslationTolerance: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if result.Lines[0].Translation != "你好" {
		t.Fatalf("expected nearby translation to align: %#v", result.Lines[0])
	}
	if result.Lines[1].Translation != "" {
		t.Fatalf("expected distant translation to remain unaligned: %#v", result.Lines[1])
	}
	if len(result.Lines[0].AlternateTexts) != 1 || result.Lines[0].AlternateTexts[0].Role != "translation" {
		t.Fatalf("expected translation alternate: %#v", result.Lines[0])
	}
}

func TestTranslationAlignmentDoesNotReuseOneTranslationLine(t *testing.T) {
	main := []byte("[00:01.00]One\n[00:01.40]Two")
	translation := []byte("[00:01.20]Only once")
	result, err := ParseWithTranslation(main, FormatLRC, translation, FormatLRC, Options{TranslationTolerance: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	translated := 0
	for _, line := range result.Lines {
		if line.Translation != "" {
			translated++
		}
	}
	if translated != 1 {
		t.Fatalf("expected one-to-one translation alignment, got %#v", result.Lines)
	}
}

func TestMalformedTimedFormatsFallBackToPlainText(t *testing.T) {
	tests := []struct {
		name    string
		format  Format
		content string
	}{
		{name: "vtt", format: FormatVTT, content: "WEBVTT\n\nnot a cue\nVisible words"},
		{name: "ttml", format: FormatTTML, content: "<tt><body><p>Visible words</body>"},
		{name: "lrc", format: FormatLRC, content: "Visible words without timing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ParseContent([]byte(test.content), test.format, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if result.Format != FormatPlain || result.TimingQuality != TimingQualityPlain || !strings.Contains(result.PlainText, "Visible words") {
				t.Fatalf("expected safe plain fallback, got %#v", result)
			}
		})
	}
}

func TestParserRejectsOversizedComplexAndUnsafeInputs(t *testing.T) {
	if _, err := ParseContent([]byte(strings.Repeat("x", 65)), FormatLRC, Options{MaxBytes: 64}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
	if _, err := ParseContent([]byte("[00:01.00]one\n[00:02.00]two"), FormatLRC, Options{MaxLines: 1}); !errors.Is(err, ErrTooComplex) {
		t.Fatalf("expected ErrTooComplex, got %v", err)
	}
	unsafe := `<!DOCTYPE tt [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><tt><body><p begin="1s" end="2s">&xxe;</p></body></tt>`
	if _, err := ParseContent([]byte(unsafe), FormatTTML, Options{}); !errors.Is(err, ErrUnsafeMarkup) {
		t.Fatalf("expected unsafe markup rejection, got %v", err)
	}
	deep := `<tt><body><div><section><p begin="1s" end="2s">deep</p></section></div></body></tt>`
	if _, err := ParseContent([]byte(deep), FormatTTML, Options{MaxXMLDepth: 3}); !errors.Is(err, ErrTooComplex) {
		t.Fatalf("expected XML depth limit, got %v", err)
	}
	hugeTime := `<tt><body><p begin="1e100s" end="1e101s">huge</p></body></tt>`
	result, err := ParseContent([]byte(hugeTime), FormatTTML, Options{})
	if err != nil || result.Format != FormatPlain {
		t.Fatalf("expected out-of-range TTML timing to degrade safely, result=%#v err=%v", result, err)
	}
	if _, err := ParseContent([]byte("not timed"), FormatLRC, Options{DisablePlainFallback: true}); !errors.Is(err, ErrNoLyrics) {
		t.Fatalf("expected plain fallback opt-out, got %v", err)
	}
}
