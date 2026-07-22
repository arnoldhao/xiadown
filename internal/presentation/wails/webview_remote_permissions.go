package wails

import "github.com/wailsapp/wails/v3/pkg/application"

// Wails alpha2.117 has not exposed WebView2's autoplay permission in its
// public constants yet. The native enum is stable and assigns autoplay kind 9.
// Only dedicated media windows opt into this permission; arbitrary remote
// capability windows continue to inherit the default-deny policy below.
const remoteMediaWebViewAutoplayPermissionKind application.CoreWebView2PermissionKind = 9

var remoteWebViewPermissionTypes = [...]application.PermissionType{
	application.PermissionMicrophone,
	application.PermissionCamera,
	application.PermissionGeolocation,
	application.PermissionNotifications,
	application.PermissionClipboardRead,
}

var remoteWebViewWindowsPermissionKinds = [...]application.CoreWebView2PermissionKind{
	application.CoreWebView2PermissionKindUnknownPermission,
	application.CoreWebView2PermissionKindMicrophone,
	application.CoreWebView2PermissionKindCamera,
	application.CoreWebView2PermissionKindGeolocation,
	application.CoreWebView2PermissionKindNotifications,
	application.CoreWebView2PermissionKindOtherSensors,
	application.CoreWebView2PermissionKindClipboardRead,
}

// withRemoteWebViewPermissionPolicy applies the capability policy before the
// native webview is created. The cross-platform map covers WebKitGTK and the
// common WebView2 permission kinds. The Windows map repeats those denies so a
// caller-provided platform override cannot weaken them, then also denies
// WebView2's sensors and unknown/future permission bucket.
func withRemoteWebViewPermissionPolicy(options application.WebviewWindowOptions) application.WebviewWindowOptions {
	permissions := make(map[application.PermissionType]application.Permission, len(options.Permissions)+len(remoteWebViewPermissionTypes))
	for permission, state := range options.Permissions {
		permissions[permission] = state
	}
	for _, permission := range remoteWebViewPermissionTypes {
		permissions[permission] = application.PermissionDeny
	}
	options.Permissions = permissions

	windowsPermissions := make(
		map[application.CoreWebView2PermissionKind]application.CoreWebView2PermissionState,
		len(options.Windows.Permissions)+len(remoteWebViewWindowsPermissionKinds),
	)
	for permission, state := range options.Windows.Permissions {
		windowsPermissions[permission] = state
	}
	for _, permission := range remoteWebViewWindowsPermissionKinds {
		windowsPermissions[permission] = application.CoreWebView2PermissionStateDeny
	}
	options.Windows.Permissions = windowsPermissions
	return options
}
