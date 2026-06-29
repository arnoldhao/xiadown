package service

import (
	"encoding/json"
	"strings"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
)

func encodeBadges(badges []dto.AppSessionBadge) string {
	if len(badges) == 0 {
		return ""
	}
	data, err := json.Marshal(badges)
	if err != nil {
		return ""
	}
	return string(data)
}

func encodeMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func accountMetadataWithExpiresAt(metadata map[string]any, siteKey string, records []appcookies.Record, now time.Time) map[string]any {
	expiresAt := cookieExpiresAt(siteKey, records, now)
	if expiresAt == "" {
		return metadata
	}
	result := make(map[string]any, len(metadata)+1)
	for key, value := range metadata {
		result[key] = value
	}
	result[appSessionAccountExpiresAtMetadataKey] = expiresAt
	return result
}
