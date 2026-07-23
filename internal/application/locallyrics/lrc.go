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
	lrcTimestamp   = regexp.MustCompile(`\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\]`)
	angleTimestamp = regexp.MustCompile(`<(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?>`)
	lrcMetadata    = regexp.MustCompile(`^\[([[:alpha:]]+):([^\]]*)\]$`)
)

type timingMarker struct {
	startIndex int
	endIndex   int
	time       time.Duration
}

type lrcDraft struct {
	start       time.Duration
	explicitEnd time.Duration
	text        string
	words       []Word
	order       int
}

func parseLRC(text string, options Options) (Result, error) {
	rawLines := strings.Split(text, "\n")
	if len(rawLines) > options.MaxLines*4 {
		return Result{}, ErrTooComplex
	}

	offset := time.Duration(0)
	title := ""
	artist := ""
	for _, rawLine := range rawLines {
		match := lrcMetadata.FindStringSubmatch(strings.TrimSpace(rawLine))
		if len(match) != 3 {
			continue
		}
		key := strings.ToLower(match[1])
		value := strings.TrimSpace(match[2])
		switch key {
		case "offset":
			milliseconds, err := strconv.ParseInt(value, 10, 64)
			if err == nil && milliseconds <= maxSupportedLyricTime.Milliseconds() && milliseconds >= -maxSupportedLyricTime.Milliseconds() {
				offset = time.Duration(milliseconds) * time.Millisecond
			}
		case "ti", "title":
			title = value
		case "ar", "artist":
			artist = value
		}
	}

	drafts := make([]lrcDraft, 0)
	order := 0
	for _, rawLine := range rawLines {
		line := strings.TrimSpace(rawLine)
		if line == "" || lrcMetadata.MatchString(line) {
			continue
		}
		lineDrafts, err := parseLRCLine(line, offset, order, options)
		if err != nil {
			return Result{}, err
		}
		order += len(lineDrafts)
		drafts = append(drafts, lineDrafts...)
		if len(drafts) > options.MaxLines {
			return Result{}, ErrTooComplex
		}
	}

	if len(drafts) == 0 {
		return buildPlainResult(text), nil
	}
	sort.SliceStable(drafts, func(left, right int) bool {
		if drafts[left].start == drafts[right].start {
			return drafts[left].order < drafts[right].order
		}
		return drafts[left].start < drafts[right].start
	})

	lines := make([]Line, 0, len(drafts))
	quality := TimingQualityLine
	format := FormatLRC
	for index, draft := range drafts {
		lineEnd := draft.explicitEnd
		endEstimated := lineEnd <= draft.start
		if lineEnd <= draft.start {
			for nextIndex := index + 1; nextIndex < len(drafts); nextIndex++ {
				if drafts[nextIndex].start > draft.start {
					lineEnd = drafts[nextIndex].start
					break
				}
			}
		}
		if lineEnd <= draft.start {
			lineEnd = draft.start + options.DefaultLineDuration
			endEstimated = true
		}

		words := append([]Word(nil), draft.words...)
		for wordIndex := range words {
			if words[wordIndex].End <= words[wordIndex].Start {
				if wordIndex+1 < len(words) && words[wordIndex+1].Start > words[wordIndex].Start {
					words[wordIndex].End = words[wordIndex+1].Start
				} else {
					words[wordIndex].End = lineEnd
				}
			}
			if words[wordIndex].End > lineEnd {
				lineEnd = words[wordIndex].End
				endEstimated = false
			}
		}
		if len(words) > 0 {
			quality = TimingQualityWord
			format = FormatEnhancedLRC
		}
		lines = append(lines, Line{
			Start:        draft.start,
			End:          lineEnd,
			EndEstimated: endEstimated,
			Text:         draft.text,
			Words:        words,
		})
	}

	return Result{
		Format:        format,
		TimingQuality: quality,
		Title:         title,
		Artist:        artist,
		Lines:         lines,
	}, nil
}

func parseLRCLine(line string, offset time.Duration, order int, options Options) ([]lrcDraft, error) {
	markers, err := collectMarkers(line, lrcTimestamp)
	if err != nil || len(markers) == 0 {
		return nil, err
	}

	leading := make([]timingMarker, 0)
	cursor := 0
	for _, marker := range markers {
		if marker.startIndex != cursor {
			break
		}
		leading = append(leading, marker)
		cursor = marker.endIndex
	}
	if len(leading) == 0 {
		return nil, nil
	}

	body := line[cursor:]
	angleMarkers, err := collectMarkers(body, angleTimestamp)
	if err != nil {
		return nil, err
	}
	if len(angleMarkers) > 0 {
		words, explicitEnd, plain, err := wordsFromMarkers(body, angleMarkers, offset, options)
		if err != nil {
			return nil, err
		}
		if plain == "" {
			return nil, nil
		}
		return []lrcDraft{{
			start:       applyLRCOffset(leading[0].time, offset),
			explicitEnd: explicitEnd,
			text:        plain,
			words:       words,
			order:       order,
		}}, nil
	}

	if hasTimedTextBetween(markers, line) {
		words, explicitEnd, plain, err := wordsFromMarkers(line, markers, offset, options)
		if err != nil {
			return nil, err
		}
		if plain == "" {
			return nil, nil
		}
		return []lrcDraft{{
			start:       applyLRCOffset(markers[0].time, offset),
			explicitEnd: explicitEnd,
			text:        plain,
			words:       words,
			order:       order,
		}}, nil
	}

	plain := normalizeDisplayText(body)
	if plain == "" {
		return nil, nil
	}
	drafts := make([]lrcDraft, 0, len(leading))
	for index, marker := range leading {
		drafts = append(drafts, lrcDraft{
			start: applyLRCOffset(marker.time, offset),
			text:  plain,
			order: order + index,
		})
	}
	return drafts, nil
}

func collectMarkers(text string, pattern *regexp.Regexp) ([]timingMarker, error) {
	indices := pattern.FindAllStringSubmatchIndex(text, -1)
	markers := make([]timingMarker, 0, len(indices))
	for _, index := range indices {
		if len(index) < 8 {
			continue
		}
		minute := text[index[2]:index[3]]
		second := text[index[4]:index[5]]
		fraction := ""
		if index[6] >= 0 {
			fraction = text[index[6]:index[7]]
		}
		value, err := parseMinuteTimestamp(minute, second, fraction)
		if err != nil {
			return nil, err
		}
		markers = append(markers, timingMarker{startIndex: index[0], endIndex: index[1], time: value})
	}
	return markers, nil
}

func parseMinuteTimestamp(minute string, second string, fraction string) (time.Duration, error) {
	minutes, err := strconv.Atoi(minute)
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.Atoi(second)
	if err != nil || seconds >= 60 {
		return 0, fmt.Errorf("invalid lyric timestamp")
	}
	milliseconds := 0
	if fraction != "" {
		switch len(fraction) {
		case 1:
			fraction += "00"
		case 2:
			fraction += "0"
		case 3:
		default:
			fraction = fraction[:3]
		}
		milliseconds, err = strconv.Atoi(fraction)
		if err != nil {
			return 0, err
		}
	}
	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(milliseconds)*time.Millisecond, nil
}

func hasTimedTextBetween(markers []timingMarker, line string) bool {
	for index := 0; index+1 < len(markers); index++ {
		if strings.TrimSpace(line[markers[index].endIndex:markers[index+1].startIndex]) != "" {
			return true
		}
	}
	return false
}

func wordsFromMarkers(text string, markers []timingMarker, offset time.Duration, options Options) ([]Word, time.Duration, string, error) {
	words := make([]Word, 0, len(markers))
	plainBuilder := strings.Builder{}
	explicitEnd := time.Duration(0)

	if len(markers) > 0 && strings.TrimSpace(text[:markers[0].startIndex]) != "" {
		prefix := text[:markers[0].startIndex]
		plainBuilder.WriteString(prefix)
	}
	for index, marker := range markers {
		segmentStart := marker.endIndex
		segmentEnd := len(text)
		endTime := time.Duration(0)
		if index+1 < len(markers) {
			segmentEnd = markers[index+1].startIndex
			endTime = applyLRCOffset(markers[index+1].time, offset)
		}
		segment := text[segmentStart:segmentEnd]
		plainBuilder.WriteString(segment)
		wordText := normalizeDisplayText(segment)
		if wordText == "" {
			if index == len(markers)-1 {
				explicitEnd = applyLRCOffset(marker.time, offset)
			}
			continue
		}
		words = append(words, Word{
			Start:         applyLRCOffset(marker.time, offset),
			End:           endTime,
			Text:          wordText,
			EndsWithSpace: trailingSpace(segment),
		})
		if len(words) > options.MaxWordsPerLine {
			return nil, 0, "", ErrTooComplex
		}
	}
	plain := normalizeDisplayText(html.UnescapeString(plainBuilder.String()))
	return words, explicitEnd, plain, nil
}

// LRC's global offset is a display compensation: positive values make lyrics
// appear earlier and therefore subtract from every encoded timestamp.
func applyLRCOffset(timestamp time.Duration, offset time.Duration) time.Duration {
	adjusted := timestamp - offset
	if adjusted < 0 {
		return 0
	}
	return adjusted
}
