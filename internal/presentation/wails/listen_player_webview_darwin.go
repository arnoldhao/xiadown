//go:build darwin && !ios

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=10.13 -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework AVKit

#include <math.h>
#include <stdlib.h>
#import <AVKit/AVKit.h>
#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <WebKit/WebKit.h>

static NSView *listenActiveAirPlayPicker = nil;
static NSInteger listenAirPlayPickerGeneration = 0;

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
		if (@available(macOS 12.0, *)) {
			hostWebView.underPageBackgroundColor = listenEmbeddedHostOriginalUnderPageBackgroundColor;
		}
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
		if (@available(macOS 12.0, *)) {
			listenEmbeddedHostOriginalUnderPageBackgroundColor = [hostWebView.underPageBackgroundColor retain];
		}
#endif
	}
	listenSetWKWebViewDrawsBackground(hostWebView, NO);
	if (listenEmbeddedHostScrollView != nil) {
		listenEmbeddedHostScrollView.drawsBackground = NO;
	}
#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120000
	if (@available(macOS 12.0, *)) {
		hostWebView.underPageBackgroundColor = [NSColor clearColor];
	}
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
		listenConfigureHostWebViewForEmbeddedUnderlay(hostWebView);

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
			[targetSuperview addSubview:listenEmbeddedContainerView positioned:NSWindowBelow relativeTo:relativeView];
		} else if (hostWebView != nil && hostWebView.superview == targetSuperview) {
			[targetSuperview addSubview:listenEmbeddedContainerView positioned:NSWindowBelow relativeTo:hostWebView];
		}

		listenEmbeddedContainerView.hidden = NO;
		listenEmbeddedContainerView.listenInteractive = NO;
		listenEmbeddedContainerView.translatesAutoresizingMaskIntoConstraints = YES;
		listenEmbeddedContainerView.frame = frame;
		listenEmbeddedContainerView.autoresizingMask = NSViewNotSizable;
		if ([listenEmbeddedContainerView respondsToSelector:@selector(setWantsLayer:)]) {
			listenEmbeddedContainerView.wantsLayer = YES;
			listenEmbeddedContainerView.layer.zPosition = 0;
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
		}
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

		NSHTTPCookie *cookie = [NSHTTPCookie cookieWithProperties:properties];
		if (cookie != nil) {
			[cookies addObject:cookie];
		}
	}

	return cookies;
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

		__block NSInteger remaining = cookies.count;
		for (NSHTTPCookie *cookie in cookies) {
			[cookieStore setCookie:cookie completionHandler:^{
				remaining -= 1;
				if (remaining <= 0) {
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
	"encoding/json"
	"unsafe"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func listenYouTubeMusicUserAgent() string {
	return youtubemusic.BrowserUserAgent
}

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

func showListenNativeEmbeddedWebView(playerNativeWindow unsafe.Pointer, hostNativeWindow unsafe.Pointer, rect ListenEmbeddedVideoRect) bool {
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
			boolToCInt(rect.Interactive),
		)
	})
	return shown != 0
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

func loadListenYouTubeMusicURL(window *application.WebviewWindow, targetURL string, cookies []appcookies.Record) {
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
		C.listenLoadYouTubeMusicURL(nativeWindow, cTargetURL, cCookies)
	})
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

func attachListenYouTubeMusicBridge(_ *application.WebviewWindow, _ string) func() {
	return nil
}
