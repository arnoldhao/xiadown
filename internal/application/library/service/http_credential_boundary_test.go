package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"xiadown/internal/application/networkpolicy"
	appytdlp "xiadown/internal/application/ytdlp"
)

func TestCredentialSafeRedirectSameOrigin302RetainsCookieAndAuth(t *testing.T) {
	t.Parallel()

	original, _ := http.NewRequest(http.MethodGet, "https://media.example/start", nil)
	redirected, _ := http.NewRequest(http.MethodGet, "https://media.example/final", nil)
	redirected.Header.Set("Cookie", "sid=COOKIE-CANARY")
	redirected.Header.Set("Authorization", "Bearer AUTH-CANARY")
	redirected.Header.Set("X-Session-Token", "SESSION-CANARY")
	redirected.Header.Set("Proxy-Authorization", "Basic PROXY-CANARY")

	if err := enforceCredentialSafeRedirect(redirected, []*http.Request{original}); err != nil {
		t.Fatalf("same-origin redirect error = %v", err)
	}
	if redirected.Header.Get("Cookie") != "sid=COOKIE-CANARY" ||
		redirected.Header.Get("Authorization") != "Bearer AUTH-CANARY" ||
		redirected.Header.Get("X-Session-Token") != "SESSION-CANARY" {
		t.Fatalf("same-origin redirect dropped scoped credentials: %#v", redirected.Header)
	}
	if redirected.Header.Get("Proxy-Authorization") != "" {
		t.Fatalf("redirect sent proxy credentials to origin: %#v", redirected.Header)
	}
}

func TestCredentialSafeRedirectCrossPortKeepsCookieButDropsOriginAuth(t *testing.T) {
	t.Parallel()

	original, _ := http.NewRequest(http.MethodGet, "https://media.example/start", nil)
	redirected, _ := http.NewRequest(http.MethodGet, "https://media.example:8443/final", nil)
	redirected.Header.Set("Cookie", "sid=COOKIE-CANARY")
	redirected.Header.Set("Authorization", "Bearer AUTH-CANARY")
	redirected.Header.Set("X-CSRF-Token", "CSRF-CANARY")
	redirected.Header.Set("X-Session-Token", "SESSION-CANARY")
	redirected.Header.Set("User-Agent", "RedirectTest")
	redirected.Header.Set("Referer", "https://page.example/watch?token=REFERER-CANARY")

	if err := enforceCredentialSafeRedirect(redirected, []*http.Request{original}); err != nil {
		t.Fatalf("cross-port redirect error = %v", err)
	}
	if redirected.Header.Get("Cookie") != "sid=COOKIE-CANARY" {
		t.Fatalf("same-host redirect dropped Cookie: %#v", redirected.Header)
	}
	for _, name := range []string{"Authorization", "X-CSRF-Token", "X-Session-Token"} {
		if redirected.Header.Get(name) != "" {
			t.Fatalf("cross-port redirect retained %s: %#v", name, redirected.Header)
		}
	}
	if redirected.Header.Get("User-Agent") != "RedirectTest" {
		t.Fatalf("public header was dropped: %#v", redirected.Header)
	}
	if redirected.Header.Get("Referer") != "https://page.example/" {
		t.Fatalf("cross-origin redirect retained a full Referer: %#v", redirected.Header)
	}
	if strings.Contains(redirected.Header.Get("Referer"), "REFERER-CANARY") {
		t.Fatalf("cross-origin redirect leaked Referer query: %#v", redirected.Header)
	}
}

func TestCredentialSafeRedirectRejectsHTTPSDowngrade(t *testing.T) {
	t.Parallel()

	original, _ := http.NewRequest(http.MethodGet, "https://media.example/start", nil)
	redirected, _ := http.NewRequest(http.MethodGet, "http://media.example/final", nil)
	redirected.Header.Set("Cookie", "sid=COOKIE-CANARY")
	redirected.Header.Set("Authorization", "Bearer AUTH-CANARY")

	err := enforceCredentialSafeRedirect(redirected, []*http.Request{original})
	if !errors.Is(err, errHTTPSRedirectDowngrade) {
		t.Fatalf("downgrade error = %v, want %v", err, errHTTPSRedirectDowngrade)
	}
	if redirected.Header.Get("Cookie") != "" || redirected.Header.Get("Authorization") != "" {
		t.Fatalf("downgraded request retained credentials: %#v", redirected.Header)
	}
}

func TestCredentialSafeRedirectRestoresTenHopLimit(t *testing.T) {
	t.Parallel()

	via := make([]*http.Request, resourceRedirectLimit)
	for index := range via {
		via[index], _ = http.NewRequest(http.MethodGet, "https://media.example/loop", nil)
	}
	redirected, _ := http.NewRequest(http.MethodGet, "https://media.example/loop", nil)
	if err := enforceCredentialSafeRedirect(redirected, via); !errors.Is(err, errTooManyRedirects) {
		t.Fatalf("redirect loop error = %v, want %v", err, errTooManyRedirects)
	}
}

func TestCredentialSafeRedirectRejectsNonHTTPAndUserinfo(t *testing.T) {
	t.Parallel()

	original, _ := http.NewRequest(http.MethodGet, "https://media.example/start", nil)
	for _, rawURL := range []string{"file:///etc/passwd", "https://user:pass@media.example/final"} {
		redirected, _ := http.NewRequest(http.MethodGet, rawURL, nil)
		if err := enforceCredentialSafeRedirect(redirected, []*http.Request{original}); err == nil ||
			(!errors.Is(err, networkpolicy.ErrDestinationBlocked) && !errors.Is(err, appytdlp.ErrUnsupportedNetworkURL)) {
			t.Fatalf("redirect %q error = %v, want URL rejection", rawURL, err)
		}
	}
}

func TestResourceHTTPClientInstallsCredentialSafeRedirectPolicy(t *testing.T) {
	t.Parallel()

	client, err := newResourceHTTPClient("")
	if err != nil {
		t.Fatalf("newResourceHTTPClient() error = %v", err)
	}
	if client.CheckRedirect == nil {
		t.Fatal("resource HTTP client has no redirect policy")
	}
}

func TestResourceRequestNeverSendsProxyAuthorizationToOrigin(t *testing.T) {
	t.Parallel()

	request, _ := http.NewRequest(http.MethodGet, "https://media.example/video.mp4", nil)
	applyResourceRequestHeaders(request, map[string]string{
		"Proxy-Authorization": "Basic PROXY-CANARY",
		"Authorization":       "Bearer ORIGIN-CANARY",
	})
	if request.Header.Get("Proxy-Authorization") != "" {
		t.Fatalf("proxy credentials reached origin request: %#v", request.Header)
	}
	if request.Header.Get("Authorization") != "Bearer ORIGIN-CANARY" {
		t.Fatalf("origin authorization was unexpectedly dropped: %#v", request.Header)
	}
}
