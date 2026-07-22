//go:build linux && cgo && !server && !android

package proxy

/*
#cgo pkg-config: gio-2.0
#include <gio/gio.h>
#include <stdlib.h>

static size_t xiadown_proxy_list_length(char **proxies) {
	if (proxies == NULL) {
		return 0;
	}
	size_t count = 0;
	while (proxies[count] != NULL) {
		count++;
	}
	return count;
}

static const char *xiadown_proxy_list_at(char **proxies, size_t index) {
	return proxies[index];
}
*/
import "C"

import (
	"errors"
	"net/url"
	"unsafe"
)

// platformSystemProxyURL asks GIO for the effective proxy decision for this
// canonical origin. The default GProxyResolver selects the desktop implementation
// (GNOME/libproxy/PAC) and GIO's Flatpak portal resolver when sandboxed.
func platformSystemProxyURL(target *url.URL) (*url.URL, error) {
	canonicalTarget, err := canonicalSystemProxyTarget(target)
	if err != nil {
		return nil, err
	}
	resolver := C.g_proxy_resolver_get_default()
	if resolver == nil || C.g_proxy_resolver_is_supported(resolver) == 0 {
		return nil, errNativeSystemProxyUnavailable
	}

	rawTarget := C.CString(canonicalTarget.String())
	defer C.free(unsafe.Pointer(rawTarget))
	var nativeError *C.GError
	proxies := C.g_proxy_resolver_lookup(resolver, rawTarget, nil, &nativeError)
	if nativeError != nil {
		C.g_error_free(nativeError)
		return nil, errors.New("GIO system proxy resolution failed")
	}
	if proxies == nil {
		return nil, errors.New("GIO system proxy resolver returned no decision")
	}
	defer C.g_strfreev(proxies)

	count := int(C.xiadown_proxy_list_length(proxies))
	candidates := make([]string, 0, count)
	for index := 0; index < count; index++ {
		candidate := C.xiadown_proxy_list_at(proxies, C.size_t(index))
		if candidate != nil {
			candidates = append(candidates, C.GoString(candidate))
		}
	}
	return firstSystemProxyCandidate(candidates)
}
