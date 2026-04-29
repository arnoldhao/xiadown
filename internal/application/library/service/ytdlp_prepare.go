package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	connectorsservice "xiadown/internal/application/connectors/service"
	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
	"xiadown/internal/domain/connectors"
)

func (service *LibraryService) PrepareYTDLPDownload(ctx context.Context, request dto.PrepareYTDLPDownloadRequest) (dto.PrepareYTDLPDownloadResponse, error) {
	resolvedURL, domain, err := validateDownloadURL(request.URL)
	if err != nil {
		return dto.PrepareYTDLPDownloadResponse{}, err
	}

	connectorID, connectorAvailable := service.resolveConnectorAvailability(ctx, domain)
	icon := ""
	if domain != "" && service.iconResolver != nil {
		if resolver, ok := service.iconResolver.(interface {
			ResolveDomainIconCached(context.Context, string) (string, bool)
		}); ok {
			if resolved, hit := resolver.ResolveDomainIconCached(ctx, domain); hit {
				icon = resolved
			}
		} else if resolved, err := service.iconResolver.ResolveDomainIcon(ctx, domain); err == nil {
			icon = resolved
		}
	}

	return dto.PrepareYTDLPDownloadResponse{
		URL:                resolvedURL,
		Domain:             domain,
		Icon:               icon,
		ConnectorID:        connectorID,
		ConnectorAvailable: connectorAvailable,
	}, nil
}

func (service *LibraryService) ResolveDomainIcon(ctx context.Context, request dto.ResolveDomainIconRequest) (dto.ResolveDomainIconResponse, error) {
	if service.iconResolver == nil {
		return dto.ResolveDomainIconResponse{}, nil
	}
	domain := strings.TrimSpace(request.Domain)
	if domain == "" {
		domain = extractRegistrableDomain(request.URL)
	}
	if domain == "" {
		return dto.ResolveDomainIconResponse{}, nil
	}
	icon, err := service.iconResolver.ResolveDomainIcon(ctx, domain)
	if err != nil {
		return dto.ResolveDomainIconResponse{Domain: domain}, nil
	}
	return dto.ResolveDomainIconResponse{Domain: domain, Icon: icon}, nil
}

func (service *LibraryService) ParseYTDLPDownload(ctx context.Context, request dto.ParseYTDLPDownloadRequest) (dto.ParseYTDLPDownloadResponse, error) {
	resolvedURL, domain, err := validateDownloadURL(request.URL)
	if err != nil {
		return dto.ParseYTDLPDownloadResponse{}, err
	}
	cookiesPath := ""
	if request.UseConnector && request.ConnectorID != "" && service.connectors != nil {
		if exported, err := service.connectors.ExportConnectorCookies(ctx, request.ConnectorID, connectorsservice.CookiesExportTXT); err == nil {
			cookiesPath = exported
			defer os.Remove(exported)
		}
	}

	info, err := appytdlp.FetchInfo(ctx, appytdlp.InfoOptions{
		ExecPath:    "",
		Tools:       service.tools,
		URL:         resolvedURL,
		CookiesPath: cookiesPath,
		ProxyURL:    service.resolveYTDLPProxy(resolvedURL),
	})
	if err != nil {
		return dto.ParseYTDLPDownloadResponse{}, err
	}

	title := strings.TrimSpace(getString(info, "title"))
	extractor := resolveYTDLPExtractor(info)
	author := resolveYTDLPAuthor(info)
	thumbnailURL := resolveYTDLPThumbnail(info)
	if domain == "" {
		domain = extractRegistrableDomain(getString(info, "webpage_url"))
	}
	formats := buildYTDLPFormatOptions(info)
	subtitles := buildYTDLPSubtitleOptions(info)

	return dto.ParseYTDLPDownloadResponse{
		Title:        title,
		Domain:       domain,
		Extractor:    extractor,
		Author:       author,
		ThumbnailURL: thumbnailURL,
		Formats:      formats,
		Subtitles:    subtitles,
	}, nil
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

func (service *LibraryService) ListTranscodePresetsForDownload(ctx context.Context, request dto.ListTranscodePresetsForDownloadRequest) ([]dto.TranscodePreset, error) {
	presets, err := service.ListTranscodePresets(ctx)
	if err != nil {
		return nil, err
	}
	mediaType := strings.ToLower(strings.TrimSpace(request.MediaType))
	if mediaType == "" {
		return presets, nil
	}
	filtered := make([]dto.TranscodePreset, 0, len(presets))
	for _, preset := range presets {
		outputType := strings.ToLower(strings.TrimSpace(preset.OutputType))
		switch mediaType {
		case "audio":
			if outputType == "audio" {
				filtered = append(filtered, preset)
			}
		case "video":
			if outputType != "audio" {
				filtered = append(filtered, preset)
			}
		default:
			filtered = append(filtered, preset)
		}
	}
	return filtered, nil
}

func validateDownloadURL(rawURL string) (string, string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", "", fmt.Errorf("invalid url scheme")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", "", fmt.Errorf("invalid url host")
	}
	return trimmed, extractRegistrableDomain(trimmed), nil
}

func (service *LibraryService) resolveConnectorAvailability(ctx context.Context, domain string) (string, bool) {
	if service.connectors == nil {
		return "", false
	}
	connectorType := connectorTypeForDomain(domain)
	if connectorType == "" {
		return "", false
	}
	items, err := service.connectors.ListConnectors(ctx)
	if err != nil {
		return "", false
	}
	for _, item := range items {
		if strings.EqualFold(item.Type, string(connectorType)) {
			available := strings.EqualFold(item.Status, string(connectors.StatusConnected)) && item.CookiesCount > 0
			return item.ID, available
		}
	}
	return "", false
}

func connectorTypeForDomain(domain string) connectors.ConnectorType {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	switch normalized {
	case "youtube.com", "youtu.be", "youtube-nocookie.com":
		return connectors.ConnectorYouTube
	case "bilibili.com", "b23.tv":
		return connectors.ConnectorBilibili
	case "tiktok.com", "tiktokv.com", "vm.tiktok.com":
		return connectors.ConnectorTikTok
	case "douyin.com", "iesdouyin.com":
		return connectors.ConnectorDouyin
	case "instagram.com":
		return connectors.ConnectorInstagram
	case "x.com", "twitter.com":
		return connectors.ConnectorX
	case "facebook.com", "fb.watch":
		return connectors.ConnectorFacebook
	case "vimeo.com", "player.vimeo.com":
		return connectors.ConnectorVimeo
	case "twitch.tv", "clips.twitch.tv":
		return connectors.ConnectorTwitch
	case "nicovideo.jp", "nico.ms", "nicovideo.cdn.nimg.jp":
		return connectors.ConnectorNiconico
	default:
		return ""
	}
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
		filesize, _ := getInt64(formatMap, "filesize")
		if filesize == 0 {
			filesize, _ = getInt64(formatMap, "filesize_approx")
		}
		label := buildYTDLPFormatLabel(formatMap, height, ext, vcodec, acodec, filesize, formatID)
		result = append(result, dto.YTDLPFormatOption{
			ID:       formatID,
			Label:    label,
			HasVideo: hasVideo,
			HasAudio: hasAudio,
			Ext:      ext,
			Height:   height,
			VCodec:   vcodec,
			ACodec:   acodec,
			Filesize: filesize,
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
	case strings.Contains(codec, "avc"):
		return "H.264"
	case strings.Contains(codec, "hev"), strings.Contains(codec, "hvc"):
		return "HEVC"
	case strings.Contains(codec, "av01"):
		return "AV1"
	case strings.Contains(codec, "vp9"):
		return "VP9"
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
