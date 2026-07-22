package wails

import "testing"

func TestBrowserDataPermissionGuideTargetsFullDiskAccess(t *testing.T) {
	request := browserDataPermissionGuideRequest("", "")
	if request.SettingsURL != "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles" {
		t.Fatalf("unexpected browser-data permission URL: %q", request.SettingsURL)
	}
	if request.PermissionName != defaultBrowserDataPermissionName || request.Hint != defaultBrowserDataPermissionHint {
		t.Fatalf("unexpected browser-data permission defaults: %#v", request)
	}
}
