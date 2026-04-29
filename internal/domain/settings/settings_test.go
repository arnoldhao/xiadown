package settings

import "testing"

func TestWithAppearanceConfigClonesNestedValues(t *testing.T) {
	source := map[string]any{
		"appearance": map[string]any{
			"themePackId": "citrus",
		},
	}

	current := DefaultSettings().WithAppearanceConfig(source)
	source["appearance"].(map[string]any)["themePackId"] = "nocturne"

	config := current.AppearanceConfig()
	appearance, ok := config["appearance"].(map[string]any)
	if !ok {
		t.Fatalf("expected appearance config map, got %#v", config["appearance"])
	}
	if appearance["themePackId"] != "citrus" {
		t.Fatalf("expected stored theme pack to be isolated from source mutation, got %#v", appearance["themePackId"])
	}

	appearance["themePackId"] = "pixel"
	nextConfig := current.AppearanceConfig()
	nextAppearance, ok := nextConfig["appearance"].(map[string]any)
	if !ok {
		t.Fatalf("expected appearance config map, got %#v", nextConfig["appearance"])
	}
	if nextAppearance["themePackId"] != "citrus" {
		t.Fatalf("expected returned theme pack to be isolated from caller mutation, got %#v", nextAppearance["themePackId"])
	}
}
