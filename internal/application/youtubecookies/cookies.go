package youtubecookies

import (
	"sort"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

// stableAuthNames mirrors the small credential set persisted by Kaset. The
// rotating CC/TS cookies deliberately do not belong here: replaying an older
// generation over a live WebKit store can make Google distrust the session.
var stableAuthNames = map[string]struct{}{
	"APISID":            {},
	"HSID":              {},
	"SAPISID":           {},
	"SID":               {},
	"SSID":              {},
	"__Secure-1PAPISID": {},
	"__Secure-3PAPISID": {},
}

var authIndicatorNames = map[string]struct{}{
	"SAPISID":           {},
	"__Secure-1PAPISID": {},
	"__Secure-3PAPISID": {},
}

func isStableAuthName(name string) bool {
	_, ok := stableAuthNames[strings.TrimSpace(name)]
	return ok
}

// Runtime returns a deterministic, owned snapshot of active YouTube/Google
// cookies. It is suitable for the in-memory request cache, but not persistence.
func Runtime(records []appcookies.Record, now time.Time) []appcookies.Record {
	if len(records) == 0 {
		return nil
	}
	result := make([]appcookies.Record, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		record.Name = strings.TrimSpace(record.Name)
		record.Domain = strings.ToLower(strings.TrimSpace(record.Domain))
		record.Path = strings.TrimSpace(record.Path)
		record.SameSite = strings.ToLower(strings.TrimSpace(record.SameSite))
		if record.Path == "" {
			record.Path = "/"
		}
		if record.Name == "" || record.Value == "" || !isYouTubeOrGoogleDomain(record.Domain) {
			continue
		}
		if record.Expires > 0 && !time.Unix(record.Expires, 0).After(now) {
			continue
		}
		key := strings.ToLower(record.Name) + "\x00" + record.Domain + "\x00" + record.Path
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, record)
	}
	sort.Slice(result, func(left int, right int) bool {
		if result[left].Domain != result[right].Domain {
			return result[left].Domain < result[right].Domain
		}
		if result[left].Path != result[right].Path {
			return result[left].Path < result[right].Path
		}
		return result[left].Name < result[right].Name
	})
	return result
}

// StableAuth returns the small subset suitable for persistent storage and
// one-time WebKit bootstrap.
func StableAuth(records []appcookies.Record, now time.Time) []appcookies.Record {
	runtimeRecords := Runtime(records, now)
	result := make([]appcookies.Record, 0, len(runtimeRecords))
	for _, record := range runtimeRecords {
		if isStableAuthName(record.Name) {
			result = append(result, record)
		}
	}
	return result
}

// MissingStableAuth returns persisted stable-auth records whose cookie
// identity is absent from the live store. Existing live values always win:
// playback bootstrap may fill a missing cookie, but must never replay an older
// value over a newer generation already owned by the shared browser store.
func MissingStableAuth(persisted []appcookies.Record, live []appcookies.Record, now time.Time) []appcookies.Record {
	stablePersisted := StableAuth(persisted, now)
	if len(stablePersisted) == 0 {
		return nil
	}
	stableLive := StableAuth(live, now)
	liveIdentities := make(map[string]struct{}, len(stableLive))
	for _, record := range stableLive {
		liveIdentities[stableAuthIdentity(record)] = struct{}{}
	}
	missing := make([]appcookies.Record, 0, len(stablePersisted))
	for _, record := range stablePersisted {
		if _, exists := liveIdentities[stableAuthIdentity(record)]; exists {
			continue
		}
		missing = append(missing, record)
	}
	return missing
}

func HasAuthForURL(records []appcookies.Record, rawURL string, now time.Time) bool {
	for _, record := range appcookies.MatchURL(records, rawURL) {
		if _, ok := authIndicatorNames[strings.TrimSpace(record.Name)]; !ok {
			continue
		}
		if record.Value == "" {
			continue
		}
		if record.Expires > 0 && !time.Unix(record.Expires, 0).After(now) {
			continue
		}
		return true
	}
	return false
}

func stableAuthIdentity(record appcookies.Record) string {
	return strings.ToLower(strings.TrimSpace(record.Name)) + "\x00" +
		strings.TrimPrefix(strings.ToLower(strings.TrimSpace(record.Domain)), ".") + "\x00" +
		strings.TrimSpace(record.Path)
}

func isYouTubeOrGoogleDomain(value string) bool {
	domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), ".")
	return domain == "youtube.com" || strings.HasSuffix(domain, ".youtube.com") ||
		domain == "google.com" || strings.HasSuffix(domain, ".google.com")
}
