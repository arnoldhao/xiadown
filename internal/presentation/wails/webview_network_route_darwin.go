//go:build darwin && !ios && !server

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Network -framework WebKit

#include <dispatch/dispatch.h>
#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <Network/Network.h>
#import <WebKit/WebKit.h>

enum {
	XiaDownWebViewNetworkRouteFailed = 0,
	XiaDownWebViewNetworkRouteApplied = 1,
};

static int xiadownApplyWebViewNetworkRouteOnMainThread(const char *host, const char *port) {
	nw_endpoint_t endpoint = nw_endpoint_create_host(host, port);
	if (endpoint == NULL) {
		return XiaDownWebViewNetworkRouteFailed;
	}
	nw_proxy_config_t config = nw_proxy_config_create_http_connect(endpoint, NULL);
	if (config == NULL) {
		nw_release(endpoint);
		return XiaDownWebViewNetworkRouteFailed;
	}

	// A gateway outage must not silently fall back to a direct WebKit
	// connection. Wails assets use a native custom scheme and therefore do
	// not require a broad hostname exception from the network route.
	nw_proxy_config_set_failover_allowed(config, false);

	// Keep the default persistent data store so existing cookies, cache and
	// authenticated connector/player sessions remain in the same profile.
	[WKWebsiteDataStore defaultDataStore].proxyConfigurations = @[config];
	nw_release(config);
	nw_release(endpoint);
	return XiaDownWebViewNetworkRouteApplied;
}

static int xiadownApplyWebViewNetworkRoute(const char *host, const char *port) {
	@autoreleasepool {
		if ([NSThread isMainThread]) {
			return xiadownApplyWebViewNetworkRouteOnMainThread(host, port);
		}
		__block int result = XiaDownWebViewNetworkRouteFailed;
		dispatch_sync(dispatch_get_main_queue(), ^{
			result = xiadownApplyWebViewNetworkRouteOnMainThread(host, port);
		});
		return result;
	}
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func applyWebViewNetworkRoutePlatform(gateway webViewNetworkGateway, _ bool) error {
	host := C.CString(gateway.Host)
	port := C.CString(gateway.Port)
	defer C.free(unsafe.Pointer(host))
	defer C.free(unsafe.Pointer(port))

	switch result := int(C.xiadownApplyWebViewNetworkRoute(host, port)); result {
	case int(C.XiaDownWebViewNetworkRouteApplied):
		return nil
	default:
		return fmt.Errorf("apply macOS WebView gateway %s: native configuration failed", gateway.URL)
	}
}
