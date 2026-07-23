//go:build linux && cgo && !gtk3 && !android && !server

package wails

/*
#cgo linux pkg-config: gtk4 webkitgtk-6.0

#include <gtk/gtk.h>
#include <webkit/webkit.h>

static WebKitWebView *xiadownFindShellWebView(GtkWidget *widget) {
	if (widget == NULL) {
		return NULL;
	}
	if (WEBKIT_IS_WEB_VIEW(widget)) {
		return WEBKIT_WEB_VIEW(widget);
	}
	for (GtkWidget *child = gtk_widget_get_first_child(widget);
		 child != NULL;
		 child = gtk_widget_get_next_sibling(child)) {
		WebKitWebView *web_view = xiadownFindShellWebView(child);
		if (web_view != NULL) {
			return web_view;
		}
	}
	return NULL;
}

static int xiadownApplyShellRemoteCapabilityPolicyDirect(GtkWindow *window) {
	if (window == NULL || !GTK_IS_WINDOW(window)) {
		return 0;
	}
	WebKitWebView *web_view = xiadownFindShellWebView(GTK_WIDGET(window));
	if (web_view == NULL) {
		return 0;
	}
	WebKitSettings *settings = webkit_web_view_get_settings(web_view);
	if (settings == NULL) {
		return 0;
	}
	// These are public WebKitGTK settings. Disable UDP-capable page APIs and
	// browser-created windows on the local shell while preserving ordinary
	// MediaSource/HTMLMediaElement playback in dedicated player WebViews.
	webkit_settings_set_enable_webrtc(settings, FALSE);
	webkit_settings_set_enable_media_stream(settings, FALSE);
	// WebKitGTK 2.48 made DNS prefetching permanently disabled, and 2.50
	// made hyperlink auditing permanently enabled. Keep tightening older
	// runtimes without calling deprecated no-op setters on newer headers.
#if !WEBKIT_CHECK_VERSION(2, 48, 0)
	webkit_settings_set_enable_dns_prefetching(settings, FALSE);
#endif
#if !WEBKIT_CHECK_VERSION(2, 50, 0)
	webkit_settings_set_enable_hyperlink_auditing(settings, FALSE);
#endif
	webkit_settings_set_javascript_can_open_windows_automatically(settings, FALSE);
	return 1;
}

static gboolean xiadownApplyShellRemoteCapabilityPolicyDeferred(gpointer data) {
	GtkWindow *window = GTK_WINDOW(data);
	xiadownApplyShellRemoteCapabilityPolicyDirect(window);
	g_object_unref(window);
	return G_SOURCE_REMOVE;
}

static int xiadownApplyShellRemoteCapabilityPolicy(void *native_window) {
	if (native_window == NULL || !GTK_IS_WINDOW(native_window)) {
		return 0;
	}
	GtkWindow *window = GTK_WINDOW(native_window);
	GMainContext *context = g_main_context_default();
	if (g_main_context_is_owner(context)) {
		return xiadownApplyShellRemoteCapabilityPolicyDirect(window);
	}
	g_object_ref(window);
	g_main_context_invoke(
		context,
		xiadownApplyShellRemoteCapabilityPolicyDeferred,
		window
	);
	return 1;
}
*/
import "C"

import "unsafe"

func applyShellRemoteCapabilityPolicy(nativeWindow unsafe.Pointer) {
	C.xiadownApplyShellRemoteCapabilityPolicy(nativeWindow)
}
