package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func (service *LibraryService) buildFileDTO(ctx context.Context, item library.LibraryFile) (dto.LibraryFileDTO, error) {
	config := library.DefaultModuleConfig()
	if service != nil {
		moduleConfig, err := service.getModuleConfig(ctx)
		if err != nil {
			return dto.LibraryFileDTO{}, err
		}
		config = moduleConfig
	}
	return service.buildFileDTOWithConfig(ctx, item, config)
}

func (service *LibraryService) buildFileDTOWithConfig(ctx context.Context, item library.LibraryFile, config library.ModuleConfig) (dto.LibraryFileDTO, error) {
	result := toLibraryFileDTO(item)
	if item.Kind != library.FileKindSubtitle || service == nil || service.subtitles == nil {
		return result, nil
	}
	document, err := service.subtitles.GetByFileID(ctx, item.ID)
	if err != nil {
		if err == library.ErrSubtitleDocumentNotFound {
			return result, nil
		}
		return dto.LibraryFileDTO{}, err
	}
	content := strings.TrimSpace(document.WorkingContent)
	if content == "" {
		content = document.OriginalContent
	}
	parsed := parseSubtitleDocument(content, detectSubtitleFormat(document.Format, item.Storage.LocalPath, document.Format))
	cueCount := len(parsed.Cues)
	if result.Media == nil {
		result.Media = &dto.LibraryMediaInfoDTO{}
	}
	if strings.TrimSpace(result.Media.Format) == "" {
		result.Media.Format = detectSubtitleFormat(document.Format, item.Storage.LocalPath, "")
	}
	result.Media.CueCount = &cueCount
	if language := detectSubtitleLanguage(item, content, config); language != "" {
		result.Media.Language = language
	}
	result.DisplayLabel = buildLibraryFileDisplayLabel(result)
	return result, nil
}

func (service *LibraryService) mustBuildFileDTO(ctx context.Context, item library.LibraryFile) dto.LibraryFileDTO {
	result, err := service.buildFileDTO(ctx, item)
	if err != nil {
		return toLibraryFileDTO(item)
	}
	return result
}

func buildLibraryFileDisplayLabel(item dto.LibraryFileDTO) string {
	codec := strings.ToUpper(strings.TrimSpace(resolveLibraryFileCodec(item)))
	language := normalizeLibraryFileLanguage(item.Media)
	prefix := buildLibraryFileDisplayPrefix(item)
	resolution := resolveLibraryFileResolution(item.Media)
	imageResolution := resolveLibraryFileImageResolution(item.Media)
	frameRate := resolveLibraryFileFrameRate(item.Media)
	bitrate := resolveLibraryFileBitrate(item.Media)

	switch strings.ToLower(strings.TrimSpace(item.Kind)) {
	case "video", "transcode":
		if label := joinLibraryFileDisplayParts(prefix, resolution, frameRate, codec); label != "" {
			return label
		}
	case "audio":
		if label := joinLibraryFileDisplayParts(prefix, codec, bitrate); label != "" {
			return label
		}
	case "subtitle":
		if label := joinLibraryFileDisplayParts(prefix, language); label != "" {
			return label
		}
	case "thumbnail":
		if label := joinLibraryFileDisplayParts(prefix, imageResolution); label != "" {
			return label
		}
	default:
		if prefix != "" {
			return prefix
		}
	}

	return firstNonEmpty(strings.TrimSpace(item.DisplayName), strings.TrimSpace(item.Name))
}

func buildLibraryFileDisplayPrefix(item dto.LibraryFileDTO) string {
	const maxPrefixTokens = 3
	const maxPrefixRunes = 20

	candidate := strings.TrimSpace(item.DisplayName)
	if candidate == "" {
		candidate = strings.TrimSpace(item.Name)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(item.Storage.LocalPath)
	}
	if candidate == "" {
		return ""
	}

	base := filepath.Base(candidate)
	if base == "" || base == "." {
		return ""
	}

	withoutExt := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if withoutExt == "" {
		withoutExt = strings.TrimSpace(base)
	}
	if withoutExt == "" {
		return ""
	}

	tokens := tokenizeLibraryFileDisplayValue(withoutExt)
	if len(tokens) == 0 {
		return withoutExt
	}

	noiseLookup := buildLibraryFileDisplayNoiseLookup(item)
	meaningful := make([]string, 0, len(tokens))
	for _, token := range tokens {
		normalized := strings.ToLower(strings.TrimSpace(token))
		if normalized == "" {
			continue
		}
		if _, exists := noiseLookup[normalized]; exists {
			continue
		}
		if _, exists := noiseLookup[condenseLibraryFileDisplayToken(normalized)]; exists {
			continue
		}
		meaningful = append(meaningful, token)
	}
	if len(meaningful) == 0 {
		meaningful = tokens
	}

	prefix, truncated := trimLibraryFileDisplayTokens(meaningful, maxPrefixTokens, maxPrefixRunes)
	if prefix == "" {
		return withoutExt
	}
	if truncated {
		return prefix + "..."
	}
	return prefix
}

func resolveLibraryFileFormat(item dto.LibraryFileDTO) string {
	if item.Media != nil && strings.TrimSpace(item.Media.Format) != "" {
		return item.Media.Format
	}
	pathCandidate := strings.TrimSpace(item.Storage.LocalPath)
	if pathCandidate == "" {
		pathCandidate = strings.TrimSpace(item.Name)
	}
	extension := strings.TrimSpace(filepath.Ext(pathCandidate))
	return strings.TrimPrefix(extension, ".")
}

func resolveLibraryFileCodec(item dto.LibraryFileDTO) string {
	media := item.Media
	if media == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(item.Kind)) {
	case "video", "transcode":
		if strings.TrimSpace(media.VideoCodec) != "" {
			return strings.TrimSpace(media.VideoCodec)
		}
	case "audio":
		if strings.TrimSpace(media.AudioCodec) != "" {
			return strings.TrimSpace(media.AudioCodec)
		}
	}
	if strings.TrimSpace(media.Codec) != "" {
		return strings.TrimSpace(media.Codec)
	}
	if strings.TrimSpace(media.VideoCodec) != "" {
		return strings.TrimSpace(media.VideoCodec)
	}
	if strings.TrimSpace(media.AudioCodec) != "" {
		return strings.TrimSpace(media.AudioCodec)
	}
	return ""
}

func resolveLibraryFileResolution(media *dto.LibraryMediaInfoDTO) string {
	if media == nil || media.Width == nil || media.Height == nil || *media.Width <= 0 || *media.Height <= 0 {
		return ""
	}
	return strconv.Itoa(*media.Height) + "p"
}

func resolveLibraryFileImageResolution(media *dto.LibraryMediaInfoDTO) string {
	if media == nil || media.Width == nil || media.Height == nil || *media.Width <= 0 || *media.Height <= 0 {
		return ""
	}
	return strconv.Itoa(*media.Width) + "x" + strconv.Itoa(*media.Height)
}

func resolveLibraryFileFrameRate(media *dto.LibraryMediaInfoDTO) string {
	if media == nil || media.FrameRate == nil || *media.FrameRate <= 0 {
		return ""
	}
	value := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", *media.FrameRate), "0"), ".")
	if value == "" {
		return ""
	}
	return value + "fps"
}

func resolveLibraryFileBitrate(media *dto.LibraryMediaInfoDTO) string {
	if media == nil || media.BitrateKbps == nil || *media.BitrateKbps <= 0 {
		return ""
	}
	return strconv.Itoa(*media.BitrateKbps) + "kbps"
}

func normalizeLibraryFileLanguage(media *dto.LibraryMediaInfoDTO) string {
	if media == nil {
		return ""
	}
	language := strings.TrimSpace(media.Language)
	if language == "" {
		return ""
	}
	if strings.EqualFold(language, "other") {
		return "OTHER"
	}
	return strings.ToUpper(strings.ReplaceAll(language, "_", "-"))
}

func joinLibraryFileDisplayParts(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		duplicate := false
		for _, existing := range parts {
			if strings.EqualFold(existing, trimmed) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " · ")
}

func tokenizeLibraryFileDisplayValue(value string) []string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(runes))
	var current strings.Builder
	current.Grow(len(runes))

	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}

	for index, currentRune := range runes {
		var previousRune rune
		var nextRune rune
		if index > 0 {
			previousRune = runes[index-1]
		}
		if index+1 < len(runes) {
			nextRune = runes[index+1]
		}

		switch {
		case unicode.IsLetter(currentRune), unicode.IsDigit(currentRune):
			current.WriteRune(currentRune)
		case currentRune == '.' && unicode.IsDigit(previousRune) && unicode.IsDigit(nextRune):
			current.WriteRune(currentRune)
		case (currentRune == 'x' || currentRune == 'X' || currentRune == '×') &&
			unicode.IsDigit(previousRune) &&
			unicode.IsDigit(nextRune):
			current.WriteRune('x')
		default:
			flush()
		}
	}
	flush()

	return tokens
}

func buildLibraryFileDisplayNoiseLookup(item dto.LibraryFileDTO) map[string]struct{} {
	lookup := make(map[string]struct{})
	addLibraryFileDisplayNoiseTokens(lookup, resolveLibraryFileFormat(item))
	addLibraryFileDisplayNoiseTokens(lookup, resolveLibraryFileCodec(item))
	addLibraryFileDisplayNoiseTokens(lookup, normalizeLibraryFileLanguage(item.Media))
	addLibraryFileDisplayNoiseTokens(lookup, resolveLibraryFileResolution(item.Media))
	addLibraryFileDisplayNoiseTokens(lookup, resolveLibraryFileImageResolution(item.Media))
	addLibraryFileDisplayNoiseTokens(lookup, resolveLibraryFileFrameRate(item.Media))
	addLibraryFileDisplayNoiseTokens(lookup, resolveLibraryFileBitrate(item.Media))
	return lookup
}

func addLibraryFileDisplayNoiseTokens(lookup map[string]struct{}, value string) {
	for _, token := range tokenizeLibraryFileDisplayValue(value) {
		normalized := strings.ToLower(strings.TrimSpace(token))
		if normalized == "" || isPureNumericLibraryFileToken(normalized) {
			continue
		}
		lookup[normalized] = struct{}{}
	}
	condensed := condenseLibraryFileDisplayToken(value)
	if condensed != "" && !isPureNumericLibraryFileToken(condensed) {
		lookup[condensed] = struct{}{}
	}
}

func condenseLibraryFileDisplayToken(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, current := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			builder.WriteRune(current)
		}
	}
	return builder.String()
}

func isPureNumericLibraryFileToken(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, current := range value {
		if !unicode.IsDigit(current) {
			return false
		}
	}
	return true
}

func trimLibraryFileDisplayTokens(tokens []string, maxTokens int, maxRunes int) (string, bool) {
	if len(tokens) == 0 || maxTokens <= 0 || maxRunes <= 0 {
		return "", false
	}

	selected := make([]string, 0, minInt(len(tokens), maxTokens))
	usedRunes := 0
	truncated := false

	for index, token := range tokens {
		if len(selected) >= maxTokens {
			truncated = true
			break
		}

		tokenRunes := utf8.RuneCountInString(token)
		separatorRunes := 0
		if len(selected) > 0 {
			separatorRunes = 1
		}

		if usedRunes+separatorRunes+tokenRunes > maxRunes {
			if len(selected) == 0 {
				selected = append(selected, truncateLibraryFileDisplayValue(token, maxRunes))
			}
			truncated = true
			break
		}

		selected = append(selected, token)
		usedRunes += separatorRunes + tokenRunes
		if index < len(tokens)-1 && len(selected) >= maxTokens {
			truncated = true
		}
	}

	return strings.Join(selected, " "), truncated
}

func truncateLibraryFileDisplayValue(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
