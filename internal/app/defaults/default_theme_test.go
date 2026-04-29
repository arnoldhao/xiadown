package defaults

import "testing"

func TestBuildAppearanceConfigUsesKnownThemePack(t *testing.T) {
	config := buildAppearanceConfig("nocturne")
	appearance, ok := config["appearance"].(map[string]any)
	if !ok {
		t.Fatalf("expected appearance config map, got %#v", config["appearance"])
	}
	if appearance["themePackId"] != "nocturne" {
		t.Fatalf("expected selected theme pack, got %#v", appearance["themePackId"])
	}
	if appearance["sidebarStyle"] != "glass" {
		t.Fatalf("expected default sidebar style, got %#v", appearance["sidebarStyle"])
	}
	if appearance["accentMode"] != "theme" {
		t.Fatalf("expected default accent mode, got %#v", appearance["accentMode"])
	}
}

func TestBuildAppearanceConfigFallsBackForUnknownThemePack(t *testing.T) {
	config := buildAppearanceConfig("unknown")
	appearance, ok := config["appearance"].(map[string]any)
	if !ok {
		t.Fatalf("expected appearance config map, got %#v", config["appearance"])
	}
	if appearance["themePackId"] != fallbackThemePackID {
		t.Fatalf("expected fallback theme pack, got %#v", appearance["themePackId"])
	}
}
