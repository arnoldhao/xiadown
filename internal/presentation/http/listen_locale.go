package http

import (
	"context"
	"net/http"
	"strings"

	"xiadown/internal/application/youtubemusic"
)

func listenRequestContextWithLocale(parent context.Context, r *http.Request) context.Context {
	return youtubemusic.WithLocale(parent, listenRequestLocale(r))
}

func listenRequestLocale(r *http.Request) string {
	if r == nil {
		return ""
	}
	for _, key := range []string{"language", "locale"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return strings.TrimSpace(r.Header.Get("Accept-Language"))
}
