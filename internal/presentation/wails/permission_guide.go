package wails

import "strings"

const (
	defaultScreenAudioPermissionName = "Screen & system audio recording"
	defaultPermissionGuideHint       = "Drag permissions to the list above"
	defaultBrowserDataPermissionName = "Full Disk Access"
	defaultBrowserDataPermissionHint = "Add XiaDown, enable access, then reopen XiaDown"
)

type permissionGuideRequest struct {
	SettingsURL    string
	PermissionName string
	Hint           string
}

func browserDataPermissionGuideRequest(permissionName string, hint string) permissionGuideRequest {
	return permissionGuideRequest{
		SettingsURL:    "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles",
		PermissionName: permissionGuideText(permissionName, defaultBrowserDataPermissionName),
		Hint:           permissionGuideText(hint, defaultBrowserDataPermissionHint),
	}
}

func screenSystemAudioPermissionGuideRequest(permissionName string, hint string) permissionGuideRequest {
	return permissionGuideRequest{
		SettingsURL:    "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture",
		PermissionName: permissionGuideText(permissionName, defaultScreenAudioPermissionName),
		Hint:           permissionGuideText(hint, defaultPermissionGuideHint),
	}
}

func permissionGuideText(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
