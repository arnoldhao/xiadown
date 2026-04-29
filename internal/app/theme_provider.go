package app

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type AppThemeProvider struct {
	app *application.App
}

func NewAppThemeProvider(app *application.App) *AppThemeProvider {
	return &AppThemeProvider{app: app}
}

func (provider *AppThemeProvider) IsDarkMode(_ context.Context) (bool, error) {
	return provider.app.Env.IsDarkMode(), nil
}

func (provider *AppThemeProvider) AccentColor(_ context.Context) (string, error) {
	if provider.app == nil {
		return "", nil
	}
	raw := strings.TrimSpace(provider.app.Env.GetAccentColor())
	if raw == "" {
		return "", nil
	}
	return normalizeAccentColor(raw), nil
}

var accentRGBPattern = regexp.MustCompile(`^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$`)

func normalizeAccentColor(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "#") && len(trimmed) == 7 {
		return strings.ToLower(trimmed)
	}
	match := accentRGBPattern.FindStringSubmatch(trimmed)
	if len(match) != 4 {
		return ""
	}
	red, err := strconv.Atoi(match[1])
	if err != nil {
		return ""
	}
	green, err := strconv.Atoi(match[2])
	if err != nil {
		return ""
	}
	blue, err := strconv.Atoi(match[3])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", clampByte(red), clampByte(green), clampByte(blue))
}

func clampByte(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}
