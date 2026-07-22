package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
)

const resourceMaxSubtitleBytes int64 = 16 << 20

var (
	resourceSubtitlePathLangPattern = regexp.MustCompile(`(?i)(?:^|[._-])([a-z]{2,3}(?:[-_][a-z0-9]{2,8}){0,2})(?:[._-](?:cc|sub|subs|caption|captions|auto|asr))?$`)
	resourceSubtitleLangPattern     = regexp.MustCompile(`(?i)^[a-z]{2,3}(?:-[a-z0-9]{2,8}){0,2}$`)
)

type resourceSubtitle struct {
	captureID      resourceRequestCaptureID
	URL            string
	Data           string
	PageURL        string
	Language       string
	Name           string
	IsAuto         bool
	Ext            string
	ContentType    string
	SourceURL      string
	RequestHeaders map[string]string
	SeenAt         time.Time
}

func (subtitle resourceSubtitle) Valid() bool {
	return (strings.TrimSpace(subtitle.URL) != "" || strings.TrimSpace(subtitle.Data) != "") &&
		strings.TrimSpace(subtitle.Ext) != ""
}

func resourceSubtitleFromResponse(rawURL string, pageURL string, mimeType string, contentType string, resourceType string, requestHeaders map[string]string, seenAt time.Time) (resourceSubtitle, bool) {
	if declaredKind := resourceSniffRawDeclaredKind(mimeType, contentType, resourceType); declaredKind != "" && declaredKind != "subtitle" {
		return resourceSubtitle{}, false
	}
	ext := resourceSubtitleExt(rawURL, mimeType, contentType, "")
	if ext == "" {
		return resourceSubtitle{}, false
	}
	language := resourceSubtitleLanguageFromURL(rawURL)
	if language == "" {
		language = "und"
	}
	return resourceSubtitle{
		URL:            strings.TrimSpace(rawURL),
		PageURL:        strings.TrimSpace(pageURL),
		Language:       language,
		Name:           resourceSubtitleDisplayName(language, ""),
		IsAuto:         resourceSubtitleLooksAutomatic(rawURL, ""),
		Ext:            ext,
		ContentType:    strings.TrimSpace(firstNonEmpty(contentType, mimeType)),
		RequestHeaders: normalizeResourceDownloadHeaders(requestHeaders, firstNonEmpty(pageURL, rawURL)),
		SeenAt:         firstNonZeroTime(seenAt, time.Now()),
	}, true
}
func resourceSubtitleOptions(subtitles []resourceSubtitle) []dto.YTDLPSubtitleOption {
	if len(subtitles) == 0 {
		return nil
	}
	result := make([]dto.YTDLPSubtitleOption, 0, len(subtitles))
	seen := map[string]struct{}{}
	for _, subtitle := range subtitles {
		if !subtitle.Valid() {
			continue
		}
		id := resourceSubtitleID(subtitle)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, dto.YTDLPSubtitleOption{
			ID:       id,
			Language: subtitle.Language,
			Name:     subtitle.Name,
			IsAuto:   subtitle.IsAuto,
			Ext:      subtitle.Ext,
		})
	}
	return result
}

func resourceSubtitleID(subtitle resourceSubtitle) string {
	language := strings.TrimSpace(subtitle.Language)
	if language == "" {
		language = "und"
	}
	id := language
	if ext := strings.TrimSpace(subtitle.Ext); ext != "" {
		id += ":" + ext
	}
	if subtitle.IsAuto {
		id += ":auto"
	}
	return id
}

func selectResourceSubtitlesForRequest(request dto.CreateYTDLPJobRequest, subtitles []resourceSubtitle) []resourceSubtitle {
	if !wantsYTDLPSubtitles(request) || len(subtitles) == 0 {
		return nil
	}
	subtitles = dedupeResourceSubtitles(subtitles)
	if request.SubtitleAll {
		result := make([]resourceSubtitle, 0, len(subtitles))
		for _, subtitle := range subtitles {
			if !request.SubtitleAuto && subtitle.IsAuto {
				continue
			}
			result = append(result, subtitle)
		}
		return result
	}
	result := make([]resourceSubtitle, 0, len(request.SubtitleLangs))
	for _, lang := range request.SubtitleLangs {
		lang = resourceNormalizeSubtitleLanguage(lang)
		if lang == "" {
			continue
		}
		candidates := make([]resourceSubtitle, 0, len(subtitles))
		for _, subtitle := range subtitles {
			if !strings.EqualFold(resourceNormalizeSubtitleLanguage(subtitle.Language), lang) {
				continue
			}
			if subtitle.IsAuto != request.SubtitleAuto {
				continue
			}
			candidates = append(candidates, subtitle)
		}
		if chosen, ok := chooseResourceSubtitleFormat(candidates, request.SubtitleFormat); ok {
			result = append(result, chosen)
		}
	}
	return dedupeResourceSubtitles(result)
}

func chooseResourceSubtitleFormat(candidates []resourceSubtitle, format string) (resourceSubtitle, bool) {
	if len(candidates) == 0 {
		return resourceSubtitle{}, false
	}
	preferences := strings.Split(strings.TrimSpace(format), "/")
	for _, preference := range preferences {
		preference = normalizeSubtitleFormat(preference)
		if preference == "" || preference == "best" {
			continue
		}
		for index := len(candidates) - 1; index >= 0; index-- {
			if strings.EqualFold(normalizeSubtitleFormat(candidates[index].Ext), preference) {
				return candidates[index], true
			}
		}
	}
	return candidates[len(candidates)-1], true
}

func (service *LibraryService) downloadResourceSubtitles(ctx context.Context, request dto.CreateYTDLPJobRequest, media resourceMedia, outputPath string) ([]string, []string) {
	selected := selectResourceSubtitlesForRequest(request, media.Subtitles)
	if len(selected) == 0 {
		if wantsYTDLPSubtitles(request) {
			return nil, []string{"no matching subtitles detected by resource sniff"}
		}
		return nil, nil
	}
	paths := make([]string, 0, len(selected))
	warnings := make([]string, 0)
	for _, subtitle := range selected {
		targetPath, err := prepareResourceSubtitleOutputPath(outputPath, subtitle)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("subtitle output failed: %v", err))
			continue
		}
		writtenPath, err := service.writeResourceSubtitle(ctx, subtitle, targetPath, firstNonEmpty(service.resolveYTDLPProxy(subtitle.URL), service.resolveYTDLPProxy(media.URL), service.resolveYTDLPProxy(request.URL)))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("subtitle download failed: %v", err))
			continue
		}
		if strings.TrimSpace(writtenPath) != "" {
			paths = append(paths, writtenPath)
		}
	}
	return dedupePaths(paths), warnings
}

func prepareResourceSubtitleOutputPath(outputPath string, subtitle resourceSubtitle) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", fmt.Errorf("output path is required")
	}
	dir := filepath.Join(filepath.Dir(outputPath), "subtitles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	baseName := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	lang := sanitizeFileName(firstNonEmpty(subtitle.Language, "und"))
	if lang == "" {
		lang = "und"
	}
	parts := []string{baseName, lang}
	if subtitle.IsAuto {
		parts = append(parts, "auto")
	}
	ext := normalizeSubtitleFormat(subtitle.Ext)
	if ext == "" {
		ext = "vtt"
	}
	return filepath.Join(dir, strings.Join(parts, ".")+"."+ext), nil
}

func (service *LibraryService) writeResourceSubtitle(ctx context.Context, subtitle resourceSubtitle, targetPath string, proxyURL string) (string, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return "", fmt.Errorf("subtitle output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	resolvedPath, err := reserveUniqueResourceOutputPath(targetPath)
	if err != nil {
		return "", err
	}
	if subtitle.Data != "" {
		if err := os.WriteFile(resolvedPath, []byte(subtitle.Data), 0o644); err != nil {
			_ = os.Remove(resolvedPath)
			return "", err
		}
		return resolvedPath, nil
	}
	rawURL := strings.TrimSpace(subtitle.URL)
	if rawURL == "" {
		return "", fmt.Errorf("subtitle url is required")
	}
	client, err := newResourceHTTPClient(proxyURL)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	applyResourceRequestHeaders(req, subtitle.RequestHeaders)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := readResourceErrorBody(resp.Body)
		if detail != "" {
			return "", fmt.Errorf("subtitle download failed: %s: %s", resp.Status, detail)
		}
		return "", fmt.Errorf("subtitle download failed: %s", resp.Status)
	}
	file, err := os.OpenFile(resolvedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, resourceMaxSubtitleBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(resolvedPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(resolvedPath)
		return "", closeErr
	}
	if written > resourceMaxSubtitleBytes {
		_ = os.Remove(resolvedPath)
		return "", fmt.Errorf("subtitle exceeds max bytes: %d > %d", written, resourceMaxSubtitleBytes)
	}
	return resolvedPath, nil
}

func resourceSubtitleExt(rawURL string, mimeType string, contentType string, formatHint string) string {
	for _, value := range []string{formatHint, mimeType, contentType} {
		lower := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
		switch lower {
		case "vtt", "webvtt", "text/vtt":
			return "vtt"
		case "srt", "subrip", "application/x-subrip", "application/srt", "text/srt":
			return "srt"
		case "ass", "text/x-ass":
			return "ass"
		case "ssa", "text/x-ssa":
			return "ssa"
		case "ttml", "dfxp", "itt", "application/ttml+xml", "application/dfxp+xml":
			return "itt"
		case "lrc", "application/lrc", "text/lrc":
			return "lrc"
		case "sbv", "text/sbv", "application/sbv":
			return "sbv"
		}
	}
	if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
		ext := normalizeSubtitleFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), "."))
		switch ext {
		case "vtt", "srt", "ass", "ssa", "itt", "lrc", "sbv", "fcpxml":
			return ext
		case "xml", "ttml", "dfxp":
			return "itt"
		}
		query := parsed.Query()
		for _, key := range []string{"fmt", "format", "ext", "type"} {
			if ext := normalizeSubtitleFormat(query.Get(key)); ext != "" {
				switch ext {
				case "vtt", "srt", "ass", "ssa", "itt", "lrc", "sbv", "fcpxml":
					return ext
				}
			}
		}
	}
	return ""
}

func resourceSubtitleLanguageFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil {
		query := parsed.Query()
		for _, key := range []string{"lang", "language", "srclang", "subtitle_lang", "sub_lang", "locale", "code"} {
			if lang := resourceNormalizeSubtitleLanguage(query.Get(key)); lang != "" {
				return lang
			}
		}
		path := strings.TrimSuffix(parsed.Path, filepath.Ext(parsed.Path))
		base := filepath.Base(path)
		if match := resourceSubtitlePathLangPattern.FindStringSubmatch(base); len(match) == 2 {
			if lang := resourceNormalizeSubtitleLanguage(match[1]); lang != "" {
				return lang
			}
		}
		segments := strings.Split(strings.Trim(path, "/"), "/")
		for index := len(segments) - 1; index >= 0; index-- {
			if lang := resourceNormalizeSubtitleLanguage(segments[index]); lang != "" {
				return lang
			}
		}
	}
	return ""
}

func resourceNormalizeSubtitleLanguage(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(value), "._- ")
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	lower := strings.ToLower(trimmed)
	switch lower {
	case "subtitle", "subtitles", "caption", "captions", "sub", "subs", "cc", "auto", "asr", "vtt", "srt":
		return ""
	}
	if len(trimmed) > 24 || strings.ContainsAny(trimmed, "/\\?&#=") {
		return ""
	}
	if !resourceSubtitleLangPattern.MatchString(trimmed) {
		return ""
	}
	hasLetter := false
	for _, item := range trimmed {
		if (item >= 'a' && item <= 'z') || (item >= 'A' && item <= 'Z') {
			hasLetter = true
			continue
		}
		if item >= '0' && item <= '9' {
			continue
		}
		if item == '-' {
			continue
		}
		return ""
	}
	if !hasLetter {
		return ""
	}
	return resourceCanonicalSubtitleLanguageTag(trimmed)
}

func resourceCanonicalSubtitleLanguageTag(value string) string {
	parts := strings.Split(value, "-")
	for index, part := range parts {
		if part == "" {
			return ""
		}
		lower := strings.ToLower(part)
		switch {
		case index == 0:
			parts[index] = lower
		case len(part) == 4 && resourceSubtitleTagPartIsAlpha(part):
			parts[index] = strings.ToUpper(lower[:1]) + lower[1:]
		case len(part) == 2 && resourceSubtitleTagPartIsAlpha(part):
			parts[index] = strings.ToUpper(lower)
		default:
			parts[index] = lower
		}
	}
	return strings.Join(parts, "-")
}

func resourceSubtitleTagPartIsAlpha(value string) bool {
	if value == "" {
		return false
	}
	for _, item := range value {
		if (item >= 'a' && item <= 'z') || (item >= 'A' && item <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func resourceSubtitleDisplayName(language string, fallback string) string {
	if trimmed := strings.TrimSpace(fallback); trimmed != "" {
		return trimmed
	}
	if lang := resourceNormalizeSubtitleLanguage(language); lang != "" {
		return lang
	}
	return "und"
}

func resourceSubtitleLooksAutomatic(values ...string) bool {
	for _, value := range values {
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "" {
			continue
		}
		if strings.Contains(lower, "auto") || strings.Contains(lower, "automatic") || strings.Contains(lower, "asr") || strings.Contains(lower, "generated") {
			return true
		}
	}
	return false
}

func resourceSubtitleAlreadySeen(existing []resourceSubtitle, subtitle resourceSubtitle) bool {
	key := resourceSubtitleKey(subtitle)
	if key == "" {
		return false
	}
	for _, item := range existing {
		if resourceSubtitleKey(item) == key {
			return true
		}
	}
	return false
}

func dedupeResourceSubtitles(items []resourceSubtitle) []resourceSubtitle {
	if len(items) == 0 {
		return nil
	}
	result := make([]resourceSubtitle, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if !item.Valid() {
			continue
		}
		if strings.TrimSpace(item.Language) == "" {
			item.Language = "und"
		}
		if strings.TrimSpace(item.Name) == "" {
			item.Name = resourceSubtitleDisplayName(item.Language, "")
		}
		item.Ext = normalizeSubtitleFormat(item.Ext)
		key := resourceSubtitleKey(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func resourceSubtitleKey(subtitle resourceSubtitle) string {
	source := firstNonEmpty(resourceComparableURL(subtitle.URL, false), strings.TrimSpace(subtitle.Data))
	if source == "" {
		return ""
	}
	return strings.Join([]string{
		source,
		resourceNormalizeSubtitleLanguage(subtitle.Language),
		normalizeSubtitleFormat(subtitle.Ext),
		fmt.Sprint(subtitle.IsAuto),
	}, "\x00")
}
