package youtubemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	lyricsResultSynced      = "synced"
	lyricsResultPlain       = "plain"
	lyricsResultUnavailable = "unavailable"
	lrcLibSearchURL         = "https://lrclib.net/api/search"
	lrcLibTimeout           = 6 * time.Second
)

var (
	lrcTimePattern           = regexp.MustCompile(`\[(\d{2,}):(\d{2})\.(\d{2,3})\]`)
	lrcMetadataPattern       = regexp.MustCompile(`^\[([a-zA-Z]+):([^\]]+)\]\s*$`)
	lrcWordPattern           = regexp.MustCompile(`<(\d{2,}):(\d{2})\.(\d{2,3})>([^<]+)`)
	lyricsRequestRetryDelays = []time.Duration{220 * time.Millisecond, 650 * time.Millisecond}
)

type LyricsSearchInfo struct {
	VideoID         string
	Title           string
	Artist          string
	DurationSeconds float64
}

type LyricLine struct {
	StartMs    int         `json:"startMs"`
	DurationMs int         `json:"durationMs"`
	Text       string      `json:"text"`
	Words      []TimedWord `json:"words,omitempty"`
}

type TimedWord struct {
	StartMs int    `json:"startMs"`
	Text    string `json:"text"`
}

type LyricsResult struct {
	Kind   string      `json:"kind"`
	Source string      `json:"source,omitempty"`
	Text   string      `json:"text,omitempty"`
	Lines  []LyricLine `json:"lines,omitempty"`
}

type lrcLibModel struct {
	ID           int      `json:"id"`
	TrackName    string   `json:"trackName"`
	ArtistName   string   `json:"artistName"`
	AlbumName    string   `json:"albumName"`
	Duration     *float64 `json:"duration"`
	Instrumental *bool    `json:"instrumental"`
	PlainLyrics  string   `json:"plainLyrics"`
	SyncedLyrics string   `json:"syncedLyrics"`
}

func (client *Client) TrackLyrics(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	normalized := normalizeLyricsSearchInfo(info)
	if !videoIDPattern.MatchString(normalized.VideoID) {
		if normalized.Title == "" {
			return LyricsResult{}, fmt.Errorf("invalid youtube video id")
		}
		lrcLibResult := client.searchLRCLibLyrics(ctx, normalized)
		if lrcLibResult.Kind == lyricsResultSynced || lrcLibResult.Kind == lyricsResultPlain {
			return lrcLibResult, nil
		}
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}

	nextData, err := client.requestLyricsYouTubeData(ctx, "next", map[string]any{
		"videoId":                       normalized.VideoID,
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
	})
	if err != nil {
		if fallback := client.searchLRCLibLyrics(ctx, normalized); fallback.Kind == lyricsResultSynced || fallback.Kind == lyricsResultPlain {
			return fallback, nil
		}
		return LyricsResult{}, err
	}

	if synced := extractTimedLyrics(nextData, "YTMusic"); synced.Kind == lyricsResultSynced {
		return synced, nil
	}

	metadata := parseTrackMetadata(nextData, normalized.VideoID)
	if normalized.Title == "" {
		normalized.Title = metadata.Title
	}
	if normalized.Artist == "" {
		normalized.Artist = metadata.Channel
	}
	if normalized.DurationSeconds <= 0 {
		normalized.DurationSeconds = durationLabelSeconds(metadata.DurationLabel)
	}

	plainFallback := LyricsResult{Kind: lyricsResultUnavailable}
	var browseErr error
	if browseID := extractLyricsBrowseID(nextData); browseID != "" {
		browseData, err := client.requestLyricsYouTubeData(ctx, "browse", map[string]any{
			"browseId": browseID,
		})
		if err != nil {
			browseErr = err
		} else {
			if synced := extractTimedLyrics(browseData, "YTMusic"); synced.Kind == lyricsResultSynced {
				return synced, nil
			}
			if plain := extractPlainLyrics(browseData, "YTMusic"); plain.Kind == lyricsResultPlain {
				plainFallback = plain
			}
		}
	}

	lrcLibResult := client.searchLRCLibLyrics(ctx, normalized)
	if lrcLibResult.Kind == lyricsResultSynced {
		return lrcLibResult, nil
	}
	if plainFallback.Kind == lyricsResultPlain {
		return plainFallback, nil
	}
	if lrcLibResult.Kind == lyricsResultPlain {
		return lrcLibResult, nil
	}
	if browseErr != nil {
		return LyricsResult{}, browseErr
	}

	return LyricsResult{Kind: lyricsResultUnavailable}, nil
}

func (client *Client) requestLyricsYouTubeData(ctx context.Context, endpoint string, body map[string]any) (map[string]any, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		data, err := client.request(ctx, endpoint, body)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !isRetryableLyricsRequestError(err) || attempt >= len(lyricsRequestRetryDelays) {
			break
		}
		delay := lyricsRequestRetryDelays[attempt]
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, wrapRequestError(ctx.Err())
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isRetryableLyricsRequestError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, ErrRequestTimedOut) || errors.Is(err, ErrNetworkUnavailable) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "youtube music api status 429") ||
		strings.Contains(lower, "youtube music api status 500") ||
		strings.Contains(lower, "youtube music api status 502") ||
		strings.Contains(lower, "youtube music api status 503") ||
		strings.Contains(lower, "youtube music api status 504")
}

func normalizeLyricsSearchInfo(info LyricsSearchInfo) LyricsSearchInfo {
	info.VideoID = strings.TrimSpace(info.VideoID)
	info.Title = strings.TrimSpace(info.Title)
	info.Artist = strings.TrimSpace(info.Artist)
	if math.IsNaN(info.DurationSeconds) || math.IsInf(info.DurationSeconds, 0) || info.DurationSeconds < 0 {
		info.DurationSeconds = 0
	}
	return info
}

func extractLyricsBrowseID(data map[string]any) string {
	contents := asMap(data["contents"])
	watchNext := asMap(contents["singleColumnMusicWatchNextResultsRenderer"])
	tabbed := asMap(watchNext["tabbedRenderer"])
	watchNextTabbed := asMap(tabbed["watchNextTabbedResultsRenderer"])
	for _, tab := range mapsFromArray(watchNextTabbed["tabs"]) {
		tabRenderer := asMap(tab["tabRenderer"])
		endpoint := asMap(tabRenderer["endpoint"])
		browseEndpoint := asMap(endpoint["browseEndpoint"])
		browseID := stringInMap(browseEndpoint, "browseId")
		if strings.HasPrefix(browseID, "MPLYt") {
			return browseID
		}
	}
	return ""
}

func extractTimedLyrics(data map[string]any, source string) LyricsResult {
	timedModel := findMapByKey(data, "timedLyricsModel")
	if timedModel == nil {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	lyricsData, ok := timedModel["lyricsData"].([]any)
	if !ok || len(lyricsData) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	lines := make([]LyricLine, 0, len(lyricsData))
	for _, item := range lyricsData {
		lineData := asMap(item)
		text := strings.TrimSpace(stringInMap(lineData, "lyricLine"))
		startMs, ok := parseFlexibleInt(lineData["startTimeMs"])
		if !ok {
			continue
		}
		durationMs, _ := parseFlexibleInt(lineData["durationMs"])
		lines = append(lines, LyricLine{
			StartMs:    max(0, startMs),
			DurationMs: max(0, durationMs),
			Text:       text,
		})
	}
	if len(lines) == 0 {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	sort.SliceStable(lines, func(left, right int) bool {
		return lines[left].StartMs < lines[right].StartMs
	})
	fillLyricLineDurations(lines)
	return LyricsResult{
		Kind:   lyricsResultSynced,
		Source: source,
		Lines:  lines,
	}
}

func extractPlainLyrics(data map[string]any, fallbackSource string) LyricsResult {
	contents := asMap(data["contents"])
	sectionList := asMap(contents["sectionListRenderer"])
	for _, section := range mapsFromArray(sectionList["contents"]) {
		shelf := asMap(section["musicDescriptionShelfRenderer"])
		if shelf == nil {
			continue
		}
		description := asMap(shelf["description"])
		text := strings.TrimSpace(strings.Join(rawRunsText(description), ""))
		if text == "" {
			continue
		}
		source := strings.TrimSpace(strings.Join(rawRunsText(asMap(shelf["footer"])), ""))
		if source == "" {
			source = fallbackSource
		}
		return LyricsResult{
			Kind:   lyricsResultPlain,
			Source: source,
			Text:   text,
		}
	}
	return LyricsResult{Kind: lyricsResultUnavailable}
}

func (client *Client) searchLRCLibLyrics(ctx context.Context, info LyricsSearchInfo) LyricsResult {
	if strings.TrimSpace(info.Title) == "" {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	searchCtx, cancel := context.WithTimeout(ctx, lrcLibTimeout)
	defer cancel()

	values := url.Values{}
	if strings.TrimSpace(info.Artist) == "" {
		values.Set("q", info.Title)
	} else {
		values.Set("track_name", info.Title)
		values.Set("artist_name", info.Artist)
	}
	requestURL := lrcLibSearchURL + "?" + values.Encode()

	req, err := http.NewRequestWithContext(searchCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	req.Header.Set("User-Agent", "XiaDown/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClientForRequest().Do(req)
	if err != nil {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}

	var models []lrcLibModel
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&models); err != nil {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	model, ok := bestLRCLibModel(models, info.DurationSeconds)
	if !ok {
		return LyricsResult{Kind: lyricsResultUnavailable}
	}
	if strings.TrimSpace(model.SyncedLyrics) != "" {
		if lines := parseLRCLines(model.SyncedLyrics); len(lines) > 0 {
			return LyricsResult{
				Kind:   lyricsResultSynced,
				Source: "LRCLib",
				Lines:  lines,
			}
		}
	}
	if strings.TrimSpace(model.PlainLyrics) != "" {
		return LyricsResult{
			Kind:   lyricsResultPlain,
			Source: "LRCLib",
			Text:   strings.TrimSpace(model.PlainLyrics),
		}
	}
	return LyricsResult{Kind: lyricsResultUnavailable}
}

func bestLRCLibModel(models []lrcLibModel, targetDuration float64) (lrcLibModel, bool) {
	synced := make([]lrcLibModel, 0, len(models))
	plain := make([]lrcLibModel, 0, len(models))
	for _, model := range models {
		instrumental := model.Instrumental != nil && *model.Instrumental
		if instrumental {
			continue
		}
		if strings.TrimSpace(model.SyncedLyrics) != "" {
			synced = append(synced, model)
			continue
		}
		if strings.TrimSpace(model.PlainLyrics) != "" {
			plain = append(plain, model)
		}
	}
	if len(synced) > 0 {
		return closestLRCLibModel(synced, targetDuration), true
	}
	if len(plain) > 0 {
		return closestLRCLibModel(plain, targetDuration), true
	}
	return lrcLibModel{}, false
}

func closestLRCLibModel(models []lrcLibModel, targetDuration float64) lrcLibModel {
	if targetDuration <= 0 {
		return models[0]
	}
	best := models[0]
	bestDiff := math.MaxFloat64
	for _, model := range models {
		duration := 0.0
		if model.Duration != nil {
			duration = *model.Duration
		}
		diff := math.Abs(duration - targetDuration)
		if diff < bestDiff {
			best = model
			bestDiff = diff
		}
	}
	return best
}

func parseLRCLines(raw string) []LyricLine {
	rawLines := strings.Split(raw, "\n")
	offsetMs := 0
	lines := make([]LyricLine, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			continue
		}
		if match := lrcMetadataPattern.FindStringSubmatch(line); len(match) == 3 {
			if strings.EqualFold(match[1], "offset") {
				if offset, err := strconv.Atoi(strings.TrimSpace(match[2])); err == nil {
					offsetMs = offset
				}
			}
			continue
		}

		matches := lrcTimePattern.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		textOnly := strings.TrimSpace(lrcTimePattern.ReplaceAllString(line, ""))
		words := parseLRCWords(textOnly, offsetMs)
		if len(words) > 0 {
			textOnly = strings.TrimSpace(lrcWordPattern.ReplaceAllString(textOnly, "$4"))
		}
		for _, match := range matches {
			startMs, ok := parseLRCTimeMatch(line, match)
			if !ok {
				continue
			}
			lines = append(lines, LyricLine{
				StartMs: max(0, startMs-offsetMs),
				Text:    textOnly,
				Words:   words,
			})
		}
	}
	if len(lines) == 0 {
		return nil
	}
	sort.SliceStable(lines, func(left, right int) bool {
		return lines[left].StartMs < lines[right].StartMs
	})
	if lines[0].StartMs > 300 {
		lines = append([]LyricLine{{
			StartMs:    0,
			DurationMs: lines[0].StartMs,
			Text:       "",
		}}, lines...)
	}
	fillLyricLineDurations(lines)
	return lines
}

func parseLRCWords(text string, offsetMs int) []TimedWord {
	matches := lrcWordPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	words := make([]TimedWord, 0, len(matches))
	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		startMs, ok := parseLRCTimeParts(match[1], match[2], match[3])
		if !ok {
			continue
		}
		words = append(words, TimedWord{
			StartMs: max(0, startMs-offsetMs),
			Text:    strings.TrimSpace(match[4]),
		})
	}
	return words
}

func parseLRCTimeMatch(line string, match []int) (int, bool) {
	if len(match) < 8 {
		return 0, false
	}
	return parseLRCTimeParts(line[match[2]:match[3]], line[match[4]:match[5]], line[match[6]:match[7]])
}

func parseLRCTimeParts(minutesText string, secondsText string, fractionText string) (int, bool) {
	minutes, err := strconv.Atoi(minutesText)
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.Atoi(secondsText)
	if err != nil {
		return 0, false
	}
	fraction := fractionText
	for len(fraction) < 3 {
		fraction += "0"
	}
	if len(fraction) > 3 {
		fraction = fraction[:3]
	}
	millis, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, false
	}
	return minutes*60*1000 + seconds*1000 + millis, true
}

func fillLyricLineDurations(lines []LyricLine) {
	for index := range lines {
		if lines[index].DurationMs > 0 {
			continue
		}
		if index < len(lines)-1 {
			lines[index].DurationMs = max(0, lines[index+1].StartMs-lines[index].StartMs)
			continue
		}
		lines[index].DurationMs = 5000
	}
}

func rawRunsText(text map[string]any) []string {
	if text == nil {
		return nil
	}
	runs, ok := text["runs"].([]any)
	if !ok {
		value, _ := text["text"].(string)
		if value == "" {
			return nil
		}
		return []string{value}
	}
	result := make([]string, 0, len(runs))
	for _, run := range runs {
		value, _ := asMap(run)["text"].(string)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func findMapByKey(value any, key string) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if found := asMap(typed[key]); found != nil {
			return found
		}
		for _, item := range typed {
			if found := findMapByKey(item, key); found != nil {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := findMapByKey(item, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func parseFlexibleInt(value any) (int, bool) {
	switch typed := value.(type) {
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		return number, err == nil
	case json.Number:
		number, err := typed.Int64()
		return int(number), err == nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	default:
		return 0, false
	}
}

func durationLabelSeconds(label string) float64 {
	parts := strings.Split(strings.TrimSpace(label), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	total := 0
	for _, part := range parts {
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0
		}
		total = total*60 + number
	}
	return float64(total)
}
