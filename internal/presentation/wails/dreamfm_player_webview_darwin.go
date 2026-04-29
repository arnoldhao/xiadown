//go:build darwin && !ios

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=10.13 -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework AVKit

#include <stdlib.h>
#import <AVKit/AVKit.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static NSView *dreamFMActiveAirPlayPicker = nil;
static NSInteger dreamFMAirPlayPickerGeneration = 0;

static WKWebView* dreamFMFindWKWebView(NSView *view) {
	if (view == nil) {
		return nil;
	}
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView*)view;
	}
	for (NSView *subview in [view subviews]) {
		WKWebView *candidate = dreamFMFindWKWebView(subview);
		if (candidate != nil) {
			return candidate;
		}
	}
	return nil;
}

static WKWebView* dreamFMWebViewForWindow(void *nativeWindow) {
	if (nativeWindow == NULL) {
		return nil;
	}

	NSWindow *window = (NSWindow*)nativeWindow;
	if (window == nil) {
		return nil;
	}

	SEL webViewSelector = NSSelectorFromString(@"webView");
	if ([window respondsToSelector:webViewSelector]) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Warc-performSelector-leaks"
		id candidate = [window performSelector:webViewSelector];
#pragma clang diagnostic pop
		if ([candidate isKindOfClass:[WKWebView class]]) {
			return (WKWebView*)candidate;
		}
	}

	return dreamFMFindWKWebView([window contentView]);
}

static void dreamFMConfigureYouTubeMusicWebView(void *nativeWindow, const char *userAgent) {
	@autoreleasepool {
		WKWebView *webView = dreamFMWebViewForWindow(nativeWindow);
		if (webView == nil || userAgent == NULL) {
			return;
		}

		NSString *customUserAgent = [NSString stringWithUTF8String:userAgent];
		if (customUserAgent.length > 0) {
			webView.customUserAgent = customUserAgent;
		}

		WKWebViewConfiguration *configuration = webView.configuration;
		if (configuration == nil) {
			return;
		}

		configuration.applicationNameForUserAgent = @"";

#if MAC_OS_X_VERSION_MAX_ALLOWED >= 101200
		if (@available(macOS 10.12, *)) {
			configuration.mediaTypesRequiringUserActionForPlayback = WKAudiovisualMediaTypeNone;
		}
#endif

		if ([configuration respondsToSelector:@selector(setAllowsAirPlayForMediaPlayback:)]) {
			configuration.allowsAirPlayForMediaPlayback = YES;
		}

#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120300
		if (@available(macOS 12.3, *)) {
			configuration.preferences.elementFullscreenEnabled = YES;
		}
#endif
	}
}

static CGFloat dreamFMClampCGFloat(CGFloat value, CGFloat minimum, CGFloat maximum) {
	if (value < minimum) {
		return minimum;
	}
	if (value > maximum) {
		return maximum;
	}
	return value;
}

static void dreamFMClearActiveAirPlayPicker(NSInteger generation) {
	if (dreamFMActiveAirPlayPicker == nil || generation != dreamFMAirPlayPickerGeneration) {
		return;
	}
	[dreamFMActiveAirPlayPicker removeFromSuperview];
	[dreamFMActiveAirPlayPicker release];
	dreamFMActiveAirPlayPicker = nil;
}

static int dreamFMShowAirPlayRoutePicker(void *nativeWindow, double anchorX, double anchorY, double anchorWidth, double anchorHeight) {
	@autoreleasepool {
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 101500
		if (@available(macOS 10.15, *)) {
			if (nativeWindow == NULL) {
				return 0;
			}

			NSWindow *window = (NSWindow*)nativeWindow;
			NSView *contentView = window.contentView;
			if (contentView == nil) {
				return 0;
			}

			NSRect bounds = contentView.bounds;
			CGFloat width = anchorWidth > 0 ? (CGFloat)anchorWidth : 40;
			CGFloat height = anchorHeight > 0 ? (CGFloat)anchorHeight : 40;
			width = dreamFMClampCGFloat(width, 24, MAX(24, bounds.size.width));
			height = dreamFMClampCGFloat(height, 24, MAX(24, bounds.size.height));

			CGFloat x = (anchorWidth > 0 || anchorHeight > 0) ? (CGFloat)anchorX : 20;
			CGFloat y = 14;
			if (anchorWidth > 0 || anchorHeight > 0) {
				if ([contentView isFlipped]) {
					y = (CGFloat)anchorY;
				} else {
					y = bounds.size.height - (CGFloat)anchorY - height;
				}
			}
			x = dreamFMClampCGFloat(x, 0, MAX(0, bounds.size.width - width));
			y = dreamFMClampCGFloat(y, 0, MAX(0, bounds.size.height - height));

			dreamFMAirPlayPickerGeneration += 1;
			if (dreamFMActiveAirPlayPicker != nil) {
				[dreamFMActiveAirPlayPicker removeFromSuperview];
				[dreamFMActiveAirPlayPicker release];
				dreamFMActiveAirPlayPicker = nil;
			}
			NSInteger generation = dreamFMAirPlayPickerGeneration;
			AVRoutePickerView *picker = [[AVRoutePickerView alloc] initWithFrame:NSMakeRect(x, y, width, height)];
			picker.alphaValue = 0.01;
			dreamFMActiveAirPlayPicker = picker;
			[contentView addSubview:picker];
			[picker layoutSubtreeIfNeeded];

			NSButton *button = nil;
			for (NSView *subview in picker.subviews) {
				if ([subview isKindOfClass:[NSButton class]]) {
					button = (NSButton*)subview;
					break;
				}
			}

			if (button == nil) {
				dreamFMClearActiveAirPlayPicker(generation);
				return 0;
			}

			[button performClick:nil];
			dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(15 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
				dreamFMClearActiveAirPlayPicker(generation);
			});
			return 1;
		}
#endif
		return 0;
	}
}

static NSString* dreamFMStringValue(id value) {
	if ([value isKindOfClass:[NSString class]]) {
		return (NSString*)value;
	}
	if ([value isKindOfClass:[NSNumber class]]) {
		return [(NSNumber*)value stringValue];
	}
	return nil;
}

static BOOL dreamFMBoolValue(id value) {
	if ([value isKindOfClass:[NSNumber class]]) {
		return [(NSNumber*)value boolValue];
	}
	if ([value isKindOfClass:[NSString class]]) {
		NSString *lower = [(NSString*)value lowercaseString];
		return [lower isEqualToString:@"true"] || [lower isEqualToString:@"1"] || [lower isEqualToString:@"yes"];
	}
	return NO;
}

static NSString *dreamFMYouTubeClientOrigin(void) {
	NSString *bundleID = [[NSBundle mainBundle] bundleIdentifier];
	if (bundleID.length == 0) {
		bundleID = @"com.dreamapp.xiadown";
	}
	return [[NSString stringWithFormat:@"https://%@", bundleID] lowercaseString];
}

static NSArray<NSHTTPCookie*>* dreamFMCookiesFromJSON(const char *cookiesJSON, NSURL *targetURL) {
	if (cookiesJSON == NULL) {
		return @[];
	}

	NSString *json = [NSString stringWithUTF8String:cookiesJSON];
	if (json.length == 0) {
		return @[];
	}

	NSData *data = [json dataUsingEncoding:NSUTF8StringEncoding];
	if (data == nil) {
		return @[];
	}

	NSError *error = nil;
	id parsed = [NSJSONSerialization JSONObjectWithData:data options:0 error:&error];
	if (error != nil || ![parsed isKindOfClass:[NSArray class]]) {
		return @[];
	}

	NSMutableArray<NSHTTPCookie*> *cookies = [NSMutableArray array];
	NSDate *now = [NSDate date];
	NSString *fallbackDomain = targetURL.host ?: @"music.youtube.com";

	for (id item in (NSArray*)parsed) {
		if (![item isKindOfClass:[NSDictionary class]]) {
			continue;
		}
		NSDictionary *dictionary = (NSDictionary*)item;
		NSString *name = dreamFMStringValue(dictionary[@"name"]);
		NSString *value = dreamFMStringValue(dictionary[@"value"]);
		NSString *domain = dreamFMStringValue(dictionary[@"domain"]);
		NSString *path = dreamFMStringValue(dictionary[@"path"]);
		if (name.length == 0 || value.length == 0) {
			continue;
		}
		if (domain.length == 0) {
			domain = fallbackDomain;
		}
		if (path.length == 0) {
			path = @"/";
		}

		NSMutableDictionary<NSHTTPCookiePropertyKey, id> *properties = [NSMutableDictionary dictionary];
		properties[NSHTTPCookieName] = name;
		properties[NSHTTPCookieValue] = value;
		properties[NSHTTPCookieDomain] = domain;
		properties[NSHTTPCookiePath] = path;

		id expiresValue = dictionary[@"expires"];
		if ([expiresValue isKindOfClass:[NSNumber class]]) {
			NSTimeInterval expiresSeconds = [(NSNumber*)expiresValue doubleValue];
			if (expiresSeconds > 0) {
				NSDate *expiresDate = [NSDate dateWithTimeIntervalSince1970:expiresSeconds];
				if ([expiresDate compare:now] != NSOrderedDescending) {
					continue;
				}
				properties[NSHTTPCookieExpires] = expiresDate;
			}
		}

		if (dreamFMBoolValue(dictionary[@"secure"])) {
			properties[NSHTTPCookieSecure] = @"TRUE";
		}
		if (dreamFMBoolValue(dictionary[@"httpOnly"])) {
			properties[(NSHTTPCookiePropertyKey)@"HttpOnly"] = @"TRUE";
		}

		NSHTTPCookie *cookie = [NSHTTPCookie cookieWithProperties:properties];
		if (cookie != nil) {
			[cookies addObject:cookie];
		}
	}

	return cookies;
}

static void dreamFMLoadRequestOnMain(WKWebView *webView, NSURLRequest *request) {
	if (webView == nil || request == nil) {
		return;
	}
	if ([NSThread isMainThread]) {
		[webView loadRequest:request];
		return;
	}
	dispatch_async(dispatch_get_main_queue(), ^{
		[webView loadRequest:request];
	});
}

static NSString *dreamFMNavigationRefererForURL(NSURL *url) {
	if (url == nil || url.host == nil) {
		return nil;
	}
	NSString *host = url.host.lowercaseString;
	if ([host isEqualToString:@"music.youtube.com"] || [host hasSuffix:@".music.youtube.com"]) {
		return @"https://music.youtube.com/";
	}
	if ([host isEqualToString:@"youtube.com"] || [host hasSuffix:@".youtube.com"]) {
		return dreamFMYouTubeClientOrigin();
	}
	return nil;
}

static void dreamFMLoadYouTubeMusicURL(void *nativeWindow, const char *targetURL, const char *cookiesJSON) {
	@autoreleasepool {
		WKWebView *webView = dreamFMWebViewForWindow(nativeWindow);
		if (webView == nil || targetURL == NULL) {
			return;
		}

		NSString *urlString = [NSString stringWithUTF8String:targetURL];
		NSURL *url = [NSURL URLWithString:urlString];
		if (url == nil) {
			return;
		}

		NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url];
		NSString *referer = dreamFMNavigationRefererForURL(url);
		if (referer.length > 0) {
			[request setValue:referer forHTTPHeaderField:@"Referer"];
		}
		NSArray<NSHTTPCookie*> *cookies = dreamFMCookiesFromJSON(cookiesJSON, url);
		WKHTTPCookieStore *cookieStore = webView.configuration.websiteDataStore.httpCookieStore;
		if (cookies.count == 0 || cookieStore == nil) {
			[webView loadRequest:request];
			return;
		}

		__block NSInteger remaining = cookies.count;
		for (NSHTTPCookie *cookie in cookies) {
			[cookieStore setCookie:cookie completionHandler:^{
				remaining -= 1;
				if (remaining <= 0) {
					dreamFMLoadRequestOnMain(webView, request);
				}
			}];
		}
	}
}

static void dreamFMEvaluateYouTubeMusicJavaScript(void *nativeWindow, const char *script) {
	@autoreleasepool {
		WKWebView *webView = dreamFMWebViewForWindow(nativeWindow);
		if (webView == nil || script == NULL) {
			return;
		}
		NSString *source = [NSString stringWithUTF8String:script];
		if (source.length == 0) {
			return;
		}
		[webView evaluateJavaScript:source completionHandler:nil];
	}
}
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func dreamFMYouTubeMusicUserAgent() string {
	return youtubemusic.BrowserUserAgent
}

func configureDreamFMYouTubeMusicNativeWindow(nativeWindow unsafe.Pointer, userAgent string) {
	if nativeWindow == nil || userAgent == "" {
		return
	}

	cUserAgent := C.CString(userAgent)
	defer C.free(unsafe.Pointer(cUserAgent))

	application.InvokeSync(func() {
		C.dreamFMConfigureYouTubeMusicWebView(nativeWindow, cUserAgent)
	})
}

func showDreamFMNativeAirPlayPicker(nativeWindow unsafe.Pointer, anchor DreamFMAirPlayAnchor) bool {
	if nativeWindow == nil {
		return false
	}

	var shown C.int
	application.InvokeSync(func() {
		shown = C.dreamFMShowAirPlayRoutePicker(
			nativeWindow,
			C.double(anchor.X),
			C.double(anchor.Y),
			C.double(anchor.Width),
			C.double(anchor.Height),
		)
	})
	return shown != 0
}

func loadDreamFMYouTubeMusicURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}

	data, _ := json.Marshal(cookies)
	cTargetURL := C.CString(targetURL)
	cCookies := C.CString(string(data))
	defer C.free(unsafe.Pointer(cTargetURL))
	defer C.free(unsafe.Pointer(cCookies))

	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		window.SetURL(targetURL)
		return
	}

	application.InvokeSync(func() {
		C.dreamFMLoadYouTubeMusicURL(nativeWindow, cTargetURL, cCookies)
	})
}

func execDreamFMYouTubeMusicJS(window *application.WebviewWindow, script string) {
	if window == nil || script == "" {
		return
	}

	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		window.ExecJS(script)
		return
	}

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	application.InvokeSync(func() {
		C.dreamFMEvaluateYouTubeMusicJavaScript(nativeWindow, cScript)
	})
}

func attachDreamFMYouTubeMusicBridge(_ *application.WebviewWindow, _ string) func() {
	return nil
}
