//go:build linux && cgo && !gtk3 && !android && !server

package wails

/*
#cgo linux pkg-config: webkitgtk-6.0

#include <stdlib.h>
#include <webkit/webkit.h>

static int xiadownApplyWebViewNetworkRoute(const char *gateway_uri) {
	WebKitNetworkSession *session = webkit_network_session_get_default();
	if (session == NULL) {
		return 0;
	}
	WebKitNetworkProxySettings *settings =
		webkit_network_proxy_settings_new(gateway_uri, NULL);
	if (settings == NULL) {
		return 0;
	}
	webkit_network_session_set_proxy_settings(
		session,
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
		return fmt.Errorf("apply GTK4 WebKitNetworkSession gateway %s: native configuration failed", gateway.URL)
	}
	return nil
}
