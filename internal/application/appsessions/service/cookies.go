package service

import (
	"context"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

// SyncRecordsForSiteKey accepts a full live browser snapshot. Providers that
// support runtime synchronization keep it in memory for request consumers and
// may persist a smaller safe subset.
func (service *AppSessionsService) SyncRecordsForSiteKey(
	ctx context.Context,
	siteKey string,
	records []appcookies.Record,
	expectedEpoch uint64,
	expectedSequence uint64,
) error {
	if service == nil || service.provider == nil {
		return appsessions.ErrUnsupported
	}
	siteKey = strings.TrimSpace(siteKey)
	if !isSupportedSiteKey(siteKey) {
		return appsessions.ErrSessionNotFound
	}
	provider, ok := service.provider.(AppSessionRuntimeCookieSyncProvider)
	if !ok {
		return appsessions.ErrUnsupported
	}
	return provider.SyncAppSessionCookies(ctx, siteKey, records, expectedEpoch, expectedSequence)
}

func (service *AppSessionsService) BeginCookieSync(siteKey string) (uint64, uint64) {
	if service == nil || service.provider == nil {
		return 0, 0
	}
	provider, ok := service.provider.(AppSessionRuntimeCookieSyncProvider)
	if !ok {
		return 0, 0
	}
	return provider.BeginAppSessionCookieSync(strings.TrimSpace(siteKey))
}

func (service *AppSessionsService) storedCookies(ctx context.Context, siteKey string) []appcookies.Record {
	if service == nil || service.provider == nil {
		return nil
	}
	records, err := service.provider.LoadAppSessionCookies(ctx, siteKey)
	if err != nil || len(records) == 0 {
		return nil
	}
	return filterAppSessionCookies(siteKey, records)
}

func filterAppSessionCookies(siteKey string, records []appcookies.Record) []appcookies.Record {
	return filterAppSessionCookiesAt(siteKey, records, time.Now())
}

func filterAppSessionCookiesAt(siteKey string, records []appcookies.Record, now time.Time) []appcookies.Record {
	if len(records) == 0 {
		return nil
	}
	filtered := appcookies.FilterByDomains(records, appSessionCookieDomains(siteKey))
	return removeExpiredCookies(filtered, now)
}

func removeExpiredCookies(records []appcookies.Record, now time.Time) []appcookies.Record {
	if len(records) == 0 {
		return nil
	}
	result := make([]appcookies.Record, 0, len(records))
	nowUnix := now.Unix()
	for _, record := range records {
		if strings.TrimSpace(record.Name) == "" {
			continue
		}
		if record.Expires > 0 && record.Expires <= nowUnix {
			continue
		}
		result = append(result, record)
	}
	return result
}
