package locallyrics

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type ttmlPart struct {
	text  string
	child *ttmlNode
}

type ttmlNode struct {
	name  string
	attrs []xml.Attr
	parts []ttmlPart
}

func parseTTML(text string, options Options) (Result, error) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<!entity") {
		return Result{}, ErrUnsafeMarkup
	}

	root, err := parseTTMLTree(text, options)
	if err != nil {
		if errors.Is(err, ErrTooComplex) || errors.Is(err, ErrUnsafeMarkup) {
			return Result{}, err
		}
		return buildPlainResult(text), nil
	}
	if root == nil {
		return buildPlainResult(text), nil
	}

	paragraphs := make([]*ttmlNode, 0)
	collectTTMLNodes(root, "p", &paragraphs)
	if len(paragraphs) > options.MaxLines {
		return Result{}, ErrTooComplex
	}
	sidecarAlternates := collectTTMLSidecarAlternates(root)

	lines := make([]Line, 0, len(paragraphs))
	quality := TimingQualityLine
	hasIncompleteWordTiming := false
	for _, paragraph := range paragraphs {
		start, hasStart := ttmlNodeTime(paragraph, "begin")
		if !hasStart {
			continue
		}
		end, hasEnd := ttmlNodeTime(paragraph, "end")
		if !hasEnd {
			if duration, ok := ttmlNodeTime(paragraph, "dur"); ok {
				end = start + duration
				hasEnd = true
			}
		}

		mainText := normalizeDisplayText(ttmlNodeMainText(paragraph, ""))
		if mainText == "" {
			continue
		}
		words, hasSyllables, err := collectTTMLWords(paragraph, options)
		if err != nil {
			return Result{}, err
		}
		words, hasFlatSyllables := reconcileTTMLWordGroups(words, mainText)
		hasSyllables = hasSyllables || hasFlatSyllables
		if len(words) > 0 && !ttmlWordsCoverText(words, mainText) {
			// Zero-duration or otherwise invalid spans are intentionally omitted
			// rather than assigned fabricated timing. If that leaves only part of
			// the visible lyric timed, expose this as a line-timed row so renderers
			// never silently drop the uncovered text.
			words = nil
			hasSyllables = false
			hasIncompleteWordTiming = true
		}
		if hasSyllables {
			quality = TimingQualitySyllable
		} else if len(words) > 0 && timingQualityRank(quality) < timingQualityRank(TimingQualityWord) {
			quality = TimingQualityWord
		}
		if !hasEnd && len(words) > 0 {
			end = words[len(words)-1].End
			hasEnd = end > start
		}

		alternates := collectTTMLAlternates(paragraph)
		if reference := ttmlParagraphReference(paragraph); reference != "" {
			alternates = mergeTTMLAlternates(alternates, sidecarAlternates[reference])
		} else {
			alternates = mergeTTMLAlternates(alternates)
		}
		translation := ""
		for _, alternate := range alternates {
			if alternate.Role == "translation" && translation == "" {
				translation = alternate.Text
			}
		}
		line := Line{
			Start:          start,
			End:            end,
			EndEstimated:   !hasEnd,
			Text:           mainText,
			Translation:    translation,
			AlternateTexts: alternates,
			Words:          words,
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return buildPlainResult(text), nil
	}
	if hasIncompleteWordTiming {
		quality = TimingQualityLine
	}
	sort.SliceStable(lines, func(left, right int) bool { return lines[left].Start < lines[right].Start })
	return Result{
		Format:        FormatTTML,
		TimingQuality: quality,
		Lines:         lines,
	}, nil
}

func parseTTMLTree(text string, options Options) (*ttmlNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(text))
	decoder.Strict = true
	stack := make([]*ttmlNode, 0, 16)
	var root *ttmlNode
	tokens := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		tokens++
		if tokens > options.MaxXMLTokens {
			return nil, ErrTooComplex
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if len(stack)+1 > options.MaxXMLDepth {
				return nil, ErrTooComplex
			}
			node := &ttmlNode{name: strings.ToLower(typed.Name.Local), attrs: append([]xml.Attr(nil), typed.Attr...)}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.parts = append(parent.parts, ttmlPart{child: node})
			} else if root == nil {
				root = node
			} else {
				return nil, fmt.Errorf("multiple XML roots")
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected XML end element")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				value := string(append([]byte(nil), typed...))
				stack[len(stack)-1].parts = append(stack[len(stack)-1].parts, ttmlPart{text: value})
			}
		case xml.Directive:
			value := strings.ToLower(string(typed))
			if strings.Contains(value, "doctype") || strings.Contains(value, "entity") {
				return nil, ErrUnsafeMarkup
			}
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed XML element")
	}
	return root, nil
}

func collectTTMLNodes(node *ttmlNode, name string, output *[]*ttmlNode) {
	if node == nil {
		return
	}
	if node.name == name {
		*output = append(*output, node)
	}
	for _, part := range node.parts {
		collectTTMLNodes(part.child, name, output)
	}
}

func ttmlAttribute(node *ttmlNode, name string) (string, bool) {
	if node == nil {
		return "", false
	}
	for _, attribute := range node.attrs {
		if strings.EqualFold(attribute.Name.Local, name) {
			return strings.TrimSpace(attribute.Value), true
		}
	}
	return "", false
}

func ttmlNodeTime(node *ttmlNode, name string) (time.Duration, bool) {
	value, ok := ttmlAttribute(node, name)
	if !ok || value == "" {
		return 0, false
	}
	parsed, err := parseTTMLTime(value)
	return parsed, err == nil
}

func parseTTMLTime(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty TTML time")
	}
	units := []struct {
		suffix string
		unit   time.Duration
	}{
		{suffix: "ms", unit: time.Millisecond},
		{suffix: "h", unit: time.Hour},
		{suffix: "m", unit: time.Minute},
		{suffix: "s", unit: time.Second},
	}
	for _, candidate := range units {
		if strings.HasSuffix(value, candidate.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, candidate.suffix))
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed > float64(maxSupportedLyricTime)/float64(candidate.unit) {
				return 0, fmt.Errorf("invalid TTML time")
			}
			return time.Duration(parsed * float64(candidate.unit)), nil
		}
	}
	// AMLL's Apple Music-style TTML uses bare decimal offset times (for
	// example begin="21.565") even though most TTML producers include the
	// explicit `s` suffix. Treat the bare value as seconds while retaining the
	// same finite/range checks used by suffixed offsets.
	if !strings.Contains(value, ":") {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed > float64(maxSupportedLyricTime)/float64(time.Second) {
			return 0, fmt.Errorf("invalid TTML time")
		}
		return time.Duration(parsed * float64(time.Second)), nil
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("unsupported TTML time")
	}
	hours := 0.0
	minutesIndex := 0
	if len(parts) == 3 {
		parsedHours, err := strconv.ParseFloat(parts[0], 64)
		if err != nil || parsedHours < 0 {
			return 0, err
		}
		hours = parsedHours
		minutesIndex = 1
	}
	minutes, err := strconv.ParseFloat(parts[minutesIndex], 64)
	if err != nil || minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("invalid TTML time")
	}
	seconds, err := strconv.ParseFloat(parts[minutesIndex+1], 64)
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, fmt.Errorf("invalid TTML time")
	}
	total := hours*float64(time.Hour) + minutes*float64(time.Minute) + seconds*float64(time.Second)
	if math.IsNaN(total) || math.IsInf(total, 0) || total < 0 || total > float64(maxSupportedLyricTime) {
		return 0, fmt.Errorf("invalid TTML time")
	}
	return time.Duration(total), nil
}

func ttmlRole(node *ttmlNode, inherited string) string {
	value, ok := ttmlAttribute(node, "role")
	if !ok || value == "" {
		return inherited
	}
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "trans"):
		return "translation"
	case strings.Contains(lower, "roman"):
		return "romanization"
	case strings.Contains(lower, "background") || strings.Contains(lower, "x-bg"):
		return "background"
	default:
		return lower
	}
}

func ttmlNodeMainText(node *ttmlNode, inheritedRole string) string {
	if node == nil {
		return ""
	}
	role := ttmlRole(node, inheritedRole)
	if role == "translation" || role == "romanization" || role == "background" {
		return ""
	}
	var builder strings.Builder
	for _, part := range node.parts {
		if part.child != nil {
			if part.child.name == "br" {
				builder.WriteByte('\n')
			} else {
				builder.WriteString(ttmlNodeMainText(part.child, role))
			}
		} else {
			builder.WriteString(ttmlSemanticTextPart(part.text))
		}
	}
	return builder.String()
}

func ttmlNodeText(node *ttmlNode) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	for _, part := range node.parts {
		if part.child != nil {
			if part.child.name == "br" {
				builder.WriteByte('\n')
			} else {
				builder.WriteString(ttmlNodeText(part.child))
			}
		} else {
			builder.WriteString(ttmlSemanticTextPart(part.text))
		}
	}
	return builder.String()
}

func ttmlSemanticTextPart(value string) string {
	// Pretty-printed TTML commonly inserts newline + indentation CharData
	// between timed spans. That layout whitespace is not lyric content. A
	// same-line space remains meaningful, however, because AMLL uses it to
	// distinguish adjacent words from adjacent syllables.
	if strings.TrimSpace(value) == "" && strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func ttmlParagraphReference(paragraph *ttmlNode) string {
	if value, ok := ttmlAttribute(paragraph, "key"); ok && value != "" {
		return normalizeTTMLReference(value)
	}
	if value, ok := ttmlAttribute(paragraph, "id"); ok && value != "" {
		return normalizeTTMLReference(value)
	}
	return ""
}

func normalizeTTMLReference(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// collectTTMLSidecarAlternates reads Apple Music's iTunesMetadata sidecar.
// Translations and transliterations reference body paragraphs through
// `<text for="L1">`, while the timed paragraph exposes `itunes:key="L1"`
// (or, less commonly, xml:id/id). Element namespaces are intentionally
// matched by local name because encoding/xml has already resolved prefixes.
func collectTTMLSidecarAlternates(root *ttmlNode) map[string][]AlternateText {
	result := make(map[string][]AlternateText)
	var walk func(*ttmlNode, bool, string, string)
	walk = func(node *ttmlNode, inITunesMetadata bool, inheritedRole string, inheritedLanguage string) {
		if node == nil {
			return
		}
		language := inheritedLanguage
		if value, ok := ttmlAttribute(node, "lang"); ok && value != "" {
			language = value
		}
		if node.name == "itunesmetadata" {
			inITunesMetadata = true
		}
		if !inITunesMetadata {
			for _, part := range node.parts {
				walk(part.child, false, inheritedRole, language)
			}
			return
		}

		role := inheritedRole
		switch node.name {
		case "translation":
			role = "translation"
		case "transliteration":
			role = "romanization"
		}
		if node.name == "text" && (role == "translation" || role == "romanization") {
			reference, ok := ttmlAttribute(node, "for")
			text := normalizeDisplayText(ttmlNodeMainText(node, ""))
			if normalizedReference := normalizeTTMLReference(reference); ok && normalizedReference != "" && text != "" {
				result[normalizedReference] = append(result[normalizedReference], AlternateText{
					Role:     role,
					Language: language,
					Text:     text,
				})
			}
		}

		for _, part := range node.parts {
			walk(part.child, inITunesMetadata, role, language)
		}
	}
	walk(root, false, "", "")
	return result
}

func mergeTTMLAlternates(groups ...[]AlternateText) []AlternateText {
	result := make([]AlternateText, 0)
	for _, group := range groups {
		for _, alternate := range group {
			alternate.Role = strings.ToLower(strings.TrimSpace(alternate.Role))
			alternate.Language = strings.TrimSpace(alternate.Language)
			alternate.Text = normalizeDisplayText(alternate.Text)
			if alternate.Role == "" || alternate.Text == "" {
				continue
			}

			duplicate := false
			for index := range result {
				existing := &result[index]
				if existing.Role != alternate.Role || existing.Text != alternate.Text {
					continue
				}
				if strings.EqualFold(existing.Language, alternate.Language) {
					duplicate = true
					break
				}
				// Treat an otherwise identical language-less entry as the same
				// alternate, retaining the more informative language tag.
				if existing.Language == "" {
					existing.Language = alternate.Language
					duplicate = true
					break
				}
				if alternate.Language == "" {
					duplicate = true
					break
				}
			}
			if !duplicate {
				result = append(result, alternate)
			}
		}
	}
	return result
}

func collectTTMLAlternates(paragraph *ttmlNode) []AlternateText {
	alternates := make([]AlternateText, 0)
	var walk func(*ttmlNode, string, string)
	walk = func(node *ttmlNode, inheritedRole string, inheritedLanguage string) {
		if node == nil {
			return
		}
		role := ttmlRole(node, inheritedRole)
		language := inheritedLanguage
		if value, ok := ttmlAttribute(node, "lang"); ok && value != "" {
			language = value
		}
		if role != inheritedRole && (role == "translation" || role == "romanization" || role == "background") {
			text := normalizeDisplayText(ttmlNodeText(node))
			if text != "" {
				alternates = append(alternates, AlternateText{Role: role, Language: language, Text: text})
			}
			return
		}
		for _, part := range node.parts {
			walk(part.child, role, language)
		}
	}
	walk(paragraph, "", "")
	return alternates
}

func collectTTMLWords(paragraph *ttmlNode, options Options) ([]Word, bool, error) {
	words := make([]Word, 0)
	hasSyllables := false
	var walk func(*ttmlNode, string) error
	walk = func(node *ttmlNode, inheritedRole string) error {
		if node == nil {
			return nil
		}
		role := ttmlRole(node, inheritedRole)
		if role == "translation" || role == "romanization" || role == "background" {
			return nil
		}

		if node.name == "span" {
			start, hasStart := ttmlNodeTime(node, "begin")
			end, hasEnd := ttmlNodeTime(node, "end")
			if !hasEnd {
				if duration, ok := ttmlNodeTime(node, "dur"); ok && hasStart {
					end = start + duration
					hasEnd = true
				}
			}
			if hasStart && hasEnd && end > start {
				timedChildren := make([]Word, 0)
				for _, part := range node.parts {
					child := part.child
					if child == nil || child.name != "span" {
						continue
					}
					childStart, childHasStart := ttmlNodeTime(child, "begin")
					childEnd, childHasEnd := ttmlNodeTime(child, "end")
					if !childHasEnd {
						if duration, ok := ttmlNodeTime(child, "dur"); ok && childHasStart {
							childEnd = childStart + duration
							childHasEnd = true
						}
					}
					childRawText := ttmlNodeMainText(child, role)
					childText := normalizeDisplayText(childRawText)
					if childHasStart && childHasEnd && childEnd > childStart && childText != "" {
						timedChildren = append(timedChildren, Word{
							Start:         childStart,
							End:           childEnd,
							Text:          childText,
							EndsWithSpace: trailingSpace(childRawText),
						})
					}
				}

				rawText := ttmlNodeMainText(node, role)
				text := normalizeDisplayText(rawText)
				groupLooksLikeWord := text != "" && !strings.ContainsFunc(strings.TrimSpace(rawText), unicode.IsSpace)
				if len(timedChildren) > 0 && groupLooksLikeWord {
					words = append(words, Word{
						Start:         start,
						End:           end,
						Text:          text,
						EndsWithSpace: trailingSpace(rawText),
						Syllables:     timedChildren,
					})
					hasSyllables = true
					if len(words) > options.MaxWordsPerLine {
						return ErrTooComplex
					}
					return nil
				}
				if len(timedChildren) == 0 && text != "" {
					words = append(words, Word{
						Start:         start,
						End:           end,
						Text:          text,
						EndsWithSpace: trailingSpace(rawText),
					})
					if len(words) > options.MaxWordsPerLine {
						return ErrTooComplex
					}
					return nil
				}
			}
		}

		for _, part := range node.parts {
			if err := walk(part.child, role); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(paragraph, ""); err != nil {
		return nil, false, err
	}
	sort.SliceStable(words, func(left, right int) bool { return words[left].Start < words[right].Start })
	return words, hasSyllables, nil
}

// reconcileTTMLWordGroups aligns flat timed spans back to the paragraph's
// readable text. AMLL commonly represents a lexical word as adjacent spans
// (`sto` + `ry`) and puts inter-word whitespace in XML character data between
// the spans. The XML tree walker intentionally ignores that character data
// for timing, so restore both the lexical grouping and the explicit trailing
// space contract here. Nested word/syllable structures remain intact.
func reconcileTTMLWordGroups(words []Word, sourceText string) ([]Word, bool) {
	if len(words) == 0 || strings.TrimSpace(sourceText) == "" {
		return words, false
	}

	type alignedWord struct {
		word       Word
		start, end int
	}
	aligned := make([]alignedWord, 0, len(words))
	cursor := 0
	for _, word := range words {
		needle := strings.TrimSpace(word.Text)
		if needle == "" {
			return words, false
		}
		relative := strings.Index(sourceText[cursor:], needle)
		if relative < 0 {
			return words, false
		}
		start := cursor + relative
		end := start + len(needle)
		aligned = append(aligned, alignedWord{word: word, start: start, end: end})
		cursor = end
	}

	result := make([]Word, 0, len(aligned))
	hasSyllables := false
	for index, current := range aligned {
		word := current.word
		nextStart := len(sourceText)
		if index+1 < len(aligned) {
			nextStart = aligned[index+1].start
		}
		separator := sourceText[current.end:nextStart]
		endsWithSpace := strings.ContainsFunc(separator, unicode.IsSpace) || word.EndsWithSpace

		if len(result) > 0 {
			previousAligned := aligned[index-1]
			between := sourceText[previousAligned.end:current.start]
			if !strings.ContainsFunc(between, unicode.IsSpace) && len(result[len(result)-1].Syllables) == 0 && len(word.Syllables) == 0 {
				previous := &result[len(result)-1]
				firstSyllable := *previous
				firstSyllable.EndsWithSpace = false
				firstSyllable.Syllables = nil
				word.EndsWithSpace = false
				previous.Syllables = []Word{firstSyllable, word}
				previous.Text += between + word.Text
				previous.End = word.End
				previous.EndsWithSpace = endsWithSpace
				hasSyllables = true
				continue
			}
			if !strings.ContainsFunc(between, unicode.IsSpace) && len(result[len(result)-1].Syllables) > 0 && len(word.Syllables) == 0 {
				previous := &result[len(result)-1]
				word.EndsWithSpace = false
				previous.Syllables = append(previous.Syllables, word)
				previous.Text += between + word.Text
				previous.End = word.End
				previous.EndsWithSpace = endsWithSpace
				hasSyllables = true
				continue
			}
		}

		word.EndsWithSpace = endsWithSpace
		result = append(result, word)
	}
	return result, hasSyllables
}

func ttmlWordsCoverText(words []Word, sourceText string) bool {
	if len(words) == 0 {
		return false
	}
	var builder strings.Builder
	for _, word := range words {
		builder.WriteString(word.Text)
		if word.EndsWithSpace {
			builder.WriteByte(' ')
		}
	}
	return normalizeDisplayText(builder.String()) == normalizeDisplayText(sourceText)
}
