package service

import (
	"encoding/json"
	"strings"

	"xiadown/internal/application/appsessions/dto"
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
