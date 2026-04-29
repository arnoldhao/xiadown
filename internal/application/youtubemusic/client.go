package youtubemusic

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"
)

const (
	apiBaseURL            = "https://music.youtube.com/youtubei/v1"
	apiKey                = "AIzaSyC9XL3ZjWddXya6X74dJoCTL-WEYFDNX30"
	clientName            = "WEB_REMIX"
	clientVersion         = "1.20231204.01.00"
	origin                = "https://music.youtube.com"
	searchSongsParams     = "EgWKAQIIAWoMEA4QChADEAQQCRAF"
	searchArtistsParams   = "EgWKAQIgAWoMEA4QChADEAQQCRAF"
	searchPlaylistsParams = "EgWKAQIoAWoMEA4QChADEAQQCRAF"
	defaultLimit          = 12
)

const (
	BrowserUserAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
	WindowsWebViewUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0"
)

var (
	ErrNotAuthenticated   = errors.New("youtube music is not authenticated")
	ErrAuthExpired        = errors.New("youtube music auth expired")
	ErrRequestTimedOut    = errors.New("youtube music request timed out")
	ErrNetworkUnavailable = errors.New("youtube music network unavailable")

	videoIDPattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	durationLabelPattern = regexp.MustCompile(`^\d{1,2}:\d{2}(?::\d{2})?$`)
)

type CookieProvider interface {
	CookiesForConnectorType(ctx context.Context, connectorType connectors.ConnectorType) ([]appcookies.Record, error)
}

type HTTPClientProvider interface {
	HTTPClient() *http.Client
}

type Client struct {
	cookies            CookieProvider
	httpClient         *http.Client
	httpClientProvider HTTPClientProvider
	now                func() time.Time
}

type Track struct {
	ID              string
	VideoID         string
	Title           string
	Channel         string
	ArtistBrowseID  string
	DurationLabel   string
	PlayCountLabel  string
	ThumbnailURL    string
	MusicVideoType  string
	IsExplicit      bool
	RawDescription  string
	ContinuationKey string
}

type textRun struct {
	Text     string
	BrowseID string
}

type LikeStatus string

const (
	LikeStatusIndifferent LikeStatus = "indifferent"
	LikeStatusLike        LikeStatus = "like"
	LikeStatusDislike     LikeStatus = "dislike"
)

type TrackMetadata struct {
	VideoID         string
	Title           string
	Channel         string
	ArtistBrowseID  string
	DurationLabel   string
	LikeStatus      LikeStatus
	LikeStatusKnown bool
	ThumbnailURL    string
	MusicVideoType  string
}

func NewClient(cookies CookieProvider) *Client {
	return &Client{
		cookies:    cookies,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		now:        time.Now,
	}
}

func NewClientWithHTTPClientProvider(cookies CookieProvider, provider HTTPClientProvider) *Client {
	client := NewClient(cookies)
	client.httpClientProvider = provider
	return client
}

func (client *Client) SearchSongs(ctx context.Context, query string, limit int) ([]Track, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}
	body := map[string]any{
		"query":  trimmed,
		"params": searchSongsParams,
	}
	data, err := client.request(ctx, "search", body)
	if err != nil {
		return nil, err
	}
	return parseSearchSongs(data, normalizeLimit(limit)), nil
}

func (client *Client) SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}
	body := map[string]any{
		"query":  trimmed,
		"params": searchArtistsParams,
	}
	data, err := client.request(ctx, "search", body)
	if err != nil {
		return nil, err
	}
	return parseSearchArtists(data, normalizeLimit(limit)), nil
}

func (client *Client) SearchPlaylists(ctx context.Context, query string, limit int) ([]Playlist, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}
	body := map[string]any{
		"query":  trimmed,
		"params": searchPlaylistsParams,
	}
	data, err := client.request(ctx, "search", body)
	if err != nil {
		return nil, err
	}
	return parseSearchPlaylists(data, normalizeLimit(limit)), nil
}

func (client *Client) Radio(ctx context.Context, videoID string, limit int) ([]Track, error) {
	trimmed := strings.TrimSpace(videoID)
	if !videoIDPattern.MatchString(trimmed) {
		return nil, fmt.Errorf("invalid youtube video id")
	}
	body := map[string]any{
		"videoId":                       trimmed,
		"playlistId":                    "RDAMVM" + trimmed,
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
	}
	data, err := client.request(ctx, "next", body)
	if err != nil {
		return nil, err
	}
	return parseRadioTracks(data, normalizeLimit(limit)), nil
}

func (client *Client) TrackMetadata(ctx context.Context, videoID string) (TrackMetadata, error) {
	trimmed := strings.TrimSpace(videoID)
	if !videoIDPattern.MatchString(trimmed) {
		return TrackMetadata{}, fmt.Errorf("invalid youtube video id")
	}
	data, err := client.request(ctx, "next", map[string]any{
		"videoId":                       trimmed,
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
	})
	if err != nil {
		return TrackMetadata{}, err
	}
	return parseTrackMetadata(data, trimmed), nil
}

func (client *Client) TrackDurations(ctx context.Context, videoIDs []string) (map[string]string, error) {
	ids := cleanVideoIDs(videoIDs, 50)
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	data, err := client.request(ctx, "music/get_queue", map[string]any{
		"videoIds": ids,
	})
	if err != nil {
		return nil, err
	}
	tracks := parseQueueTracks(data, len(ids))
	durations := make(map[string]string, len(tracks))
	for _, track := range tracks {
		videoID := strings.TrimSpace(track.VideoID)
		duration := strings.TrimSpace(track.DurationLabel)
		if videoID == "" || duration == "" {
			continue
		}
		durations[videoID] = duration
	}
	return durations, nil
}

func (client *Client) RateSong(ctx context.Context, videoID string, rating LikeStatus) error {
	trimmed := strings.TrimSpace(videoID)
	if !videoIDPattern.MatchString(trimmed) {
		return fmt.Errorf("invalid youtube video id")
	}
	endpoint := ""
	switch rating {
	case LikeStatusLike:
		endpoint = "like/like"
	case LikeStatusDislike:
		endpoint = "like/dislike"
	default:
		endpoint = "like/removelike"
	}
	_, err := client.request(ctx, endpoint, map[string]any{
		"target": map[string]any{
			"videoId": trimmed,
		},
	})
	return err
}

func (client *Client) request(ctx context.Context, endpoint string, body map[string]any) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("youtube music client is nil")
	}
	requestBody := make(map[string]any, len(body)+1)
	for key, value := range body {
		requestBody[key] = value
	}
	requestBody["context"] = buildContext()

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestURL, err := url.Parse(apiBaseURL + "/" + strings.Trim(endpoint, "/"))
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("key", apiKey)
	query.Set("prettyPrint", "false")
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	headers, err := client.authHeaders(ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.httpClientForRequest().Do(req)
	if err != nil {
		return nil, wrapRequestError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrAuthExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("youtube music api status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		return nil, wrapRequestError(err)
	}
	return result, nil
}

func (client *Client) httpClientForRequest() *http.Client {
	if client != nil && client.httpClientProvider != nil {
		if provided := client.httpClientProvider.HTTPClient(); provided != nil {
			return provided
		}
	}
	if client != nil && client.httpClient != nil {
		return client.httpClient
	}
	return http.DefaultClient
}

func (client *Client) authHeaders(ctx context.Context) (map[string]string, error) {
	if client.cookies == nil {
		return nil, ErrNotAuthenticated
	}
	records, err := client.cookies.CookiesForConnectorType(ctx, connectors.ConnectorYouTube)
	if err != nil {
		if errors.Is(err, connectors.ErrNoCookies) || errors.Is(err, connectors.ErrConnectorNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrNotAuthenticated, err)
		}
		return nil, err
	}
	matched := appcookies.MatchURL(records, origin+"/")
	if len(matched) == 0 {
		return nil, ErrNotAuthenticated
	}

	now := client.now
	if now == nil {
		now = time.Now
	}
	currentTime := now()
	cookieHeader := buildCookieHeader(matched, currentTime)
	if cookieHeader == "" {
		return nil, ErrNotAuthenticated
	}
	sapisid := findSAPISID(matched, currentTime)
	if sapisid == "" {
		return nil, ErrAuthExpired
	}

	timestamp := currentTime.Unix()
	hashInput := fmt.Sprintf("%d %s %s", timestamp, sapisid, origin)
	hash := sha1.Sum([]byte(hashInput))
	return map[string]string{
		"Authorization":   fmt.Sprintf("SAPISIDHASH %d_%x", timestamp, hash),
		"Content-Type":    "application/json",
		"Cookie":          cookieHeader,
		"Origin":          origin,
		"Referer":         origin,
		"User-Agent":      BrowserUserAgent,
		"X-Goog-AuthUser": "0",
		"X-Origin":        origin,
	}, nil
}

func wrapRequestError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if isRequestTimeoutError(err) {
		return fmt.Errorf("%w: %w", ErrRequestTimedOut, err)
	}
	if isRequestNetworkError(err) {
		return fmt.Errorf("%w: %w", ErrNetworkUnavailable, err)
	}
	return err
}

func isRequestTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "tls handshake timeout")
}

func isRequestNetworkError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	if lower == "" {
		return false
	}
	if lower == "eof" || strings.Contains(lower, ": eof") || strings.Contains(lower, " eof") {
		return true
	}
	for _, marker := range []string{
		"no such host",
		"network is unreachable",
		"connection refused",
		"connection reset",
		"connection closed",
		"unexpected eof",
		"temporary failure",
		"dial tcp",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func buildContext() map[string]any {
	_, offsetSeconds := time.Now().Zone()
	return map[string]any{
		"client": map[string]any{
			"clientName":       clientName,
			"clientVersion":    clientVersion,
			"hl":               "en",
			"gl":               "US",
			"browserName":      "Safari",
			"browserVersion":   "17.0",
			"osName":           "Macintosh",
			"osVersion":        "10_15_7",
			"platform":         "DESKTOP",
			"userAgent":        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
			"utcOffsetMinutes": -offsetSeconds / 60,
		},
		"user": map[string]any{
			"lockedSafetyMode": false,
		},
	}
}

func buildCookieHeader(records []appcookies.Record, now time.Time) string {
	parts := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.Name)
		value := strings.TrimSpace(record.Value)
		if name == "" || value == "" || strings.ContainsAny(name, ";\r\n\t ") || isExpired(record, now) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func findSAPISID(records []appcookies.Record, now time.Time) string {
	for _, name := range []string{"__Secure-3PAPISID", "SAPISID", "__Secure-1PAPISID"} {
		for _, record := range records {
			if record.Name == name && !isExpired(record, now) && strings.TrimSpace(record.Value) != "" {
				return strings.TrimSpace(record.Value)
			}
		}
	}
	return ""
}

func isExpired(record appcookies.Record, now time.Time) bool {
	return record.Expires > 0 && record.Expires <= now.Unix()
}

func parseSearchSongs(data map[string]any, limit int) []Track {
	renderers := collectRendererMaps(data, "musicResponsiveListItemRenderer")
	tracks := make([]Track, 0, len(renderers))
	seen := make(map[string]struct{}, len(renderers))
	for _, renderer := range renderers {
		track, ok := trackFromMusicResponsiveRenderer(renderer)
		if !ok {
			continue
		}
		if _, exists := seen[track.VideoID]; exists {
			continue
		}
		seen[track.VideoID] = struct{}{}
		tracks = append(tracks, track)
		if len(tracks) >= limit {
			break
		}
	}
	return tracks
}

func parseSearchArtists(data map[string]any, limit int) []Artist {
	renderers := collectRendererMaps(data, "musicResponsiveListItemRenderer")
	artists := make([]Artist, 0, len(renderers))
	seen := make(map[string]struct{}, len(renderers))
	for _, renderer := range renderers {
		artist, ok := artistFromSearchResponsiveRenderer(renderer)
		if !ok {
			continue
		}
		if _, exists := seen[artist.ID]; exists {
			continue
		}
		seen[artist.ID] = struct{}{}
		artists = append(artists, artist)
		if len(artists) >= limit {
			break
		}
	}
	return artists
}

func parseSearchPlaylists(data map[string]any, limit int) []Playlist {
	renderers := collectRendererMaps(data, "musicResponsiveListItemRenderer")
	playlists := make([]Playlist, 0, len(renderers))
	seen := make(map[string]struct{}, len(renderers))
	for _, renderer := range renderers {
		playlist, ok := playlistFromResponsiveRenderer(renderer)
		if !ok {
			continue
		}
		if _, exists := seen[playlist.ID]; exists {
			continue
		}
		seen[playlist.ID] = struct{}{}
		playlists = append(playlists, playlist)
		if len(playlists) >= limit {
			break
		}
	}
	return playlists
}

func parseRadioTracks(data map[string]any, limit int) []Track {
	renderers := collectRendererMaps(data, "playlistPanelVideoRenderer")
	tracks := make([]Track, 0, len(renderers))
	seen := make(map[string]struct{}, len(renderers))
	for _, renderer := range renderers {
		track, ok := trackFromPlaylistPanelRenderer(renderer)
		if !ok {
			continue
		}
		if _, exists := seen[track.VideoID]; exists {
			continue
		}
		seen[track.VideoID] = struct{}{}
		tracks = append(tracks, track)
		if len(tracks) >= limit {
			break
		}
	}
	return tracks
}

func trackFromMusicResponsiveRenderer(renderer map[string]any) (Track, bool) {
	videoID := stringInMap(asMap(renderer["playlistItemData"]), "videoId")
	if videoID == "" {
		videoID = findFirstStringByKey(renderer, "videoId")
	}
	if !videoIDPattern.MatchString(videoID) {
		return Track{}, false
	}

	flexColumns := mapsFromArray(renderer["flexColumns"])
	title := ""
	metadataRuns := make([]string, 0, 8)
	for index, column := range flexColumns {
		navigationRuns := textRunsWithNavigationFromFlexColumn(column)
		runs := textValuesFromRuns(navigationRuns)
		if index == 0 && len(runs) > 0 {
			title = strings.TrimSpace(runs[0])
			continue
		}
		metadataRuns = append(metadataRuns, runs...)
	}
	if title == "" {
		title = firstUsefulText(collectTextRuns(renderer))
	}
	if title == "" {
		title = videoID
	}

	channel, artistBrowseID := artistRunFromFlexColumns(flexColumns)
	duration := durationFromFixedColumns(renderer)
	if duration == "" {
		duration = firstDurationLabel(metadataRuns)
	}
	playCount := firstPlayCountLabel(metadataRuns)

	return Track{
		ID:             videoID,
		VideoID:        videoID,
		Title:          title,
		Channel:        channel,
		ArtistBrowseID: artistBrowseID,
		DurationLabel:  duration,
		PlayCountLabel: playCount,
		ThumbnailURL:   lastThumbnailURL(renderer),
		MusicVideoType: musicVideoTypeFromRenderer(renderer),
	}, true
}

func trackFromPlaylistPanelRenderer(renderer map[string]any) (Track, bool) {
	videoID := stringInMap(renderer, "videoId")
	if !videoIDPattern.MatchString(videoID) {
		return Track{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = videoID
	}
	channel, artistBrowseID := firstCreatorRun(textRunsWithNavigation(asMap(renderer["longBylineText"])))
	if channel == "" {
		channel, artistBrowseID = firstCreatorRun(textRunsWithNavigation(asMap(renderer["shortBylineText"])))
	}
	duration := firstUsefulText(runsText(asMap(renderer["lengthText"])))
	if duration == "" {
		duration = firstDurationLabel(collectTextRuns(renderer))
	}
	return Track{
		ID:             videoID,
		VideoID:        videoID,
		Title:          title,
		Channel:        fallbackString(channel, "YouTube Music"),
		ArtistBrowseID: artistBrowseID,
		DurationLabel:  duration,
		ThumbnailURL:   lastThumbnailURL(renderer),
		MusicVideoType: musicVideoTypeFromRenderer(renderer),
	}, true
}

func artistFromSearchResponsiveRenderer(renderer map[string]any) (Artist, bool) {
	navigationEndpoint := asMap(renderer["navigationEndpoint"])
	browseEndpoint := asMap(navigationEndpoint["browseEndpoint"])
	browseID := stringInMap(browseEndpoint, "browseId")
	if browseID == "" || (!isArtistBrowseID(browseID) && !isArtistPageType(pageTypeFromBrowseEndpoint(browseEndpoint))) {
		return Artist{}, false
	}

	flexColumns := mapsFromArray(renderer["flexColumns"])
	title := ""
	if len(flexColumns) > 0 {
		title = firstUsefulText(textRunsFromFlexColumn(flexColumns[0]))
	}
	if title == "" {
		title = firstUsefulText(collectTextRuns(renderer))
	}
	if title == "" {
		title = browseID
	}

	subtitleRuns := make([]string, 0, 4)
	for _, column := range flexColumns[1:] {
		subtitleRuns = append(subtitleRuns, textRunsFromFlexColumn(column)...)
	}
	return Artist{
		ID:           browseID,
		Name:         title,
		Subtitle:     firstUsefulText(subtitleRuns),
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

func parseTrackMetadata(data map[string]any, videoID string) TrackMetadata {
	renderers := collectRendererMaps(data, "playlistPanelVideoRenderer")
	for _, renderer := range renderers {
		currentVideoID := stringInMap(renderer, "videoId")
		if currentVideoID != "" && currentVideoID != videoID {
			continue
		}
		likeStatus, likeStatusKnown := likeStatusFromRenderer(renderer)
		track, _ := trackFromPlaylistPanelRenderer(renderer)
		return TrackMetadata{
			VideoID:         fallbackString(track.VideoID, fallbackString(currentVideoID, videoID)),
			Title:           track.Title,
			Channel:         track.Channel,
			ArtistBrowseID:  track.ArtistBrowseID,
			DurationLabel:   track.DurationLabel,
			LikeStatus:      likeStatus,
			LikeStatusKnown: likeStatusKnown,
			ThumbnailURL:    fallbackString(track.ThumbnailURL, lastThumbnailURL(renderer)),
			MusicVideoType:  musicVideoTypeFromRenderer(renderer),
		}
	}
	return TrackMetadata{
		VideoID:         videoID,
		LikeStatus:      LikeStatusIndifferent,
		LikeStatusKnown: false,
	}
}

func musicVideoTypeFromRenderer(renderer map[string]any) string {
	if renderer == nil {
		return ""
	}
	if direct := stringInMap(renderer, "musicVideoType"); direct != "" {
		return direct
	}
	navigationEndpoint := asMap(renderer["navigationEndpoint"])
	watchEndpoint := asMap(navigationEndpoint["watchEndpoint"])
	if value := musicVideoTypeFromWatchEndpoint(watchEndpoint); value != "" {
		return value
	}
	return findFirstStringByKey(renderer, "musicVideoType")
}

func musicVideoTypeFromWatchEndpoint(watchEndpoint map[string]any) string {
	if watchEndpoint == nil {
		return ""
	}
	configs := asMap(watchEndpoint["watchEndpointMusicSupportedConfigs"])
	musicConfig := asMap(configs["watchEndpointMusicConfig"])
	return strings.TrimSpace(stringInMap(musicConfig, "musicVideoType"))
}

func likeStatusFromRenderer(renderer map[string]any) (LikeStatus, bool) {
	menuRenderer := asMap(asMap(renderer["menu"])["menuRenderer"])
	if menuRenderer == nil {
		return LikeStatusIndifferent, false
	}
	for _, button := range mapsFromArray(menuRenderer["topLevelButtons"]) {
		likeButton := asMap(button["likeButtonRenderer"])
		if likeButton == nil {
			continue
		}
		status := strings.ToUpper(stringInMap(likeButton, "likeStatus"))
		if status == "" {
			continue
		}
		switch status {
		case "LIKE":
			return LikeStatusLike, true
		case "DISLIKE":
			return LikeStatusDislike, true
		default:
			return LikeStatusIndifferent, true
		}
	}
	return LikeStatusIndifferent, false
}

func collectRendererMaps(value any, key string) []map[string]any {
	result := make([]map[string]any, 0)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for itemKey, itemValue := range typed {
				if itemKey == key {
					if renderer := asMap(itemValue); renderer != nil {
						result = append(result, renderer)
					}
					continue
				}
				walk(itemValue)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return result
}

func collectTextRuns(value any) []string {
	result := make([]string, 0)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if runs := runsText(typed); len(runs) > 0 {
				result = append(result, runs...)
			}
			for _, itemValue := range typed {
				walk(itemValue)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return result
}

func textRunsFromFlexColumn(column map[string]any) []string {
	return textValuesFromRuns(textRunsWithNavigationFromFlexColumn(column))
}

func textRunsWithNavigationFromFlexColumn(column map[string]any) []textRun {
	renderer := asMap(column["musicResponsiveListItemFlexColumnRenderer"])
	if renderer == nil {
		return nil
	}
	return textRunsWithNavigation(asMap(renderer["text"]))
}

func runsText(text map[string]any) []string {
	return textValuesFromRuns(textRunsWithNavigation(text))
}

func textRunsWithNavigation(text map[string]any) []textRun {
	if text == nil {
		return nil
	}
	runs, ok := text["runs"].([]any)
	if !ok {
		if simple := strings.TrimSpace(stringInMap(text, "text")); simple != "" {
			return []textRun{{Text: simple}}
		}
		return nil
	}
	result := make([]textRun, 0, len(runs))
	for _, run := range runs {
		runMap := asMap(run)
		value := strings.TrimSpace(stringInMap(runMap, "text"))
		if value != "" {
			result = append(result, textRun{
				Text:     value,
				BrowseID: browseIDFromNavigationEndpoint(asMap(runMap["navigationEndpoint"])),
			})
		}
	}
	return result
}

func textValuesFromRuns(runs []textRun) []string {
	if len(runs) == 0 {
		return nil
	}
	values := make([]string, 0, len(runs))
	for _, run := range runs {
		if trimmed := strings.TrimSpace(run.Text); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func durationFromFixedColumns(renderer map[string]any) string {
	for _, fixedColumn := range mapsFromArray(renderer["fixedColumns"]) {
		columnRenderer := asMap(fixedColumn["musicResponsiveListItemFixedColumnRenderer"])
		for _, run := range runsText(asMap(columnRenderer["text"])) {
			if durationLabelPattern.MatchString(strings.TrimSpace(run)) {
				return strings.TrimSpace(run)
			}
		}
	}
	return ""
}

func firstDurationLabel(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if durationLabelPattern.MatchString(trimmed) {
			return trimmed
		}
	}
	return ""
}

func firstPlayCountLabel(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if isPlayCountLabel(trimmed) {
			return trimmed
		}
	}
	return ""
}

func isPlayCountLabel(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isSeparatorText(trimmed) || durationLabelPattern.MatchString(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "playlist") || strings.Contains(lower, "songs") {
		return false
	}
	hasCount := strings.ContainsAny(trimmed, "0123456789")
	if !hasCount && !strings.Contains(lower, "no views") && !strings.Contains(lower, "no plays") {
		return false
	}
	return strings.Contains(lower, "views") ||
		strings.Contains(lower, "plays") ||
		strings.HasSuffix(lower, " play") ||
		strings.Contains(trimmed, "播放") ||
		strings.Contains(trimmed, "观看") ||
		strings.Contains(trimmed, "觀看")
}

func firstCreatorText(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if !isUsefulCreatorText(trimmed) {
			continue
		}
		return trimmed
	}
	return ""
}

func firstCreatorRun(values []textRun) (string, string) {
	for _, run := range values {
		trimmed := strings.TrimSpace(run.Text)
		if !isUsefulCreatorText(trimmed) {
			continue
		}
		return trimmed, strings.TrimSpace(run.BrowseID)
	}
	return "", ""
}

func artistRunFromFlexColumns(flexColumns []map[string]any) (string, string) {
	artists := artistRunsFromFlexColumns(flexColumns)
	if len(artists) == 0 {
		return "", ""
	}
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		names = append(names, strings.TrimSpace(artist.Text))
	}
	return strings.Join(names, ", "), strings.TrimSpace(artists[0].BrowseID)
}

func artistRunsFromFlexColumns(flexColumns []map[string]any) []textRun {
	if len(flexColumns) <= 1 {
		return nil
	}
	return artistRuns(textRunsWithNavigationFromFlexColumn(flexColumns[1]))
}

func artistRuns(values []textRun) []textRun {
	artists := make([]textRun, 0, len(values))
	for _, run := range values {
		browseID := strings.TrimSpace(run.BrowseID)
		if !isArtistBrowseID(browseID) {
			continue
		}
		trimmed := strings.TrimSpace(run.Text)
		if trimmed == "" || isSeparatorText(trimmed) || isKasetContentTypeKeyword(trimmed) {
			continue
		}
		artists = append(artists, textRun{Text: trimmed, BrowseID: browseID})
	}
	return artists
}

func firstUsefulText(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && !isSeparatorText(trimmed) {
			return trimmed
		}
	}
	return ""
}

func isUsefulCreatorText(value string) bool {
	if value == "" || isSeparatorText(value) || durationLabelPattern.MatchString(value) {
		return false
	}
	if isPlayCountLabel(value) {
		return false
	}
	switch strings.ToLower(value) {
	case "song", "video", "album", "single", "ep", "playlist", "youtube", "youtube music":
		return false
	}
	lower := strings.ToLower(value)
	return !strings.Contains(lower, "views") &&
		!strings.Contains(lower, "songs")
}

func isKasetContentTypeKeyword(value string) bool {
	switch value {
	case "Song", "Video", "Album", "Playlist", "Artist", "Episode", "Podcast":
		return true
	default:
		return false
	}
}

func isSeparatorText(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "•", "·", "-", "|", ",", "&":
		return true
	default:
		return false
	}
}

func lastThumbnailURL(value any) string {
	if thumbnailURL := lastDirectThumbnailURL(value); thumbnailURL != "" {
		return thumbnailURL
	}
	urls := make([]string, 0)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if thumbnails, ok := typed["thumbnails"].([]any); ok {
				for _, thumbnail := range thumbnails {
					rawURL := stringInMap(asMap(thumbnail), "url")
					if isHTTPURL(rawURL) {
						urls = append(urls, normalizeYouTubeMusicImageURL(rawURL))
					}
				}
			}
			for _, itemValue := range typed {
				walk(itemValue)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	if len(urls) == 0 {
		return ""
	}
	return urls[len(urls)-1]
}

func lastDirectThumbnailURL(value any) string {
	mapped := asMap(value)
	if mapped == nil {
		return ""
	}
	for _, candidate := range []any{
		nestedMap(mapped, "thumbnail", "musicThumbnailRenderer", "thumbnail"),
		nestedMap(mapped, "thumbnail", "croppedSquareThumbnailRenderer", "thumbnail"),
		nestedMap(mapped, "thumbnail"),
		nestedMap(mapped, "thumbnailRenderer", "musicThumbnailRenderer", "thumbnail"),
		nestedMap(mapped, "thumbnailRenderer", "croppedSquareThumbnailRenderer", "thumbnail"),
		nestedMap(mapped, "foregroundThumbnail", "musicThumbnailRenderer", "thumbnail"),
		mapped,
		nestedMap(mapped, "thumbnailDetails"),
		nestedMap(mapped, "image"),
	} {
		if thumbnailURL := lastURLFromThumbnails(candidate); thumbnailURL != "" {
			return thumbnailURL
		}
	}
	return ""
}

func lastURLFromThumbnails(value any) string {
	thumbnails := mapsFromArray(asMap(value)["thumbnails"])
	for index := len(thumbnails) - 1; index >= 0; index-- {
		rawURL := stringInMap(thumbnails[index], "url")
		if isHTTPURL(rawURL) {
			return normalizeYouTubeMusicImageURL(rawURL)
		}
	}
	return ""
}

func nestedMap(value map[string]any, keys ...string) map[string]any {
	current := value
	for _, key := range keys {
		current = asMap(current[key])
		if current == nil {
			return nil
		}
	}
	return current
}

func isHTTPURL(value string) bool {
	normalized := normalizeYouTubeMusicImageURL(value)
	return strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://")
}

func normalizeYouTubeMusicImageURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed
	}
	return trimmed
}

func findFirstStringByKey(value any, key string) string {
	var result string
	var walk func(any)
	walk = func(current any) {
		if result != "" {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			for itemKey, itemValue := range typed {
				if itemKey == key {
					if value, ok := itemValue.(string); ok {
						result = strings.TrimSpace(value)
						return
					}
				}
				walk(itemValue)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return result
}

func mapsFromArray(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped := asMap(item); mapped != nil {
			result = append(result, mapped)
		}
	}
	return result
}

func asMap(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	return mapped
}

func stringInMap(mapped map[string]any, key string) string {
	if mapped == nil {
		return ""
	}
	value, _ := mapped[key].(string)
	return strings.TrimSpace(value)
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func cleanVideoIDs(videoIDs []string, limit int) []string {
	if limit <= 0 {
		limit = 50
	}
	result := make([]string, 0, min(len(videoIDs), limit))
	seen := make(map[string]struct{}, len(videoIDs))
	for _, videoID := range videoIDs {
		trimmed := strings.TrimSpace(videoID)
		if !videoIDPattern.MatchString(trimmed) {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	return max(1, min(limit, 50))
}

func FormatDuration(seconds float64) string {
	totalSeconds := int(seconds + 0.5)
	if totalSeconds <= 0 {
		return ""
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60
	if hours > 0 {
		return strconv.Itoa(hours) + ":" + pad2(minutes) + ":" + pad2(secs)
	}
	return strconv.Itoa(minutes) + ":" + pad2(secs)
}

func pad2(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
