package youtubecookies

import (
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

func TestStableAuthExcludesRotatingSecurityCookies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	records := []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PAPISID", Value: "stable-secure", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "SIDCC", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-1PSIDCC", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSIDCC", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-1PSIDTS", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSIDTS", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}

	stable := StableAuth(records, now)
	if len(stable) != 2 {
		t.Fatalf("expected only stable auth cookies, got %#v", stable)
	}
	for _, record := range stable {
		if !isStableAuthName(record.Name) {
			t.Fatalf("rotating cookie survived persistence filter: %#v", record)
		}
	}
}

func TestHasAuthForURLHonoursDomainAndExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	records := []appcookies.Record{
		{Name: "SAPISID", Value: "expired", Domain: ".youtube.com", Path: "/", Expires: now.Add(-time.Second).Unix()},
		{Name: "__Secure-3PAPISID", Value: "active", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}
	if !HasAuthForURL(records, "https://music.youtube.com/watch", now) {
		t.Fatal("expected active parent-domain auth cookie to match YouTube Music")
	}
	if HasAuthForURL(records, "https://accounts.google.com/", now) {
		t.Fatal("youtube.com cookie must not match accounts.google.com")
	}
}
