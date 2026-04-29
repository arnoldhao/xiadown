package defaults

import (
	"crypto/rand"
	"math/big"

	domainsettings "xiadown/internal/domain/settings"
)

const fallbackThemePackID = "citrus"

var themePackIDs = []string{
	"graphite",
	"citrus",
	"pixel",
	"tape",
	"teal",
	"cloud",
	"damson",
	"fuchsia",
	"cobalt",
	"terracotta",
	"lavender",
	"nocturne",
}

func WithRandomThemePack(current domainsettings.Settings) domainsettings.Settings {
	return current.WithAppearanceConfig(buildAppearanceConfig(randomThemePackID()))
}

func randomThemePackID() string {
	if len(themePackIDs) == 0 {
		return fallbackThemePackID
	}
	index, err := randomIndex(len(themePackIDs))
	if err != nil {
		return fallbackThemePackID
	}
	return themePackIDs[index]
}

func randomIndex(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func buildAppearanceConfig(themePackID string) map[string]any {
	if !isThemePackID(themePackID) {
		themePackID = fallbackThemePackID
	}
	return map[string]any{
		"appearance": map[string]any{
			"themePackId":  themePackID,
			"sidebarStyle": "glass",
			"accentMode":   "theme",
		},
	}
}

func isThemePackID(themePackID string) bool {
	for _, candidate := range themePackIDs {
		if candidate == themePackID {
			return true
		}
	}
	return false
}
