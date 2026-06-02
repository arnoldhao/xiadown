package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/appsessions"
)

type listenErrorResponse struct {
	Error listenErrorBody `json:"error"`
}

type listenErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Source    string `json:"source,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

func writeListenError(w http.ResponseWriter, r *http.Request, status int, code string, message string, detail string, source string, retryable bool) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(listenErrorResponse{
		Error: listenErrorBody{
			Code:      strings.TrimSpace(code),
			Message:   strings.TrimSpace(message),
			Detail:    strings.TrimSpace(detail),
			Source:    strings.TrimSpace(source),
			Retryable: retryable,
		},
	})
}

func writeListenMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeListenError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.", "", "", false)
}

func writeListenBadRequest(w http.ResponseWriter, r *http.Request, code string, message string, detail string) {
	writeListenError(w, r, http.StatusBadRequest, code, message, detail, "", false)
}

func writeListenServiceUnavailable(w http.ResponseWriter, r *http.Request, code string, message string, detail string) {
	writeListenError(w, r, http.StatusServiceUnavailable, code, message, detail, "", true)
}

func writeListenYouTubeMusicUnavailable(w http.ResponseWriter, r *http.Request, err error, fallbackMessage string, source string) {
	writeListenError(
		w,
		r,
		listenYouTubeMusicErrorHTTPStatus(err),
		listenYouTubeMusicErrorCode(err),
		listenYouTubeMusicErrorMessage(err, fallbackMessage),
		strings.TrimSpace(errorString(err)),
		source,
		listenYouTubeMusicErrorRetryable(err),
	)
}

func listenYouTubeMusicErrorCode(err error) string {
	switch {
	case isListenMissingCookiesError(err):
		return "youtube_cookies_missing"
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "youtube_auth_expired"
	case isListenTimeoutError(err):
		return "youtube_timeout"
	case isListenNetworkError(err):
		return "youtube_network_unavailable"
	case isListenTransientStatusError(err):
		return "youtube_transient_unavailable"
	default:
		return "youtube_music_unavailable"
	}
}

func listenYouTubeMusicErrorHTTPStatus(err error) int {
	switch {
	case isListenMissingCookiesError(err), errors.Is(err, youtubemusic.ErrAuthExpired):
		return http.StatusUnauthorized
	case isListenTimeoutError(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusServiceUnavailable
	}
}

func listenYouTubeMusicErrorMessage(err error, fallback string) string {
	switch {
	case isListenMissingCookiesError(err):
		return "YouTube Music cookies are missing."
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "YouTube Music authentication expired."
	case isListenTimeoutError(err):
		return "YouTube Music request timed out."
	case isListenNetworkError(err):
		return "YouTube Music network unavailable."
	default:
		if trimmed := strings.TrimSpace(fallback); trimmed != "" {
			return trimmed
		}
		return "YouTube Music unavailable."
	}
}

func listenYouTubeMusicErrorRetryable(err error) bool {
	return isListenTimeoutError(err) || isListenNetworkError(err) || isListenTransientStatusError(err)
}

func listenYouTubeMusicErrorRetryableFromCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "youtube_timeout", "youtube_network_unavailable", "youtube_transient_unavailable":
		return true
	default:
		return false
	}
}

func isListenMissingCookiesError(err error) bool {
	return errors.Is(err, youtubemusic.ErrNotAuthenticated) ||
		errors.Is(err, appsessions.ErrNoCookies) ||
		errors.Is(err, appsessions.ErrSessionNotFound) ||
		errors.Is(err, appsessions.ErrInvalidSession)
}

func isListenTimeoutError(err error) bool {
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

func isListenNetworkError(err error) bool {
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

func isListenTransientStatusError(err error) bool {
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

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
