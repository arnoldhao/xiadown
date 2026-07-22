//go:build darwin && cgo && !ios && !server

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Availability.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

static const void *xiadownRemoteCapabilityUIDelegateKey = &xiadownRemoteCapabilityUIDelegateKey;

@interface XiaDownRemoteCapabilityUIDelegate : NSObject <WKUIDelegate>
@property(nonatomic, assign) id<WKUIDelegate> forwardedUIDelegate;
@end

@implementation XiaDownRemoteCapabilityUIDelegate
@synthesize forwardedUIDelegate = _forwardedUIDelegate;

- (BOOL)respondsToSelector:(SEL)selector {
	return [super respondsToSelector:selector] ||
		[self.forwardedUIDelegate respondsToSelector:selector];
}

- (id)forwardingTargetForSelector:(SEL)selector {
	if ([self.forwardedUIDelegate respondsToSelector:selector]) {
		return self.forwardedUIDelegate;
	}
	return [super forwardingTargetForSelector:selector];
}

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 120000
- (void)webView:(WKWebView *)webView
	requestMediaCapturePermissionForOrigin:(WKSecurityOrigin *)origin
	initiatedByFrame:(WKFrameInfo *)frame
	type:(WKMediaCaptureType)type
	decisionHandler:(void (^)(WKPermissionDecision decision))decisionHandler API_AVAILABLE(macos(12.0)) {
	decisionHandler(WKPermissionDecisionDeny);
}
#endif

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 270000
- (void)webView:(WKWebView *)webView
	requestGeolocationPermissionForOrigin:(WKSecurityOrigin *)origin
	initiatedByFrame:(WKFrameInfo *)frame
	decisionHandler:(void (^)(WKPermissionDecision decision))decisionHandler API_AVAILABLE(macos(27.0)) {
	decisionHandler(WKPermissionDecisionDeny);
}
#endif
@end

static WKWebView *xiadownRemoteCapabilityFindWebView(NSView *view) {
	if (view == nil) {
		return nil;
	}
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView *)view;
	}
	for (NSView *child in view.subviews) {
		WKWebView *candidate = xiadownRemoteCapabilityFindWebView(child);
		if (candidate != nil) {
			return candidate;
		}
	}
	return nil;
}

static int xiadownInstallRemoteCapabilityPolicy(void *nativeWindow) {
	@autoreleasepool {
		if (nativeWindow == NULL) {
			return 0;
		}
		NSWindow *window = (NSWindow *)nativeWindow;
		WKWebView *webView = xiadownRemoteCapabilityFindWebView(window.contentView);
		if (webView == nil) {
			return 0;
		}
		XiaDownRemoteCapabilityUIDelegate *policy =
			(XiaDownRemoteCapabilityUIDelegate *)objc_getAssociatedObject(
				webView,
				xiadownRemoteCapabilityUIDelegateKey
			);
		if (policy == nil) {
			policy = [[XiaDownRemoteCapabilityUIDelegate alloc] init];
			policy.forwardedUIDelegate = webView.UIDelegate;
			objc_setAssociatedObject(
				webView,
				xiadownRemoteCapabilityUIDelegateKey,
				policy,
				OBJC_ASSOCIATION_RETAIN_NONATOMIC
			);
			[policy release];
		} else if (webView.UIDelegate != policy) {
			policy.forwardedUIDelegate = webView.UIDelegate;
		}
		webView.UIDelegate = policy;
		return webView.UIDelegate == policy ? 1 : 0;
	}
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// registerWebViewRemoteCapabilityPolicy installs the macOS policy after the
// WKWebView exists. Its forwarding delegate preserves Wails' existing UI
// callbacks while denying media capture and, on SDKs that expose it,
// geolocation. App Session installs its own equivalent delegate so popup
// WebViews inherit the same fail-closed decision.
func registerWebViewRemoteCapabilityPolicy(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	install := func() {
		nativeWindow := window.NativeWindow()
		if nativeWindow == nil {
			return
		}
		application.InvokeSync(func() {
			C.xiadownInstallRemoteCapabilityPolicy(nativeWindow)
		})
	}

	// Startup and lazy shell windows register before Run, when Wails has not
	// created their NSWindow/WKWebView yet. Install immediately for already-live
	// dedicated windows, then retry at the earliest public WebKit event and once
	// more at runtime-ready in case Wails replaced its UI delegate while loading.
	install()
	window.OnWindowEvent(events.Mac.WebViewDidStartProvisionalNavigation, func(_ *application.WindowEvent) {
		install()
	})
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		install()
	})
}
