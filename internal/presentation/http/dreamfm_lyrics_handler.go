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
	"xiadown/internal/domain/connectors"
)

const dreamFMLyricsTimeout = 25 * time.Second

type dreamFMYouTubeMusicLyricsClient interface {
	TrackLyrics(ctx context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error)
}

type DreamFMLyricsHandler struct {
	ytMusic dreamFMYouTubeMusicLyricsClient
}

type DreamFMLyricsResponse struct {
	VideoID string                   `json:"videoId"`
	Kind    string                   `json:"kind"`
	Source  string                   `json:"source,omitempty"`
	Text    string                   `json:"text,omitempty"`
	Lines   []youtubemusic.LyricLine `json:"lines,omitempty"`
}

func NewDreamFMLyricsHandler(ytMusic dreamFMYouTubeMusicLyricsClient) *DreamFMLyricsHandler {
	return &DreamFMLyricsHandler{ytMusic: ytMusic}
}

func (handler *DreamFMLyricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	if handler.ytMusic == nil {
		writeDreamFMLyricsError(
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
	if videoID != "" && !youtubeVideoIDPattern.MatchString(videoID) {
		writeDreamFMLyricsError(
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
		writeDreamFMLyricsError(
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

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMLyricsTimeout)
	defer cancel()

	durationSeconds, _ := strconv.ParseFloat(strings.TrimSpace(r.URL.Query().Get("duration")), 64)
	result, err := handler.ytMusic.TrackLyrics(ctx, youtubemusic.LyricsSearchInfo{
		VideoID:         videoID,
		Title:           title,
		Artist:          artist,
		DurationSeconds: durationSeconds,
	})
	if err != nil {
		writeDreamFMLyricsError(
			w,
			r,
			dreamFMLyricsErrorHTTPStatus(err),
			dreamFMLyricsErrorCode(err),
			dreamFMLyricsErrorMessage(err),
			strings.TrimSpace(err.Error()),
			dreamFMLyricsErrorRetryable(err),
		)
		return
	}

	writeDreamFMLyricsJSON(w, r, DreamFMLyricsResponse{
		VideoID: dreamFMLyricsResponseID(videoID, r.URL.Query().Get("key"), title, artist),
		Kind:    strings.TrimSpace(result.Kind),
		Source:  strings.TrimSpace(result.Source),
		Text:    result.Text,
		Lines:   result.Lines,
	})
}

func dreamFMLyricsResponseID(videoID string, key string, title string, artist string) string {
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

func dreamFMLyricsErrorCode(err error) string {
	switch {
	case isDreamFMLyricsMissingCookiesError(err):
		return "youtube_cookies_missing"
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "youtube_auth_expired"
	case isDreamFMLyricsTimeoutError(err):
		return "youtube_timeout"
	case isDreamFMLyricsNetworkError(err):
		return "youtube_network_unavailable"
	default:
		return "lyrics_unavailable"
	}
}

func dreamFMLyricsErrorHTTPStatus(err error) int {
	switch {
	case isDreamFMLyricsMissingCookiesError(err), errors.Is(err, youtubemusic.ErrAuthExpired):
		return http.StatusUnauthorized
	case isDreamFMLyricsTimeoutError(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusServiceUnavailable
	}
}

func dreamFMLyricsErrorMessage(err error) string {
	switch {
	case isDreamFMLyricsMissingCookiesError(err):
		return "YouTube Music cookies are missing."
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "YouTube Music authentication expired."
	case isDreamFMLyricsTimeoutError(err):
		return "YouTube Music lyrics request timed out."
	case isDreamFMLyricsNetworkError(err):
		return "YouTube Music lyrics network unavailable."
	default:
		return "YouTube Music lyrics unavailable."
	}
}

func dreamFMLyricsErrorRetryable(err error) bool {
	if err == nil {
		return false
	}
	return isDreamFMLyricsTimeoutError(err) ||
		isDreamFMLyricsNetworkError(err) ||
		isDreamFMLyricsTransientStatusError(err)
}

func isDreamFMLyricsMissingCookiesError(err error) bool {
	return errors.Is(err, youtubemusic.ErrNotAuthenticated) ||
		errors.Is(err, connectors.ErrNoCookies) ||
		errors.Is(err, connectors.ErrConnectorNotFound)
}

func isDreamFMLyricsTimeoutError(err error) bool {
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

func isDreamFMLyricsNetworkError(err error) bool {
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

func isDreamFMLyricsTransientStatusError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "youtube music api status 429") ||
		strings.Contains(lower, "youtube music api status 500") ||
		strings.Contains(lower, "youtube music api status 502") ||
		strings.Contains(lower, "youtube music api status 503") ||
		strings.Contains(lower, "youtube music api status 504")
}

func writeDreamFMLyricsError(w http.ResponseWriter, r *http.Request, status int, code string, message string, detail string, retryable bool) {
	writeDreamFMLyricsJSONStatus(w, r, status, dreamFMErrorResponse{
		Error: dreamFMErrorBody{
			Code:      code,
			Message:   message,
			Detail:    detail,
			Retryable: retryable,
		},
	})
}

func writeDreamFMLyricsJSON(w http.ResponseWriter, r *http.Request, response any) {
	writeDreamFMLyricsJSONStatus(w, r, http.StatusOK, response)
}

func writeDreamFMLyricsJSONStatus(w http.ResponseWriter, r *http.Request, status int, response any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
