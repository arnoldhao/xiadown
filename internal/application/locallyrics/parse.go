package locallyrics

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	timedLRCProbe = regexp.MustCompile(`\[[0-9]{1,3}:[0-9]{2}(?:[.:][0-9]{1,3})?\]`)
	enhancedProbe = regexp.MustCompile(`<[0-9]{1,3}:[0-9]{2}(?:[.:][0-9]{1,3})?>`)
	xmlTagProbe   = regexp.MustCompile(`(?s)<[^>]+>`)
)

// ParseContent parses trusted in-memory bytes subject to the same limits used
// for files. The source metadata in options is intentionally supplied by the
// more explicit embedded and sidecar entry points.
func ParseContent(content []byte, hint Format, options Options) (Result, error) {
	options = normalizeOptions(options)
	if int64(len(content)) > options.MaxBytes {
		return Result{}, ErrTooLarge
	}
	text := normalizeInputText(content)
	if strings.TrimSpace(text) == "" {
		return Result{}, ErrNoLyrics
	}

	format := normalizeFormatHint(hint)
	if format == "" || format == FormatPlain {
		format = detectFormat(text)
	}

	var (
		result Result
		err    error
	)
	switch format {
	case FormatLRC, FormatEnhancedLRC:
		result, err = parseLRC(text, options)
	case FormatVTT:
		result, err = parseVTT(text, options)
	case FormatTTML:
		result, err = parseTTML(text, options)
	default:
		result = buildPlainResult(text)
	}
	if err != nil {
		return Result{}, err
	}
	if len(result.Lines) == 0 && result.TimingQuality != TimingQualityPlain {
		result = buildPlainResult(text)
	}
	if result.TimingQuality == TimingQualityPlain && options.DisablePlainFallback {
		return Result{}, fmt.Errorf("%w: timed lyrics unavailable", ErrNoLyrics)
	}
	return normalizeResult(result, options), nil
}

// ParseWithTranslation parses two explicitly supplied streams and aligns the
// secondary stream to the main timeline. It is used by sidecar and embedded
// entry points so translation behavior remains identical.
func ParseWithTranslation(mainContent []byte, mainHint Format, translationContent []byte, translationHint Format, options Options) (Result, error) {
	options = normalizeOptions(options)
	mainResult, err := ParseContent(mainContent, mainHint, options)
	if err != nil {
		return Result{}, err
	}
	if len(translationContent) == 0 {
		return mainResult, nil
	}
	translationResult, err := ParseContent(translationContent, translationHint, options)
	if err != nil {
		return Result{}, fmt.Errorf("parse translation: %w", err)
	}
	mergeTranslation(&mainResult, translationResult, options.TranslationTolerance)
	return mainResult, nil
}

func normalizeFormatHint(format Format) Format {
	switch strings.ToLower(strings.TrimSpace(string(format))) {
	case "lrc":
		return FormatLRC
	case "enhanced-lrc", "enhanced_lrc", "elrc":
		return FormatEnhancedLRC
	case "vtt", "webvtt":
		return FormatVTT
	case "ttml", "xml":
		return FormatTTML
	case "plain", "txt", "text":
		return FormatPlain
	default:
		return ""
	}
}

func detectFormat(text string) Format {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<tt") && (strings.Contains(lower, "<p") || strings.Contains(lower, "xmlns")) {
		return FormatTTML
	}
	if strings.HasPrefix(lower, "webvtt") || strings.Contains(trimmed, "-->") {
		return FormatVTT
	}
	if timedLRCProbe.MatchString(trimmed) {
		if enhancedProbe.MatchString(trimmed) || hasInlineBracketTiming(trimmed) {
			return FormatEnhancedLRC
		}
		return FormatLRC
	}
	return FormatPlain
}

func hasInlineBracketTiming(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		matches := timedLRCProbe.FindAllStringIndex(line, -1)
		if len(matches) < 2 {
			continue
		}
		for index := 0; index < len(matches)-1; index++ {
			between := line[matches[index][1]:matches[index+1][0]]
			if strings.TrimSpace(between) != "" {
				return true
			}
		}
	}
	return false
}

func normalizeInputText(content []byte) string {
	text := string(content)
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\x00", "")
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "�")
	}
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func buildPlainResult(text string) Result {
	plain := plainTextFromMarkup(text)
	return Result{
		Format:        FormatPlain,
		TimingQuality: TimingQualityPlain,
		PlainText:     plain,
	}
}

func plainTextFromMarkup(text string) string {
	text = timedLRCProbe.ReplaceAllString(text, "")
	text = enhancedProbe.ReplaceAllString(text, "")
	text = xmlTagProbe.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	lines := make([]string, 0)
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.Join(strings.Fields(raw), " "))
		if line == "" || strings.EqualFold(line, "WEBVTT") || strings.Contains(line, "-->") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func normalizeResult(result Result, options Options) Result {
	if len(result.Lines) > options.MaxLines {
		result.Lines = result.Lines[:options.MaxLines]
	}
	sort.SliceStable(result.Lines, func(left, right int) bool {
		return result.Lines[left].Start < result.Lines[right].Start
	})
	for index := range result.Lines {
		line := &result.Lines[index]
		if line.Start < 0 {
			line.Start = 0
		}
		if line.End < 0 {
			line.End = 0
		}
		if len(line.Words) > options.MaxWordsPerLine {
			line.Words = line.Words[:options.MaxWordsPerLine]
		}
		if line.End <= line.Start {
			if index+1 < len(result.Lines) && result.Lines[index+1].Start > line.Start {
				line.End = result.Lines[index+1].Start
			} else {
				line.End = line.Start + options.DefaultLineDuration
			}
			line.EndEstimated = true
		}
		for wordIndex := range line.Words {
			word := &line.Words[wordIndex]
			if word.Start < 0 {
				word.Start = 0
			}
			if word.End < 0 {
				word.End = 0
			}
			if word.End <= word.Start {
				if wordIndex+1 < len(line.Words) && line.Words[wordIndex+1].Start > word.Start {
					word.End = line.Words[wordIndex+1].Start
				} else {
					word.End = line.End
				}
			}
			if word.End > line.End {
				line.End = word.End
				line.EndEstimated = false
			}
		}
	}
	if result.PlainText == "" && len(result.Lines) > 0 {
		parts := make([]string, 0, len(result.Lines))
		for _, line := range result.Lines {
			if strings.TrimSpace(line.Text) != "" {
				parts = append(parts, line.Text)
			}
		}
		result.PlainText = strings.Join(parts, "\n")
	}
	return result
}

func trailingSpace(text string) bool {
	for index := len(text); index > 0; {
		r, size := utf8.DecodeLastRuneInString(text[:index])
		if r == utf8.RuneError && size == 0 {
			return false
		}
		return unicode.IsSpace(r)
	}
	return false
}

func normalizeDisplayText(text string) string {
	return strings.Join(strings.Fields(html.UnescapeString(text)), " ")
}

func mergeTranslation(main *Result, translation Result, tolerance time.Duration) {
	if main == nil || len(main.Lines) == 0 || len(translation.Lines) == 0 {
		return
	}
	translationLines := translation.Lines
	translationIndex := 0
	for lineIndex := range main.Lines {
		line := &main.Lines[lineIndex]
		for translationIndex < len(translationLines) && translationLines[translationIndex].Start < line.Start-tolerance {
			translationIndex++
		}
		if translationIndex >= len(translationLines) || translationLines[translationIndex].Start > line.Start+tolerance {
			continue
		}

		best := translationIndex
		if translationIndex+1 < len(translationLines) && translationLines[translationIndex+1].Start <= line.Start+tolerance {
			currentDifference := durationAbs(translationLines[translationIndex].Start - line.Start)
			nextDifference := durationAbs(translationLines[translationIndex+1].Start - line.Start)
			if nextDifference < currentDifference {
				best = translationIndex + 1
			}
		}
		translationIndex = best + 1
		if strings.TrimSpace(translationLines[best].Text) == "" {
			continue
		}
		line.Translation = translationLines[best].Text
		line.AlternateTexts = append(line.AlternateTexts, AlternateText{
			Role: "translation",
			Text: translationLines[best].Text,
		})
	}
}

func durationAbs(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
