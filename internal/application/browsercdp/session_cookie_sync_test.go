package browsercdp

import (
	"errors"
	"testing"

	appcookies "xiadown/internal/application/cookies"
)

const (
	testCookiePrimaryURL   = "https://www.example.test/"
	testCookieProfileURL   = "https://space.example.test/profile/demo"
	testCookiePrimaryHost  = "www.example.test"
	testCookieProfileHost  = "space.example.test"
	testCookieSharedDomain = ".example.test"
)

func TestCookieSyncKeys_IncludeCookieScopeDomains(t *testing.T) {
	t.Parallel()

	keys := cookieSyncKeys(testCookieProfileURL, []appcookies.Record{
		{Name: "session_token", Domain: testCookieSharedDomain, Path: "/"},
		{Name: "site_session", Domain: testCookiePrimaryHost, Path: "/"},
	})

	expected := map[string]struct{}{
		testCookieProfileHost: {},
		"example.test":        {},
		testCookiePrimaryHost: {},
	}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %#v", len(expected), len(keys), keys)
	}
	for _, key := range keys {
		if _, ok := expected[key]; !ok {
			t.Fatalf("unexpected key %q in %#v", key, keys)
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		t.Fatalf("missing keys: %#v", expected)
	}
}

func TestCookieSyncFingerprint_ReusesBroadCookieDomainAcrossSubdomains(t *testing.T) {
	t.Parallel()

	fingerprint := "same-cookie-set"
	values := map[string]string{}
	initialKeys := cookieSyncKeys(testCookiePrimaryURL, []appcookies.Record{
		{Name: "session_token", Domain: testCookieSharedDomain, Path: "/"},
	})
	rememberCookieSyncFingerprint(values, initialKeys, fingerprint)

	nextKeys := cookieSyncKeys(testCookieProfileURL, []appcookies.Record{
		{Name: "session_token", Domain: testCookieSharedDomain, Path: "/"},
	})
	if !hasCookieSyncFingerprint(values, nextKeys, fingerprint) {
		t.Fatalf("expected sync fingerprint to be reused across subdomains: values=%#v keys=%#v", values, nextKeys)
	}
}

func TestIsRecoverableCookieSyncError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "timeout", err: errors.New("connector cookie sync failed: context deadline exceeded"), want: true},
		{name: "destroyed", err: errors.New("execution context was destroyed"), want: true},
		{name: "closed", err: errors.New("target closed"), want: true},
		{name: "validation", err: errors.New("invalid cookie domain"), want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isRecoverableCookieSyncError(tc.err); got != tc.want {
				t.Fatalf("expected %v, got %v for %v", tc.want, got, tc.err)
			}
		})
	}
}

func TestPreferredPageURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		candidates []string
		want       string
	}{
		{
			name:       "preserve known non placeholder over about blank",
			candidates: []string{"about:blank", testCookiePrimaryURL},
			want:       testCookiePrimaryURL,
		},
		{
			name:       "prefer observed real url",
			candidates: []string{testCookieProfileURL, testCookiePrimaryURL},
			want:       testCookieProfileURL,
		},
		{
			name:       "fallback to requested real url",
			candidates: []string{"about:blank", "", "https://www.example.test"},
			want:       "https://www.example.test",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := preferredPageURL(tc.candidates...); got != tc.want {
				t.Fatalf("expected %q, got %q for %#v", tc.want, got, tc.candidates)
			}
		})
	}
}
