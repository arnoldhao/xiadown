//go:build darwin && cgo && !ios && !server

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c -fblocks
#cgo LDFLAGS: -framework AppKit -framework Foundation -framework QuartzCore

#include <stddef.h>
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>
#import <objc/runtime.h>

static const void *xiadownMainStartupOverlayKey = &xiadownMainStartupOverlayKey;

@interface XiaDownMainStartupOverlay : NSVisualEffectView
@property(nonatomic, assign) CFTimeInterval installedAt;
@property(nonatomic, assign, getter=isDismissing) BOOL dismissing;
@property(nonatomic, retain) NSImageView *startupIconView;
@end

@implementation XiaDownMainStartupOverlay
- (void)dealloc {
	[_startupIconView release];
	[super dealloc];
}
@end

static BOOL xiadownStartupOverlayUsesDarkAppearance(NSWindow *window) {
	NSAppearanceName match = [window.effectiveAppearance
		bestMatchFromAppearancesWithNames:@[
			NSAppearanceNameAqua,
			NSAppearanceNameDarkAqua
		]];
	return [match isEqualToString:NSAppearanceNameDarkAqua];
}

static NSImage *xiadownStartupOverlayImage(const unsigned char *bytes, size_t length) {
	if (bytes != NULL && length > 0) {
		NSData *data = [NSData dataWithBytes:bytes length:length];
		NSImage *image = [[NSImage alloc] initWithData:data];
		if (image != nil) {
			return image;
		}
	}
	return [[[NSApplication sharedApplication] applicationIconImage] retain];
}

// The NSWindow is still hidden when this runs. Installing the complete native
// surface before Window.Show guarantees that the first composited frame already
// contains the app icon, even when the WebView has not received a byte of HTML.
static int xiadownInstallMainStartupOverlay(
	void *nativeWindow,
	const unsigned char *iconBytes,
	size_t iconLength,
	int requestedDarkAppearance
) {
	@autoreleasepool {
		if (nativeWindow == NULL) {
			return 0;
		}

		NSWindow *window = (NSWindow *)nativeWindow;
		XiaDownMainStartupOverlay *existing =
			(XiaDownMainStartupOverlay *)objc_getAssociatedObject(
				window,
				xiadownMainStartupOverlayKey
			);
		if (existing != nil) {
			return 1;
		}

		NSView *contentView = window.contentView;
		if (contentView == nil) {
			return 0;
		}

		NSImage *image = xiadownStartupOverlayImage(iconBytes, iconLength);
		if (image == nil) {
			return 0;
		}

		XiaDownMainStartupOverlay *overlay =
			[[XiaDownMainStartupOverlay alloc] initWithFrame:contentView.bounds];
		overlay.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
		overlay.blendingMode = NSVisualEffectBlendingModeBehindWindow;
		overlay.material = NSVisualEffectMaterialUnderWindowBackground;
		overlay.state = NSVisualEffectStateActive;
		overlay.wantsLayer = YES;
		overlay.layer.masksToBounds = YES;
		overlay.installedAt = CACurrentMediaTime();

		BOOL dark = requestedDarkAppearance < 0
			? xiadownStartupOverlayUsesDarkAppearance(window)
			: requestedDarkAppearance == 1;
		NSColor *tint = dark
			? [NSColor colorWithSRGBRed:0.067 green:0.094 blue:0.078 alpha:0.88]
			: [NSColor colorWithSRGBRed:0.925 green:0.957 blue:0.929 alpha:0.86];
		overlay.layer.backgroundColor = tint.CGColor;

		NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSZeroRect];
		iconView.translatesAutoresizingMaskIntoConstraints = NO;
		iconView.image = image;
		iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
		iconView.wantsLayer = YES;
		iconView.layer.shadowColor = NSColor.blackColor.CGColor;
		iconView.layer.shadowOffset = CGSizeMake(0.0, -7.0);
		iconView.layer.shadowRadius = 13.0;
		iconView.layer.shadowOpacity = dark ? 0.34 : 0.20;
		overlay.startupIconView = iconView;

		BOOL reduceMotion = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;
		// Match the inline HTML fallback exactly: three small dots with a
		// 1.2-second wave and 120ms stagger. A platform spinner here made the
		// same launch look like two unrelated loading states when ownership
		// moved from AppKit to the WebView.
		NSStackView *progress = [[NSStackView alloc] initWithFrame:NSZeroRect];
		progress.translatesAutoresizingMaskIntoConstraints = NO;
		progress.orientation = NSUserInterfaceLayoutOrientationHorizontal;
		progress.alignment = NSLayoutAttributeCenterY;
		progress.spacing = 5.0;
		NSColor *dotColor = dark
			? [NSColor colorWithSRGBRed:0.91 green:0.94 blue:0.92 alpha:1.0]
			: [NSColor colorWithSRGBRed:0.08 green:0.12 blue:0.09 alpha:1.0];
		for (NSInteger index = 0; index < 3; index++) {
			NSView *dot = [[NSView alloc] initWithFrame:NSZeroRect];
			dot.translatesAutoresizingMaskIntoConstraints = NO;
			dot.wantsLayer = YES;
			dot.layer.backgroundColor = dotColor.CGColor;
			dot.layer.cornerRadius = 2.5;
			dot.layer.opacity = reduceMotion ? 0.46 : 0.26;
			[NSLayoutConstraint activateConstraints:@[
				[dot.widthAnchor constraintEqualToConstant:5.0],
				[dot.heightAnchor constraintEqualToConstant:5.0]
			]];
			[progress addArrangedSubview:dot];

			if (!reduceMotion) {
				CAKeyframeAnimation *opacity =
					[CAKeyframeAnimation animationWithKeyPath:@"opacity"];
				opacity.values = @[@0.26, @0.72, @0.26, @0.26];
				opacity.keyTimes = @[@0.0, @0.35, @0.70, @1.0];

				CAKeyframeAnimation *lift =
					[CAKeyframeAnimation animationWithKeyPath:@"transform.translation.y"];
				lift.values = @[@0.0, @2.5, @0.0, @0.0];
				lift.keyTimes = opacity.keyTimes;

				CAAnimationGroup *wave = [CAAnimationGroup animation];
				wave.animations = @[opacity, lift];
				wave.duration = 1.2;
				wave.beginTime = CACurrentMediaTime() + ((double)index * 0.12);
				wave.repeatCount = HUGE_VALF;
				wave.timingFunction = [CAMediaTimingFunction
					functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
				[dot.layer addAnimation:wave forKey:@"xiadown-startup-dot"];
			}
			[dot release];
		}

		[overlay addSubview:iconView];
		[overlay addSubview:progress];
		[NSLayoutConstraint activateConstraints:@[
			[iconView.widthAnchor constraintEqualToConstant:112.0],
			[iconView.heightAnchor constraintEqualToConstant:112.0],
			[iconView.centerXAnchor constraintEqualToAnchor:overlay.centerXAnchor],
			[iconView.centerYAnchor constraintEqualToAnchor:overlay.centerYAnchor constant:-10.0],
			[progress.centerXAnchor constraintEqualToAnchor:overlay.centerXAnchor],
			[progress.topAnchor constraintEqualToAnchor:iconView.bottomAnchor constant:18.0]
		]];

		if (!reduceMotion) {
			CABasicAnimation *breathe = [CABasicAnimation animationWithKeyPath:@"transform.scale"];
			breathe.fromValue = @0.975;
			breathe.toValue = @1.0;
			breathe.duration = 0.9;
			breathe.autoreverses = YES;
			breathe.repeatCount = HUGE_VALF;
			breathe.timingFunction = [CAMediaTimingFunction
				functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
			[iconView.layer addAnimation:breathe forKey:@"xiadown-startup-breathe"];
		}

		[contentView addSubview:overlay positioned:NSWindowAbove relativeTo:nil];
		objc_setAssociatedObject(
			window,
			xiadownMainStartupOverlayKey,
			overlay,
			OBJC_ASSOCIATION_RETAIN_NONATOMIC
		);

		[progress release];
		[iconView release];
		[overlay release];
		[image release];
		return 1;
	}
}

static void xiadownDismissMainStartupOverlay(
	void *nativeWindow,
	double minimumVisibleSeconds
) {
	@autoreleasepool {
		if (nativeWindow == NULL) {
			return;
		}

		NSWindow *window = (NSWindow *)nativeWindow;
		XiaDownMainStartupOverlay *overlay =
			(XiaDownMainStartupOverlay *)objc_getAssociatedObject(
				window,
				xiadownMainStartupOverlayKey
			);
		if (overlay == nil || overlay.isDismissing) {
			return;
		}
		overlay.dismissing = YES;

		CFTimeInterval elapsed = CACurrentMediaTime() - overlay.installedAt;
		CFTimeInterval remaining = MAX(0.0, minimumVisibleSeconds - elapsed);
		void (^performDismissal)(void) = ^{
			XiaDownMainStartupOverlay *current =
				(XiaDownMainStartupOverlay *)objc_getAssociatedObject(
					window,
					xiadownMainStartupOverlayKey
				);
			if (current == nil) {
				return;
			}

			BOOL reduceMotion = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;
			NSTimeInterval duration = reduceMotion ? 0.01 : 0.12;
			[current.startupIconView.layer removeAnimationForKey:@"xiadown-startup-breathe"];
			if (!reduceMotion) {
				CABasicAnimation *expand = [CABasicAnimation animationWithKeyPath:@"transform.scale"];
				expand.fromValue = @1.0;
				expand.toValue = @1.075;
				expand.duration = duration;
				expand.fillMode = kCAFillModeForwards;
				expand.removedOnCompletion = NO;
				expand.timingFunction = [CAMediaTimingFunction
					functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
				[current.startupIconView.layer addAnimation:expand forKey:@"xiadown-startup-dismiss"];
			}

			[NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
				context.duration = duration;
				context.timingFunction = [CAMediaTimingFunction
					functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
				current.animator.alphaValue = 0.0;
			} completionHandler:^{
				[current removeFromSuperview];
				objc_setAssociatedObject(
					window,
					xiadownMainStartupOverlayKey,
					nil,
					OBJC_ASSOCIATION_ASSIGN
				);
			}];
		};

		if (remaining <= 0.0) {
			performDismissal();
			return;
		}
		dispatch_after(
			dispatch_time(DISPATCH_TIME_NOW, (int64_t)(remaining * NSEC_PER_SEC)),
			dispatch_get_main_queue(),
			performDismissal
		);
	}
}
*/
import "C"

import (
	"runtime"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Keep a short anti-flash floor so the native icon visibly registers without
// holding an already-ready application before the 120ms cross-fade begins.
const minimumMainStartupOverlayVisible = 120 * time.Millisecond

func registerMainWindowStartupOverlayEvents(
	manager *WindowManager,
	window *application.WebviewWindow,
) {
	if manager == nil || window == nil {
		return
	}
	window.OnWindowEvent(
		events.Mac.WebViewDidStartProvisionalNavigation,
		func(_ *application.WindowEvent) {
			manager.ensureMainWindowStartupOverlay()
		},
	)
	window.OnWindowEvent(
		events.Mac.WebViewDidFinishNavigation,
		func(_ *application.WindowEvent) {
			manager.markMainWindowHTMLSurfaceReady()
		},
	)
}

func supportsMainWindowStartupOverlay() bool { return true }

func installMainWindowStartupOverlay(
	nativeWindow unsafe.Pointer,
	icon []byte,
	appearance string,
) bool {
	if nativeWindow == nil {
		return false
	}
	var iconPointer *C.uchar
	if len(icon) > 0 {
		iconPointer = (*C.uchar)(unsafe.Pointer(&icon[0]))
	}
	darkAppearance := C.int(-1)
	if appearance == "dark" {
		darkAppearance = 1
	} else if appearance == "light" {
		darkAppearance = 0
	}
	installed := C.xiadownInstallMainStartupOverlay(
		nativeWindow,
		iconPointer,
		C.size_t(len(icon)),
		darkAppearance,
	) == 1
	runtime.KeepAlive(icon)
	return installed
}

func dismissMainWindowStartupOverlay(nativeWindow unsafe.Pointer) {
	if nativeWindow == nil {
		return
	}
	C.xiadownDismissMainStartupOverlay(
		nativeWindow,
		C.double(minimumMainStartupOverlayVisible.Seconds()),
	)
}
