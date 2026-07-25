package wails

import (
	"context"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubecookies"
)

func loadListenPlaybackCookies(
	ctx context.Context,
	provider listenPlayerCookieProvider,
) []appcookies.Record {
	if provider == nil {
		return nil
	}
	records, err := provider.RecordsForSiteKey(ctx, "youtube")
	if err != nil {
		return nil
	}
	return filterListenPlaybackCookies(records, time.Now())
}

func filterListenPlaybackCookies(records []appcookies.Record, now time.Time) []appcookies.Record {
	return youtubecookies.StableAuth(records, now)
}

func planListenPlaybackCookieRestore(
	persisted []appcookies.Record,
	current []appcookies.Record,
	now time.Time,
	storeAvailable bool,
) []appcookies.Record {
	if !storeAvailable {
		return nil
	}
	return youtubecookies.MissingStableAuth(persisted, current, now)
}
