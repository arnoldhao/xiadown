package library

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	catalogIDMaxLength          = 255
	catalogNameMaxLength        = 255
	catalogDescriptionMaxLength = 4096
	catalogOpaqueValueMaxLength = 4096
)

func normalizeCatalogID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || len([]rune(value)) > catalogIDMaxLength {
		return "", false
	}
	for _, item := range value {
		if unicode.IsSpace(item) || unicode.IsControl(item) {
			return "", false
		}
	}
	return value, true
}

func normalizeCatalogName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || len([]rune(value)) > catalogNameMaxLength {
		return "", false
	}
	for _, item := range value {
		if unicode.IsControl(item) {
			return "", false
		}
	}
	return value, true
}

func normalizeCatalogDescription(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len([]rune(value)) > catalogDescriptionMaxLength {
		return "", false
	}
	for _, item := range value {
		if unicode.IsControl(item) && item != '\n' && item != '\r' && item != '\t' {
			return "", false
		}
	}
	return value, true
}

func normalizeCatalogOpaqueValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len([]rune(value)) > catalogOpaqueValueMaxLength {
		return "", false
	}
	for _, item := range value {
		if unicode.IsControl(item) {
			return "", false
		}
	}
	return value, true
}

func normalizeCatalogTimes(createdAtParam, updatedAtParam *time.Time) (time.Time, time.Time, bool) {
	now := time.Now().UTC()
	createdAt := now
	if createdAtParam != nil && !createdAtParam.IsZero() {
		createdAt = createdAtParam.UTC()
	}
	updatedAt := createdAt
	if updatedAtParam != nil && !updatedAtParam.IsZero() {
		updatedAt = updatedAtParam.UTC()
	}
	return createdAt, updatedAt, !updatedAt.Before(createdAt)
}

func normalizeOptionalCatalogTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func normalizeCatalogRevision(value int64) (int64, bool) {
	if value == 0 {
		return 1, true
	}
	return value, value > 0
}

func normalizeCatalogEnum(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
