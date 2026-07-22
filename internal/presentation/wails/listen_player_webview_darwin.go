//go:build darwin && !ios

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework AVKit

#include <math.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#import <AVKit/AVKit.h>
#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

static NSView *listenActiveAirPlayPicker = nil;
static NSInteger listenAirPlayPickerGeneration = 0;
static const void *listenNavigationGenerationKey = &listenNavigationGenerationKey;
static const void *listenRSSBilibiliNavigationPolicyKey = &listenRSSBilibiliNavigationPolicyKey;
static const void *listenRSSBilibiliDocumentStartScriptKey = &listenRSSBilibiliDocumentStartScriptKey;
static const void *listenRSSSiteNavigationPolicyKey = &listenRSSSiteNavigationPolicyKey;
static id listenRSSBilibiliFullscreenEscapeMonitor = nil;
static NSWindow *listenRSSBilibiliFullscreenEscapeWindow = nil;
static id listenRSSBilibiliFullscreenWillEnterObserver = nil;
static id listenRSSBilibiliFullscreenDidEnterObserver = nil;
static id listenRSSBilibiliFullscreenDidExitObserver = nil;
static NSUInteger listenRSSBilibiliFullscreenEscapeGeneration = 0;
static NSUInteger listenRSSBilibiliFullscreenEnteringGeneration = 0;
static BOOL listenRSSBilibiliFullscreenEscapeExitRequested = NO;
static BOOL listenRSSBilibiliFullscreenEscapeEntering = NO;

static void listenRSSBilibiliWaitForFullscreenEscapeExit(
	NSWindow *window,
	NSUInteger generation,
	NSUInteger attempt
) {
	if (window == nil || window != listenRSSBilibiliFullscreenEscapeWindow ||
		generation != listenRSSBilibiliFullscreenEscapeGeneration) {
		return;
	}
	if ((window.styleMask & NSWindowStyleMaskFullScreen) == 0 || attempt >= 80) {
		listenRSSBilibiliFullscreenEscapeExitRequested = NO;
		return;
	}
	dispatch_after(
		dispatch_time(DISPATCH_TIME_NOW, (int64_t)(50 * NSEC_PER_MSEC)),
		dispatch_get_main_queue(),
		^{ listenRSSBilibiliWaitForFullscreenEscapeExit(window, generation, attempt + 1); }
	);
}

static void listenRSSBilibiliAttemptFullscreenEscapeExit(
	NSWindow *window,
	NSUInteger generation,
	NSUInteger attempt
) {
	if (window == nil || window != listenRSSBilibiliFullscreenEscapeWindow ||
		generation != listenRSSBilibiliFullscreenEscapeGeneration) {
		return;
	}
	if ((window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
		[window toggleFullScreen:nil];
		listenRSSBilibiliWaitForFullscreenEscapeExit(window, generation, 0);
		return;
	}
	if (attempt >= 80) {
		listenRSSBilibiliFullscreenEscapeExitRequested = NO;
		return;
	}
	// Escape can arrive while AppKit's enter animation is still in flight.
	// Wait for the native fullscreen style instead of depending on the remote
	// document, which may navigate or rebuild its player during that interval.
	dispatch_after(
		dispatch_time(DISPATCH_TIME_NOW, (int64_t)(50 * NSEC_PER_MSEC)),
		dispatch_get_main_queue(),
		^{ listenRSSBilibiliAttemptFullscreenEscapeExit(window, generation, attempt + 1); }
	);
}

static void listenRemoveRSSBilibiliFullscreenEscapeMonitor(void *nativeWindow) {
	@autoreleasepool {
		NSWindow *window = (NSWindow*)nativeWindow;
		if (window != nil && listenRSSBilibiliFullscreenEscapeWindow != window) {
			return;
		}
		listenRSSBilibiliFullscreenEscapeGeneration += 1;
		listenRSSBilibiliFullscreenEnteringGeneration += 1;
		listenRSSBilibiliFullscreenEscapeExitRequested = NO;
		listenRSSBilibiliFullscreenEscapeEntering = NO;
		if (listenRSSBilibiliFullscreenEscapeMonitor != nil) {
			[NSEvent removeMonitor:listenRSSBilibiliFullscreenEscapeMonitor];
			listenRSSBilibiliFullscreenEscapeMonitor = nil;
		}
		NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
		if (listenRSSBilibiliFullscreenWillEnterObserver != nil) {
			[center removeObserver:listenRSSBilibiliFullscreenWillEnterObserver];
			listenRSSBilibiliFullscreenWillEnterObserver = nil;
		}
		if (listenRSSBilibiliFullscreenDidEnterObserver != nil) {
			[center removeObserver:listenRSSBilibiliFullscreenDidEnterObserver];
			listenRSSBilibiliFullscreenDidEnterObserver = nil;
		}
		if (listenRSSBilibiliFullscreenDidExitObserver != nil) {
			[center removeObserver:listenRSSBilibiliFullscreenDidExitObserver];
			listenRSSBilibiliFullscreenDidExitObserver = nil;
		}
		listenRSSBilibiliFullscreenEscapeWindow = nil;
	}
}

static int listenInstallRSSBilibiliFullscreenEscapeMonitor(void *nativeWindow) {
	@autoreleasepool {
		NSWindow *window = (NSWindow*)nativeWindow;
		if (window == nil) {
			return 0;
		}
		listenRemoveRSSBilibiliFullscreenEscapeMonitor(NULL);
		listenRSSBilibiliFullscreenEscapeWindow = window;
		listenRSSBilibiliFullscreenEscapeGeneration += 1;
		NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
		listenRSSBilibiliFullscreenWillEnterObserver = [center
			addObserverForName:NSWindowWillEnterFullScreenNotification
			object:window
			queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *notification) {
				if (notification.object == listenRSSBilibiliFullscreenEscapeWindow) {
					listenRSSBilibiliFullscreenEscapeEntering = YES;
					listenRSSBilibiliFullscreenEnteringGeneration += 1;
					NSUInteger generation = listenRSSBilibiliFullscreenEnteringGeneration;
					dispatch_after(
						dispatch_time(DISPATCH_TIME_NOW, (int64_t)(5 * NSEC_PER_SEC)),
						dispatch_get_main_queue(),
						^{
							if (window == listenRSSBilibiliFullscreenEscapeWindow &&
								generation == listenRSSBilibiliFullscreenEnteringGeneration &&
								(window.styleMask & NSWindowStyleMaskFullScreen) == 0) {
								listenRSSBilibiliFullscreenEscapeEntering = NO;
							}
						}
					);
				}
			}
		];
		listenRSSBilibiliFullscreenDidEnterObserver = [center
			addObserverForName:NSWindowDidEnterFullScreenNotification
			object:window
			queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *notification) {
				if (notification.object == listenRSSBilibiliFullscreenEscapeWindow) {
					listenRSSBilibiliFullscreenEnteringGeneration += 1;
					listenRSSBilibiliFullscreenEscapeEntering = NO;
				}
			}
		];
		listenRSSBilibiliFullscreenDidExitObserver = [center
			addObserverForName:NSWindowDidExitFullScreenNotification
			object:window
			queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *notification) {
				if (notification.object == listenRSSBilibiliFullscreenEscapeWindow) {
					listenRSSBilibiliFullscreenEnteringGeneration += 1;
					listenRSSBilibiliFullscreenEscapeEntering = NO;
					listenRSSBilibiliFullscreenEscapeExitRequested = NO;
				}
			}
		];
		listenRSSBilibiliFullscreenEscapeMonitor = [NSEvent
			addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown
			handler:^NSEvent *(NSEvent *event) {
				NSWindow *target = listenRSSBilibiliFullscreenEscapeWindow;
				if (event.keyCode != 53 || target == nil ||
					(event.window != target && NSApp.keyWindow != target)) {
					return event;
				}
				BOOL fullscreen = (target.styleMask & NSWindowStyleMaskFullScreen) != 0;
				if (!fullscreen && !listenRSSBilibiliFullscreenEscapeEntering) {
					// Keep ordinary embedded-window Escape semantics untouched. Only
					// consume the key once AppKit owns (or is entering) fullscreen.
					return event;
				}
				if (!listenRSSBilibiliFullscreenEscapeExitRequested) {
					listenRSSBilibiliFullscreenEscapeExitRequested = YES;
					listenRSSBilibiliFullscreenEscapeGeneration += 1;
					listenRSSBilibiliAttemptFullscreenEscapeExit(
						target,
						listenRSSBilibiliFullscreenEscapeGeneration,
						0
					);
				}
				return nil;
			}
		];
		if (listenRSSBilibiliFullscreenEscapeMonitor == nil) {
			listenRemoveRSSBilibiliFullscreenEscapeMonitor(nativeWindow);
			return 0;
		}
		return 1;
	}
}

static WKWebView* listenFindWKWebView(NSView *view) {
	if (view == nil) {
		return nil;
	}
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView*)view;
	}
	for (NSView *subview in [view subviews]) {
		WKWebView *candidate = listenFindWKWebView(subview);
		if (candidate != nil) {
			return candidate;
		}
	}
	return nil;
}

static NSScrollView* listenFindNSScrollView(NSView *view) {
	if (view == nil) {
		return nil;
	}
	if ([view isKindOfClass:[NSScrollView class]]) {
		return (NSScrollView*)view;
	}
	for (NSView *subview in [view subviews]) {
		NSScrollView *candidate = listenFindNSScrollView(subview);
		if (candidate != nil) {
			return candidate;
		}
	}
	return nil;
}

static WKWebView* listenWebViewForWindow(void *nativeWindow) {
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

	return listenFindWKWebView([window contentView]);
}

static BOOL listenRSSBilibiliIsPositiveDecimalIdentifier(
	NSString *identifier,
	NSString *prefix
) {
	if (identifier.length <= prefix.length ||
		[[identifier substringToIndex:prefix.length] caseInsensitiveCompare:prefix] != NSOrderedSame) {
		return NO;
	}
	unichar firstDigit = [identifier characterAtIndex:prefix.length];
	if (firstDigit < '1' || firstDigit > '9') {
		return NO;
	}
	for (NSUInteger index = prefix.length + 1; index < identifier.length; index++) {
		unichar character = [identifier characterAtIndex:index];
		if (character < '0' || character > '9') {
			return NO;
		}
	}
	return YES;
}

// Bilibili exposes two playback adapters on the same trusted origin. Ordinary
// videos use /video/BV... or /video/av..., while PGC/Bangumi playback uses
// /bangumi/play/ep... or /bangumi/play/ss.... Keep both grammars explicit so
// broad same-origin navigation never becomes an implicit third adapter.
static BOOL listenRSSBilibiliIsCanonicalVideoPath(NSString *path) {
	if (path.length == 0) {
		return NO;
	}
	NSString *normalized = path;
	if (normalized.length > 1 && [normalized hasSuffix:@"/"]) {
		normalized = [normalized substringToIndex:normalized.length - 1];
	}
	NSArray<NSString*> *components = [normalized componentsSeparatedByString:@"/"];
	if (components.count == 4 && components[0].length == 0 &&
		[components[1] isEqualToString:@"bangumi"] &&
		[components[2] isEqualToString:@"play"]) {
		NSString *bangumiID = components[3];
		return listenRSSBilibiliIsPositiveDecimalIdentifier(bangumiID, @"ep") ||
			listenRSSBilibiliIsPositiveDecimalIdentifier(bangumiID, @"ss");
	}
	if (components.count != 3 || components[0].length != 0 ||
		![components[1] isEqualToString:@"video"]) {
		return NO;
	}
	NSString *videoID = components[2];
	if (videoID.length == 12 &&
		[[videoID substringToIndex:2] caseInsensitiveCompare:@"BV"] == NSOrderedSame) {
		for (NSUInteger index = 2; index < videoID.length; index++) {
			unichar character = [videoID characterAtIndex:index];
			BOOL digit = character >= '0' && character <= '9';
			BOOL lower = character >= 'a' && character <= 'z';
			BOOL upper = character >= 'A' && character <= 'Z';
			if (!digit && !lower && !upper) {
				return NO;
			}
		}
		return YES;
	}
	if (listenRSSBilibiliIsPositiveDecimalIdentifier(videoID, @"av")) {
		return YES;
	}
	return NO;
}

static BOOL listenRSSBilibiliIsTrustedVideoPageURL(NSURL *url) {
	if (url == nil) {
		return NO;
	}
	if (![url.scheme.lowercaseString isEqualToString:@"https"] ||
		![url.host.lowercaseString isEqualToString:@"www.bilibili.com"] ||
		url.user.length > 0 || url.password.length > 0) {
		return NO;
	}
	NSNumber *port = url.port;
	NSURLComponents *components = [NSURLComponents componentsWithURL:url resolvingAgainstBaseURL:NO];
	return (port == nil || port.integerValue == 443) &&
		components != nil && listenRSSBilibiliIsCanonicalVideoPath(components.percentEncodedPath);
}

static NSString* listenRSSBilibiliVideoIDForURL(NSURL *url) {
	if (!listenRSSBilibiliIsTrustedVideoPageURL(url)) {
		return nil;
	}
	NSURLComponents *components = [NSURLComponents componentsWithURL:url resolvingAgainstBaseURL:NO];
	NSString *path = components.percentEncodedPath;
	if (path.length > 1 && [path hasSuffix:@"/"]) {
		path = [path substringToIndex:path.length - 1];
	}
	NSArray<NSString*> *parts = [path componentsSeparatedByString:@"/"];
	NSString *videoID = nil;
	if (parts.count == 3 && [parts[1] isEqualToString:@"video"]) {
		videoID = parts[2];
	} else if (parts.count == 4 && [parts[1] isEqualToString:@"bangumi"] &&
		[parts[2] isEqualToString:@"play"]) {
		videoID = parts[3];
	} else {
		return nil;
	}
	if (videoID.length >= 2 &&
		[[videoID substringToIndex:2] caseInsensitiveCompare:@"BV"] == NSOrderedSame) {
		return [@"BV" stringByAppendingString:[videoID substringFromIndex:2]];
	}
	if (videoID.length >= 2 &&
		[[videoID substringToIndex:2] caseInsensitiveCompare:@"av"] == NSOrderedSame) {
		return [@"av" stringByAppendingString:[videoID substringFromIndex:2]];
	}
	if (videoID.length >= 2 &&
		[[videoID substringToIndex:2] caseInsensitiveCompare:@"ep"] == NSOrderedSame) {
		return [@"ep" stringByAppendingString:[videoID substringFromIndex:2]];
	}
	if (videoID.length >= 2 &&
		[[videoID substringToIndex:2] caseInsensitiveCompare:@"ss"] == NSOrderedSame) {
		return [@"ss" stringByAppendingString:[videoID substringFromIndex:2]];
	}
	return nil;
}

static BOOL listenRSSBilibiliAllowsTopLevelURL(NSURL *url, NSString *expectedVideoID) {
	NSString *videoID = listenRSSBilibiliVideoIDForURL(url);
	return videoID.length > 0 && expectedVideoID.length > 0 &&
		[videoID isEqualToString:expectedVideoID];
}

// WKUserContentController normally injects the bridge at document start. Keep
// a trusted-document fallback in case a framework reconfiguration drops that
// registration before the remote navigation commits. The fallback never runs
// in about:blank or an off-origin redirect, and the bridge itself is session
// idempotent, so this cannot broaden the App Session messaging surface or
// reset a player that was installed successfully at document start.
static void listenEvaluateRSSBilibiliAssociatedBridgeForTrustedDocument(
	WKWebView *webView,
	NSString *expectedVideoID
) {
	if (webView == nil ||
		!listenRSSBilibiliAllowsTopLevelURL(webView.URL, expectedVideoID)) {
		return;
	}
	NSString *source = (NSString*)objc_getAssociatedObject(
		webView,
		listenRSSBilibiliDocumentStartScriptKey
	);
	if (source.length == 0) {
		return;
	}
	[webView evaluateJavaScript:source completionHandler:nil];
}

// Wails owns the original delegates, so this proxy forwards every callback
// except the two capabilities that can escape a playback-only WebView:
// replacing its top-level document and creating an in-App child WebView.
@interface ListenRSSBilibiliNavigationPolicy : NSObject <WKNavigationDelegate, WKUIDelegate>
@property(nonatomic, assign) id<WKNavigationDelegate> listenForwardedNavigationDelegate;
@property(nonatomic, assign) id<WKUIDelegate> listenForwardedUIDelegate;
@property(nonatomic, copy) NSString *listenExpectedVideoID;
@end

@implementation ListenRSSBilibiliNavigationPolicy
@synthesize listenForwardedNavigationDelegate = _listenForwardedNavigationDelegate;
@synthesize listenForwardedUIDelegate = _listenForwardedUIDelegate;
@synthesize listenExpectedVideoID = _listenExpectedVideoID;

- (void)dealloc {
	[_listenExpectedVideoID release];
	[super dealloc];
}

- (BOOL)respondsToSelector:(SEL)selector {
	return [super respondsToSelector:selector] ||
		[self.listenForwardedNavigationDelegate respondsToSelector:selector] ||
		[self.listenForwardedUIDelegate respondsToSelector:selector];
}

- (id)forwardingTargetForSelector:(SEL)selector {
	if ([self.listenForwardedNavigationDelegate respondsToSelector:selector]) {
		return self.listenForwardedNavigationDelegate;
	}
	if ([self.listenForwardedUIDelegate respondsToSelector:selector]) {
		return self.listenForwardedUIDelegate;
	}
	return [super forwardingTargetForSelector:selector];
}

- (void)listenForwardAllowedNavigationForWebView:(WKWebView*)webView
	                                  action:(WKNavigationAction*)navigationAction
	                         decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
	id<WKNavigationDelegate> delegate = self.listenForwardedNavigationDelegate;
	if (delegate != nil && [delegate respondsToSelector:@selector(webView:decidePolicyForNavigationAction:decisionHandler:)]) {
		[delegate webView:webView decidePolicyForNavigationAction:navigationAction decisionHandler:decisionHandler];
		return;
	}
	decisionHandler(WKNavigationActionPolicyAllow);
}

- (void)webView:(WKWebView*)webView
	decidePolicyForNavigationAction:(WKNavigationAction*)navigationAction
	decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
	// targetFrame == nil is WebKit's window.open / target=_blank signal. It
	// must be cancelled before Wails can materialize and focus a sibling window.
	if (navigationAction.targetFrame == nil) {
		decisionHandler(WKNavigationActionPolicyCancel);
		return;
	}
	if (!navigationAction.targetFrame.mainFrame ||
		listenRSSBilibiliAllowsTopLevelURL(
			navigationAction.request.URL,
			self.listenExpectedVideoID
		)) {
		[self listenForwardAllowedNavigationForWebView:webView action:navigationAction decisionHandler:decisionHandler];
		return;
	}
	decisionHandler(WKNavigationActionPolicyCancel);
}

- (void)webView:(WKWebView*)webView didCommitNavigation:(WKNavigation*)navigation {
	listenEvaluateRSSBilibiliAssociatedBridgeForTrustedDocument(
		webView,
		self.listenExpectedVideoID
	);
	id<WKNavigationDelegate> delegate = self.listenForwardedNavigationDelegate;
	if (delegate != nil && [delegate respondsToSelector:@selector(webView:didCommitNavigation:)]) {
		[delegate webView:webView didCommitNavigation:navigation];
	}
}

- (void)webView:(WKWebView*)webView didFinishNavigation:(WKNavigation*)navigation {
	listenEvaluateRSSBilibiliAssociatedBridgeForTrustedDocument(
		webView,
		self.listenExpectedVideoID
	);
	id<WKNavigationDelegate> delegate = self.listenForwardedNavigationDelegate;
	if (delegate != nil && [delegate respondsToSelector:@selector(webView:didFinishNavigation:)]) {
		[delegate webView:webView didFinishNavigation:navigation];
	}
}

- (WKWebView*)webView:(WKWebView*)webView
	createWebViewWithConfiguration:(WKWebViewConfiguration*)configuration
	forNavigationAction:(WKNavigationAction*)navigationAction
	windowFeatures:(WKWindowFeatures*)windowFeatures {
	// The canonical page may request an external login, ad, or recommendation
	// window. Returning nil keeps App Session cookies in this WebView and stops
	// a sibling window from stealing focus from the station.
	return nil;
}
@end

static int listenInstallRSSBilibiliNavigationPolicy(
	void *nativeWindow,
	const char *expectedVideoID
) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		NSString *expected = expectedVideoID == NULL
			? nil
			: [NSString stringWithUTF8String:expectedVideoID];
		if (webView == nil || expected.length == 0) {
			return 0;
		}
		ListenRSSBilibiliNavigationPolicy *policy =
			(ListenRSSBilibiliNavigationPolicy*)objc_getAssociatedObject(webView, listenRSSBilibiliNavigationPolicyKey);
		if (policy == nil) {
			policy = [[ListenRSSBilibiliNavigationPolicy alloc] init];
			policy.listenForwardedNavigationDelegate = webView.navigationDelegate;
			policy.listenForwardedUIDelegate = webView.UIDelegate;
			objc_setAssociatedObject(
				webView,
				listenRSSBilibiliNavigationPolicyKey,
				policy,
				OBJC_ASSOCIATION_RETAIN_NONATOMIC
			);
			[policy release];
		}
		policy.listenExpectedVideoID = expected;
		webView.navigationDelegate = policy;
		webView.UIDelegate = policy;
		return webView.navigationDelegate == policy && webView.UIDelegate == policy ? 1 : 0;
	}
}

static NSArray<NSString*> *listenRSSSiteScopesFromJSON(const char *scopesJSON) {
	if (scopesJSON == NULL) {
		return @[];
	}
	NSData *data = [[NSString stringWithUTF8String:scopesJSON] dataUsingEncoding:NSUTF8StringEncoding];
	if (data == nil) {
		return @[];
	}
	NSError *error = nil;
	id parsed = [NSJSONSerialization JSONObjectWithData:data options:0 error:&error];
	if (error != nil || ![parsed isKindOfClass:[NSArray class]]) {
		return @[];
	}
	NSMutableArray<NSString*> *scopes = [NSMutableArray array];
	for (id value in (NSArray*)parsed) {
		if (![value isKindOfClass:[NSString class]]) {
			continue;
		}
		NSString *scope = [(NSString*)value lowercaseString];
		while ([scope hasPrefix:@"."]) {
			scope = [scope substringFromIndex:1];
		}
		while ([scope hasSuffix:@"."]) {
			scope = [scope substringToIndex:scope.length - 1];
		}
		if (scope.length > 0 && ![scopes containsObject:scope]) {
			[scopes addObject:scope];
		}
	}
	return scopes;
}

static BOOL listenRSSSiteHostMatchesScope(NSString *host, NSString *scope) {
	if (host.length == 0 || scope.length == 0) {
		return NO;
	}
	return [host isEqualToString:scope] ||
		[host hasSuffix:[@"." stringByAppendingString:scope]];
}

static BOOL listenRSSSiteAllowsTopLevelURL(NSURL *url, NSArray<NSString*> *scopes) {
	if (url == nil) {
		return NO;
	}
	if ([url.absoluteString isEqualToString:@"about:blank"]) {
		return scopes.count > 0;
	}
	if (![url.scheme.lowercaseString isEqualToString:@"https"] ||
		url.user.length > 0 || url.password.length > 0 ||
		(url.port != nil && url.port.integerValue != 443)) {
		return NO;
	}
	NSString *host = url.host.lowercaseString;
	while ([host hasSuffix:@"."]) {
		host = [host substringToIndex:host.length - 1];
	}
	for (NSString *scope in scopes) {
		if (listenRSSSiteHostMatchesScope(host, scope)) {
			return YES;
		}
	}
	return NO;
}

@interface ListenRSSSiteNavigationPolicy : NSObject <WKNavigationDelegate, WKUIDelegate>
@property(nonatomic, assign) id<WKNavigationDelegate> listenForwardedNavigationDelegate;
@property(nonatomic, assign) id<WKUIDelegate> listenForwardedUIDelegate;
@property(nonatomic, copy) NSArray<NSString*> *listenAllowedScopes;
@end

@implementation ListenRSSSiteNavigationPolicy
@synthesize listenForwardedNavigationDelegate = _listenForwardedNavigationDelegate;
@synthesize listenForwardedUIDelegate = _listenForwardedUIDelegate;
@synthesize listenAllowedScopes = _listenAllowedScopes;

- (void)dealloc {
	[_listenAllowedScopes release];
	[super dealloc];
}

- (BOOL)respondsToSelector:(SEL)selector {
	return [super respondsToSelector:selector] ||
		[self.listenForwardedNavigationDelegate respondsToSelector:selector] ||
		[self.listenForwardedUIDelegate respondsToSelector:selector];
}

- (id)forwardingTargetForSelector:(SEL)selector {
	if ([self.listenForwardedNavigationDelegate respondsToSelector:selector]) {
		return self.listenForwardedNavigationDelegate;
	}
	if ([self.listenForwardedUIDelegate respondsToSelector:selector]) {
		return self.listenForwardedUIDelegate;
	}
	return [super forwardingTargetForSelector:selector];
}

- (void)webView:(WKWebView*)webView
	decidePolicyForNavigationAction:(WKNavigationAction*)navigationAction
	decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
	if (navigationAction.targetFrame == nil) {
		decisionHandler(WKNavigationActionPolicyCancel);
		return;
	}
	if (navigationAction.targetFrame.mainFrame &&
		!listenRSSSiteAllowsTopLevelURL(navigationAction.request.URL, self.listenAllowedScopes)) {
		decisionHandler(WKNavigationActionPolicyCancel);
		return;
	}
	id<WKNavigationDelegate> delegate = self.listenForwardedNavigationDelegate;
	if (delegate != nil && [delegate respondsToSelector:@selector(webView:decidePolicyForNavigationAction:decisionHandler:)]) {
		[delegate webView:webView decidePolicyForNavigationAction:navigationAction decisionHandler:decisionHandler];
		return;
	}
	decisionHandler(WKNavigationActionPolicyAllow);
}

- (WKWebView*)webView:(WKWebView*)webView
	createWebViewWithConfiguration:(WKWebViewConfiguration*)configuration
	forNavigationAction:(WKNavigationAction*)navigationAction
	windowFeatures:(WKWindowFeatures*)windowFeatures {
	return nil;
}
@end

static int listenInstallRSSSiteNavigationPolicy(void *nativeWindow, const char *scopesJSON) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		NSArray<NSString*> *scopes = listenRSSSiteScopesFromJSON(scopesJSON);
		if (webView == nil || scopes.count == 0) {
			return 0;
		}
		ListenRSSSiteNavigationPolicy *policy =
			(ListenRSSSiteNavigationPolicy*)objc_getAssociatedObject(webView, listenRSSSiteNavigationPolicyKey);
		if (policy == nil) {
			policy = [[ListenRSSSiteNavigationPolicy alloc] init];
			policy.listenForwardedNavigationDelegate = webView.navigationDelegate;
			policy.listenForwardedUIDelegate = webView.UIDelegate;
			objc_setAssociatedObject(
				webView,
				listenRSSSiteNavigationPolicyKey,
				policy,
				OBJC_ASSOCIATION_RETAIN_NONATOMIC
			);
			[policy release];
		}
		policy.listenAllowedScopes = scopes;
		webView.navigationDelegate = policy;
		webView.UIDelegate = policy;
		return webView.navigationDelegate == policy && webView.UIDelegate == policy ? 1 : 0;
	}
}

static void listenRemoveRSSSiteNavigationPolicy(void *nativeWindow) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		if (webView == nil) {
			return;
		}
		// Invalidate a cookie-gated first navigation before releasing the
		// delegate. WKHTTPCookieStore completion blocks may outlive Close and
		// otherwise issue their retained request after the player is gone.
		NSInteger navigationGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue] + 1;
		objc_setAssociatedObject(
			webView,
			listenNavigationGenerationKey,
			@(navigationGeneration),
			OBJC_ASSOCIATION_RETAIN_NONATOMIC
		);
		[webView stopLoading];
		ListenRSSSiteNavigationPolicy *policy =
			(ListenRSSSiteNavigationPolicy*)objc_getAssociatedObject(webView, listenRSSSiteNavigationPolicyKey);
		if (policy == nil) {
			return;
		}
		if (webView.navigationDelegate == policy) {
			webView.navigationDelegate = policy.listenForwardedNavigationDelegate;
		}
		if (webView.UIDelegate == policy) {
			webView.UIDelegate = policy.listenForwardedUIDelegate;
		}
		objc_setAssociatedObject(webView, listenRSSSiteNavigationPolicyKey, nil, OBJC_ASSOCIATION_ASSIGN);
	}
}

// The transport bridge must exist before Bilibili creates its player DOM. A
// WebViewDidFinishNavigation/ExecJS hook is too late: the remote controls can
// paint and the first media lifecycle events can already have fired. Keep the
// script isolated to the main frame so untrusted player subframes cannot gain
// XiaDown's native messaging contract.
static void listenRemoveRSSBilibiliDocumentStartScript(void *nativeWindow) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		if (webView == nil) {
			return;
		}
		NSString *source = (NSString*)objc_getAssociatedObject(
			webView,
			listenRSSBilibiliDocumentStartScriptKey
		);
		if (source.length == 0) {
			return;
		}
		WKUserContentController *controller = webView.configuration.userContentController;
		if (controller != nil) {
			// WKUserContentController only offers remove-all on the supported macOS
			// baseline. Preserve Wails' runtime scripts while removing our exact,
			// session-scoped source.
			NSArray<WKUserScript*> *scripts = [controller.userScripts copy];
			[controller removeAllUserScripts];
			for (WKUserScript *script in scripts) {
				if (![script.source isEqualToString:source]) {
					[controller addUserScript:script];
				}
			}
			[scripts release];
		}
		objc_setAssociatedObject(
			webView,
			listenRSSBilibiliDocumentStartScriptKey,
			nil,
			OBJC_ASSOCIATION_ASSIGN
		);
	}
}

static int listenInstallRSSBilibiliDocumentStartScript(void *nativeWindow, const char *bridgeScript) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		if (webView == nil || bridgeScript == NULL) {
			return 0;
		}
		WKUserContentController *controller = webView.configuration.userContentController;
		NSString *source = [NSString stringWithUTF8String:bridgeScript];
		if (controller == nil || source.length == 0) {
			return 0;
		}

		listenRemoveRSSBilibiliDocumentStartScript(nativeWindow);
		WKUserScript *script = [[WKUserScript alloc]
			initWithSource:source
			injectionTime:WKUserScriptInjectionTimeAtDocumentStart
			forMainFrameOnly:YES];
		[controller addUserScript:script];
		[script release];
		objc_setAssociatedObject(
			webView,
			listenRSSBilibiliDocumentStartScriptKey,
			source,
			OBJC_ASSOCIATION_COPY_NONATOMIC
		);
		return 1;
	}
}

@interface ListenEmbeddedContainerView : NSView
@property(nonatomic, assign) BOOL listenInteractive;
@end

@implementation ListenEmbeddedContainerView
@synthesize listenInteractive = _listenInteractive;

- (NSView*)hitTest:(NSPoint)point {
	if (!self.listenInteractive) {
		return nil;
	}
	return [super hitTest:point];
}
@end

static WKWebView *listenEmbeddedWebView = nil;
static NSWindow *listenEmbeddedPlayerWindow = nil;
static ListenEmbeddedContainerView *listenEmbeddedContainerView = nil;
static NSView *listenEmbeddedOriginalSuperview = nil;
static NSRect listenEmbeddedOriginalFrame;
static BOOL listenEmbeddedOriginalHidden = NO;
static BOOL listenEmbeddedOriginalWantsLayer = NO;
static BOOL listenEmbeddedOriginalMasksToBounds = NO;
static CGFloat listenEmbeddedOriginalCornerRadius = 0;
static BOOL listenEmbeddedOriginalTranslatesAutoresizingMaskIntoConstraints = YES;
static NSUInteger listenEmbeddedOriginalAutoresizingMask = NSViewNotSizable;
static WKWebView *listenEmbeddedHostWebView = nil;
static NSScrollView *listenEmbeddedHostScrollView = nil;
static BOOL listenEmbeddedHostOriginalWantsLayer = NO;
static BOOL listenEmbeddedHostOriginalDrawsBackgroundKnown = NO;
static BOOL listenEmbeddedHostOriginalDrawsBackground = YES;
static BOOL listenEmbeddedHostOriginalScrollDrawsBackground = YES;
static NSColor *listenEmbeddedHostOriginalUnderPageBackgroundColor = nil;

// Returns -1 when the public state is unavailable, 0 when the WebView is
// inline, and 1 while WebKit owns it for an entering, active, or exiting
// fullscreen presentation. WebKit moves the WKWebView out of the player
// window during fullscreen, so prefer the embedded-view reference retained
// before that move instead of searching the placeholder hierarchy.
static int listenEmbeddedWebViewOwnsFullscreenPresentation(void *nativeWindow) {
	@autoreleasepool {
		if (nativeWindow == NULL) {
			return -1;
		}
		NSWindow *playerWindow = (NSWindow*)nativeWindow;
		WKWebView *webView = nil;
		if (listenEmbeddedPlayerWindow == playerWindow && listenEmbeddedWebView != nil) {
			webView = listenEmbeddedWebView;
		}
		if (webView == nil) {
			webView = listenWebViewForWindow(nativeWindow);
		}
		if (webView == nil) {
			return -1;
		}
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 130000
		WKFullscreenState state = webView.fullscreenState;
		return state == WKFullscreenStateNotInFullscreen ? 0 : 1;
#endif
		return -1;
	}
}

static void listenResetEmbeddedWebViewState(void) {
	listenEmbeddedWebView = nil;
	listenEmbeddedPlayerWindow = nil;
	listenEmbeddedOriginalSuperview = nil;
	listenEmbeddedOriginalFrame = NSZeroRect;
	listenEmbeddedOriginalHidden = NO;
	listenEmbeddedOriginalWantsLayer = NO;
	listenEmbeddedOriginalMasksToBounds = NO;
	listenEmbeddedOriginalCornerRadius = 0;
	listenEmbeddedOriginalTranslatesAutoresizingMaskIntoConstraints = YES;
	listenEmbeddedOriginalAutoresizingMask = NSViewNotSizable;
}

static BOOL listenReadWKWebViewDrawsBackground(WKWebView *webView, BOOL *known) {
	if (known != NULL) {
		*known = NO;
	}
	if (webView == nil) {
		return YES;
	}
	@try {
		id value = [webView valueForKey:@"drawsBackground"];
		if ([value respondsToSelector:@selector(boolValue)]) {
			if (known != NULL) {
				*known = YES;
			}
			return [value boolValue];
		}
	} @catch (NSException *exception) {
	}
	return YES;
}

static void listenSetWKWebViewDrawsBackground(WKWebView *webView, BOOL drawsBackground) {
	if (webView == nil) {
		return;
	}
	@try {
		[webView setValue:@(drawsBackground) forKey:@"drawsBackground"];
	} @catch (NSException *exception) {
	}
}

static void listenRestoreEmbeddedHostWebView(void) {
	WKWebView *hostWebView = listenEmbeddedHostWebView;
	if (hostWebView != nil) {
		if (listenEmbeddedHostOriginalDrawsBackgroundKnown) {
			listenSetWKWebViewDrawsBackground(hostWebView, listenEmbeddedHostOriginalDrawsBackground);
		}
		if (listenEmbeddedHostScrollView != nil) {
			listenEmbeddedHostScrollView.drawsBackground = listenEmbeddedHostOriginalScrollDrawsBackground;
		}
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120000
		hostWebView.underPageBackgroundColor = listenEmbeddedHostOriginalUnderPageBackgroundColor;
#endif
		hostWebView.wantsLayer = listenEmbeddedHostOriginalWantsLayer;
		[hostWebView setNeedsDisplay:YES];
	}
	[listenEmbeddedHostOriginalUnderPageBackgroundColor release];
	[listenEmbeddedHostScrollView release];
	listenEmbeddedHostWebView = nil;
	listenEmbeddedHostScrollView = nil;
	listenEmbeddedHostOriginalWantsLayer = NO;
	listenEmbeddedHostOriginalDrawsBackgroundKnown = NO;
	listenEmbeddedHostOriginalDrawsBackground = YES;
	listenEmbeddedHostOriginalScrollDrawsBackground = YES;
	listenEmbeddedHostOriginalUnderPageBackgroundColor = nil;
}

static int listenRestoreActiveEmbeddedWebView(void) {
	WKWebView *webView = listenEmbeddedWebView;
	if (webView == nil) {
		if (listenEmbeddedContainerView != nil) {
			[listenEmbeddedContainerView removeFromSuperview];
			[listenEmbeddedContainerView release];
			listenEmbeddedContainerView = nil;
		}
		listenRestoreEmbeddedHostWebView();
		listenResetEmbeddedWebViewState();
		return 0;
	}

	NSView *targetSuperview = listenEmbeddedOriginalSuperview;
	if (targetSuperview == nil && listenEmbeddedPlayerWindow != nil) {
		targetSuperview = listenEmbeddedPlayerWindow.contentView;
	}
	if (targetSuperview != nil && webView.superview != targetSuperview) {
		[webView retain];
		[webView removeFromSuperview];
		[targetSuperview addSubview:webView positioned:NSWindowAbove relativeTo:nil];
		[webView release];
	}
	if (targetSuperview != nil) {
		webView.translatesAutoresizingMaskIntoConstraints = listenEmbeddedOriginalTranslatesAutoresizingMaskIntoConstraints;
		webView.frame = listenEmbeddedOriginalFrame;
		webView.hidden = listenEmbeddedOriginalHidden;
		webView.autoresizingMask = listenEmbeddedOriginalAutoresizingMask;
	}
	if ([webView respondsToSelector:@selector(setWantsLayer:)] && webView.layer != nil) {
		webView.layer.cornerRadius = listenEmbeddedOriginalCornerRadius;
		webView.layer.masksToBounds = listenEmbeddedOriginalMasksToBounds;
		webView.layer.zPosition = 0;
		webView.wantsLayer = listenEmbeddedOriginalWantsLayer;
	}
	if (listenEmbeddedContainerView != nil) {
		[listenEmbeddedContainerView removeFromSuperview];
		[listenEmbeddedContainerView release];
		listenEmbeddedContainerView = nil;
	}
	listenRestoreEmbeddedHostWebView();
	listenResetEmbeddedWebViewState();
	return 1;
}

static void listenConfigureHostWebViewForEmbeddedUnderlay(WKWebView *hostWebView) {
	if (hostWebView == nil) {
		return;
	}
	if (listenEmbeddedHostWebView != hostWebView) {
		listenRestoreEmbeddedHostWebView();
		listenEmbeddedHostWebView = hostWebView;
		listenEmbeddedHostOriginalWantsLayer = hostWebView.wantsLayer;
		listenEmbeddedHostOriginalDrawsBackground =
			listenReadWKWebViewDrawsBackground(hostWebView, &listenEmbeddedHostOriginalDrawsBackgroundKnown);
		listenEmbeddedHostScrollView = [listenFindNSScrollView(hostWebView) retain];
		listenEmbeddedHostOriginalScrollDrawsBackground =
			listenEmbeddedHostScrollView != nil ? listenEmbeddedHostScrollView.drawsBackground : YES;
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120000
		listenEmbeddedHostOriginalUnderPageBackgroundColor = [hostWebView.underPageBackgroundColor retain];
#endif
	}
	listenSetWKWebViewDrawsBackground(hostWebView, NO);
	if (listenEmbeddedHostScrollView != nil) {
		listenEmbeddedHostScrollView.drawsBackground = NO;
	}
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120000
	hostWebView.underPageBackgroundColor = [NSColor clearColor];
#endif
	hostWebView.wantsLayer = YES;
	if (hostWebView.layer != nil) {
		hostWebView.layer.backgroundColor = [NSColor clearColor].CGColor;
	}
	[hostWebView setNeedsDisplay:YES];
}

static CGFloat listenClampFrameValue(CGFloat value, CGFloat minimum, CGFloat maximum) {
	if (value < minimum) {
		return minimum;
	}
	if (value > maximum) {
		return maximum;
	}
	return value;
}

static BOOL listenFinitePositiveDouble(double value) {
	return isfinite(value) && value > 0;
}

static CGFloat listenBackingScaleForView(NSView *view) {
	CGFloat scale = 0;
	if (view != nil && view.window != nil) {
		scale = view.window.backingScaleFactor;
	}
	if (scale <= 0 && NSScreen.mainScreen != nil) {
		scale = NSScreen.mainScreen.backingScaleFactor;
	}
	return scale > 0 ? scale : 1;
}

static NSRect listenClampFrameToBounds(NSRect frame, NSRect bounds) {
	frame.size.width = listenClampFrameValue(frame.size.width, 1, MAX(1, bounds.size.width));
	frame.size.height = listenClampFrameValue(frame.size.height, 1, MAX(1, bounds.size.height));
	frame.origin.x = listenClampFrameValue(frame.origin.x, 0, MAX(0, bounds.size.width - frame.size.width));
	frame.origin.y = listenClampFrameValue(frame.origin.y, 0, MAX(0, bounds.size.height - frame.size.height));
	return frame;
}

static NSRect listenAlignFrameToBackingPixels(NSView *view, NSRect frame, NSRect bounds) {
	CGFloat scale = listenBackingScaleForView(view);
	CGFloat minX = floor(frame.origin.x * scale) / scale;
	CGFloat minY = floor(frame.origin.y * scale) / scale;
	CGFloat maxX = ceil((frame.origin.x + frame.size.width) * scale) / scale;
	CGFloat maxY = ceil((frame.origin.y + frame.size.height) * scale) / scale;
	return listenClampFrameToBounds(NSMakeRect(minX, minY, maxX - minX, maxY - minY), bounds);
}

static NSRect listenEmbeddedFrameForContainerView(NSView *containerView, double x, double y, double width, double height) {
	NSRect bounds = containerView.bounds;
	CGFloat frameWidth = listenClampFrameValue((CGFloat)width, 1, MAX(1, bounds.size.width));
	CGFloat frameHeight = listenClampFrameValue((CGFloat)height, 1, MAX(1, bounds.size.height));
	CGFloat frameX = listenClampFrameValue((CGFloat)x, 0, MAX(0, bounds.size.width - frameWidth));
	CGFloat topY = listenClampFrameValue((CGFloat)y, 0, MAX(0, bounds.size.height - frameHeight));
	CGFloat frameY = containerView.isFlipped ? topY : bounds.size.height - topY - frameHeight;
	return listenAlignFrameToBackingPixels(containerView, NSMakeRect(frameX, frameY, frameWidth, frameHeight), bounds);
}

static NSRect listenEmbeddedFrameFromHostWebView(WKWebView *hostWebView, NSView *contentView, double x, double y, double width, double height, double centerX, double centerY, double viewportWidth, double viewportHeight) {
	if (hostWebView == nil || contentView == nil) {
		return listenEmbeddedFrameForContainerView(contentView, x, y, width, height);
	}
	NSRect hostFrame = hostWebView.frame;
	if (hostWebView.superview != nil && hostWebView.superview != contentView) {
		hostFrame = [hostWebView.superview convertRect:hostFrame toView:contentView];
	}

	NSRect bounds = contentView.bounds;
	CGFloat hostWidth = MAX(1, hostFrame.size.width);
	CGFloat hostHeight = MAX(1, hostFrame.size.height);
	BOOL hasViewport = listenFinitePositiveDouble(viewportWidth) && listenFinitePositiveDouble(viewportHeight);
	CGFloat cssViewportWidth = hasViewport ? (CGFloat)viewportWidth : hostWidth;
	CGFloat cssViewportHeight = hasViewport ? (CGFloat)viewportHeight : hostHeight;
	CGFloat scaleX = hostWidth / MAX(1, cssViewportWidth);
	CGFloat scaleY = hostHeight / MAX(1, cssViewportHeight);
	CGFloat frameWidth = listenClampFrameValue((CGFloat)width * scaleX, 1, hostWidth);
	CGFloat frameHeight = listenClampFrameValue((CGFloat)height * scaleY, 1, hostHeight);
	CGFloat localX = 0;
	CGFloat topY = 0;
	if (hasViewport && isfinite(centerX) && isfinite(centerY)) {
		CGFloat localCenterX = hostWidth / 2 + (((CGFloat)centerX - cssViewportWidth / 2) * scaleX);
		CGFloat localCenterY = hostHeight / 2 + (((CGFloat)centerY - cssViewportHeight / 2) * scaleY);
		localCenterX = listenClampFrameValue(localCenterX, frameWidth / 2, hostWidth - frameWidth / 2);
		localCenterY = listenClampFrameValue(localCenterY, frameHeight / 2, hostHeight - frameHeight / 2);
		localX = localCenterX - frameWidth / 2;
		topY = localCenterY - frameHeight / 2;
	} else {
		localX = listenClampFrameValue((CGFloat)x * scaleX, 0, MAX(0, hostWidth - frameWidth));
		topY = listenClampFrameValue((CGFloat)y * scaleY, 0, MAX(0, hostHeight - frameHeight));
	}
	CGFloat frameX = hostFrame.origin.x + localX;
	CGFloat frameY = contentView.isFlipped ?
		hostFrame.origin.y + topY :
		hostFrame.origin.y + hostHeight - topY - frameHeight;
	NSRect converted = NSMakeRect(frameX, frameY, frameWidth, frameHeight);

	return listenAlignFrameToBackingPixels(contentView, converted, bounds);
}

static CGFloat listenEmbeddedRadiusForFrame(double radius, NSRect frame) {
	CGFloat maximum = MAX(0, MIN(frame.size.width, frame.size.height) / 2.0);
	return listenClampFrameValue((CGFloat)radius, 0, maximum);
}

static int listenShowEmbeddedWebView(void *playerNativeWindow, void *hostNativeWindow, double x, double y, double width, double height, double centerX, double centerY, double viewportWidth, double viewportHeight, double radius, int interactive) {
	@autoreleasepool {
		if (playerNativeWindow == NULL || hostNativeWindow == NULL) {
			return 0;
		}

		NSWindow *playerWindow = (NSWindow*)playerNativeWindow;
		WKWebView *webView = listenWebViewForWindow(playerNativeWindow);
		if (webView == nil &&
			listenEmbeddedWebView != nil &&
			listenEmbeddedPlayerWindow == playerWindow) {
			webView = listenEmbeddedWebView;
		}
		NSWindow *hostWindow = (NSWindow*)hostNativeWindow;
		NSView *contentView = hostWindow.contentView;
		WKWebView *hostWebView = listenWebViewForWindow(hostNativeWindow);
		if (webView == nil || contentView == nil || webView == hostWebView) {
			return 0;
		}
		NSView *targetSuperview = hostWebView != nil && hostWebView.superview != nil ?
			hostWebView.superview :
			contentView;

		if (listenEmbeddedWebView != nil && listenEmbeddedWebView != webView) {
			listenRestoreActiveEmbeddedWebView();
		}
		BOOL interactiveOverlay = interactive > 1;
		if (!interactiveOverlay) {
			listenConfigureHostWebViewForEmbeddedUnderlay(hostWebView);
		}

		if (listenEmbeddedWebView != webView) {
			listenEmbeddedWebView = webView;
			listenEmbeddedPlayerWindow = playerWindow;
			listenEmbeddedOriginalSuperview = webView.superview;
			listenEmbeddedOriginalFrame = webView.frame;
			listenEmbeddedOriginalHidden = webView.hidden;
			listenEmbeddedOriginalWantsLayer = webView.wantsLayer;
			listenEmbeddedOriginalMasksToBounds = webView.layer != nil ? webView.layer.masksToBounds : NO;
			listenEmbeddedOriginalCornerRadius = webView.layer != nil ? webView.layer.cornerRadius : 0;
			listenEmbeddedOriginalTranslatesAutoresizingMaskIntoConstraints = webView.translatesAutoresizingMaskIntoConstraints;
			listenEmbeddedOriginalAutoresizingMask = webView.autoresizingMask;
		}

		NSRect frame = hostWebView != nil ?
			listenEmbeddedFrameFromHostWebView(hostWebView, targetSuperview, x, y, width, height, centerX, centerY, viewportWidth, viewportHeight) :
			listenEmbeddedFrameForContainerView(targetSuperview, x, y, width, height);
		if (listenEmbeddedContainerView == nil) {
			listenEmbeddedContainerView = [[ListenEmbeddedContainerView alloc] initWithFrame:frame];
		}
		if (listenEmbeddedContainerView.superview != targetSuperview) {
			[listenEmbeddedContainerView removeFromSuperview];
			NSView *relativeView = hostWebView != nil && hostWebView.superview == targetSuperview ? (NSView*)hostWebView : nil;
			[targetSuperview addSubview:listenEmbeddedContainerView positioned:(interactiveOverlay ? NSWindowAbove : NSWindowBelow) relativeTo:relativeView];
		} else if (hostWebView != nil && hostWebView.superview == targetSuperview) {
			[targetSuperview addSubview:listenEmbeddedContainerView positioned:(interactiveOverlay ? NSWindowAbove : NSWindowBelow) relativeTo:hostWebView];
		}

		listenEmbeddedContainerView.hidden = NO;
		listenEmbeddedContainerView.listenInteractive = interactive != 0;
		listenEmbeddedContainerView.translatesAutoresizingMaskIntoConstraints = YES;
		listenEmbeddedContainerView.frame = frame;
		listenEmbeddedContainerView.autoresizingMask = NSViewNotSizable;
		if ([listenEmbeddedContainerView respondsToSelector:@selector(setWantsLayer:)]) {
			listenEmbeddedContainerView.wantsLayer = YES;
			listenEmbeddedContainerView.layer.zPosition = interactiveOverlay ? 1 : 0;
			listenEmbeddedContainerView.layer.cornerRadius = listenEmbeddedRadiusForFrame(radius, frame);
			listenEmbeddedContainerView.layer.masksToBounds = YES;
			listenEmbeddedContainerView.layer.backgroundColor = [NSColor blackColor].CGColor;
		}

		if (webView.superview != listenEmbeddedContainerView) {
			[webView retain];
			[webView removeFromSuperview];
			[listenEmbeddedContainerView addSubview:webView positioned:NSWindowAbove relativeTo:nil];
			[webView release];
		}

		webView.hidden = NO;
		webView.translatesAutoresizingMaskIntoConstraints = YES;
		webView.frame = listenEmbeddedContainerView.bounds;
		webView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
		if ([webView respondsToSelector:@selector(setWantsLayer:)]) {
			webView.wantsLayer = YES;
			webView.layer.zPosition = 0;
			webView.layer.cornerRadius = listenEmbeddedRadiusForFrame(radius, listenEmbeddedContainerView.bounds);
			webView.layer.masksToBounds = YES;
		}
		[webView setNeedsLayout:YES];
		[webView setNeedsDisplay:YES];
		[listenEmbeddedContainerView setNeedsLayout:YES];
		[listenEmbeddedContainerView setNeedsDisplay:YES];
		[targetSuperview setNeedsLayout:YES];
		[targetSuperview setNeedsDisplay:YES];
		[hostWindow displayIfNeeded];
		return 1;
	}
}

static int listenHideEmbeddedWebView(void *playerNativeWindow) {
	@autoreleasepool {
		if (playerNativeWindow != NULL &&
			listenEmbeddedPlayerWindow != nil &&
			listenEmbeddedPlayerWindow != (NSWindow*)playerNativeWindow) {
			return 0;
		}
		return listenRestoreActiveEmbeddedWebView();
	}
}

static void listenConfigureYouTubeMusicWebView(void *nativeWindow, const char *userAgent, const char *adBlockScript) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
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
		configuration.mediaTypesRequiringUserActionForPlayback = WKAudiovisualMediaTypeNone;
#endif

		if ([configuration respondsToSelector:@selector(setAllowsAirPlayForMediaPlayback:)]) {
			configuration.allowsAirPlayForMediaPlayback = YES;
		}

#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120300
		configuration.preferences.elementFullscreenEnabled = YES;
#endif

		if (adBlockScript != NULL && configuration.userContentController != nil) {
			NSString *source = [NSString stringWithUTF8String:adBlockScript];
			if (source.length > 0) {
				BOOL installed = NO;
				for (WKUserScript *script in configuration.userContentController.userScripts) {
					if ([script.source rangeOfString:@"__xiadownYouTubeAdBlockerInstalled"].location != NSNotFound) {
						installed = YES;
						break;
					}
				}
				if (!installed) {
					WKUserScript *script = [[WKUserScript alloc] initWithSource:source injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:NO];
					[configuration.userContentController addUserScript:script];
					[script release];
				}
			}
		}
	}
}

static CGFloat listenClampCGFloat(CGFloat value, CGFloat minimum, CGFloat maximum) {
	if (value < minimum) {
		return minimum;
	}
	if (value > maximum) {
		return maximum;
	}
	return value;
}

static void listenClearActiveAirPlayPicker(NSInteger generation) {
	if (listenActiveAirPlayPicker == nil || generation != listenAirPlayPickerGeneration) {
		return;
	}
	[listenActiveAirPlayPicker removeFromSuperview];
	[listenActiveAirPlayPicker release];
	listenActiveAirPlayPicker = nil;
}

static int listenShowAirPlayRoutePicker(void *nativeWindow, double anchorX, double anchorY, double anchorWidth, double anchorHeight) {
	@autoreleasepool {
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 101500
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
		width = listenClampCGFloat(width, 24, MAX(24, bounds.size.width));
		height = listenClampCGFloat(height, 24, MAX(24, bounds.size.height));

		CGFloat x = (anchorWidth > 0 || anchorHeight > 0) ? (CGFloat)anchorX : 20;
		CGFloat y = 14;
		if (anchorWidth > 0 || anchorHeight > 0) {
			if ([contentView isFlipped]) {
				y = (CGFloat)anchorY;
			} else {
				y = bounds.size.height - (CGFloat)anchorY - height;
			}
		}
		x = listenClampCGFloat(x, 0, MAX(0, bounds.size.width - width));
		y = listenClampCGFloat(y, 0, MAX(0, bounds.size.height - height));

		listenAirPlayPickerGeneration += 1;
		if (listenActiveAirPlayPicker != nil) {
			[listenActiveAirPlayPicker removeFromSuperview];
			[listenActiveAirPlayPicker release];
			listenActiveAirPlayPicker = nil;
		}
		NSInteger generation = listenAirPlayPickerGeneration;
		AVRoutePickerView *picker = [[AVRoutePickerView alloc] initWithFrame:NSMakeRect(x, y, width, height)];
		picker.alphaValue = 0.01;
		listenActiveAirPlayPicker = picker;
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
			listenClearActiveAirPlayPicker(generation);
			return 0;
		}

		[button performClick:nil];
		dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(15 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
			listenClearActiveAirPlayPicker(generation);
		});
		return 1;
#endif
		return 0;
	}
}

static NSString* listenStringValue(id value) {
	if ([value isKindOfClass:[NSString class]]) {
		return (NSString*)value;
	}
	if ([value isKindOfClass:[NSNumber class]]) {
		return [(NSNumber*)value stringValue];
	}
	return nil;
}

static BOOL listenBoolValue(id value) {
	if ([value isKindOfClass:[NSNumber class]]) {
		return [(NSNumber*)value boolValue];
	}
	if ([value isKindOfClass:[NSString class]]) {
		NSString *lower = [(NSString*)value lowercaseString];
		return [lower isEqualToString:@"true"] || [lower isEqualToString:@"1"] || [lower isEqualToString:@"yes"];
	}
	return NO;
}

static NSString *listenYouTubeClientOrigin(void) {
	NSString *bundleID = [[NSBundle mainBundle] bundleIdentifier];
	if (bundleID.length == 0) {
		bundleID = @"com.dreamapp.xiadown";
	}
	return [[NSString stringWithFormat:@"https://%@", bundleID] lowercaseString];
}

static NSArray<NSHTTPCookie*>* listenCookiesFromJSON(const char *cookiesJSON, NSURL *targetURL) {
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
		NSString *name = listenStringValue(dictionary[@"name"]);
		NSString *value = listenStringValue(dictionary[@"value"]);
		NSString *domain = listenStringValue(dictionary[@"domain"]);
		NSString *path = listenStringValue(dictionary[@"path"]);
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

		if (listenBoolValue(dictionary[@"secure"])) {
			properties[NSHTTPCookieSecure] = @"TRUE";
		}
		if (listenBoolValue(dictionary[@"httpOnly"])) {
			properties[(NSHTTPCookiePropertyKey)@"HttpOnly"] = @"TRUE";
		}
		NSString *sameSite = [listenStringValue(dictionary[@"sameSite"]) lowercaseString];
		if ([sameSite isEqualToString:@"lax"] ||
			[sameSite isEqualToString:@"strict"] ||
			[sameSite isEqualToString:@"none"]) {
			properties[(NSHTTPCookiePropertyKey)@"SameSite"] = [sameSite capitalizedString];
		}

		NSHTTPCookie *cookie = [NSHTTPCookie cookieWithProperties:properties];
		if (cookie != nil) {
			[cookies addObject:cookie];
		}
	}

	return cookies;
}

static NSData* listenCookiesToJSONData(NSArray<NSHTTPCookie*> *cookies) {
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
		return nil;
	}
	return data;
}

static char* listenCopyCStringFromData(NSData *data) {
	if (data == nil) {
		return NULL;
	}
	char *result = (char*)malloc(data.length + 1);
	if (result == NULL) {
		return NULL;
	}
	memcpy(result, data.bytes, data.length);
	result[data.length] = '\0';
	return result;
}

// The caller and the asynchronous WebKit completion each own one reference.
// This keeps a timed-out read alive until a late completion can discard its
// result instead of leaking it. Phase 0 is waiting, 1 is completed, and 2 is
// abandoned by the caller.
typedef struct {
	atomic_int references;
	atomic_int phase;
	char *result;
	dispatch_semaphore_t semaphore;
} ListenCookieReadState;

static ListenCookieReadState* listenCookieReadStateCreate(void) {
	ListenCookieReadState *state = (ListenCookieReadState*)calloc(1, sizeof(ListenCookieReadState));
	if (state == NULL) {
		return NULL;
	}
	atomic_init(&state->references, 2);
	atomic_init(&state->phase, 0);
	state->semaphore = dispatch_semaphore_create(0);
	if (state->semaphore == NULL) {
		free(state);
		return NULL;
	}
	return state;
}

static void listenCookieReadStateRelease(ListenCookieReadState *state) {
	if (state == NULL) {
		return;
	}
	if (atomic_fetch_sub(&state->references, 1) == 1) {
		if (state->result != NULL) {
			free(state->result);
		}
#if !OS_OBJECT_USE_OBJC_RETAIN_RELEASE
		if (state->semaphore != NULL) {
			dispatch_release(state->semaphore);
		}
#endif
		free(state);
	}
}

static void listenCookieReadStateComplete(
	ListenCookieReadState *state,
	char *candidate
) {
	int expected = 0;
	if (atomic_compare_exchange_strong(&state->phase, &expected, 1)) {
		state->result = candidate;
	} else if (candidate != NULL) {
		free(candidate);
	}
	dispatch_semaphore_signal(state->semaphore);
	listenCookieReadStateRelease(state);
}

// Cookie-store reads are dispatched from a Go worker to the WebKit main
// thread. Returning an empty result on the main thread avoids blocking the
// queue that must deliver getAllCookies' completion handler.
static char* listenReadCookiesJSON(void) {
	if ([NSThread isMainThread]) {
		return NULL;
	}
	ListenCookieReadState *state = listenCookieReadStateCreate();
	if (state == NULL) {
		return NULL;
	}
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			WKHTTPCookieStore *store = [WKWebsiteDataStore defaultDataStore].httpCookieStore;
			if (store == nil) {
				listenCookieReadStateComplete(state, NULL);
				return;
			}
			[store getAllCookies:^(NSArray<NSHTTPCookie *> *cookies) {
				listenCookieReadStateComplete(
					state,
					listenCopyCStringFromData(listenCookiesToJSONData(cookies))
				);
			}];
		}
	});
	long status = dispatch_semaphore_wait(state->semaphore, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
	if (status != 0) {
		int expected = 0;
		if (atomic_compare_exchange_strong(&state->phase, &expected, 2)) {
			listenCookieReadStateRelease(state);
			return NULL;
		}
		// The completion won the timeout race and is about to signal. Waiting
		// here is bounded by its synchronous serialization and allocation work.
		dispatch_semaphore_wait(state->semaphore, DISPATCH_TIME_FOREVER);
	}
	char *result = state->result;
	state->result = NULL;
	listenCookieReadStateRelease(state);
	return result;
}

static void listenLoadRequestOnMain(WKWebView *webView, NSURLRequest *request) {
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

static NSString *listenNavigationRefererForURL(NSURL *url) {
	if (url == nil || url.host == nil) {
		return nil;
	}
	NSString *host = url.host.lowercaseString;
	if ([host isEqualToString:@"music.youtube.com"] || [host hasSuffix:@".music.youtube.com"]) {
		return @"https://music.youtube.com/";
	}
	if ([host isEqualToString:@"youtube.com"] || [host hasSuffix:@".youtube.com"]) {
		return listenYouTubeClientOrigin();
	}
	return nil;
}

static BOOL listenCookieMatchesHost(NSHTTPCookie *cookie, NSString *host) {
	if (cookie == nil || host.length == 0) {
		return NO;
	}
	NSString *domain = cookie.domain.lowercaseString;
	while ([domain hasPrefix:@"."]) {
		domain = [domain substringFromIndex:1];
	}
	NSString *normalizedHost = host.lowercaseString;
	return [normalizedHost isEqualToString:domain] ||
		[normalizedHost hasSuffix:[@"." stringByAppendingString:domain]];
}

static BOOL listenHasLiveYouTubeAuthCookie(NSArray<NSHTTPCookie*> *cookies, NSURL *url) {
	NSSet<NSString*> *authNames = [NSSet setWithArray:@[
		@"SAPISID",
		@"__Secure-1PAPISID",
		@"__Secure-3PAPISID",
	]];
	NSDate *now = [NSDate date];
	for (NSHTTPCookie *cookie in cookies) {
		if (![authNames containsObject:cookie.name] || !listenCookieMatchesHost(cookie, url.host)) {
			continue;
		}
		if (cookie.value.length == 0 || (cookie.expiresDate != nil && [cookie.expiresDate compare:now] != NSOrderedDescending)) {
			continue;
		}
		return YES;
	}
	return NO;
}

static void listenLoadYouTubeMusicURL(void *nativeWindow, const char *targetURL, const char *cookiesJSON) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		if (webView == nil || targetURL == NULL) {
			return;
		}

		NSString *urlString = [NSString stringWithUTF8String:targetURL];
		NSURL *url = [NSURL URLWithString:urlString];
		if (url == nil) {
			return;
		}

		NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url];
		NSInteger navigationGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue] + 1;
		objc_setAssociatedObject(
			webView,
			listenNavigationGenerationKey,
			@(navigationGeneration),
			OBJC_ASSOCIATION_RETAIN_NONATOMIC
		);
		NSString *referer = listenNavigationRefererForURL(url);
		if (referer.length > 0) {
			[request setValue:referer forHTTPHeaderField:@"Referer"];
		}
		NSArray<NSHTTPCookie*> *cookies = listenCookiesFromJSON(cookiesJSON, url);
		WKHTTPCookieStore *cookieStore = webView.configuration.websiteDataStore.httpCookieStore;
		if (cookies.count == 0 || cookieStore == nil) {
			[webView loadRequest:request];
			return;
		}

		// Close the gap between the Go-side snapshot and the actual mutation.
		// Another window or WebKit network response may have installed fresher
		// auth while the request crossed the language boundary.
		[cookieStore getAllCookies:^(NSArray<NSHTTPCookie *> *currentCookies) {
			NSInteger currentGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue];
			if (currentGeneration != navigationGeneration) {
				return;
			}
			if (listenHasLiveYouTubeAuthCookie(currentCookies, url)) {
				listenLoadRequestOnMain(webView, request);
				return;
			}
			__block NSInteger remaining = cookies.count;
			for (NSHTTPCookie *cookie in cookies) {
				[cookieStore setCookie:cookie completionHandler:^{
					remaining -= 1;
					NSInteger latestGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue];
					if (remaining <= 0 && latestGeneration == navigationGeneration) {
						listenLoadRequestOnMain(webView, request);
					}
				}];
			}
		}];
	}
}

// The RSS player accepts only Bilibili's canonical video page, supplies a
// same-site navigation Referer, and waits for every cookie mutation before
// issuing the first navigation request.
static void listenLoadRSSBilibiliURL(void *nativeWindow, const char *targetURL, const char *cookiesJSON) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		if (webView == nil || targetURL == NULL) {
			return;
		}
		NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:targetURL]];
		if (!listenRSSBilibiliIsTrustedVideoPageURL(url)) {
			return;
		}

		NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url];
		[request setValue:@"https://www.bilibili.com/" forHTTPHeaderField:@"Referer"];
		NSInteger navigationGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue] + 1;
		objc_setAssociatedObject(
			webView,
			listenNavigationGenerationKey,
			@(navigationGeneration),
			OBJC_ASSOCIATION_RETAIN_NONATOMIC
		);
		NSArray<NSHTTPCookie*> *cookies = listenCookiesFromJSON(cookiesJSON, url);
		WKHTTPCookieStore *cookieStore = webView.configuration.websiteDataStore.httpCookieStore;
		if (cookies.count == 0 || cookieStore == nil) {
			[webView loadRequest:request];
			return;
		}

		__block NSInteger remaining = cookies.count;
		for (NSHTTPCookie *cookie in cookies) {
			[cookieStore setCookie:cookie completionHandler:^{
				remaining -= 1;
				NSInteger latestGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue];
				if (remaining <= 0 && latestGeneration == navigationGeneration) {
					listenLoadRequestOnMain(webView, request);
				}
			}];
		}
	}
}

// Generic RSS site playback keeps the page interactive, but still waits for
// the complete cookie snapshot before the first request and verifies the
// target against the installed top-level navigation scope.
static void listenLoadRSSSiteURL(void *nativeWindow, const char *targetURL, const char *cookiesJSON) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
		if (webView == nil || targetURL == NULL) {
			return;
		}
		ListenRSSSiteNavigationPolicy *policy =
			(ListenRSSSiteNavigationPolicy*)objc_getAssociatedObject(webView, listenRSSSiteNavigationPolicyKey);
		NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:targetURL]];
		if (policy == nil || !listenRSSSiteAllowsTopLevelURL(url, policy.listenAllowedScopes)) {
			return;
		}
		NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url];
		NSInteger navigationGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue] + 1;
		objc_setAssociatedObject(
			webView,
			listenNavigationGenerationKey,
			@(navigationGeneration),
			OBJC_ASSOCIATION_RETAIN_NONATOMIC
		);
		NSArray<NSHTTPCookie*> *cookies = listenCookiesFromJSON(cookiesJSON, url);
		WKHTTPCookieStore *cookieStore = webView.configuration.websiteDataStore.httpCookieStore;
		if (cookies.count == 0 || cookieStore == nil) {
			[webView loadRequest:request];
			return;
		}
		__block NSInteger remaining = cookies.count;
		for (NSHTTPCookie *cookie in cookies) {
			[cookieStore setCookie:cookie completionHandler:^{
				remaining -= 1;
				NSInteger latestGeneration = [objc_getAssociatedObject(webView, listenNavigationGenerationKey) integerValue];
				if (remaining <= 0 && latestGeneration == navigationGeneration) {
					listenLoadRequestOnMain(webView, request);
				}
			}];
		}
	}
}

static void listenEvaluateYouTubeMusicJavaScript(void *nativeWindow, const char *script) {
	@autoreleasepool {
		WKWebView *webView = listenWebViewForWindow(nativeWindow);
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
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubecookies"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type listenCookieRuntimeSyncMonitorEntry struct {
	generation uint64
	cancel     context.CancelFunc
}

var listenCookieRuntimeSyncMonitors struct {
	sync.Mutex
	next   uint64
	active map[uintptr]listenCookieRuntimeSyncMonitorEntry
}

const listenCookieRuntimeSharedStoreKey uintptr = 1

func listenYouTubeMusicUserAgent() string {
	return youtubemusic.BrowserUserAgent
}

func syncListenNativeVideoHostBackground(_ *application.WebviewWindow, _ application.RGBA) {}

func configureListenYouTubeMusicNativeWindow(nativeWindow unsafe.Pointer, userAgent string) {
	if nativeWindow == nil || userAgent == "" {
		return
	}

	cUserAgent := C.CString(userAgent)
	defer C.free(unsafe.Pointer(cUserAgent))
	cAdBlockScript := C.CString(listenYouTubeAdBlockScript())
	defer C.free(unsafe.Pointer(cAdBlockScript))

	application.InvokeSync(func() {
		C.listenConfigureYouTubeMusicWebView(nativeWindow, cUserAgent, cAdBlockScript)
	})
}

func installRSSVideoPlayerNativeFullscreenEscape(
	window *application.WebviewWindow,
) func() {
	if window == nil || window.NativeWindow() == nil {
		return nil
	}
	nativeWindow := window.NativeWindow()
	installed := false
	application.InvokeSync(func() {
		installed = C.listenInstallRSSBilibiliFullscreenEscapeMonitor(nativeWindow) != 0
	})
	if !installed {
		return nil
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			application.InvokeSync(func() {
				C.listenRemoveRSSBilibiliFullscreenEscapeMonitor(nativeWindow)
			})
		})
	}
}

func installListenNativeWindowFullscreenEscape(_ *application.WebviewWindow) func() {
	// AppKit owns the standard Escape gesture for a fullscreen NSWindow. RSS
	// keeps its stronger transition-aware monitor above for its frameless page.
	return nil
}

func showListenNativeAirPlayPicker(nativeWindow unsafe.Pointer, anchor ListenAirPlayAnchor) bool {
	if nativeWindow == nil {
		return false
	}

	var shown C.int
	application.InvokeSync(func() {
		shown = C.listenShowAirPlayRoutePicker(
			nativeWindow,
			C.double(anchor.X),
			C.double(anchor.Y),
			C.double(anchor.Width),
			C.double(anchor.Height),
		)
	})
	return shown != 0
}

func boolToCInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func showListenNativeEmbeddedWebViewMode(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect, interactiveMode C.int) bool {
	if playerNativeWindow == nil || hostNativeWindow == nil {
		return false
	}

	var shown C.int
	application.InvokeSync(func() {
		shown = C.listenShowEmbeddedWebView(
			playerNativeWindow,
			hostNativeWindow,
			C.double(rect.X),
			C.double(rect.Y),
			C.double(rect.Width),
			C.double(rect.Height),
			C.double(rect.CenterX),
			C.double(rect.CenterY),
			C.double(rect.ViewportWidth),
			C.double(rect.ViewportHeight),
			C.double(rect.Radius),
			interactiveMode,
		)
	})
	return shown != 0
}

func showListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
	return showListenNativeEmbeddedWebViewMode(playerNativeWindow, hostNativeWindow, rect, boolToCInt(rect.Interactive))
}

func showListenNativeEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	return showListenNativeEmbeddedWebView(playerWindow.NativeWindow(), hostWindow.NativeWindow(), rect)
}

// RSS Bilibili uses the same video-underlay contract as the YouTube surface.
// React owns the transport, poster, loading state, and rounded reveal hole;
// the player document itself is deliberately non-interactive.
func showRSSNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
	rect.Interactive = false
	return showListenNativeEmbeddedWebView(playerNativeWindow, hostNativeWindow, rect)
}

func showRSSNativeEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	return showRSSNativeEmbeddedWebView(playerWindow.NativeWindow(), hostWindow.NativeWindow(), rect)
}

// The generic RSS site page owns its controls. Mode 2 places the WebView
// above the host WebView and keeps the host opaque, unlike the optimized
// player underlays where React owns transport controls.
func showRSSNativeInteractiveEmbeddedWebViewWindow(playerWindow *application.WebviewWindow, hostWindow *application.WebviewWindow, rect ListenEmbeddedVideoRect) bool {
	if playerWindow == nil || hostWindow == nil {
		return false
	}
	rect.Interactive = true
	return showListenNativeEmbeddedWebViewMode(playerWindow.NativeWindow(), hostWindow.NativeWindow(), rect, C.int(2))
}

func hideListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer) bool {
	if playerNativeWindow == nil {
		return false
	}

	var hidden C.int
	application.InvokeSync(func() {
		hidden = C.listenHideEmbeddedWebView(playerNativeWindow)
	})
	return hidden != 0
}

func detachListenNativeEmbeddedWebViewForFullscreen(playerNativeWindow unsafe.Pointer) bool {
	return hideListenNativeEmbeddedWebView(playerNativeWindow)
}

func listenNativeEmbeddedVideoFullscreenOwnsPresentation(nativeWindow unsafe.Pointer) (bool, bool) {
	if nativeWindow == nil {
		return false, false
	}
	state := C.int(-1)
	application.InvokeSync(func() {
		state = C.listenEmbeddedWebViewOwnsFullscreenPresentation(nativeWindow)
	})
	if state < 0 {
		return false, false
	}
	return state != 0, true
}

func listenEmbeddedVideoUsesNativeWindowFullscreen() bool {
	return true
}

func listenEmbeddedVideoFullscreenAllowsHostGeometry() bool {
	return false
}

func loadListenYouTubeMusicURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	if window == nil || targetURL == "" {
		return
	}

	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		window.SetURL(targetURL)
		return
	}
	storedCookies, readErr := readListenYouTubeMusicCookies(window)
	restoreCookies := planListenPlaybackCookieRestore(
		cookies,
		storedCookies,
		targetURL,
		time.Now(),
		readErr == nil,
	)
	data, _ := json.Marshal(restoreCookies)
	cTargetURL := C.CString(targetURL)
	cCookies := C.CString(string(data))
	defer C.free(unsafe.Pointer(cTargetURL))
	defer C.free(unsafe.Pointer(cCookies))

	application.InvokeSync(func() {
		C.listenLoadYouTubeMusicURL(nativeWindow, cTargetURL, cCookies)
	})
}

// loadRSSVideoPlayerURL configures the dedicated Bilibili WebView and performs
// cookie registration plus navigation as one native transaction. The native
// loader does not issue the request until every WKHTTPCookieStore completion
// has fired, including when this function is entered from WebKit's main thread.
func loadRSSVideoPlayerURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
	expectedAdapter, expectedVideoID, validTarget := rssBilibiliPlaybackIdentityFromURL(targetURL)
	if window == nil || targetURL == "" || targetURL == rssBilibiliPlayerBlankURL ||
		!validTarget ||
		!rssBilibiliAllowsTopLevelNavigationForPlayback(targetURL, expectedAdapter, expectedVideoID) {
		return
	}
	configureConnectorAppSessionNativeWindow(window.NativeWindow(), appSessionWebViewUserAgent("bilibili"))
	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		window.SetURL(targetURL)
		return
	}
	data, _ := json.Marshal(cookies)
	cTargetURL := C.CString(targetURL)
	cCookies := C.CString(string(data))
	cExpectedVideoID := C.CString(expectedVideoID)
	defer C.free(unsafe.Pointer(cTargetURL))
	defer C.free(unsafe.Pointer(cCookies))
	defer C.free(unsafe.Pointer(cExpectedVideoID))
	application.InvokeSync(func() {
		if C.listenInstallRSSBilibiliNavigationPolicy(nativeWindow, cExpectedVideoID) != 0 {
			C.listenLoadRSSBilibiliURL(nativeWindow, cTargetURL, cCookies)
		}
	})
}

func loadRSSSitePlayerURL(
	window *application.WebviewWindow,
	targetURL string,
	siteKey string,
	cookies []appcookies.Record,
	allowedDomains []string,
	registrableSite string,
) {
	policy, allowed := webViewRemoteNavigationPolicyForRSSSite(targetURL, allowedDomains, registrableSite)
	if window == nil || !allowed || !policy.allows(targetURL) {
		return
	}
	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		return
	}
	configureConnectorAppSessionNativeWindow(nativeWindow, appSessionWebViewUserAgent(siteKey))
	scopes := cloneWebViewNavigationDomains(allowedDomains)
	if len(scopes) == 0 && strings.TrimSpace(registrableSite) != "" {
		scopes = []string{strings.ToLower(strings.TrimSpace(registrableSite))}
	}
	cookieData, _ := json.Marshal(cookies)
	scopeData, _ := json.Marshal(scopes)
	cTargetURL := C.CString(targetURL)
	cCookies := C.CString(string(cookieData))
	cScopes := C.CString(string(scopeData))
	defer C.free(unsafe.Pointer(cTargetURL))
	defer C.free(unsafe.Pointer(cCookies))
	defer C.free(unsafe.Pointer(cScopes))
	application.InvokeSync(func() {
		if C.listenInstallRSSSiteNavigationPolicy(nativeWindow, cScopes) != 0 {
			C.listenLoadRSSSiteURL(nativeWindow, cTargetURL, cCookies)
		}
	})
}

func releaseRSSVideoPlayerWindowFeatures(window *application.WebviewWindow) {
	if window == nil || window.NativeWindow() == nil {
		return
	}
	application.InvokeSync(func() {
		C.listenRemoveRSSBilibiliDocumentStartScript(window.NativeWindow())
	})
}

func releaseRSSSitePlayerWindowFeatures(window *application.WebviewWindow) {
	if window == nil || window.NativeWindow() == nil {
		return
	}
	application.InvokeSync(func() {
		C.listenRemoveRSSSiteNavigationPolicy(window.NativeWindow())
	})
}

func readListenYouTubeMusicCookies(window *application.WebviewWindow) ([]appcookies.Record, error) {
	if window == nil {
		return nil, errors.New("player window unavailable")
	}
	if window.NativeWindow() == nil {
		return nil, errors.New("player native window unavailable")
	}
	return readListenSharedYouTubeCookies()
}

func readListenSharedYouTubeCookies() ([]appcookies.Record, error) {
	raw := C.listenReadCookiesJSON()
	if raw == nil {
		return nil, errors.New("WebKit cookie store read failed")
	}
	defer C.free(unsafe.Pointer(raw))
	var records []appcookies.Record
	if err := json.Unmarshal([]byte(C.GoString(raw)), &records); err != nil {
		return nil, err
	}
	return youtubecookies.Runtime(records, time.Now()), nil
}

func scheduleListenYouTubeCookieSync(window *application.WebviewWindow, provider listenPlayerCookieProvider) {
	syncProvider, ok := provider.(listenPlayerCookieSyncProvider)
	if !ok || window == nil || window.NativeWindow() == nil {
		return
	}
	key := listenCookieRuntimeSharedStoreKey
	ctx := context.Background()
	monitorContext, cancel := context.WithCancel(ctx)
	listenCookieRuntimeSyncMonitors.Lock()
	listenCookieRuntimeSyncMonitors.next++
	generation := listenCookieRuntimeSyncMonitors.next
	if listenCookieRuntimeSyncMonitors.active == nil {
		listenCookieRuntimeSyncMonitors.active = make(map[uintptr]listenCookieRuntimeSyncMonitorEntry)
	}
	previous := listenCookieRuntimeSyncMonitors.active[key]
	listenCookieRuntimeSyncMonitors.active[key] = listenCookieRuntimeSyncMonitorEntry{
		generation: generation,
		cancel:     cancel,
	}
	listenCookieRuntimeSyncMonitors.Unlock()
	if previous.cancel != nil {
		previous.cancel()
	}
	go monitorListenYouTubeCookieSync(window, syncProvider, monitorContext, key, generation, time.Now())
}

func monitorListenYouTubeCookieSync(
	window *application.WebviewWindow,
	provider listenPlayerCookieSyncProvider,
	ctx context.Context,
	key uintptr,
	generation uint64,
	started time.Time,
) {
	defer finishListenYouTubeCookieSync(key, generation)
	for _, after := range []time.Duration{
		time.Second,
		3 * time.Second,
		10 * time.Second,
		30 * time.Second,
		60 * time.Second,
	} {
		wait := time.Until(started.Add(after))
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if !listenYouTubeCookieSyncIsCurrent(key, generation) || window == nil || window.NativeWindow() == nil {
			return
		}
		epoch, sequence := provider.BeginCookieSync("youtube")
		if epoch == 0 || sequence == 0 {
			return
		}
		records, err := readListenYouTubeMusicCookies(window)
		if ctx.Err() != nil || !listenYouTubeCookieSyncIsCurrent(key, generation) {
			return
		}
		if err == nil {
			err = provider.SyncRecordsForSiteKey(ctx, "youtube", records, epoch, sequence)
		}
		if err != nil {
			continue
		}
	}
}

func finishListenYouTubeCookieSync(key uintptr, generation uint64) {
	var cancel context.CancelFunc
	listenCookieRuntimeSyncMonitors.Lock()
	entry, exists := listenCookieRuntimeSyncMonitors.active[key]
	if exists && entry.generation == generation {
		cancel = entry.cancel
		delete(listenCookieRuntimeSyncMonitors.active, key)
	}
	listenCookieRuntimeSyncMonitors.Unlock()
	if cancel != nil {
		cancel()
	}
}

func listenYouTubeCookieSyncIsCurrent(key uintptr, generation uint64) bool {
	listenCookieRuntimeSyncMonitors.Lock()
	entry, exists := listenCookieRuntimeSyncMonitors.active[key]
	listenCookieRuntimeSyncMonitors.Unlock()
	return exists && entry.generation == generation
}

func execListenYouTubeMusicJS(window *application.WebviewWindow, script string) {
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
		C.listenEvaluateYouTubeMusicJavaScript(nativeWindow, cScript)
	})
}

func hideListenYouTubeMediaWindow(window *application.WebviewWindow) {
	if window != nil {
		window.Hide()
	}
}

func attachListenYouTubeMusicBridge(window *application.WebviewWindow, script string) (func(), bool) {
	// WebKit receives the bridge through WebviewWindowOptions.JS. There is no
	// extra native registration to release on this platform.
	return nil, window != nil && script != ""
}

func attachRSSVideoPlayerDocumentStartBridge(
	window *application.WebviewWindow,
	script string,
) (func(), bool) {
	if window == nil || window.NativeWindow() == nil || script == "" {
		return nil, false
	}
	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))
	var installed C.int
	application.InvokeSync(func() {
		installed = C.listenInstallRSSBilibiliDocumentStartScript(window.NativeWindow(), cScript)
	})
	if installed == 0 {
		return nil, false
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			if window.NativeWindow() == nil {
				return
			}
			application.InvokeSync(func() {
				C.listenRemoveRSSBilibiliDocumentStartScript(window.NativeWindow())
			})
		})
	}, true
}
