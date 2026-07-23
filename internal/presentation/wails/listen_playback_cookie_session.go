package wails

import (
	"context"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubecookies"
)

type listenPlayerCookieSyncProvider interface {
	BeginCookieSync(siteKey string) (epoch uint64, sequence uint64)
	SyncRecordsForSiteKey(
		ctx context.Context,
		siteKey string,
		records []appcookies.Record,
		expectedEpoch uint64,
		expectedSequence uint64,
	) error
}

func planListenPlaybackCookieRestore(
	persisted []appcookies.Record,
	current []appcookies.Record,
	targetURL string,
	now time.Time,
	storeAvailable bool,
) []appcookies.Record {
	if !storeAvailable {
		return nil
	}
	if youtubecookies.HasAuthForURL(current, targetURL, now) {
		return nil
	}
	stable := youtubecookies.StableAuth(persisted, now)
	if len(stable) == 0 {
		return nil
	}
	return stable
}
