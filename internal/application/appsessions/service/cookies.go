package service

import (
	"context"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubecookies"
)

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

func persistentAppSessionCookies(siteKey string, records []appcookies.Record, now time.Time) []appcookies.Record {
	filtered := filterAppSessionCookiesAt(siteKey, records, now)
	if strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return youtubecookies.StableAuth(filtered, now)
	}
	return filtered
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
