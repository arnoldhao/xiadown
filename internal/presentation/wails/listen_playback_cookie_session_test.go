package wails

import (
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

func TestPlaybackCookieRestoreSeedsOnlyStableAuth(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	persisted := []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "SID", Value: "stable-sid", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-1PSIDTS", Value: "old-ts", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSIDCC", Value: "old-cc", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}

	plan := planListenPlaybackCookieRestore(
		persisted,
		nil,
		now,
		true,
	)
	if len(plan) != 2 {
		t.Fatalf("seed should contain only stable auth, got %#v", plan)
	}
	for _, record := range plan {
		if record.Name == "__Secure-1PSIDTS" || record.Name == "__Secure-3PSIDCC" {
			t.Fatalf("rotating cookie entered bootstrap: %#v", record)
		}
	}
}

func TestPlaybackCookieRestoreNeverOverwritesExistingLiveStore(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	persisted := []appcookies.Record{
		{Name: "SAPISID", Value: "persisted-a", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-1PSIDTS", Value: "old-a", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}
	current := []appcookies.Record{
		{Name: "SAPISID", Value: "live", Domain: ".youtube.com", Path: "/", Expires: now.Add(2 * time.Hour).Unix()},
		{Name: "__Secure-1PSIDTS", Value: "rotated-b", Domain: ".youtube.com", Path: "/", Expires: now.Add(2 * time.Hour).Unix()},
	}

	for _, targetURL := range []string{
		"https://www.youtube.com/watch?v=regular",
		"https://music.youtube.com/watch?v=music",
	} {
		plan := planListenPlaybackCookieRestore(persisted, current, now, true)
		if len(plan) != 0 {
			t.Fatalf("target %s would overwrite live B with persisted A: %#v", targetURL, plan)
		}
	}
}

func TestPlaybackCookieRestoreFillsMissingCrossDomainStableAuth(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	persisted := []appcookies.Record{
		{Name: "SAPISID", Value: "persisted-google", Domain: ".google.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PAPISID", Value: "persisted-youtube", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}
	current := []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "newer-live-youtube", Domain: ".youtube.com", Path: "/", Expires: now.Add(2 * time.Hour).Unix()},
		{Name: "__Secure-ROLLOUT_TOKEN", Value: "runtime", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}

	plan := planListenPlaybackCookieRestore(
		persisted,
		current,
		now,
		true,
	)
	if len(plan) != 1 ||
		plan[0].Name != "SAPISID" ||
		plan[0].Domain != ".google.com" ||
		plan[0].Value != "persisted-google" {
		t.Fatalf("cross-domain restore plan = %#v, want only missing Google auth", plan)
	}
}

func TestPlaybackCookieRestoreFailsClosedWhenStoreCannotBeRead(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	plan := planListenPlaybackCookieRestore(
		[]appcookies.Record{{Name: "SAPISID", Value: "persisted", Domain: ".youtube.com", Path: "/"}},
		nil,
		now,
		false,
	)
	if len(plan) != 0 {
		t.Fatalf("unreadable live store must never be overwritten: %#v", plan)
	}
}
