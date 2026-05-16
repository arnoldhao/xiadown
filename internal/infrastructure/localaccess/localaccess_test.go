package localaccess

import (
	"net/http"
	"testing"
)

func TestValidTokenFromPathAndHeader(t *testing.T) {
	t.Parallel()

	token := "abc123"
	pathRequest, err := http.NewRequest(http.MethodGet, HTTPBasePath(token)+"/health", nil)
	if err != nil {
		t.Fatalf("build path request: %v", err)
	}
	if !ValidToken(pathRequest, token) {
		t.Fatal("expected path token to be valid")
	}

	headerRequest, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("build header request: %v", err)
	}
	headerRequest.Header.Set(TokenHeaderName, token)
	if !ValidToken(headerRequest, token) {
		t.Fatal("expected header token to be valid")
	}
}

func TestStripTokenPath(t *testing.T) {
	t.Parallel()

	stripped, ok := StripTokenPath(HTTPBasePath("abc123") + "/api/listen/image")
	if !ok {
		t.Fatal("expected token path to be stripped")
	}
	if stripped != "/api/listen/image" {
		t.Fatalf("unexpected stripped path: %q", stripped)
	}
}

func TestTrustedOriginAllowsOnlyDesktopAndLoopback(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{
		"wails://wails.localhost",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://[::1]:5173",
	} {
		if !TrustedOrigin(origin) {
			t.Fatalf("expected trusted origin %q", origin)
		}
	}
	for _, origin := range []string{
		"https://example.com",
		"file:///tmp/index.html",
		"chrome-extension://abc",
	} {
		if TrustedOrigin(origin) {
			t.Fatalf("expected untrusted origin %q", origin)
		}
	}
}
