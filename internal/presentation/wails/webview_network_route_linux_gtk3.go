//go:build linux && cgo && gtk3 && !android && !server

package wails

/*
#cgo linux pkg-config: webkit2gtk-4.1

#include <stdlib.h>
#include <webkit2/webkit2.h>

static int xiadownApplyWebViewNetworkRoute(const char *gateway_uri) {
	WebKitWebContext *context = webkit_web_context_get_default();
	if (context == NULL) {
		return 0;
	}
	WebKitWebsiteDataManager *manager =
		webkit_web_context_get_website_data_manager(context);
	if (manager == NULL) {
		return 0;
	}
	// Preserve the default persistent WebsiteDataManager (and its cookies).
	// Wails assets use a custom scheme and require no network bypass.
	WebKitNetworkProxySettings *settings =
		webkit_network_proxy_settings_new(gateway_uri, NULL);
	if (settings == NULL) {
		return 0;
	}
	webkit_website_data_manager_set_network_proxy_settings(
		manager,
		WEBKIT_NETWORK_PROXY_MODE_CUSTOM,
		settings
	);
	webkit_network_proxy_settings_free(settings);
	return 1;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func applyWebViewNetworkRoutePlatform(gateway webViewNetworkGateway, _ bool) error {
	gatewayURI := C.CString(gateway.URL)
	defer C.free(unsafe.Pointer(gatewayURI))
	if C.xiadownApplyWebViewNetworkRoute(gatewayURI) == 0 {
		return fmt.Errorf("apply GTK3 WebKitWebsiteDataManager gateway %s: native configuration failed", gateway.URL)
	}
	return nil
}
