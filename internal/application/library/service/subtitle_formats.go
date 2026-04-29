package service

import (
	"encoding/xml"
	"html"
	"regexp"
	"strconv"
	"strings"

	"xiadown/internal/application/library/dto"
)

var (
	subtitleTagPattern = regexp.MustCompile(`(?s)<[^>]+>`)
	ittCuePattern      = regexp.MustCompile(`(?is)<p\b([^>]*)>(.*?)</p>`)
	attrPattern        = regexp.MustCompile(`([a-zA-Z_:][a-zA-Z0-9_:\-]*)="([^"]*)"`)
)

func normalizeSubtitleFormat(value string) string {
	switch normalizeTranscodeFormat(value) {
	case "webvtt":
		return "vtt"
	case "dfxp", "ttml", "xml":
		return "itt"
	default:
		return normalizeTranscodeFormat(value)
	}
}

func detectSubtitleFormat(explicit string, path string, fallback string) string {
	if trimmed := normalizeSubtitleFormat(explicit); trimmed != "" {
		return trimmed
	}
	if trimmed := normalizeSubtitleFormat(normalizeFileExtension(path)); trimmed != "" {
		return trimmed
	}
	return firstNonEmpty(normalizeSubtitleFormat(fallback), "srt")
}

func parseSubtitleDocument(content string, format string) dto.SubtitleDocument {
	normalized := detectSubtitleFormat(format, "", format)
	switch normalized {
	case "vtt":
		return dto.SubtitleDocument{Format: normalized, Cues: parseVTTSubtitleCues(content)}
	case "ass", "ssa":
		return dto.SubtitleDocument{Format: normalized, Cues: parseASSSubtitleCues(content)}
	case "itt":
		return dto.SubtitleDocument{Format: normalized, Cues: parseITTSubtitleCues(content)}
	case "fcpxml":
		return dto.SubtitleDocument{Format: normalized, Cues: parseFCPXMLSubtitleCues(content)}
	default:
		return dto.SubtitleDocument{Format: normalized, Cues: parseSRTSubtitleCues(content)}
	}
}

func parseSRTSubtitleCues(content string) []dto.SubtitleCue {
	blocks := splitSubtitleBlocks(content)
	result := make([]dto.SubtitleCue, 0, len(blocks))
	for _, block := range blocks {
		lines := splitNonEmptyLines(block)
		if len(lines) < 2 {
			continue
		}
		index := 0
		if _, err := strconv.Atoi(strings.TrimSpace(lines[0])); err == nil && len(lines) >= 2 {
			index = 1
		}
		if index >= len(lines) || !strings.Contains(lines[index], "-->") {
			continue
		}
		start, end := parseSubtitleTimingLine(lines[index])
		text := strings.Join(lines[index+1:], "\n")
		result = append(result, dto.SubtitleCue{
			Index: len(result) + 1,
			Start: start,
			End:   end,
			Text:  strings.TrimSpace(text),
		})
	}
	return result
}

func parseVTTSubtitleCues(content string) []dto.SubtitleCue {
	blocks := splitSubtitleBlocks(content)
	result := make([]dto.SubtitleCue, 0, len(blocks))
	for _, block := range blocks {
		lines := splitNonEmptyLines(block)
		if len(lines) == 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(lines[0]), "WEBVTT") {
			continue
		}
		if isVTTMetaBlock(lines[0]) {
			continue
		}
		timingIndex := 0
		if !strings.Contains(lines[0], "-->") {
			timingIndex = 1
		}
		if timingIndex >= len(lines) || !strings.Contains(lines[timingIndex], "-->") {
			continue
		}
		start, end := parseSubtitleTimingLine(lines[timingIndex])
		text := strings.Join(lines[timingIndex+1:], "\n")
		result = append(result, dto.SubtitleCue{
			Index: len(result) + 1,
			Start: start,
			End:   end,
			Text:  strings.TrimSpace(text),
		})
	}
	return result
}

func parseASSSubtitleCues(content string) []dto.SubtitleCue {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	result := make([]dto.SubtitleCue, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(strings.ToLower(line), "dialogue:") {
			continue
		}
		payload := strings.TrimSpace(line[len("Dialogue:"):])
		parts := strings.SplitN(payload, ",", 10)
		if len(parts) < 10 {
			continue
		}
		text := strings.ReplaceAll(parts[9], `\N`, "\n")
		text = strings.ReplaceAll(text, `\n`, "\n")
		result = append(result, dto.SubtitleCue{
			Index: len(result) + 1,
			Start: strings.TrimSpace(parts[1]),
			End:   strings.TrimSpace(parts[2]),
			Text:  strings.TrimSpace(text),
		})
	}
	return result
}

func parseITTSubtitleCues(content string) []dto.SubtitleCue {
	matches := ittCuePattern.FindAllStringSubmatch(content, -1)
	result := make([]dto.SubtitleCue, 0, len(matches))
	for _, match := range matches {
		attrs := parseSubtitleAttributes(match[1])
		result = append(result, dto.SubtitleCue{
			Index: len(result) + 1,
			Start: strings.TrimSpace(attrs["begin"]),
			End:   strings.TrimSpace(attrs["end"]),
			Text:  stripSubtitleMarkup(match[2]),
		})
	}
	return result
}

func parseFCPXMLSubtitleCues(content string) []dto.SubtitleCue {
	type titleRow struct {
		Start    string `xml:"start,attr"`
		Duration string `xml:"duration,attr"`
		Name     string `xml:"name,attr"`
		Text     string `xml:",innerxml"`
	}
	type fcpxmlDocument struct {
		Titles []titleRow `xml:"library>event>project>sequence>spine>title"`
	}

	var document fcpxmlDocument
	if err := xml.Unmarshal([]byte(content), &document); err != nil {
		return nil
	}
	result := make([]dto.SubtitleCue, 0, len(document.Titles))
	for _, title := range document.Titles {
		text := strings.TrimSpace(title.Name)
		if text == "" {
			text = stripSubtitleMarkup(title.Text)
		}
		result = append(result, dto.SubtitleCue{
			Index: len(result) + 1,
			Start: strings.TrimSpace(title.Start),
			End:   strings.TrimSpace(title.Duration),
			Text:  text,
		})
	}
	return result
}

func splitSubtitleBlocks(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(normalized, "\n\n")
}

func splitNonEmptyLines(block string) []string {
	rawLines := strings.Split(block, "\n")
	result := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.TrimRight(rawLine, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

func parseSubtitleTimingLine(line string) (string, string) {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return "", ""
	}
	end := strings.TrimSpace(parts[1])
	if fields := strings.Fields(end); len(fields) > 0 {
		end = fields[0]
	}
	return strings.TrimSpace(parts[0]), end
}

func isVTTMetaBlock(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "NOTE") ||
		strings.HasPrefix(trimmed, "STYLE") ||
		strings.HasPrefix(trimmed, "REGION")
}

func parseSubtitleAttributes(raw string) map[string]string {
	result := make(map[string]string)
	for _, match := range attrPattern.FindAllStringSubmatch(raw, -1) {
		if len(match) < 3 {
			continue
		}
		result[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
	}
	return result
}

func stripSubtitleMarkup(content string) string {
	withoutTags := subtitleTagPattern.ReplaceAllString(content, "")
	return strings.TrimSpace(html.UnescapeString(withoutTags))
}
