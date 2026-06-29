package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"xiadown/internal/application/apperrors"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
)

const ytdlpParseInfoTimeout = 90 * time.Second

func (service *LibraryService) ParseYTDLPDownload(ctx context.Context, request dto.ParseYTDLPDownloadRequest) (dto.ParseYTDLPDownloadResponse, error) {
	resolvedURL, domain, err := validateDownloadURL(request.URL)
	if err != nil {
		return dto.ParseYTDLPDownloadResponse{}, err
	}
	cookiesPath := ""
	if request.UseAppSession && request.AppSessionID != "" && service.appSessions != nil {
		if exported, err := service.appSessions.ExportAppSessionCookies(ctx, request.AppSessionID, appsessionsservice.CookiesExportTXT); err == nil {
			cookiesPath = exported
			defer os.Remove(exported)
		}
	}

	info, err := appytdlp.FetchInfo(ctx, appytdlp.InfoOptions{
		ExecPath:     "",
		Tools:        service.tools,
		URL:          resolvedURL,
		CookiesPath:  cookiesPath,
		ProxyURL:     service.resolveYTDLPProxy(resolvedURL),
		FlatPlaylist: true,
		Timeout:      ytdlpParseInfoTimeout,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return dto.ParseYTDLPDownloadResponse{}, apperrors.Wrap(apperrors.CodeParsing, "parse download link timed out", err)
		}
		return dto.ParseYTDLPDownloadResponse{}, err
	}

	title := strings.TrimSpace(getString(info, "title"))
	extractor := resolveYTDLPExtractor(info)
	author := resolveYTDLPAuthor(info)
	thumbnailURL := resolveYTDLPThumbnail(info)
	pageURL := strings.TrimSpace(getString(info, "webpage_url", "original_url"))
	if domain == "" {
		domain = extractRegistrableDomain(pageURL)
	}
	playlistItems := service.buildYTDLPPlaylistItems(ctx, info)
	if len(playlistItems) > 0 {
		return dto.ParseYTDLPDownloadResponse{
			Title:         title,
			Domain:        domain,
			Extractor:     extractor,
			Author:        author,
			ThumbnailURL:  thumbnailURL,
			PageURL:       pageURL,
			PlaylistItems: playlistItems,
			Formats:       []dto.YTDLPFormatOption{},
			Subtitles:     []dto.YTDLPSubtitleOption{},
		}, nil
	}
	formats := buildYTDLPFormatOptions(info)
	subtitles := buildYTDLPSubtitleOptions(info)
	if len(formats) == 0 {
		return dto.ParseYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeResourceNoMediaDetected, "no downloadable formats found")
	}

	return dto.ParseYTDLPDownloadResponse{
		Title:        title,
		Domain:       domain,
		Extractor:    extractor,
		Author:       author,
		ThumbnailURL: thumbnailURL,
		PageURL:      pageURL,
		Formats:      formats,
		Subtitles:    subtitles,
	}, nil
}

func (service *LibraryService) buildYTDLPPlaylistItems(ctx context.Context, info map[string]any) []dto.PreparedYTDLPDownloadURL {
	rawEntries, ok := info["entries"].([]any)
	if !ok || len(rawEntries) == 0 {
		return nil
	}
	result := make([]dto.PreparedYTDLPDownloadURL, 0, len(rawEntries))
	seen := make(map[string]struct{}, len(rawEntries))
	for _, rawEntry := range rawEntries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		candidate := resolveYTDLPPlaylistEntryURL(info, entry)
		if candidate == "" {
			continue
		}
		resolvedURL, domain, ok := normalizeDownloadURLWithDomain(candidate)
		if !ok {
			resolvedURL, domain, ok = normalizeKnownVideoSuffix(candidate)
		}
		if !ok || resolvedURL == "" {
			continue
		}
		if _, exists := seen[resolvedURL]; exists {
			continue
		}
		seen[resolvedURL] = struct{}{}
		result = append(result, service.prepareYTDLPDownloadURL(ctx, normalizedDownloadURL{URL: resolvedURL, Domain: domain}))
	}
	return result
}

func resolveYTDLPPlaylistEntryURL(playlistInfo map[string]any, entry map[string]any) string {
	for _, key := range []string{"webpage_url", "original_url", "url"} {
		candidate := strings.TrimSpace(getString(entry, key))
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") {
			return candidate
		}
		if strings.HasPrefix(candidate, "//") {
			return "https:" + candidate
		}
	}

	entryID := strings.TrimSpace(getString(entry, "id"))
	if entryID == "" {
		entryID = strings.TrimSpace(getString(entry, "url"))
	}
	if entryID == "" {
		return ""
	}
	extractor := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		getString(entry, "ie_key", "extractor_key", "extractor"),
		getString(playlistInfo, "ie_key", "extractor_key", "extractor"),
		getString(playlistInfo, "extractor"),
	)))
	if strings.Contains(extractor, "youtube") {
		return "https://www.youtube.com/watch?v=" + entryID
	}
	if strings.Contains(extractor, "bilibili") && strings.HasPrefix(strings.ToLower(entryID), "bv") {
		return "https://www.bilibili.com/video/" + entryID
	}
	return ""
}

func resolveYTDLPExtractor(info map[string]any) string {
	return getString(info, "extractor", "extractor_key")
}

func resolveYTDLPAuthor(info map[string]any) string {
	return getString(info, "uploader", "channel", "creator", "artist")
}

func resolveYTDLPThumbnail(info map[string]any) string {
	if thumbnail := getString(info, "thumbnail"); thumbnail != "" {
		return thumbnail
	}
	items, ok := info["thumbnails"].([]any)
	if !ok {
		return ""
	}
	bestURL := ""
	bestArea := int64(0)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		url := getString(entry, "url")
		if url == "" {
			continue
		}
		width, _ := getInt64(entry, "width")
		height, _ := getInt64(entry, "height")
		area := width * height
		if area > bestArea {
			bestArea = area
			bestURL = url
		} else if bestURL == "" {
			bestURL = url
		}
	}
	return bestURL
}

func buildYTDLPFormatOptions(info map[string]any) []dto.YTDLPFormatOption {
	rawFormats, ok := info["formats"].([]any)
	if !ok {
		return nil
	}
	result := make([]dto.YTDLPFormatOption, 0, len(rawFormats))
	for _, item := range rawFormats {
		formatMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		formatID := strings.TrimSpace(getString(formatMap, "format_id"))
		if formatID == "" {
			continue
		}
		vcodec := strings.TrimSpace(getString(formatMap, "vcodec"))
		acodec := strings.TrimSpace(getString(formatMap, "acodec"))
		hasVideo := vcodec != "" && vcodec != "none"
		hasAudio := acodec != "" && acodec != "none"
		if !hasVideo && !hasAudio {
			continue
		}
		height := getInt(formatMap, "height")
		ext := strings.TrimSpace(getString(formatMap, "ext"))
		formatNote := strings.TrimSpace(getString(formatMap, "format_note"))
		language := strings.TrimSpace(getString(formatMap, "language"))
		tbr, _ := getFloat64(formatMap, "tbr")
		abr, _ := getFloat64(formatMap, "abr")
		vbr, _ := getFloat64(formatMap, "vbr")
		audioChannels := getInt(formatMap, "audio_channels")
		filesize, _ := getInt64(formatMap, "filesize")
		if filesize == 0 {
			filesize, _ = getInt64(formatMap, "filesize_approx")
		}
		label := buildYTDLPFormatLabel(formatMap, height, ext, vcodec, acodec, filesize, formatID)
		result = append(result, dto.YTDLPFormatOption{
			ID:            formatID,
			Label:         label,
			HasVideo:      hasVideo,
			HasAudio:      hasAudio,
			Ext:           ext,
			Height:        height,
			VCodec:        vcodec,
			ACodec:        acodec,
			FormatNote:    formatNote,
			Language:      language,
			TBR:           tbr,
			ABR:           abr,
			VBR:           vbr,
			AudioChannels: audioChannels,
			Filesize:      filesize,
		})
	}
	return result
}

func buildYTDLPFormatLabel(formatMap map[string]any, height int, ext string, vcodec string, acodec string, sizeBytes int64, fallback string) string {
	parts := make([]string, 0, 3)
	if height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", height))
	}
	if note := strings.TrimSpace(getString(formatMap, "format_note")); note != "" {
		parts = append(parts, note)
	}
	if ext != "" {
		parts = append(parts, ext)
	}
	if codecLabel := formatCodecLabel(vcodec, acodec); codecLabel != "" {
		parts = append(parts, codecLabel)
	}
	if sizeLabel := formatBytesLabel(sizeBytes); sizeLabel != "" {
		parts = append(parts, sizeLabel)
	}
	if len(parts) == 0 {
		if value := strings.TrimSpace(getString(formatMap, "format")); value != "" {
			return value
		}
		return fallback
	}
	return strings.Join(parts, " · ")
}

func formatCodecLabel(vcodec string, acodec string) string {
	vc := strings.ToLower(strings.TrimSpace(vcodec))
	ac := strings.ToLower(strings.TrimSpace(acodec))
	if vc != "" && vc != "none" {
		return normalizeCodecLabel(vc)
	}
	if ac != "" && ac != "none" {
		return normalizeCodecLabel(ac)
	}
	return ""
}

func normalizeCodecLabel(codec string) string {
	switch {
	case strings.Contains(codec, "avc"), strings.Contains(codec, "h264"):
		return "H.264"
	case strings.Contains(codec, "hev"), strings.Contains(codec, "hvc"), strings.Contains(codec, "h265"):
		return "H.265"
	case strings.Contains(codec, "av01"):
		return "AV1"
	case strings.Contains(codec, "vp9"):
		return "VP9"
	case strings.Contains(codec, "bytevc2"):
		return "ByteVC2"
	case strings.Contains(codec, "bytevc1"):
		return "ByteVC1"
	case strings.Contains(codec, "mp4a"):
		return "AAC"
	case strings.Contains(codec, "opus"):
		return "Opus"
	case strings.Contains(codec, "vorbis"):
		return "Vorbis"
	default:
		return codec
	}
}

func formatBytesLabel(sizeBytes int64) string {
	if sizeBytes <= 0 {
		return ""
	}
	value := float64(sizeBytes)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	precision := 0
	if value < 10 && unitIndex > 0 {
		precision = 1
	}
	return fmt.Sprintf("%.*f%s", precision, value, units[unitIndex])
}

func buildYTDLPSubtitleOptions(info map[string]any) []dto.YTDLPSubtitleOption {
	result := make([]dto.YTDLPSubtitleOption, 0)
	seen := map[string]struct{}{}
	appendOptions := func(raw map[string]any, isAuto bool) {
		for lang, list := range raw {
			language := strings.TrimSpace(lang)
			entries, ok := list.([]any)
			if !ok {
				continue
			}
			for _, entry := range entries {
				entryMap, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				ext := strings.TrimSpace(getString(entryMap, "ext"))
				name := strings.TrimSpace(getString(entryMap, "name"))
				if language == "" && name == "" {
					continue
				}
				id := language
				if ext != "" {
					id = id + ":" + ext
				}
				if isAuto {
					id = id + ":auto"
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				result = append(result, dto.YTDLPSubtitleOption{
					ID:       id,
					Language: language,
					Name:     name,
					IsAuto:   isAuto,
					Ext:      ext,
				})
			}
		}
	}
	if raw, ok := info["subtitles"].(map[string]any); ok {
		appendOptions(raw, false)
	}
	if raw, ok := info["automatic_captions"].(map[string]any); ok {
		appendOptions(raw, true)
	}
	return result
}

func getInt(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	if raw, ok := values[key]; ok {
		switch value := raw.(type) {
		case float64:
			return int(value)
		case int:
			return value
		case int64:
			return int(value)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				return parsed
			}
		}
	}
	return 0
}
