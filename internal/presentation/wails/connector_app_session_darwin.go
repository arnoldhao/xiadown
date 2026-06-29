//go:build darwin && !ios

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=10.13 -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework Security

#include <dispatch/dispatch.h>
#include <math.h>
#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <Security/Security.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

static WKWebView* connectorAppSessionFindWKWebView(NSView *view);
static const void *connectorAppSessionUIDelegateKey = &connectorAppSessionUIDelegateKey;

@interface XiaDownConnectorAppSessionUIDelegate : NSObject <WKUIDelegate, NSWindowDelegate>
@property(nonatomic, retain) NSMutableArray<NSWindow *> *popupWindows;
@end

@implementation XiaDownConnectorAppSessionUIDelegate
@synthesize popupWindows = _popupWindows;

- (instancetype)init {
	self = [super init];
	if (self != nil) {
		_popupWindows = [[NSMutableArray alloc] init];
	}
	return self;
}

- (void)dealloc {
	NSArray<NSWindow *> *windows = [_popupWindows copy];
	for (NSWindow *window in windows) {
		window.delegate = nil;
		[window close];
	}
	[windows release];
	[_popupWindows release];
	[super dealloc];
}

- (WKWebView *)webView:(WKWebView *)webView
	createWebViewWithConfiguration:(WKWebViewConfiguration *)configuration
	forNavigationAction:(WKNavigationAction *)navigationAction
	windowFeatures:(WKWindowFeatures *)windowFeatures {
	if (navigationAction.targetFrame == nil || !navigationAction.targetFrame.mainFrame) {
		if (configuration == nil) {
			return nil;
		}
		configuration.preferences.javaScriptCanOpenWindowsAutomatically = YES;

		NSWindow *parentWindow = webView.window;
		NSRect parentFrame = parentWindow != nil ? parentWindow.frame : NSMakeRect(0, 0, 560, 720);
		NSRect popupFrame = NSMakeRect(NSMidX(parentFrame) - 280, NSMidY(parentFrame) - 360, 560, 720);
		NSWindow *popupWindow = [[NSWindow alloc]
			initWithContentRect:popupFrame
			styleMask:(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable)
			backing:NSBackingStoreBuffered
			defer:NO];
		popupWindow.title = parentWindow.title.length > 0 ? parentWindow.title : @"Sign In";
		popupWindow.releasedWhenClosed = NO;
		popupWindow.delegate = self;

		WKWebView *popupWebView = [[WKWebView alloc] initWithFrame:popupWindow.contentView.bounds configuration:configuration];
		popupWebView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
		popupWebView.customUserAgent = webView.customUserAgent;
		popupWebView.UIDelegate = self;
		objc_setAssociatedObject(popupWebView, connectorAppSessionUIDelegateKey, self, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
		popupWindow.contentView = popupWebView;
		[popupWebView release];

		[self.popupWindows addObject:popupWindow];
		if (parentWindow != nil) {
			[parentWindow addChildWindow:popupWindow ordered:NSWindowAbove];
		}
		[popupWindow makeKeyAndOrderFront:nil];
		[popupWindow release];
		return popupWebView;
	}
	return nil;
}

- (void)webViewDidClose:(WKWebView *)webView {
	NSWindow *window = webView.window;
	if (window != nil) {
		[window close];
	}
}

- (void)windowWillClose:(NSNotification *)notification {
	NSWindow *window = notification.object;
	if (![window isKindOfClass:[NSWindow class]]) {
		return;
	}
	NSWindow *parentWindow = window.parentWindow;
	if (parentWindow != nil) {
		[parentWindow removeChildWindow:window];
	}
	WKWebView *webView = connectorAppSessionFindWKWebView(window.contentView);
	if (webView != nil) {
		webView.UIDelegate = nil;
		objc_setAssociatedObject(webView, connectorAppSessionUIDelegateKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
	}
	window.delegate = nil;
	[self.popupWindows removeObject:window];
}
@end

static WKWebView* connectorAppSessionFindWKWebView(NSView *view) {
	if (view == nil) {
		return nil;
	}
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView*)view;
	}
	for (NSView *subview in [view subviews]) {
		WKWebView *candidate = connectorAppSessionFindWKWebView(subview);
		if (candidate != nil) {
			return candidate;
		}
	}
	return nil;
}

static WKWebView* connectorAppSessionWebViewForWindow(void *nativeWindow) {
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
	return connectorAppSessionFindWKWebView([window contentView]);
}

static NSString* connectorAppSessionStringValue(id value) {
	if ([value isKindOfClass:[NSString class]]) {
		return (NSString*)value;
	}
	if ([value isKindOfClass:[NSNumber class]]) {
		return [(NSNumber*)value stringValue];
	}
	return nil;
}

static BOOL connectorAppSessionBoolValue(id value) {
	if ([value isKindOfClass:[NSNumber class]]) {
		return [(NSNumber*)value boolValue];
	}
	if ([value isKindOfClass:[NSString class]]) {
		NSString *lower = [(NSString*)value lowercaseString];
		return [lower isEqualToString:@"true"] || [lower isEqualToString:@"1"] || [lower isEqualToString:@"yes"];
	}
	return NO;
}

static NSArray<NSHTTPCookie*>* connectorAppSessionCookiesFromJSON(const char *cookiesJSON, NSURL *targetURL) {
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
	NSString *fallbackDomain = targetURL.host ?: @"youtube.com";
	for (id item in (NSArray*)parsed) {
		if (![item isKindOfClass:[NSDictionary class]]) {
			continue;
		}
		NSDictionary *dictionary = (NSDictionary*)item;
		NSString *name = connectorAppSessionStringValue(dictionary[@"name"]);
		NSString *value = connectorAppSessionStringValue(dictionary[@"value"]);
		NSString *domain = connectorAppSessionStringValue(dictionary[@"domain"]);
		NSString *path = connectorAppSessionStringValue(dictionary[@"path"]);
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
		if (connectorAppSessionBoolValue(dictionary[@"secure"])) {
			properties[NSHTTPCookieSecure] = @"TRUE";
		}
		if (connectorAppSessionBoolValue(dictionary[@"httpOnly"])) {
			properties[(NSHTTPCookiePropertyKey)@"HttpOnly"] = @"TRUE";
		}
		NSString *sameSite = connectorAppSessionStringValue(dictionary[@"sameSite"]);
		if (sameSite.length > 0) {
			properties[(NSHTTPCookiePropertyKey)@"SameSite"] = sameSite;
		}
		NSHTTPCookie *cookie = [NSHTTPCookie cookieWithProperties:properties];
		if (cookie != nil) {
			[cookies addObject:cookie];
		}
	}
	return cookies;
}

static NSData* connectorAppSessionCookiesToJSONData(NSArray<NSHTTPCookie*> *cookies) {
	NSMutableArray *items = [NSMutableArray arrayWithCapacity:cookies.count];
	for (NSHTTPCookie *cookie in cookies) {
		NSMutableDictionary *item = [NSMutableDictionary dictionary];
		item[@"name"] = cookie.name ?: @"";
		item[@"value"] = cookie.value ?: @"";
		item[@"domain"] = cookie.domain ?: @"";
		item[@"path"] = cookie.path ?: @"/";
		item[@"expires"] = cookie.expiresDate != nil ? @((long long)floor([cookie.expiresDate timeIntervalSince1970])) : @(0);
		item[@"httpOnly"] = @([cookie isHTTPOnly]);
		item[@"secure"] = @([cookie isSecure]);
		id sameSite = cookie.properties[(NSHTTPCookiePropertyKey)@"SameSite"];
		if ([sameSite isKindOfClass:[NSString class]]) {
			item[@"sameSite"] = sameSite;
		}
		[items addObject:item];
	}
	NSError *error = nil;
	NSData *data = [NSJSONSerialization dataWithJSONObject:items options:0 error:&error];
	if (error != nil || data == nil) {
		return [@"[]" dataUsingEncoding:NSUTF8StringEncoding];
	}
	return data;
}

static char* connectorAppSessionCopyCStringFromData(NSData *data) {
	if (data == nil) {
		return strdup("");
	}
	char *result = (char*)malloc(data.length + 1);
	if (result == NULL) {
		return NULL;
	}
	memcpy(result, data.bytes, data.length);
	result[data.length] = '\0';
	return result;
}

static void connectorAppSessionConfigureWindow(void *nativeWindow, const char *userAgent) {
	@autoreleasepool {
		WKWebView *webView = connectorAppSessionWebViewForWindow(nativeWindow);
		if (webView == nil) {
			return;
		}
		if (userAgent != NULL) {
			NSString *customUserAgent = [NSString stringWithUTF8String:userAgent];
			if (customUserAgent.length > 0) {
				webView.customUserAgent = customUserAgent;
			}
		}
		webView.configuration.applicationNameForUserAgent = @"";
		webView.configuration.preferences.javaScriptCanOpenWindowsAutomatically = YES;
		XiaDownConnectorAppSessionUIDelegate *delegate = [[XiaDownConnectorAppSessionUIDelegate alloc] init];
		webView.UIDelegate = delegate;
		objc_setAssociatedObject(webView, connectorAppSessionUIDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
		[delegate release];
	}
}

static void connectorAppSessionLoadURL(void *nativeWindow, const char *targetURL) {
	@autoreleasepool {
		WKWebView *webView = connectorAppSessionWebViewForWindow(nativeWindow);
		if (webView == nil || targetURL == NULL) {
			return;
		}
		NSString *urlString = [NSString stringWithUTF8String:targetURL];
		NSURL *url = [NSURL URLWithString:urlString];
		if (url == nil) {
			return;
		}
		[webView loadRequest:[NSURLRequest requestWithURL:url]];
	}
}

static void connectorAppSessionSetCookies(void *nativeWindow, const char *targetURL, const char *cookiesJSON) {
	dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
	void (^work)(void) = ^{
		@autoreleasepool {
			WKWebView *webView = connectorAppSessionWebViewForWindow(nativeWindow);
			if (webView == nil || targetURL == NULL || cookiesJSON == NULL) {
				dispatch_semaphore_signal(semaphore);
				return;
			}
			NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:targetURL]];
			NSArray<NSHTTPCookie*> *cookies = connectorAppSessionCookiesFromJSON(cookiesJSON, url);
			WKHTTPCookieStore *cookieStore = webView.configuration.websiteDataStore.httpCookieStore;
			if (cookieStore == nil || cookies.count == 0) {
				dispatch_semaphore_signal(semaphore);
				return;
			}
			__block NSInteger pending = cookies.count;
			for (NSHTTPCookie *cookie in cookies) {
				[cookieStore setCookie:cookie completionHandler:^{
					pending -= 1;
					if (pending <= 0) {
						dispatch_semaphore_signal(semaphore);
					}
				}];
			}
		}
	};
	if ([NSThread isMainThread]) {
		work();
		return;
	}
	dispatch_async(dispatch_get_main_queue(), work);
	dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
}

static char* connectorAppSessionReadCookiesJSON(void *nativeWindow) {
	__block char *result = NULL;
	dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			WKWebView *webView = connectorAppSessionWebViewForWindow(nativeWindow);
			if (webView == nil || webView.configuration.websiteDataStore.httpCookieStore == nil) {
				result = strdup("[]");
				dispatch_semaphore_signal(semaphore);
				return;
			}
			[webView.configuration.websiteDataStore.httpCookieStore getAllCookies:^(NSArray<NSHTTPCookie *> *cookies) {
				NSData *data = connectorAppSessionCookiesToJSONData(cookies);
				result = connectorAppSessionCopyCStringFromData(data);
				dispatch_semaphore_signal(semaphore);
			}];
		}
	});
	dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
	return result != NULL ? result : strdup("[]");
}

static BOOL connectorAppSessionHostMatchesSuffix(NSString *host, NSString *suffix) {
	if (host.length == 0 || suffix.length == 0) {
		return NO;
	}
	NSString *normalizedHost = [[host lowercaseString] stringByTrimmingCharactersInSet:[NSCharacterSet characterSetWithCharactersInString:@"."]];
	NSString *normalizedSuffix = [[suffix lowercaseString] stringByTrimmingCharactersInSet:[NSCharacterSet characterSetWithCharactersInString:@"."]];
	return [normalizedHost isEqualToString:normalizedSuffix] ||
		[normalizedHost hasSuffix:[@"." stringByAppendingString:normalizedSuffix]];
}

static BOOL connectorAppSessionShouldClearHost(NSString *host) {
	if (host.length == 0) {
		return NO;
	}
	NSArray<NSString*> *suffixes = @[
		@"youtube.com",
		@"music.youtube.com",
		@"youtu.be",
		@"youtube-nocookie.com",
		@"google.com",
		@"accounts.google.com",
		@"googleusercontent.com",
		@"gstatic.com",
		@"ytimg.com",
	];
	for (NSString *suffix in suffixes) {
		if (connectorAppSessionHostMatchesSuffix(host, suffix)) {
			return YES;
		}
	}
	return NO;
}

static NSArray<NSString*>* connectorAppSessionClearSuffixesFromJSON(const char *domainsJSON) {
	if (domainsJSON == NULL) {
		return @[];
	}
	NSString *json = [NSString stringWithUTF8String:domainsJSON];
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
	NSMutableArray<NSString*> *suffixes = [NSMutableArray array];
	for (id item in (NSArray*)parsed) {
		NSString *suffix = connectorAppSessionStringValue(item);
		if (suffix.length > 0) {
			[suffixes addObject:suffix];
		}
	}
	return suffixes;
}

static BOOL connectorAppSessionShouldClearHostForSuffixes(NSString *host, NSArray<NSString*> *suffixes) {
	if (host.length == 0 || suffixes.count == 0) {
		return NO;
	}
	for (NSString *suffix in suffixes) {
		if (connectorAppSessionHostMatchesSuffix(host, suffix)) {
			return YES;
		}
	}
	return NO;
}

static void connectorAppSessionClearWebsiteData(void) {
	dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
	void (^work)(void) = ^{
		@autoreleasepool {
			WKWebsiteDataStore *dataStore = [WKWebsiteDataStore defaultDataStore];
			if (dataStore == nil) {
				dispatch_semaphore_signal(semaphore);
				return;
			}
			dispatch_group_t group = dispatch_group_create();

			dispatch_group_enter(group);
			[dataStore.httpCookieStore getAllCookies:^(NSArray<NSHTTPCookie *> *cookies) {
				dispatch_group_t cookieGroup = dispatch_group_create();
				for (NSHTTPCookie *cookie in cookies) {
					if (!connectorAppSessionShouldClearHost(cookie.domain)) {
						continue;
					}
					dispatch_group_enter(cookieGroup);
					[dataStore.httpCookieStore deleteCookie:cookie completionHandler:^{
						dispatch_group_leave(cookieGroup);
					}];
				}
				dispatch_group_notify(cookieGroup, dispatch_get_main_queue(), ^{
					dispatch_group_leave(group);
				});
			}];

			NSSet<NSString*> *dataTypes = [WKWebsiteDataStore allWebsiteDataTypes];
			dispatch_group_enter(group);
			[dataStore fetchDataRecordsOfTypes:dataTypes completionHandler:^(NSArray<WKWebsiteDataRecord *> *records) {
				NSMutableArray<WKWebsiteDataRecord *> *matchingRecords = [NSMutableArray array];
				for (WKWebsiteDataRecord *record in records) {
					if (connectorAppSessionShouldClearHost(record.displayName)) {
						[matchingRecords addObject:record];
					}
				}
				if (matchingRecords.count == 0) {
					dispatch_group_leave(group);
					return;
				}
				[dataStore removeDataOfTypes:dataTypes forDataRecords:matchingRecords completionHandler:^{
					dispatch_group_leave(group);
				}];
			}];

			dispatch_group_notify(group, dispatch_get_main_queue(), ^{
				dispatch_semaphore_signal(semaphore);
			});
		}
	};
	if ([NSThread isMainThread]) {
		work();
		return;
	} else {
		dispatch_async(dispatch_get_main_queue(), work);
	}
	dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
}

static void connectorAppSessionClearWebsiteDataForDomains(const char *domainsJSON) {
	NSArray<NSString*> *suffixes = connectorAppSessionClearSuffixesFromJSON(domainsJSON);
	if (suffixes.count == 0) {
		return;
	}
	dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
	void (^work)(void) = ^{
		@autoreleasepool {
			WKWebsiteDataStore *dataStore = [WKWebsiteDataStore defaultDataStore];
			if (dataStore == nil) {
				dispatch_semaphore_signal(semaphore);
				return;
			}
			dispatch_group_t group = dispatch_group_create();

			dispatch_group_enter(group);
			[dataStore.httpCookieStore getAllCookies:^(NSArray<NSHTTPCookie *> *cookies) {
				dispatch_group_t cookieGroup = dispatch_group_create();
				for (NSHTTPCookie *cookie in cookies) {
					if (!connectorAppSessionShouldClearHostForSuffixes(cookie.domain, suffixes)) {
						continue;
					}
					dispatch_group_enter(cookieGroup);
					[dataStore.httpCookieStore deleteCookie:cookie completionHandler:^{
						dispatch_group_leave(cookieGroup);
					}];
				}
				dispatch_group_notify(cookieGroup, dispatch_get_main_queue(), ^{
					dispatch_group_leave(group);
				});
			}];

			NSSet<NSString*> *dataTypes = [WKWebsiteDataStore allWebsiteDataTypes];
			dispatch_group_enter(group);
			[dataStore fetchDataRecordsOfTypes:dataTypes completionHandler:^(NSArray<WKWebsiteDataRecord *> *records) {
				NSMutableArray<WKWebsiteDataRecord *> *matchingRecords = [NSMutableArray array];
				for (WKWebsiteDataRecord *record in records) {
					if (connectorAppSessionShouldClearHostForSuffixes(record.displayName, suffixes)) {
						[matchingRecords addObject:record];
					}
				}
				if (matchingRecords.count == 0) {
					dispatch_group_leave(group);
					return;
				}
				[dataStore removeDataOfTypes:dataTypes forDataRecords:matchingRecords completionHandler:^{
					dispatch_group_leave(group);
				}];
			}];

			dispatch_group_notify(group, dispatch_get_main_queue(), ^{
				dispatch_semaphore_signal(semaphore);
			});
		}
	};
	if ([NSThread isMainThread]) {
		work();
		return;
	} else {
		dispatch_async(dispatch_get_main_queue(), work);
	}
	dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
}

static NSString* connectorAppSessionKeychainService(void) {
	return @"com.dreamapp.xiadown.connector-app-session";
}

static int connectorAppSessionSaveKeychain(const char *accountValue, const char *jsonValue) {
	@autoreleasepool {
		if (accountValue == NULL || jsonValue == NULL) {
			return (int)errSecParam;
		}
		NSString *account = [NSString stringWithUTF8String:accountValue];
		NSData *data = [[NSString stringWithUTF8String:jsonValue] dataUsingEncoding:NSUTF8StringEncoding];
		if (account.length == 0 || data == nil) {
			return (int)errSecParam;
		}
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: connectorAppSessionKeychainService(),
			(id)kSecAttrAccount: account,
		};
		NSDictionary *attributes = @{
			(id)kSecValueData: data,
			(id)kSecAttrAccessible: (id)kSecAttrAccessibleWhenUnlocked,
		};
		OSStatus status = SecItemUpdate((CFDictionaryRef)query, (CFDictionaryRef)attributes);
		if (status == errSecItemNotFound) {
			NSMutableDictionary *newItem = [query mutableCopy];
			[newItem addEntriesFromDictionary:attributes];
			status = SecItemAdd((CFDictionaryRef)newItem, nil);
			[newItem release];
		}
		return (int)status;
	}
}

static char* connectorAppSessionLoadKeychain(const char *accountValue) {
	@autoreleasepool {
		if (accountValue == NULL) {
			return NULL;
		}
		NSString *account = [NSString stringWithUTF8String:accountValue];
		if (account.length == 0) {
			return NULL;
		}
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: connectorAppSessionKeychainService(),
			(id)kSecAttrAccount: account,
			(id)kSecReturnData: @YES,
			(id)kSecMatchLimit: (id)kSecMatchLimitOne,
		};
		CFTypeRef result = NULL;
		OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
		if (status != errSecSuccess || result == NULL) {
			if (result != NULL) {
				CFRelease(result);
			}
			return NULL;
		}
		NSData *data = [(NSData*)result retain];
		CFRelease(result);
		char *out = connectorAppSessionCopyCStringFromData(data);
		[data release];
		return out;
	}
}

static int connectorAppSessionDeleteKeychain(const char *accountValue) {
	@autoreleasepool {
		if (accountValue == NULL) {
			return (int)errSecParam;
		}
		NSString *account = [NSString stringWithUTF8String:accountValue];
		if (account.length == 0) {
			return (int)errSecParam;
		}
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: connectorAppSessionKeychainService(),
			(id)kSecAttrAccount: account,
		};
		OSStatus status = SecItemDelete((CFDictionaryRef)query);
		return status == errSecItemNotFound ? (int)errSecSuccess : (int)status;
	}
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func connectorAppSessionNativeSupported() bool {
	return true
}

func connectorAppSessionCaptureBeforeClose() bool {
	return true
}

func prepareConnectorAppSessionNativeWindow(window *application.WebviewWindow, targetURL string, siteKey string, records []appcookies.Record, _ []string) {
	if window == nil {
		return
	}
	nativeWindow := window.NativeWindow()
	configureConnectorAppSessionNativeWindow(nativeWindow, appSessionWebViewUserAgent(siteKey))
	if len(records) > 0 {
		setConnectorAppSessionNativeCookies(nativeWindow, targetURL, records)
	}
}

func configureConnectorAppSessionNativeWindow(nativeWindow unsafe.Pointer, userAgent string) {
	if nativeWindow == nil {
		return
	}
	cUserAgent := C.CString(userAgent)
	defer C.free(unsafe.Pointer(cUserAgent))
	application.InvokeSync(func() {
		C.connectorAppSessionConfigureWindow(nativeWindow, cUserAgent)
	})
}

func loadConnectorAppSessionNativeURL(window *application.WebviewWindow, targetURL string) {
	if window == nil || targetURL == "" {
		return
	}
	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		window.SetURL(targetURL)
		return
	}
	cTargetURL := C.CString(targetURL)
	defer C.free(unsafe.Pointer(cTargetURL))
	application.InvokeSync(func() {
		C.connectorAppSessionLoadURL(nativeWindow, cTargetURL)
	})
}

func setConnectorAppSessionNativeCookies(nativeWindow unsafe.Pointer, targetURL string, records []appcookies.Record) {
	if nativeWindow == nil || targetURL == "" || len(records) == 0 {
		return
	}
	data, _ := json.Marshal(records)
	cTargetURL := C.CString(targetURL)
	cCookies := C.CString(string(data))
	defer C.free(unsafe.Pointer(cTargetURL))
	defer C.free(unsafe.Pointer(cCookies))
	C.connectorAppSessionSetCookies(nativeWindow, cTargetURL, cCookies)
}

func readConnectorAppSessionNativeCookies(ctx context.Context, nativeWindow unsafe.Pointer) ([]appcookies.Record, error) {
	if nativeWindow == nil {
		return nil, appsessions.ErrSessionDead
	}
	done := make(chan struct{})
	var raw *C.char
	go func() {
		raw = C.connectorAppSessionReadCookiesJSON(nativeWindow)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}
	if raw == nil {
		return nil, appsessions.ErrNoCookies
	}
	defer C.free(unsafe.Pointer(raw))
	return appcookies.DecodeJSON(C.GoString(raw)), nil
}

func readConnectorAppSessionNativeWindowCookies(ctx context.Context, window *application.WebviewWindow, _ []string) ([]appcookies.Record, error) {
	if window == nil {
		return nil, appsessions.ErrSessionDead
	}
	return readConnectorAppSessionNativeCookies(ctx, window.NativeWindow())
}

func saveSiteAppSessionStoredCookies(siteKey string, records []appcookies.Record) error {
	if len(records) == 0 {
		return appsessions.ErrNoCookies
	}
	data, err := appcookies.EncodeJSON(records)
	if err != nil {
		return err
	}
	cAccount := C.CString(siteAppSessionAccount(siteKey))
	cData := C.CString(data)
	defer C.free(unsafe.Pointer(cAccount))
	defer C.free(unsafe.Pointer(cData))
	status := C.connectorAppSessionSaveKeychain(cAccount, cData)
	if status != 0 {
		return fmt.Errorf("save connector app session keychain: status %d", int(status))
	}
	return nil
}

func loadSiteAppSessionStoredCookies(siteKey string) ([]appcookies.Record, error) {
	cAccount := C.CString(siteAppSessionAccount(siteKey))
	defer C.free(unsafe.Pointer(cAccount))
	raw := C.connectorAppSessionLoadKeychain(cAccount)
	if raw == nil {
		return nil, appsessions.ErrNoCookies
	}
	defer C.free(unsafe.Pointer(raw))
	records := appcookies.DecodeJSON(C.GoString(raw))
	if len(records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func clearSiteAppSessionStoredCookies(siteKey string, domains []string) error {
	cAccount := C.CString(siteAppSessionAccount(siteKey))
	defer C.free(unsafe.Pointer(cAccount))
	status := C.connectorAppSessionDeleteKeychain(cAccount)
	if status != 0 {
		return fmt.Errorf("delete connector app session keychain: status %d", int(status))
	}
	if len(domains) > 0 {
		data, err := json.Marshal(domains)
		if err == nil {
			cDomains := C.CString(string(data))
			C.connectorAppSessionClearWebsiteDataForDomains(cDomains)
			C.free(unsafe.Pointer(cDomains))
		}
	} else {
		C.connectorAppSessionClearWebsiteData()
	}
	return nil
}

func clearConnectorAppSessionNativeRuntimeData(_ context.Context, _ *application.App, _ string, _ []string) error {
	return nil
}
