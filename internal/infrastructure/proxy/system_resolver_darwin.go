//go:build darwin && cgo && !server && !ios

package proxy

/*
#cgo LDFLAGS: -framework CoreFoundation -framework CFNetwork
#include <CoreFoundation/CoreFoundation.h>
#include <CFNetwork/CFProxySupport.h>
#include <stdlib.h>
#include <string.h>

enum {
	XIADOWN_PROXY_ERROR = 0,
	XIADOWN_PROXY_DIRECT = 1,
	XIADOWN_PROXY_HTTP = 2,
	XIADOWN_PROXY_SOCKS = 3
};

typedef struct {
	int kind;
	char *host;
	int port;
	char *username;
	char *password;
} xiadown_proxy_result;

typedef struct {
	Boolean done;
	CFArrayRef proxies;
	CFErrorRef error;
} xiadown_pac_context;

static char *xiadown_copy_cf_string(CFStringRef value) {
	if (value == NULL || CFGetTypeID(value) != CFStringGetTypeID()) {
		return NULL;
	}
	CFIndex length = CFStringGetLength(value);
	CFIndex maximum = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8);
	if (maximum < 0) {
		return NULL;
	}
	char *buffer = (char *)calloc((size_t)maximum + 1, 1);
	if (buffer == NULL) {
		return NULL;
	}
	if (!CFStringGetCString(value, buffer, maximum + 1, kCFStringEncodingUTF8)) {
		free(buffer);
		return NULL;
	}
	return buffer;
}

static void xiadown_clear_secret(char *value) {
	if (value != NULL) {
		memset(value, 0, strlen(value));
		free(value);
	}
}

static void xiadown_proxy_result_release(xiadown_proxy_result *result) {
	if (result == NULL) {
		return;
	}
	free(result->host);
	xiadown_clear_secret(result->username);
	xiadown_clear_secret(result->password);
	memset(result, 0, sizeof(*result));
}

static CFTypeRef xiadown_dictionary_value(CFDictionaryRef dictionary, CFStringRef key, CFTypeID type) {
	if (dictionary == NULL) {
		return NULL;
	}
	CFTypeRef value = CFDictionaryGetValue(dictionary, key);
	if (value == NULL || CFGetTypeID(value) != type) {
		return NULL;
	}
	return value;
}

static int xiadown_copy_proxy_port(CFDictionaryRef dictionary) {
	CFTypeRef value = CFDictionaryGetValue(dictionary, kCFProxyPortNumberKey);
	if (value == NULL) {
		return 0;
	}
	int port = 0;
	if (CFGetTypeID(value) == CFNumberGetTypeID()) {
		if (!CFNumberGetValue((CFNumberRef)value, kCFNumberIntType, &port)) {
			return 0;
		}
	} else if (CFGetTypeID(value) == CFStringGetTypeID()) {
		port = CFStringGetIntValue((CFStringRef)value);
	}
	return port;
}

static void xiadown_pac_callback(void *client, CFArrayRef proxyList, CFErrorRef error) {
	xiadown_pac_context *context = (xiadown_pac_context *)client;
	if (proxyList != NULL) {
		context->proxies = (CFArrayRef)CFRetain(proxyList);
	}
	if (error != NULL) {
		context->error = (CFErrorRef)CFRetain(error);
	}
	context->done = true;
}

static CFArrayRef xiadown_execute_pac_url(CFURLRef pacURL, CFURLRef targetURL) {
	xiadown_pac_context callback = { false, NULL, NULL };
	CFStreamClientContext client = { 0, &callback, NULL, NULL, NULL };
	CFRunLoopSourceRef source = CFNetworkExecuteProxyAutoConfigurationURL(
		pacURL,
		targetURL,
		xiadown_pac_callback,
		&client
	);
	if (source == NULL) {
		return NULL;
	}

	CFRunLoopRef runLoop = CFRunLoopGetCurrent();
	CFRunLoopAddSource(runLoop, source, kCFRunLoopDefaultMode);
	CFAbsoluteTime deadline = CFAbsoluteTimeGetCurrent() + 15.0;
	while (!callback.done) {
		CFTimeInterval remaining = deadline - CFAbsoluteTimeGetCurrent();
		if (remaining <= 0) {
			break;
		}
		if (remaining > 0.25) {
			remaining = 0.25;
		}
		CFRunLoopRunInMode(kCFRunLoopDefaultMode, remaining, true);
	}
	CFRunLoopSourceInvalidate(source);
	CFRunLoopRemoveSource(runLoop, source, kCFRunLoopDefaultMode);
	CFRelease(source);

	Boolean failed = callback.error != NULL;
	if (callback.error != NULL) {
		CFRelease(callback.error);
	}
	if (!callback.done || failed) {
		if (callback.proxies != NULL) {
			CFRelease(callback.proxies);
		}
		return NULL;
	}
	return callback.proxies;
}

static int xiadown_select_first_proxy(CFArrayRef proxies, CFURLRef targetURL, int depth, xiadown_proxy_result *result) {
	if (proxies == NULL || result == NULL || depth > 4 || CFArrayGetCount(proxies) == 0) {
		return 0;
	}

	// CFNetwork orders this array. Do not skip an earlier malformed or
	// unsupported route and accidentally turn a later DIRECT into a bypass.
	CFTypeRef rawCandidate = CFArrayGetValueAtIndex(proxies, 0);
	if (rawCandidate == NULL || CFGetTypeID(rawCandidate) != CFDictionaryGetTypeID()) {
		return 0;
	}
	CFDictionaryRef candidate = (CFDictionaryRef)rawCandidate;
	CFStringRef type = (CFStringRef)xiadown_dictionary_value(candidate, kCFProxyTypeKey, CFStringGetTypeID());
	if (type == NULL) {
		return 0;
	}

	if (CFEqual(type, kCFProxyTypeAutoConfigurationURL)) {
		CFTypeRef rawPACURL = CFDictionaryGetValue(candidate, kCFProxyAutoConfigurationURLKey);
		CFURLRef pacURL = NULL;
		Boolean releasePACURL = false;
		if (rawPACURL != NULL && CFGetTypeID(rawPACURL) == CFURLGetTypeID()) {
			pacURL = (CFURLRef)rawPACURL;
		} else if (rawPACURL != NULL && CFGetTypeID(rawPACURL) == CFStringGetTypeID()) {
			pacURL = CFURLCreateWithString(kCFAllocatorDefault, (CFStringRef)rawPACURL, NULL);
			releasePACURL = true;
		}
		if (pacURL == NULL) {
			return 0;
		}
		CFArrayRef expanded = xiadown_execute_pac_url(pacURL, targetURL);
		if (releasePACURL) {
			CFRelease(pacURL);
		}
		if (expanded == NULL) {
			return 0;
		}
		int selected = xiadown_select_first_proxy(expanded, targetURL, depth + 1, result);
		CFRelease(expanded);
		return selected;
	}

	if (CFEqual(type, kCFProxyTypeAutoConfigurationJavaScript)) {
		CFStringRef script = (CFStringRef)xiadown_dictionary_value(
			candidate,
			kCFProxyAutoConfigurationJavaScriptKey,
			CFStringGetTypeID()
		);
		if (script == NULL) {
			return 0;
		}
		CFErrorRef executionError = NULL;
		CFArrayRef expanded = CFNetworkCopyProxiesForAutoConfigurationScript(script, targetURL, &executionError);
		Boolean failed = executionError != NULL;
		if (executionError != NULL) {
			CFRelease(executionError);
		}
		if (expanded == NULL || failed) {
			if (expanded != NULL) {
				CFRelease(expanded);
			}
			return 0;
		}
		int selected = xiadown_select_first_proxy(expanded, targetURL, depth + 1, result);
		CFRelease(expanded);
		return selected;
	}

	if (CFEqual(type, kCFProxyTypeNone)) {
		result->kind = XIADOWN_PROXY_DIRECT;
		return 1;
	}
	if (CFEqual(type, kCFProxyTypeHTTP) || CFEqual(type, kCFProxyTypeHTTPS)) {
		result->kind = XIADOWN_PROXY_HTTP;
	} else if (CFEqual(type, kCFProxyTypeSOCKS)) {
		result->kind = XIADOWN_PROXY_SOCKS;
	} else {
		return 0;
	}

	CFStringRef host = (CFStringRef)xiadown_dictionary_value(candidate, kCFProxyHostNameKey, CFStringGetTypeID());
	result->host = xiadown_copy_cf_string(host);
	result->port = xiadown_copy_proxy_port(candidate);
	CFStringRef username = (CFStringRef)xiadown_dictionary_value(candidate, kCFProxyUsernameKey, CFStringGetTypeID());
	CFStringRef password = (CFStringRef)xiadown_dictionary_value(candidate, kCFProxyPasswordKey, CFStringGetTypeID());
	result->username = xiadown_copy_cf_string(username);
	result->password = xiadown_copy_cf_string(password);
	if (result->host == NULL || result->port < 1 || result->port > 65535) {
		xiadown_proxy_result_release(result);
		return 0;
	}
	return 1;
}

static int xiadown_resolve_pac_script(const char *rawScript, const char *rawTargetURL, xiadown_proxy_result *result) {
	if (rawScript == NULL || rawTargetURL == NULL || result == NULL) {
		return 0;
	}
	memset(result, 0, sizeof(*result));
	CFStringRef script = CFStringCreateWithCString(kCFAllocatorDefault, rawScript, kCFStringEncodingUTF8);
	CFStringRef targetString = CFStringCreateWithCString(kCFAllocatorDefault, rawTargetURL, kCFStringEncodingUTF8);
	if (script == NULL || targetString == NULL) {
		if (script != NULL) CFRelease(script);
		if (targetString != NULL) CFRelease(targetString);
		return 0;
	}
	CFURLRef targetURL = CFURLCreateWithString(kCFAllocatorDefault, targetString, NULL);
	CFRelease(targetString);
	if (targetURL == NULL) {
		CFRelease(script);
		return 0;
	}
	CFErrorRef executionError = NULL;
	CFArrayRef proxies = CFNetworkCopyProxiesForAutoConfigurationScript(script, targetURL, &executionError);
	CFRelease(script);
	Boolean failed = executionError != NULL;
	if (executionError != NULL) {
		CFRelease(executionError);
	}
	if (proxies == NULL || failed) {
		if (proxies != NULL) CFRelease(proxies);
		CFRelease(targetURL);
		return 0;
	}
	int selected = xiadown_select_first_proxy(proxies, targetURL, 0, result);
	CFRelease(proxies);
	CFRelease(targetURL);
	return selected;
}

static int xiadown_resolve_pac_url(const char *rawPACURL, const char *rawTargetURL, xiadown_proxy_result *result) {
	if (rawPACURL == NULL || rawTargetURL == NULL || result == NULL) {
		return 0;
	}
	memset(result, 0, sizeof(*result));
	CFStringRef pacString = CFStringCreateWithCString(kCFAllocatorDefault, rawPACURL, kCFStringEncodingUTF8);
	CFStringRef targetString = CFStringCreateWithCString(kCFAllocatorDefault, rawTargetURL, kCFStringEncodingUTF8);
	if (pacString == NULL || targetString == NULL) {
		if (pacString != NULL) CFRelease(pacString);
		if (targetString != NULL) CFRelease(targetString);
		return 0;
	}
	CFURLRef pacURL = CFURLCreateWithString(kCFAllocatorDefault, pacString, NULL);
	CFURLRef targetURL = CFURLCreateWithString(kCFAllocatorDefault, targetString, NULL);
	CFRelease(pacString);
	CFRelease(targetString);
	if (pacURL == NULL || targetURL == NULL) {
		if (pacURL != NULL) CFRelease(pacURL);
		if (targetURL != NULL) CFRelease(targetURL);
		return 0;
	}
	CFArrayRef proxies = xiadown_execute_pac_url(pacURL, targetURL);
	CFRelease(pacURL);
	if (proxies == NULL) {
		CFRelease(targetURL);
		return 0;
	}
	int selected = xiadown_select_first_proxy(proxies, targetURL, 0, result);
	CFRelease(proxies);
	CFRelease(targetURL);
	return selected;
}

static int xiadown_resolve_system_proxy(const char *rawTargetURL, xiadown_proxy_result *result) {
	if (rawTargetURL == NULL || result == NULL) {
		return 0;
	}
	memset(result, 0, sizeof(*result));
	CFStringRef targetString = CFStringCreateWithCString(kCFAllocatorDefault, rawTargetURL, kCFStringEncodingUTF8);
	if (targetString == NULL) {
		return 0;
	}
	CFURLRef targetURL = CFURLCreateWithString(kCFAllocatorDefault, targetString, NULL);
	CFRelease(targetString);
	if (targetURL == NULL) {
		return 0;
	}
	CFDictionaryRef settings = CFNetworkCopySystemProxySettings();
	if (settings == NULL) {
		CFRelease(targetURL);
		return 0;
	}
	CFArrayRef proxies = CFNetworkCopyProxiesForURL(targetURL, settings);
	CFRelease(settings);
	if (proxies == NULL) {
		CFRelease(targetURL);
		return 0;
	}
	int selected = xiadown_select_first_proxy(proxies, targetURL, 0, result);
	CFRelease(proxies);
	CFRelease(targetURL);
	return selected;
}
*/
import "C"

import (
	"errors"
	"net/url"
	"unsafe"
)

// platformSystemProxyURL delegates effective global/scoped settings,
// exceptions, PAC URL and inline PAC JavaScript handling to public CFNetwork
// APIs for the canonical destination origin shared by every App surface.
func platformSystemProxyURL(target *url.URL) (*url.URL, error) {
	canonicalTarget, err := canonicalSystemProxyTarget(target)
	if err != nil {
		return nil, err
	}
	rawTarget := C.CString(canonicalTarget.String())
	defer C.free(unsafe.Pointer(rawTarget))
	var result C.xiadown_proxy_result
	if C.xiadown_resolve_system_proxy(rawTarget, &result) == 0 {
		return nil, errors.New("CFNetwork system proxy resolution failed")
	}
	defer C.xiadown_proxy_result_release(&result)
	return darwinProxyResultURL(&result)
}

func darwinProxyResultURL(result *C.xiadown_proxy_result) (*url.URL, error) {
	switch int(result.kind) {
	case C.XIADOWN_PROXY_DIRECT:
		return nil, nil
	case C.XIADOWN_PROXY_HTTP:
		return systemProxyURLFromParts(
			"http",
			C.GoString(result.host),
			int(result.port),
			goStringOrEmpty(result.username),
			goStringOrEmpty(result.password),
		)
	case C.XIADOWN_PROXY_SOCKS:
		return systemProxyURLFromParts(
			"socks5",
			C.GoString(result.host),
			int(result.port),
			goStringOrEmpty(result.username),
			goStringOrEmpty(result.password),
		)
	default:
		return nil, errors.New("CFNetwork returned an unsupported proxy decision")
	}
}

// resolveDarwinPACScript and resolveDarwinPACURL keep the PAC execution path
// deterministic and directly testable without changing this Mac's settings.
func resolveDarwinPACScript(script string, target *url.URL) (*url.URL, error) {
	return resolveDarwinPACTestInput(script, target, false)
}

func resolveDarwinPACURL(pacURL string, target *url.URL) (*url.URL, error) {
	return resolveDarwinPACTestInput(pacURL, target, true)
}

func resolveDarwinPACTestInput(input string, target *url.URL, inputIsURL bool) (*url.URL, error) {
	canonicalTarget, err := canonicalSystemProxyTarget(target)
	if err != nil {
		return nil, err
	}
	rawInput := C.CString(input)
	defer C.free(unsafe.Pointer(rawInput))
	rawTarget := C.CString(canonicalTarget.String())
	defer C.free(unsafe.Pointer(rawTarget))
	var result C.xiadown_proxy_result
	var resolved C.int
	if inputIsURL {
		resolved = C.xiadown_resolve_pac_url(rawInput, rawTarget, &result)
	} else {
		resolved = C.xiadown_resolve_pac_script(rawInput, rawTarget, &result)
	}
	if resolved == 0 {
		return nil, errors.New("CFNetwork PAC resolution failed")
	}
	defer C.xiadown_proxy_result_release(&result)
	return darwinProxyResultURL(&result)
}

func goStringOrEmpty(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}
