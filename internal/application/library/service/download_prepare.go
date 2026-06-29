package service

import (
	"context"
	"net"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/publicsuffix"
	"mvdan.cc/xurls/v2"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/library/dto"
)

func (service *LibraryService) PrepareYTDLPDownload(ctx context.Context, request dto.PrepareYTDLPDownloadRequest) (dto.PrepareYTDLPDownloadResponse, error) {
	resolvedItems, err := parseDownloadURLs(request.URL)
	if err != nil {
		return dto.PrepareYTDLPDownloadResponse{}, err
	}
	if len(resolvedItems) == 0 {
		return dto.PrepareYTDLPDownloadResponse{}, apperrors.New(apperrors.CodeDownloadURLInvalid, "invalid url or unsupported video path")
	}

	preparedURLs := make([]dto.PreparedYTDLPDownloadURL, 0, len(resolvedItems))
	for _, item := range resolvedItems {
		preparedURLs = append(preparedURLs, service.prepareYTDLPDownloadURL(ctx, item))
	}
	first := preparedURLs[0]
	mode := "single"
	if len(preparedURLs) > 1 {
		mode = "batch"
	}
	return dto.PrepareYTDLPDownloadResponse{
		Mode:                      mode,
		URL:                       first.URL,
		Domain:                    first.Domain,
		Icon:                      first.Icon,
		AppSessionID:              first.AppSessionID,
		AppSessionAvailable:       first.AppSessionAvailable,
		AppSessionCredentialMode:  first.AppSessionCredentialMode,
		AppSessionCredentialState: first.AppSessionCredentialState,
		Reachable:                 first.Reachable,
		URLs:                      preparedURLs,
	}, nil
}

type normalizedDownloadURL struct {
	URL    string
	Domain string
}

func (service *LibraryService) prepareYTDLPDownloadURL(ctx context.Context, item normalizedDownloadURL) dto.PreparedYTDLPDownloadURL {
	resolvedURL := strings.TrimSpace(item.URL)
	domain := strings.TrimSpace(item.Domain)
	appSessionAvailability := service.resolveAppSessionAvailability(ctx, domain)
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

	return dto.PreparedYTDLPDownloadURL{
		URL:                       resolvedURL,
		Domain:                    domain,
		Icon:                      icon,
		AppSessionID:              appSessionAvailability.ID,
		AppSessionAvailable:       appSessionAvailability.Available,
		AppSessionCredentialMode:  appSessionAvailability.CredentialMode,
		AppSessionCredentialState: appSessionAvailability.CredentialState,
	}
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
	resolvedItems, err := parseDownloadURLs(rawURL)
	if err != nil {
		return "", "", err
	}
	if len(resolvedItems) != 1 {
		return "", "", apperrors.New(apperrors.CodeDownloadURLMultiple, "multiple download urls require batch download")
	}
	return resolvedItems[0].URL, resolvedItems[0].Domain, nil
}

func parseDownloadURLs(rawURL string) ([]normalizedDownloadURL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, apperrors.New(apperrors.CodeDownloadURLRequired, "url is required")
	}

	result := make([]normalizedDownloadURL, 0, 1)
	seen := make(map[string]struct{})
	addCandidate := func(candidate string) {
		candidate = sanitizeExtractedURLCandidate(candidate)
		if candidate == "" {
			return
		}
		if resolvedURL, domain, ok := normalizeDownloadURLWithDomain(candidate); ok {
			if _, exists := seen[resolvedURL]; !exists {
				seen[resolvedURL] = struct{}{}
				result = append(result, normalizedDownloadURL{URL: resolvedURL, Domain: domain})
			}
			return
		}
		if resolvedURL, domain, ok := normalizeKnownVideoSuffix(candidate); ok {
			if _, exists := seen[resolvedURL]; !exists {
				seen[resolvedURL] = struct{}{}
				result = append(result, normalizedDownloadURL{URL: resolvedURL, Domain: domain})
			}
		}
	}

	addCandidate(trimmed)

	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == trimmed || containsWhitespace(line) {
			continue
		}
		addCandidate(line)
	}

	for _, candidate := range explicitURLPattern.FindAllString(trimmed, -1) {
		addCandidate(candidate)
	}
	for _, candidate := range relaxedURLPattern.FindAllString(trimmed, -1) {
		candidate = sanitizeExtractedURLCandidate(candidate)
		if candidate == "" || strings.Contains(candidate, "://") || isRelaxedEmailCandidate(candidate) || isLowConfidenceRelaxedCandidate(trimmed, candidate) {
			continue
		}
		addCandidate(candidate)
	}

	if len(result) == 0 {
		return nil, apperrors.New(apperrors.CodeDownloadURLInvalid, "invalid url or unsupported video path")
	}
	return result, nil
}

func containsWhitespace(value string) bool {
	return strings.ContainsAny(value, " \t\r\n")
}

func sanitizeExtractedURLCandidate(candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if fields := strings.Fields(trimmed); len(fields) > 0 {
		trimmed = fields[0]
	}
	trimmed = strings.Trim(trimmed, "\"'“”‘’<>")
	trimmed = strings.TrimLeft(trimmed, "([{【《「『")
	trimmed = strings.TrimRight(trimmed, ".,;!?)\\]}。，；！？】》」』、\"'“”‘’>")
	return strings.TrimSpace(trimmed)
}

func isRelaxedEmailCandidate(candidate string) bool {
	if strings.Contains(candidate, "://") || !strings.Contains(candidate, "@") {
		return false
	}
	parsed, err := url.Parse("https://" + candidate)
	if err != nil {
		return true
	}
	return parsed.User != nil && strings.TrimSpace(parsed.Path) == ""
}

func isLowConfidenceRelaxedCandidate(rawInput string, candidate string) bool {
	if strings.EqualFold(strings.TrimSpace(rawInput), strings.TrimSpace(candidate)) {
		return false
	}
	parsed, err := url.Parse("https://" + candidate)
	if err != nil {
		return true
	}
	path := strings.TrimSpace(parsed.EscapedPath())
	return (path == "" || path == "/") && parsed.RawQuery == "" && parsed.Fragment == ""
}

func normalizeDownloadURLWithDomain(rawURL string) (string, string, bool) {
	for _, candidate := range downloadURLCandidates(rawURL) {
		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
		if scheme != "http" && scheme != "https" {
			continue
		}
		domain, ok := validRegistrableDomain(parsed.Hostname())
		if !ok {
			continue
		}
		parsed.Scheme = scheme
		return parsed.String(), domain, true
	}
	return "", "", false
}

func downloadURLCandidates(rawURL string) []string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "//") {
		return []string{"https:" + trimmed}
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && strings.TrimSpace(parsed.Scheme) != "" {
		if strings.Contains(trimmed, "://") {
			return []string{trimmed}
		}
		return []string{trimmed, "https://" + strings.TrimLeft(trimmed, "/")}
	}
	if err == nil && strings.TrimSpace(parsed.Host) != "" {
		parsed.Scheme = "https"
		return []string{parsed.String()}
	}
	return []string{"https://" + strings.TrimLeft(trimmed, "/")}
}

func validRegistrableDomain(host string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if normalized == "" || strings.ContainsAny(normalized, " \t\r\n/\\") {
		return "", false
	}
	if ip := net.ParseIP(normalized); ip != nil {
		return normalized, true
	}
	if !isDNSHostname(normalized) {
		return "", false
	}
	registrable, err := publicsuffix.EffectiveTLDPlusOne(normalized)
	if err != nil || strings.TrimSpace(registrable) == "" {
		return "", false
	}
	return registrable, true
}

func isDNSHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
				continue
			}
			return false
		}
	}
	return true
}

var (
	explicitURLPattern        = mustStrictXURLPattern("https?://")
	relaxedURLPattern         = xurls.Relaxed()
	youtubeIDPattern          = regexp.MustCompile(`^[0-9A-Za-z_-]{11}$`)
	bilibiliVideoIDPattern    = regexp.MustCompile(`(?i)^(?:av\d+|bv[^/?#&]+)$`)
	bilibiliVideoPathPattern  = regexp.MustCompile(`(?i)^video/(?:av\d+|bv[^/?#&]+)(?:[/?#].*)?$`)
	bilibiliFestivalPathMatch = regexp.MustCompile(`(?i)^festival/[^/?#]+(?:[/?#].*)?$`)
)

func mustStrictXURLPattern(scheme string) *regexp.Regexp {
	pattern, err := xurls.StrictMatchingScheme(scheme)
	if err != nil {
		panic(err)
	}
	return pattern
}

func normalizeKnownVideoSuffix(rawURL string) (string, string, bool) {
	if resolvedURL, ok := normalizeYouTubeVideoSuffix(rawURL); ok {
		return resolvedURL, "youtube.com", true
	}
	if resolvedURL, ok := normalizeBilibiliVideoSuffix(rawURL); ok {
		return resolvedURL, "bilibili.com", true
	}
	return "", "", false
}

func normalizeYouTubeVideoSuffix(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}
	if youtubeIDPattern.MatchString(trimmed) {
		return "https://www.youtube.com/watch?v=" + trimmed, true
	}

	suffix := strings.TrimLeft(trimmed, "/")
	switch {
	case strings.HasPrefix(suffix, "?"):
		suffix = "watch" + suffix
	case strings.HasPrefix(strings.ToLower(suffix), "v="):
		suffix = "watch?" + suffix
	case strings.HasPrefix(suffix, "#!?"):
		suffix = "watch?" + strings.TrimPrefix(suffix, "#!?")
	case strings.HasPrefix(suffix, "#!"):
		suffix = strings.TrimPrefix(suffix, "#!")
	}

	candidate := "https://www.youtube.com/" + suffix
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}
	path := strings.Trim(strings.ToLower(parsed.EscapedPath()), "/")
	switch path {
	case "watch", "watch.php", "watch_popup", "watch_popup.php", "movie", "movie.php":
		if id := firstYouTubeVideoID(parsed.Query()["v"]); id != "" {
			return parsed.String(), true
		}
	case "v", "embed", "e", "shorts", "live":
		return "", false
	default:
		for _, prefix := range []string{"v/", "embed/", "e/", "shorts/", "live/"} {
			if !strings.HasPrefix(path, prefix) {
				continue
			}
			id := strings.Trim(strings.TrimPrefix(path, prefix), "/")
			if youtubeIDPattern.MatchString(id) {
				return parsed.String(), true
			}
			return "", false
		}
	}
	return "", false
}

func firstYouTubeVideoID(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if youtubeIDPattern.MatchString(trimmed) {
			return trimmed
		}
	}
	return ""
}

func normalizeBilibiliVideoSuffix(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}
	suffix := strings.TrimLeft(trimmed, "/")
	if bilibiliVideoIDPattern.MatchString(suffix) {
		return "https://www.bilibili.com/video/" + suffix, true
	}
	if bilibiliVideoPathPattern.MatchString(suffix) {
		return "https://www.bilibili.com/" + suffix, true
	}
	if !bilibiliFestivalPathMatch.MatchString(suffix) {
		return "", false
	}
	candidate := "https://www.bilibili.com/" + suffix
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}
	if bilibiliVideoIDPattern.MatchString(strings.TrimSpace(parsed.Query().Get("bvid"))) {
		return parsed.String(), true
	}
	return "", false
}

type appSessionAvailability struct {
	ID              string
	Available       bool
	CredentialMode  string
	CredentialState string
}

func (service *LibraryService) resolveAppSessionAvailability(ctx context.Context, domain string) appSessionAvailability {
	if service.appSessions == nil {
		return appSessionAvailability{}
	}
	siteKey := appSessionSiteKeyForDomain(domain)
	if siteKey == "" {
		return appSessionAvailability{}
	}
	items, err := service.appSessions.ListAppSessions(ctx)
	if err != nil {
		return appSessionAvailability{}
	}
	for _, item := range items {
		if strings.EqualFold(item.SiteKey, siteKey) {
			state := strings.TrimSpace(item.CredentialState)
			if state == "" {
				state = strings.TrimSpace(item.Status)
			}
			return appSessionAvailability{
				ID:              item.ID,
				Available:       strings.EqualFold(item.Status, "connected") && appSessionCredentialStateCanExportCookies(state),
				CredentialMode:  "app_session",
				CredentialState: state,
			}
		}
	}
	return appSessionAvailability{}
}

func appSessionCredentialStateCanExportCookies(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "app_session", "cookies":
		return true
	default:
		return false
	}
}

func appSessionSiteKeyForDomain(domain string) string {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	switch normalized {
	case "youtube.com", "youtu.be", "youtube-nocookie.com":
		return "youtube"
	case "bilibili.com", "b23.tv":
		return "bilibili"
	case "tiktok.com", "tiktokv.com", "vm.tiktok.com":
		return "tiktok"
	case "douyin.com", "iesdouyin.com",
		"xiaohongshu.com", "rednote.com", "xhs.cn",
		"xhslink.com", "xhslink.cn", "xhsurl.com", "rl.ink":
		return "china_private"
	case "instagram.com":
		return "instagram"
	case "x.com", "twitter.com":
		return "x"
	case "facebook.com", "fb.watch":
		return "facebook"
	case "vimeo.com", "player.vimeo.com":
		return "vimeo"
	case "twitch.tv", "clips.twitch.tv":
		return "twitch"
	case "nicovideo.jp", "nico.ms", "nicovideo.cdn.nimg.jp":
		return "niconico"
	default:
		return ""
	}
}
