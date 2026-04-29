package http

import (
	"net/http"
	"strings"
)

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	allowHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
	if allowHeaders == "" {
		allowHeaders = "Content-Type, Authorization, User-Agent"
	}
	w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
}
