package locallyrics

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	vttTimingLine = regexp.MustCompile(`^((?:\d{1,2}:)?\d{2}:\d{2}\.\d{3})\s*-->\s*((?:\d{1,2}:)?\d{2}:\d{2}\.\d{3})(?:\s+.*)?$`)
	vttInlineTime = regexp.MustCompile(`<((?:\d{1,2}:)?\d{2}:\d{2}\.\d{3})>`)
	vttMarkup     = regexp.MustCompile(`(?s)<[^>]*>`)
)

func parseVTT(text string, options Options) (Result, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(text, "\r", ""))
	blocks := regexp.MustCompile(`\n[ \t]*\n`).Split(normalized, -1)
	if len(blocks) > options.MaxLines*2 {
		return Result{}, ErrTooComplex
	}

	lines := make([]Line, 0)
	quality := TimingQualityLine
	for _, block := range blocks {
		blockLines := strings.Split(block, "\n")
		if len(blockLines) == 0 {
			continue
		}
		first := strings.TrimSpace(blockLines[0])
		upperFirst := strings.ToUpper(first)
		if upperFirst == "WEBVTT" || strings.HasPrefix(upperFirst, "NOTE") || upperFirst == "STYLE" || upperFirst == "REGION" {
			continue
		}

		timingIndex := -1
		var timingMatch []string
		for index, rawLine := range blockLines {
			match := vttTimingLine.FindStringSubmatch(strings.TrimSpace(rawLine))
			if len(match) == 3 {
				timingIndex = index
				timingMatch = match
				break
			}
		}
		if timingIndex < 0 {
			continue
		}

		start, err := parseVTTTimestamp(timingMatch[1])
		if err != nil {
			continue
		}
		end, err := parseVTTTimestamp(timingMatch[2])
		if err != nil || end <= start {
			continue
		}
		cueText := strings.Join(blockLines[timingIndex+1:], "\n")
		plain := cleanVTTText(cueText)
		if plain == "" {
			continue
		}

		words, err := parseVTTWords(cueText, start, end, options)
		if err != nil {
			return Result{}, err
		}
		if len(words) > 0 {
			quality = TimingQualityWord
		}
		lines = append(lines, Line{
			Start: start,
			End:   end,
			Text:  plain,
			Words: words,
		})
		if len(lines) > options.MaxLines {
			return Result{}, ErrTooComplex
		}
	}

	if len(lines) == 0 {
		return buildPlainResult(text), nil
	}
	sort.SliceStable(lines, func(left, right int) bool { return lines[left].Start < lines[right].Start })
	return Result{
		Format:        FormatVTT,
		TimingQuality: quality,
		Lines:         lines,
	}, nil
}

func parseVTTTimestamp(value string) (time.Duration, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid WebVTT timestamp")
	}
	hours := 0
	minutesIndex := 0
	if len(parts) == 3 {
		parsedHours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		hours = parsedHours
		minutesIndex = 1
	}
	minutes, err := strconv.Atoi(parts[minutesIndex])
	if err != nil || minutes >= 60 {
		return 0, fmt.Errorf("invalid WebVTT timestamp")
	}
	secondsParts := strings.Split(parts[minutesIndex+1], ".")
	if len(secondsParts) != 2 || len(secondsParts[1]) != 3 {
		return 0, fmt.Errorf("invalid WebVTT timestamp")
	}
	seconds, err := strconv.Atoi(secondsParts[0])
	if err != nil || seconds >= 60 {
		return 0, fmt.Errorf("invalid WebVTT timestamp")
	}
	milliseconds, err := strconv.Atoi(secondsParts[1])
	if err != nil {
		return 0, err
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(milliseconds)*time.Millisecond, nil
}

func parseVTTWords(cueText string, cueStart time.Duration, cueEnd time.Duration, options Options) ([]Word, error) {
	indices := vttInlineTime.FindAllStringSubmatchIndex(cueText, -1)
	if len(indices) == 0 {
		return nil, nil
	}
	markers := make([]timingMarker, 0, len(indices))
	for _, index := range indices {
		if len(index) < 4 {
			continue
		}
		value, err := parseVTTTimestamp(cueText[index[2]:index[3]])
		if err != nil || value < cueStart || value > cueEnd {
			continue
		}
		markers = append(markers, timingMarker{startIndex: index[0], endIndex: index[1], time: value})
	}
	if len(markers) == 0 {
		return nil, nil
	}

	words := make([]Word, 0, len(markers)+1)
	prefix := cueText[:markers[0].startIndex]
	if text := cleanVTTText(prefix); text != "" && markers[0].time > cueStart {
		words = append(words, Word{
			Start:         cueStart,
			End:           markers[0].time,
			Text:          text,
			EndsWithSpace: trailingSpace(vttMarkup.ReplaceAllString(prefix, "")),
		})
	}
	for index, marker := range markers {
		segmentEnd := len(cueText)
		wordEnd := cueEnd
		if index+1 < len(markers) {
			segmentEnd = markers[index+1].startIndex
			wordEnd = markers[index+1].time
		}
		segment := cueText[marker.endIndex:segmentEnd]
		wordText := cleanVTTText(segment)
		if wordText == "" || wordEnd <= marker.time {
			continue
		}
		words = append(words, Word{
			Start:         marker.time,
			End:           wordEnd,
			Text:          wordText,
			EndsWithSpace: trailingSpace(vttMarkup.ReplaceAllString(segment, "")),
		})
		if len(words) > options.MaxWordsPerLine {
			return nil, ErrTooComplex
		}
	}
	return words, nil
}

func cleanVTTText(text string) string {
	withoutTags := vttMarkup.ReplaceAllString(text, "")
	return normalizeDisplayText(html.UnescapeString(withoutTags))
}
