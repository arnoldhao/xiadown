package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/appsessions"
)

const listenLyricsTimeout = 35 * time.Second

type listenYouTubeMusicLyricsClient interface {
	TrackLyrics(ctx context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error)
}

type listenYouTubeMusicLyricsCandidateClient interface {
	SearchLyricsCandidates(ctx context.Context, info youtubemusic.LyricsSearchInfo) ([]youtubemusic.LyricsCandidate, error)
	TrackLyricsCandidate(ctx context.Context, providerID string, providerTrackID string, plainOnly bool) (youtubemusic.LyricsResult, error)
}

type ListenLyricsHandler struct {
	ytMusic listenYouTubeMusicLyricsClient
}

type ListenLyricsResponse struct {
	VideoID         string                   `json:"videoId"`
	Kind            string                   `json:"kind"`
	Source          string                   `json:"source,omitempty"`
	ProviderID      string                   `json:"providerId,omitempty"`
	ProviderTrackID string                   `json:"providerTrackId,omitempty"`
	Attribution     string                   `json:"attribution,omitempty"`
	TimingQuality   string                   `json:"timingQuality,omitempty"`
	Confidence      int                      `json:"confidence,omitempty"`
	Text            string                   `json:"text,omitempty"`
	Lines           []youtubemusic.LyricLine `json:"lines,omitempty"`
}

func NewListenLyricsHandler(ytMusic listenYouTubeMusicLyricsClient) *ListenLyricsHandler {
	return &ListenLyricsHandler{ytMusic: ytMusic}
}

func (handler *ListenLyricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeListenMethodNotAllowed(w, r)
		return
	}
	setCORSHeaders(w, r)
	path := strings.TrimSuffix(r.URL.Path, "/")
	if strings.HasSuffix(path, "/candidates") {
		handler.serveCandidates(w, r)
		return
	}
	if strings.HasSuffix(path, "/candidate") {
		handler.serveCandidate(w, r)
		return
	}

	if handler.ytMusic == nil {
		writeListenLyricsError(
			w,
			r,
			http.StatusServiceUnavailable,
			"youtube_music_client_unavailable",
			"YouTube Music client unavailable.",
			"",
			false,
		)
		return
	}

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	title := strings.TrimSpace(r.URL.Query().Get("title"))
	artist := strings.TrimSpace(r.URL.Query().Get("artist"))
	album := strings.TrimSpace(r.URL.Query().Get("album"))
	if videoID != "" && !youtubeVideoIDPattern.MatchString(videoID) {
		writeListenLyricsError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_video_id",
			"Invalid YouTube video id.",
			"id: "+videoID,
			false,
		)
		return
	}
	if videoID == "" && title == "" {
		writeListenLyricsError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_lyrics_query",
			"Track title or YouTube video id is required.",
			"",
			false,
		)
		return
	}

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenLyricsTimeout)
	defer cancel()

	durationSeconds, _ := strconv.ParseFloat(strings.TrimSpace(r.URL.Query().Get("duration")), 64)
	plainOnly := listenLyricsPlainOnly(r)
	result, err := handler.ytMusic.TrackLyrics(ctx, youtubemusic.LyricsSearchInfo{
		VideoID:         videoID,
		Title:           title,
		Artist:          artist,
		Album:           album,
		DurationSeconds: durationSeconds,
		PlainOnly:       plainOnly,
	})
	if err != nil {
		writeListenLyricsError(
			w,
			r,
			listenLyricsErrorHTTPStatus(err),
			listenLyricsErrorCode(err),
			listenLyricsErrorMessage(err),
			strings.TrimSpace(err.Error()),
			listenLyricsErrorRetryable(err),
		)
		return
	}

	writeListenLyricsJSON(w, r, listenLyricsResponseFromResult(
		listenLyricsResponseID(videoID, r.URL.Query().Get("key"), title, artist),
		result,
	))
}

func (handler *ListenLyricsHandler) serveCandidates(w http.ResponseWriter, r *http.Request) {
	client, ok := handler.ytMusic.(listenYouTubeMusicLyricsCandidateClient)
	if !ok {
		writeListenLyricsError(w, r, http.StatusServiceUnavailable, "lyrics_candidate_search_unavailable", "Lyrics candidate search unavailable.", "", true)
		return
	}
	info, ok := listenLyricsSearchInfoFromRequest(r)
	if !ok {
		writeListenLyricsError(w, r, http.StatusBadRequest, "invalid_lyrics_query", "Track title is required.", "", false)
		return
	}
	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenLyricsTimeout)
	defer cancel()
	candidates, err := client.SearchLyricsCandidates(ctx, info)
	if err != nil {
		writeListenLyricsError(w, r, listenLyricsErrorHTTPStatus(err), listenLyricsErrorCode(err), listenLyricsErrorMessage(err), strings.TrimSpace(err.Error()), listenLyricsErrorRetryable(err))
		return
	}
	writeListenLyricsJSON(w, r, candidates)
}

func (handler *ListenLyricsHandler) serveCandidate(w http.ResponseWriter, r *http.Request) {
	client, ok := handler.ytMusic.(listenYouTubeMusicLyricsCandidateClient)
	if !ok {
		writeListenLyricsError(w, r, http.StatusServiceUnavailable, "lyrics_candidate_preview_unavailable", "Lyrics candidate preview unavailable.", "", true)
		return
	}
	providerID := strings.TrimSpace(r.URL.Query().Get("provider"))
	providerTrackID := strings.TrimSpace(r.URL.Query().Get("providerTrackId"))
	if providerID == "" || providerTrackID == "" {
		writeListenLyricsError(w, r, http.StatusBadRequest, "invalid_lyrics_candidate", "Lyrics candidate identity is required.", "", false)
		return
	}
	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenLyricsTimeout)
	defer cancel()
	result, err := client.TrackLyricsCandidate(ctx, providerID, providerTrackID, listenLyricsPlainOnly(r))
	if err != nil {
		writeListenLyricsError(w, r, listenLyricsErrorHTTPStatus(err), listenLyricsErrorCode(err), listenLyricsErrorMessage(err), strings.TrimSpace(err.Error()), listenLyricsErrorRetryable(err))
		return
	}
	writeListenLyricsJSON(w, r, listenLyricsResponseFromResult(
		listenLyricsResponseID(r.URL.Query().Get("id"), r.URL.Query().Get("key"), r.URL.Query().Get("title"), r.URL.Query().Get("artist")),
		result,
	))
}

func listenLyricsSearchInfoFromRequest(r *http.Request) (youtubemusic.LyricsSearchInfo, bool) {
	durationSeconds, _ := strconv.ParseFloat(strings.TrimSpace(r.URL.Query().Get("duration")), 64)
	info := youtubemusic.LyricsSearchInfo{
		VideoID:         strings.TrimSpace(r.URL.Query().Get("id")),
		Title:           strings.TrimSpace(r.URL.Query().Get("title")),
		Artist:          strings.TrimSpace(r.URL.Query().Get("artist")),
		Album:           strings.TrimSpace(r.URL.Query().Get("album")),
		DurationSeconds: durationSeconds,
		PlainOnly:       listenLyricsPlainOnly(r),
	}
	return info, info.Title != ""
}

func listenLyricsResponseFromResult(videoID string, result youtubemusic.LyricsResult) ListenLyricsResponse {
	return ListenLyricsResponse{
		VideoID:         strings.TrimSpace(videoID),
		Kind:            strings.TrimSpace(result.Kind),
		Source:          strings.TrimSpace(result.Source),
		ProviderID:      strings.TrimSpace(result.ProviderID),
		ProviderTrackID: strings.TrimSpace(result.ProviderTrackID),
		Attribution:     strings.TrimSpace(result.Attribution),
		TimingQuality:   strings.TrimSpace(result.TimingQuality),
		Confidence:      result.Confidence,
		Text:            result.Text,
		Lines:           result.Lines,
	}
}

func listenLyricsPlainOnly(r *http.Request) bool {
	query := r.URL.Query()
	for _, key := range []string{"synced", "syncedLyrics"} {
		raw := strings.TrimSpace(query.Get(key))
		if raw == "" {
			continue
		}
		if parsed, err := strconv.ParseBool(raw); err == nil {
			return !parsed
		}
		return strings.EqualFold(raw, "plain") || strings.EqualFold(raw, "plain-only")
	}
	rawPlainOnly := strings.TrimSpace(query.Get("plainOnly"))
	if rawPlainOnly != "" {
		parsed, err := strconv.ParseBool(rawPlainOnly)
		return err == nil && parsed
	}
	mode := strings.TrimSpace(query.Get("mode"))
	return strings.EqualFold(mode, "plain") || strings.EqualFold(mode, "plain-only")
}

func listenLyricsResponseID(videoID string, key string, title string, artist string) string {
	for _, value := range []string{videoID, key, strings.TrimSpace(title + " " + artist), title} {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		runes := []rune(trimmed)
		if len(runes) > 180 {
			return string(runes[:180])
		}
		return trimmed
	}
	return ""
}

func listenLyricsErrorCode(err error) string {
	switch {
	case isListenLyricsMissingCookiesError(err):
		return "youtube_cookies_missing"
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "youtube_auth_expired"
	case isListenLyricsTimeoutError(err):
		return "youtube_timeout"
	case isListenLyricsNetworkError(err):
		return "youtube_network_unavailable"
	case isListenLyricsTransientStatusError(err):
		return "lyrics_provider_transient"
	default:
		return "lyrics_unavailable"
	}
}

func listenLyricsErrorHTTPStatus(err error) int {
	switch {
	case isListenLyricsMissingCookiesError(err), errors.Is(err, youtubemusic.ErrAuthExpired):
		return http.StatusUnauthorized
	case isListenLyricsTimeoutError(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusServiceUnavailable
	}
}

func listenLyricsErrorMessage(err error) string {
	switch {
	case isListenLyricsMissingCookiesError(err):
		return "YouTube Music cookies are missing."
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "YouTube Music authentication expired."
	case isListenLyricsTimeoutError(err):
		return "YouTube Music lyrics request timed out."
	case isListenLyricsNetworkError(err):
		return "YouTube Music lyrics network unavailable."
	case isListenLyricsTransientStatusError(err):
		return "Lyrics provider is temporarily unavailable."
	default:
		return "YouTube Music lyrics unavailable."
	}
}

func listenLyricsErrorRetryable(err error) bool {
	if err == nil {
		return false
	}
	return isListenLyricsTimeoutError(err) ||
		isListenLyricsNetworkError(err) ||
		isListenLyricsTransientStatusError(err)
}

func isListenLyricsMissingCookiesError(err error) bool {
	return errors.Is(err, youtubemusic.ErrNotAuthenticated) ||
		errors.Is(err, appsessions.ErrNoCookies) ||
		errors.Is(err, appsessions.ErrSessionNotFound) ||
		errors.Is(err, appsessions.ErrInvalidSession)
}

func isListenLyricsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, youtubemusic.ErrRequestTimedOut) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "tls handshake timeout")
}

func isListenLyricsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, youtubemusic.ErrNetworkUnavailable) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	if lower == "eof" || strings.Contains(lower, ": eof") || strings.Contains(lower, " eof") {
		return true
	}
	for _, marker := range []string{
		"no such host",
		"network is unreachable",
		"connection refused",
		"connection reset",
		"connection closed",
		"temporary failure",
		"dial tcp",
		"unexpected eof",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isListenLyricsTransientStatusError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, status := range []string{"status 429", "status 500", "status 502", "status 503", "status 504"} {
		if strings.Contains(lower, status) {
			return true
		}
	}
	return strings.Contains(lower, "lrclib response invalid")
}

func writeListenLyricsError(w http.ResponseWriter, r *http.Request, status int, code string, message string, detail string, retryable bool) {
	writeListenLyricsJSONStatus(w, r, status, listenErrorResponse{
		Error: listenErrorBody{
			Code:      code,
			Message:   message,
			Detail:    detail,
			Retryable: retryable,
		},
	})
}

func writeListenLyricsJSON(w http.ResponseWriter, r *http.Request, response any) {
	writeListenLyricsJSONStatus(w, r, http.StatusOK, response)
}

func writeListenLyricsJSONStatus(w http.ResponseWriter, r *http.Request, status int, response any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
