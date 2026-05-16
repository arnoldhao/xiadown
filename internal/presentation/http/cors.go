package http

import (
	"net/http"
	"strings"

	"xiadown/internal/infrastructure/localaccess"
)

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	if origin := localaccess.TrustedRequestOrigin(r); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	allowHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
	if allowHeaders == "" {
		allowHeaders = "Content-Type, Authorization, User-Agent, " + localaccess.TokenHeaderName
	}
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
}
