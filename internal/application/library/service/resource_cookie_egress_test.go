package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	appytdlp "xiadown/internal/application/ytdlp"
)

func TestPrepareCapturedCookieJarUsesExactSecureHostAndRemovesHeader(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	original := map[string]string{
		"cOoKiE":     `sid=COOKIE-CANARY; theme=dark; sid=less-specific`,
		"User-Agent": "CookieJarTest",
	}
	commandHeaders, path, cleanup, err := prepareCapturedCookieJar("https://Media.Example:8443/watch/index.m3u8", original, now)
	if err != nil {
		t.Fatalf("prepareCapturedCookieJar() error = %v", err)
	}
	defer cleanup()
	if path == "" {
		t.Fatal("expected a temporary cookie jar")
	}
	if strings.Contains(path, "COOKIE-CANARY") {
		t.Fatalf("temporary path leaked cookie value: %q", path)
	}
	if _, ok := cookieEgressTestHeader(commandHeaders, "Cookie"); ok {
		t.Fatalf("command headers retained Cookie: %#v", commandHeaders)
	}
	if commandHeaders["User-Agent"] != "CookieJarTest" {
		t.Fatalf("ordinary headers were not preserved: %#v", commandHeaders)
	}
	if original["cOoKiE"] == "" {
		t.Fatal("input map was mutated")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat temporary jar: %v", err)
	}
	assertPrivateResourceCookieJar(t, path, info.Mode())
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temporary jar: %v", err)
	}
	dataLines := netscapeCookieDataLines(string(payload))
	if len(dataLines) != 2 {
		t.Fatalf("cookie rows = %d, want 2; payload:\n%s", len(dataLines), payload)
	}
	wantExpiry := now.Add(resourceCookieJarLifetime).Unix()
	seen := map[string]string{}
	for _, line := range dataLines {
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			t.Fatalf("invalid Netscape row %q", line)
		}
		if fields[0] != "media.example" || fields[1] != "FALSE" || fields[2] != "/" || fields[3] != "TRUE" {
			t.Fatalf("unexpected cookie scope: %#v", fields[:4])
		}
		expires, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil || expires != wantExpiry {
			t.Fatalf("expiry = %q, want %d", fields[4], wantExpiry)
		}
		seen[fields[5]] = fields[6]
	}
	if seen["sid"] != "COOKIE-CANARY" || seen["theme"] != "dark" {
		t.Fatalf("unexpected cookie values: %#v", seen)
	}

	cleanup()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary jar survived cleanup: %v", err)
	}
}

func TestPrepareCapturedCookieJarRejectsInvalidCookieWithoutLeakingValue(t *testing.T) {
	t.Parallel()

	_, path, cleanup, err := prepareCapturedCookieJar("https://media.example/video.m3u8", map[string]string{
		"Cookie": "=INVALID-COOKIE-CANARY",
	}, time.Now())
	defer cleanup()
	if !errors.Is(err, errCapturedCookieHeaderInvalid) {
		t.Fatalf("error = %v, want %v", err, errCapturedCookieHeaderInvalid)
	}
	if path != "" {
		t.Fatalf("invalid cookie created jar %q", path)
	}
	if strings.Contains(err.Error(), "INVALID-COOKIE-CANARY") {
		t.Fatalf("error leaked cookie value: %v", err)
	}
}

func TestCapturedCookieJarReachesYTDLPWithoutCookieInArguments(t *testing.T) {
	t.Parallel()

	const canary = "COOKIE-MUST-NOT-REACH-ARGV"
	commandHeaders, path, cleanup, err := prepareCapturedCookieJar(
		"https://media.example/live/index.m3u8",
		map[string]string{"Cookie": "sid=" + canary, "Referer": "https://media.example/watch"},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("prepareCapturedCookieJar() error = %v", err)
	}
	defer cleanup()
	command, err := appytdlp.BuildCommand(context.Background(), appytdlp.CommandOptions{
		ExecPath: filepath.Join(t.TempDir(), "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL: "https://media.example/live/index.m3u8",
		},
		OutputTemplate: filepath.Join(t.TempDir(), "%(title)s.%(ext)s"),
		Headers:        commandHeaders,
		CookiesPath:    path,
	})
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}
	if got := argumentValue(command.Args, "--cookies"); got != path {
		t.Fatalf("--cookies = %q, want %q; args=%v", got, path, command.Args)
	}
	for name, args := range map[string][]string{"args": command.Args, "sanitized args": command.SanitizedArgs} {
		joined := strings.Join(args, "\n")
		if strings.Contains(joined, canary) || strings.Contains(joined, "Cookie:") {
			t.Fatalf("%s leaked Cookie: %v", name, args)
		}
	}
}

func TestPrepareCapturedCookieJarUsesStrictYTDLPHeaderWhitelist(t *testing.T) {
	t.Parallel()

	headers, path, cleanup, err := prepareCapturedCookieJar(
		"https://media.example/live/index.m3u8",
		map[string]string{
			"User-Agent":      "WhitelistTest",
			"Accept":          "video/*",
			"Accept-Language": "zh-CN",
			"Referer":         "https://Page.Example:443/watch/private?token=REFERER-CANARY",
			"Origin":          "https://Page.Example:443/private/path",
			"Priority":        "u=1",
			"If-Range":        "UNKNOWN-CANARY",
		},
		time.Now(),
	)
	defer cleanup()
	if err != nil {
		t.Fatalf("prepareCapturedCookieJar() error = %v", err)
	}
	if path != "" {
		t.Fatalf("headers without Cookie created jar %q", path)
	}
	want := map[string]string{
		"User-Agent":      "WhitelistTest",
		"Accept":          "video/*",
		"Accept-Language": "zh-CN",
		"Referer":         "https://page.example/",
		"Origin":          "https://page.example",
	}
	if len(headers) != len(want) {
		t.Fatalf("command headers = %#v, want %#v", headers, want)
	}
	for name, value := range want {
		if headers[name] != value {
			t.Fatalf("%s = %q, want %q; headers=%#v", name, headers[name], value, headers)
		}
	}
	for _, canary := range []string{"REFERER-CANARY", "UNKNOWN-CANARY"} {
		for _, value := range headers {
			if strings.Contains(value, canary) {
				t.Fatalf("strict whitelist leaked %q in %#v", canary, headers)
			}
		}
	}
}

func TestPrepareCapturedCookieJarRejectsHeaderAuthenticatedHLSWithoutSecret(t *testing.T) {
	t.Parallel()

	const canary = "X-SESSION-TOKEN-CANARY"
	_, path, cleanup, err := prepareCapturedCookieJar(
		"https://media.example/live/index.m3u8",
		map[string]string{"X-Session-Token": canary},
		time.Now(),
	)
	defer cleanup()
	if !errors.Is(err, errCapturedHeaderAuthUnsupported) {
		t.Fatalf("header-auth error = %v, want %v", err, errCapturedHeaderAuthUnsupported)
	}
	if path != "" {
		t.Fatalf("unsupported header auth created jar %q", path)
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("header-auth error leaked secret: %v", err)
	}
	if code := resolveYTDLPErrorCode("", err); code != ytdlpErrorCodeUnsupportedHeaderAuth {
		t.Fatalf("header-auth error code = %q, want %q", code, ytdlpErrorCodeUnsupportedHeaderAuth)
	}
}

func TestScopeCapturedRequestHeadersUsesInitialExactHost(t *testing.T) {
	t.Parallel()

	headers := map[string]string{
		"Cookie":              "sid=COOKIE-CANARY",
		"Authorization":       "Bearer AUTH-CANARY",
		"X-CSRF-Token":        "CSRF-CANARY",
		"X-Session-Token":     "SESSION-CANARY",
		"Proxy-Authorization": "Basic PROXY-CANARY",
		"User-Agent":          "ScopeTest",
	}
	initial := "https://media.example:8443/live/index.m3u8"
	tests := []struct {
		name       string
		target     string
		wantCookie bool
		wantOrigin bool
	}{
		{name: "same exact origin", target: "https://media.example:8443/segment.ts", wantCookie: true, wantOrigin: true},
		{name: "same host different port", target: "https://media.example/segment.ts", wantCookie: true, wantOrigin: false},
		{name: "subdomain", target: "https://cdn.media.example/segment.ts", wantCookie: false, wantOrigin: false},
		{name: "sibling host", target: "https://other.example/segment.ts", wantCookie: false, wantOrigin: false},
		{name: "HTTPS downgrade", target: "http://media.example:8443/segment.ts", wantCookie: false, wantOrigin: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := scopeCapturedRequestHeaders(headers, initial, test.target)
			_, hasCookie := cookieEgressTestHeader(got, "Cookie")
			_, hasAuthorization := cookieEgressTestHeader(got, "Authorization")
			_, hasCSRF := cookieEgressTestHeader(got, "X-CSRF-Token")
			_, hasSession := cookieEgressTestHeader(got, "X-Session-Token")
			if hasCookie != test.wantCookie || hasAuthorization != test.wantOrigin || hasCSRF != test.wantOrigin || hasSession != test.wantOrigin {
				t.Fatalf("scoped headers = %#v, want cookie=%v origin=%v", got, test.wantCookie, test.wantOrigin)
			}
			if _, hasProxy := cookieEgressTestHeader(got, "Proxy-Authorization"); hasProxy {
				t.Fatalf("proxy credentials reached origin: %#v", got)
			}
			if got["User-Agent"] != "ScopeTest" {
				t.Fatalf("ordinary header missing: %#v", got)
			}
		})
	}
}

func netscapeCookieDataLines(payload string) []string {
	var result []string
	for _, line := range strings.Split(payload, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result = append(result, line)
	}
	return result
}

func argumentValue(args []string, key string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key {
			return args[index+1]
		}
	}
	return ""
}

func cookieEgressTestHeader(headers map[string]string, key string) (string, bool) {
	for name, value := range headers {
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(key)) {
			return value, true
		}
	}
	return "", false
}
