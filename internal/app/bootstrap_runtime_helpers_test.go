package app

import "testing"

func TestAppIconAssetPath(t *testing.T) {
	t.Parallel()

	if got := appIconAssetPath("windows"); got != "frontend/dist/appicon_windows.png" {
		t.Fatalf("expected windows app icon path to use appicon_windows.png asset, got %q", got)
	}

	if got := appIconAssetPath("darwin"); got != "frontend/dist/appicon.png" {
		t.Fatalf("expected darwin app icon path to keep default asset, got %q", got)
	}

	if got := appIconAssetPath("linux"); got != "frontend/dist/appicon.png" {
		t.Fatalf("expected non-windows app icon path to keep default asset, got %q", got)
	}
}

func TestTrayIconAssetPath(t *testing.T) {
	t.Parallel()

	if got := trayIconAssetPath("windows"); got != "frontend/dist/tray_windows.ico" {
		t.Fatalf("expected windows tray icon path to use tray_windows.ico asset, got %q", got)
	}

	if got := trayIconAssetPath("darwin"); got != "frontend/dist/tray.png" {
		t.Fatalf("expected darwin tray icon path to keep default asset, got %q", got)
	}

	if got := trayIconAssetPath("linux"); got != "frontend/dist/tray.png" {
		t.Fatalf("expected non-windows tray icon path to keep default asset, got %q", got)
	}
}

func TestSingleInstanceUniqueIDSeparatesDevelopmentFromRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		want    string
	}{
		{version: "dev", want: productionSingleInstanceID + ".dev"},
		{version: "development", want: productionSingleInstanceID + ".dev"},
		{version: "", want: productionSingleInstanceID + ".dev"},
		{version: "1.2.3-alpha.1", want: productionSingleInstanceID + ".dev"},
		{version: "1.2.3", want: productionSingleInstanceID},
		{version: "v1.2.3", want: productionSingleInstanceID},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			if got := singleInstanceUniqueID(test.version); got != test.want {
				t.Fatalf("single instance ID for %q = %q, want %q", test.version, got, test.want)
			}
		})
	}
}
