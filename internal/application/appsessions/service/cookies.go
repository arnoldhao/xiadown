package service

import (
	"context"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
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
	if len(records) == 0 {
		return nil
	}
	filtered := appcookies.FilterByDomains(records, appSessionCookieDomains(siteKey))
	return removeExpiredCookies(filtered, time.Now())
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
		if record.Expires > 0 && record.Expires < nowUnix {
			continue
		}
		result = append(result, record)
	}
	return result
}
