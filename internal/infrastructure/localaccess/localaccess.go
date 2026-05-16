package localaccess

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	TokenHeaderName = "X-XiaDown-Access-Token"
	TokenQueryName  = "access_token"
	TokenPathPrefix = "/_xiadown"
)

func NewToken() string {
	var payload [32]byte
	if _, err := rand.Read(payload[:]); err != nil {
		panic("generate local access token: " + err.Error())
	}
	return hex.EncodeToString(payload[:])
}

func HTTPBasePath(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	return TokenPathPrefix + "/" + url.PathEscape(trimmed)
}

func WebSocketPath(token string) string {
	base := HTTPBasePath(token)
	if base == "" {
		return "/ws"
	}
	return base + "/ws"
}

func RequestToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := strings.TrimSpace(r.Header.Get(TokenHeaderName)); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.URL.Query().Get(TokenQueryName)); token != "" {
		return token
	}
	token, _ := TokenFromPath(r.URL.Path)
	return token
}

func TokenFromPath(path string) (string, bool) {
	value := strings.TrimSpace(path)
	if value == "" {
		return "", false
	}
	if value == TokenPathPrefix {
		return "", false
	}
	prefix := TokenPathPrefix + "/"
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	remainder := strings.TrimPrefix(value, prefix)
	index := strings.IndexByte(remainder, '/')
	if index < 0 {
		if token, err := url.PathUnescape(remainder); err == nil {
			return strings.TrimSpace(token), strings.TrimSpace(token) != ""
		}
		return "", false
	}
	token, err := url.PathUnescape(remainder[:index])
	if err != nil {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func StripTokenPath(path string) (string, bool) {
	value := strings.TrimSpace(path)
	prefix := TokenPathPrefix + "/"
	if !strings.HasPrefix(value, prefix) {
		return path, false
	}
	remainder := strings.TrimPrefix(value, prefix)
	index := strings.IndexByte(remainder, '/')
	if index < 0 {
		return "/", true
	}
	stripped := remainder[index:]
	if stripped == "" {
		return "/", true
	}
	return stripped, true
}

func ValidToken(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	actual := strings.TrimSpace(RequestToken(r))
	if actual == "" || len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func TrustedOrigin(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return false
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if scheme == "wails" {
		return true
	}
	if host == "wails.localhost" {
		return true
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func TrustedRequestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if TrustedOrigin(origin) {
		return origin
	}
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return ""
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	refererOrigin := parsed.Scheme + "://" + parsed.Host
	if TrustedOrigin(refererOrigin) {
		return refererOrigin
	}
	return ""
}
