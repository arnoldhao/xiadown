package service

import (
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

func TestPersistentAppSessionCookiesKeepsOnlyYouTubeStableAuth(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	persisted := persistentAppSessionCookies("youtube", []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "SID", Value: "stable-sid", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "LOGIN_INFO", Value: "runtime", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSIDCC", Value: "rotating-cc", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSIDTS", Value: "rotating-ts", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}, now)
	if len(persisted) != 2 {
		t.Fatalf("persistent YouTube cookies = %#v, want stable-only", persisted)
	}
	for _, record := range persisted {
		if record.Name != "SAPISID" && record.Name != "SID" {
			t.Fatalf("runtime cookie entered App Session persistence: %#v", record)
		}
	}
}
