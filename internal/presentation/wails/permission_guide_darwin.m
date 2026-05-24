//go:build darwin && cgo && !ios

#import "permission_guide_darwin.h"

#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#import <QuartzCore/QuartzCore.h>
#import <dispatch/dispatch.h>
#include <math.h>
#include <string.h>

static NSPanel *xiadownPermissionPanel = nil;
static NSTimer *xiadownPermissionTrackingTimer = nil;
static NSInteger xiadownPermissionMissingSettingsFrames = 0;
static BOOL xiadownPermissionIsDragging = NO;
static NSPasteboardType xiadownPromisedFileURLPasteboardType = @"com.apple.pasteboard.promised-file-url";

static void xiadownActivateSystemSettings(void);
static void xiadownClosePermissionPanel(void);
static NSTextField *xiadownPermissionLabel(NSString *text, NSRect frame, CGFloat fontSize, NSColor *color, NSTextAlignment alignment);
static void xiadownApplyDreamIconButtonSurface(NSView *view, CGFloat radius);
static NSRect xiadownClampFrameToVisibleArea(NSRect frame);
static void xiadownSetPermissionPanelDragging(BOOL dragging);

@interface XiaDownAppBundlePasteboardWriter : NSObject <NSPasteboardWriting>
@property(nonatomic, retain) NSURL *appURL;
- (instancetype)initWithAppURL:(NSURL *)appURL;
@end

@implementation XiaDownAppBundlePasteboardWriter

@synthesize appURL = _appURL;

- (instancetype)initWithAppURL:(NSURL *)appURL {
	self = [super init];
	if (self) {
		_appURL = [appURL retain];
	}
	return self;
}

- (void)dealloc {
	[_appURL release];
	[super dealloc];
}

- (NSArray<NSPasteboardType> *)writableTypesForPasteboard:(NSPasteboard *)pasteboard {
	return @[
		NSPasteboardTypeFileURL,
		NSPasteboardTypeURL,
		xiadownPromisedFileURLPasteboardType,
		NSPasteboardTypeString
	];
}

- (id)pasteboardPropertyListForType:(NSPasteboardType)type {
	if ([type isEqualToString:NSPasteboardTypeFileURL] ||
		[type isEqualToString:NSPasteboardTypeURL] ||
		[type isEqualToString:xiadownPromisedFileURLPasteboardType]) {
		return self.appURL.absoluteString;
	}
	return self.appURL.path ?: @"";
}

@end

@interface XiaDownPermissionPanelActions : NSObject
+ (instancetype)sharedActions;
- (void)closePermissionPanel:(id)sender;
@end

@implementation XiaDownPermissionPanelActions

+ (instancetype)sharedActions {
	static XiaDownPermissionPanelActions *actions = nil;
	if (actions == nil) {
		actions = [[XiaDownPermissionPanelActions alloc] init];
	}
	return actions;
}

- (void)closePermissionPanel:(id)sender {
	xiadownClosePermissionPanel();
	xiadownActivateSystemSettings();
}

@end

@interface XiaDownPermissionItemView : NSView
- (instancetype)initWithFrame:(NSRect)frame appURL:(NSURL *)appURL permissionName:(NSString *)permissionName;
@end

@implementation XiaDownPermissionItemView

- (NSView *)hitTest:(NSPoint)point {
	return nil;
}

- (instancetype)initWithFrame:(NSRect)frame appURL:(NSURL *)appURL permissionName:(NSString *)permissionName {
	self = [super initWithFrame:frame];
	if (self) {
		self.wantsLayer = YES;
		self.layer.cornerRadius = 10;
		self.layer.masksToBounds = YES;
		self.layer.borderWidth = 1;
		self.layer.borderColor = [[NSColor separatorColor] colorWithAlphaComponent:0.38].CGColor;
		self.layer.backgroundColor = [[NSColor controlBackgroundColor] colorWithAlphaComponent:0.62].CGColor;

		NSView *iconTile = [[[NSView alloc] initWithFrame:NSMakeRect(10, 10, 36, 36)] autorelease];
		iconTile.wantsLayer = YES;
		xiadownApplyDreamIconButtonSurface(iconTile, 11);
		[self addSubview:iconTile];

		NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:appURL.path];
		icon.size = NSMakeSize(26, 26);
		NSImageView *iconView = [[[NSImageView alloc] initWithFrame:NSMakeRect(5, 5, 26, 26)] autorelease];
		iconView.image = icon;
		iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
		[iconTile addSubview:iconView];

		NSTextField *permissionLabel = xiadownPermissionLabel(
			permissionName ?: @"Permission",
			NSMakeRect(58, 19, frame.size.width - 114, 18),
			13,
			[NSColor labelColor],
			NSTextAlignmentLeft
		);
		permissionLabel.font = [NSFont systemFontOfSize:13 weight:NSFontWeightSemibold];
		[self addSubview:permissionLabel];

		NSView *badge = [[[NSView alloc] initWithFrame:NSMakeRect(frame.size.width - 42, 13, 30, 30)] autorelease];
		badge.wantsLayer = YES;
		xiadownApplyDreamIconButtonSurface(badge, 10);
		[self addSubview:badge];

		NSTextField *arrow = xiadownPermissionLabel(
			@"↗",
			NSMakeRect(0, 5, 30, 18),
			14,
			[NSColor tertiaryLabelColor],
			NSTextAlignmentCenter
		);
		arrow.font = [NSFont systemFontOfSize:14 weight:NSFontWeightSemibold];
		[badge addSubview:arrow];
	}
	return self;
}

@end

@interface XiaDownPermissionDragSourceView : NSView <NSDraggingSource>
@property(nonatomic, retain) NSURL *appURL;
@property(nonatomic, assign) NSPoint mouseDownPoint;
@property(nonatomic, assign) BOOL draggingStarted;
@property(nonatomic, assign) NSRect dragRect;
- (instancetype)initWithFrame:(NSRect)frame dragRect:(NSRect)dragRect appURL:(NSURL *)appURL;
- (void)beginAppBundleDragWithEvent:(NSEvent *)event atPoint:(NSPoint)draggingPoint;
@end

@implementation XiaDownPermissionDragSourceView

@synthesize appURL = _appURL;
@synthesize mouseDownPoint = _mouseDownPoint;
@synthesize draggingStarted = _draggingStarted;
@synthesize dragRect = _dragRect;

- (instancetype)initWithFrame:(NSRect)frame dragRect:(NSRect)dragRect appURL:(NSURL *)appURL {
	self = [super initWithFrame:frame];
	if (self) {
		self.appURL = appURL;
		self.dragRect = dragRect;
	}
	return self;
}

- (void)dealloc {
	[_appURL release];
	[super dealloc];
}

- (NSView *)hitTest:(NSPoint)point {
	return NSPointInRect(point, self.dragRect) ? self : nil;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (BOOL)mouseDownCanMoveWindow {
	return NO;
}

- (void)mouseDown:(NSEvent *)event {
	self.mouseDownPoint = [self convertPoint:event.locationInWindow fromView:nil];
	self.draggingStarted = NO;
}

- (void)mouseDragged:(NSEvent *)event {
	if (self.appURL == nil || self.draggingStarted) {
		return;
	}
	NSPoint currentPoint = [self convertPoint:event.locationInWindow fromView:nil];
	CGFloat deltaX = currentPoint.x - self.mouseDownPoint.x;
	CGFloat deltaY = currentPoint.y - self.mouseDownPoint.y;
	if (hypot(deltaX, deltaY) < 4) {
		return;
	}
	[self beginAppBundleDragWithEvent:event atPoint:currentPoint];
}

- (void)beginAppBundleDragWithEvent:(NSEvent *)event atPoint:(NSPoint)draggingPoint {
	if (self.appURL == nil || self.draggingStarted) {
		return;
	}
	self.draggingStarted = YES;

	XiaDownAppBundlePasteboardWriter *writer = [[XiaDownAppBundlePasteboardWriter alloc] initWithAppURL:self.appURL];
	NSDraggingItem *dragItem = [[NSDraggingItem alloc] initWithPasteboardWriter:writer];
	[writer release];

	NSRect draggingFrame = NSMakeRect(
		draggingPoint.x - 28,
		draggingPoint.y - 28,
		56,
		56
	);
	NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:self.appURL.path];
	icon.size = NSMakeSize(56, 56);
	[dragItem setDraggingFrame:draggingFrame contents:icon];
	NSDraggingSession *session = [self beginDraggingSessionWithItems:@[dragItem] event:event source:self];
	session.animatesToStartingPositionsOnCancelOrFail = YES;
	session.draggingFormation = NSDraggingFormationNone;
	[dragItem release];
}

- (void)mouseUp:(NSEvent *)event {
	self.draggingStarted = NO;
}

- (void)resetCursorRects {
	[self addCursorRect:self.dragRect cursor:[NSCursor openHandCursor]];
}

- (void)draggingSession:(NSDraggingSession *)session willBeginAtPoint:(NSPoint)screenPoint {
	xiadownSetPermissionPanelDragging(YES);
}

- (NSDragOperation)draggingSession:(NSDraggingSession *)session
	sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
	return NSDragOperationCopy;
}

- (BOOL)ignoreModifierKeysForDraggingSession:(NSDraggingSession *)session {
	return YES;
}

- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)screenPoint operation:(NSDragOperation)operation {
	dispatch_async(dispatch_get_main_queue(), ^{
		self.draggingStarted = NO;
		xiadownSetPermissionPanelDragging(NO);
		xiadownClosePermissionPanel();
		xiadownActivateSystemSettings();
		dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 150 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
			xiadownActivateSystemSettings();
		});
	});
}

@end

static NSString *xiadownStringFromCString(const char *value, NSString *fallback) {
	if (value == NULL || strlen(value) == 0) {
		return fallback;
	}
	NSString *string = [NSString stringWithUTF8String:value];
	return string.length > 0 ? string : fallback;
}

static NSTextField *xiadownPermissionLabel(NSString *text, NSRect frame, CGFloat fontSize, NSColor *color, NSTextAlignment alignment) {
	NSTextField *label = [[[NSTextField alloc] initWithFrame:frame] autorelease];
	label.stringValue = text ?: @"";
	label.editable = NO;
	label.selectable = NO;
	label.bezeled = NO;
	label.drawsBackground = NO;
	label.lineBreakMode = NSLineBreakByTruncatingTail;
	label.alignment = alignment;
	label.font = [NSFont systemFontOfSize:fontSize weight:NSFontWeightRegular];
	label.textColor = color;
	return label;
}

static void xiadownApplyDreamIconButtonSurface(NSView *view, CGFloat radius) {
	view.wantsLayer = YES;
	view.layer.cornerRadius = radius;
	view.layer.masksToBounds = YES;
	view.layer.borderWidth = 0;
	view.layer.backgroundColor = [[NSColor controlBackgroundColor] colorWithAlphaComponent:0.64].CGColor;
	view.layer.shadowOpacity = 0;
}

static NSURL *xiadownResolveAppBundleURL(void) {
	NSBundle *bundle = [NSBundle mainBundle];
	NSURL *bundleURL = bundle.bundleURL;
	if ([bundleURL.pathExtension caseInsensitiveCompare:@"app"] == NSOrderedSame) {
		return bundleURL;
	}

	NSString *executablePath = bundle.executableURL.path;
	if (executablePath.length == 0) {
		NSArray<NSString *> *arguments = [NSProcessInfo processInfo].arguments;
		executablePath = arguments.count > 0 ? arguments.firstObject : nil;
	}
	if (executablePath.length == 0) {
		return nil;
	}

	NSRange range = [executablePath rangeOfString:@".app/Contents/MacOS" options:NSCaseInsensitiveSearch | NSBackwardsSearch];
	if (range.location == NSNotFound) {
		return nil;
	}
	NSString *appPath = [executablePath substringToIndex:range.location + range.length - [@"/Contents/MacOS" length]];
	if (appPath.length == 0) {
		return nil;
	}
	return [NSURL fileURLWithPath:appPath isDirectory:YES];
}

static NSRunningApplication *xiadownSystemSettingsApplication(void) {
	NSArray<NSString *> *bundleIDs = @[@"com.apple.SystemSettings", @"com.apple.systempreferences"];
	for (NSString *bundleID in bundleIDs) {
		NSArray<NSRunningApplication *> *applications = [NSRunningApplication runningApplicationsWithBundleIdentifier:bundleID];
		if (applications.count > 0) {
			return applications.firstObject;
		}
	}
	return nil;
}

static void xiadownActivateSystemSettings(void) {
	NSRunningApplication *application = xiadownSystemSettingsApplication();
	if (application != nil) {
		[application activateWithOptions:NSApplicationActivateIgnoringOtherApps];
	}
}

static void xiadownOpenSettingsURL(NSString *settingsURL) {
	NSURL *url = [NSURL URLWithString:settingsURL ?: @""];
	if (url != nil) {
		[[NSWorkspace sharedWorkspace] openURL:url];
	}
}

static CGFloat xiadownMaxScreenY(void) {
	CGFloat maxY = 0;
	for (NSScreen *screen in [NSScreen screens]) {
		maxY = MAX(maxY, NSMaxY(screen.frame));
	}
	return maxY > 0 ? maxY : NSMaxY([NSScreen mainScreen].frame);
}

static NSRect xiadownAppKitFrameFromCGWindowBounds(CGRect bounds) {
	CGFloat maxY = xiadownMaxScreenY();
	return NSMakeRect(bounds.origin.x, maxY - CGRectGetMaxY(bounds), bounds.size.width, bounds.size.height);
}

static BOOL xiadownSystemSettingsWindowFrame(NSRect *result) {
	NSRunningApplication *application = xiadownSystemSettingsApplication();
	if (application == nil) {
		return NO;
	}

	pid_t processIdentifier = application.processIdentifier;
	NSArray *windows = [(NSArray *)CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	) autorelease];
	CGFloat bestArea = 0;
	NSRect bestFrame = NSZeroRect;

	for (NSDictionary *window in windows) {
		NSNumber *ownerPID = window[(id)kCGWindowOwnerPID];
		NSNumber *layer = window[(id)kCGWindowLayer];
		NSNumber *alpha = window[(id)kCGWindowAlpha];
		if (ownerPID.intValue != processIdentifier || layer.intValue != 0 || alpha.doubleValue <= 0.01) {
			continue;
		}

		CGRect bounds = CGRectZero;
		if (!CGRectMakeWithDictionaryRepresentation((CFDictionaryRef)window[(id)kCGWindowBounds], &bounds)) {
			continue;
		}
		CGFloat area = bounds.size.width * bounds.size.height;
		if (area > bestArea) {
			bestArea = area;
			bestFrame = xiadownAppKitFrameFromCGWindowBounds(bounds);
		}
	}

	if (bestArea <= 40000) {
		return NO;
	}
	if (result != NULL) {
		*result = bestFrame;
	}
	return YES;
}

static NSScreen *xiadownScreenForRect(NSRect rect) {
	NSScreen *bestScreen = [NSScreen mainScreen];
	CGFloat bestArea = 0;
	for (NSScreen *screen in [NSScreen screens]) {
		NSRect intersection = NSIntersectionRect(screen.frame, rect);
		CGFloat area = intersection.size.width * intersection.size.height;
		if (area > bestArea) {
			bestArea = area;
			bestScreen = screen;
		}
	}
	return bestScreen;
}

static NSRect xiadownClampFrameToVisibleArea(NSRect frame) {
	NSScreen *screen = xiadownScreenForRect(frame);
	NSRect visible = screen ? screen.visibleFrame : [NSScreen mainScreen].visibleFrame;
	frame.origin.x = MIN(MAX(frame.origin.x, NSMinX(visible) + 16), NSMaxX(visible) - frame.size.width - 16);
	frame.origin.y = MIN(MAX(frame.origin.y, NSMinY(visible) + 16), NSMaxY(visible) - frame.size.height - 16);
	return frame;
}

static NSRect xiadownPermissionPanelFallbackFrame(CGFloat width, CGFloat height) {
	NSScreen *screen = [NSScreen mainScreen];
	NSRect visible = screen ? screen.visibleFrame : NSMakeRect(0, 0, 1280, 800);
	return xiadownClampFrameToVisibleArea(NSMakeRect(
		NSMaxX(visible) - width - 28,
		NSMaxY(visible) - height - 96,
		width,
		height
	));
}

static NSRect xiadownPermissionPanelFrameNearSettings(NSRect settingsFrame, CGFloat width, CGFloat height) {
	CGFloat sidebarWidth = MIN(244, settingsFrame.size.width * 0.38);
	CGFloat contentX = NSMinX(settingsFrame) + sidebarWidth;
	CGFloat contentWidth = MAX(width, settingsFrame.size.width - sidebarWidth);
	CGFloat x = contentX + (contentWidth - width) / 2;
	CGFloat y = NSMinY(settingsFrame) - height + 18;
	return xiadownClampFrameToVisibleArea(NSMakeRect(x, y, width, height));
}

static void xiadownSnapPermissionPanelToSettings(void) {
	if (xiadownPermissionPanel == nil || xiadownPermissionIsDragging) {
		return;
	}
	NSRect settingsFrame = NSZeroRect;
	NSRect panelFrame = xiadownPermissionPanel.frame;
	if (xiadownSystemSettingsWindowFrame(&settingsFrame)) {
		xiadownPermissionMissingSettingsFrames = 0;
		NSRect target = xiadownPermissionPanelFrameNearSettings(settingsFrame, panelFrame.size.width, panelFrame.size.height);
		if (fabs(target.origin.x - panelFrame.origin.x) > 1 || fabs(target.origin.y - panelFrame.origin.y) > 1) {
			[xiadownPermissionPanel setFrame:target display:YES animate:NO];
		}
		return;
	}

	xiadownPermissionMissingSettingsFrames += 1;
	if (xiadownPermissionMissingSettingsFrames > 100) {
		xiadownClosePermissionPanel();
	}
}

static void xiadownStartPermissionPanelTracking(void) {
	if (xiadownPermissionTrackingTimer != nil) {
		[xiadownPermissionTrackingTimer invalidate];
		xiadownPermissionTrackingTimer = nil;
	}
	xiadownPermissionMissingSettingsFrames = 0;
	xiadownPermissionTrackingTimer = [NSTimer timerWithTimeInterval:0.05 repeats:YES block:^(NSTimer *timer) {
		xiadownSnapPermissionPanelToSettings();
		if (xiadownPermissionPanel == nil) {
			[timer invalidate];
		}
	}];
	[[NSRunLoop mainRunLoop] addTimer:xiadownPermissionTrackingTimer forMode:NSRunLoopCommonModes];
}

static void xiadownSetPermissionPanelDragging(BOOL dragging) {
	xiadownPermissionIsDragging = dragging;
	if (xiadownPermissionPanel == nil) {
		return;
	}
	xiadownPermissionPanel.ignoresMouseEvents = dragging;
	xiadownPermissionPanel.alphaValue = dragging ? 0.68 : 1;
	if (dragging) {
		[xiadownPermissionPanel orderBack:nil];
	} else {
		[xiadownPermissionPanel orderFrontRegardless];
	}
}

static void xiadownClosePermissionPanel(void) {
	if (xiadownPermissionTrackingTimer != nil) {
		[xiadownPermissionTrackingTimer invalidate];
		xiadownPermissionTrackingTimer = nil;
	}
	xiadownPermissionIsDragging = NO;
	if (xiadownPermissionPanel == nil) {
		return;
	}
	[xiadownPermissionPanel orderOut:nil];
	[xiadownPermissionPanel release];
	xiadownPermissionPanel = nil;
}

static NSVisualEffectView *xiadownPermissionRootView(NSRect frame) {
	NSVisualEffectView *root = [[[NSVisualEffectView alloc] initWithFrame:frame] autorelease];
	root.material = NSVisualEffectMaterialWindowBackground;
	root.blendingMode = NSVisualEffectBlendingModeBehindWindow;
	root.state = NSVisualEffectStateActive;
	root.wantsLayer = YES;
	root.layer.cornerRadius = 14;
	root.layer.masksToBounds = YES;
	root.layer.borderWidth = 1;
	root.layer.borderColor = [[NSColor separatorColor] colorWithAlphaComponent:0.32].CGColor;
	root.layer.backgroundColor = [[NSColor windowBackgroundColor] colorWithAlphaComponent:0.74].CGColor;
	return root;
}

static void xiadownShowPermissionPanel(NSURL *appURL, NSString *permissionName, NSString *hint) {
	if (appURL == nil) {
		return;
	}

	xiadownClosePermissionPanel();

	const CGFloat width = 376;
	const CGFloat height = 154;
	NSRect settingsFrame = NSZeroRect;
	NSRect frame = xiadownSystemSettingsWindowFrame(&settingsFrame)
		? xiadownPermissionPanelFrameNearSettings(settingsFrame, width, height)
		: xiadownPermissionPanelFallbackFrame(width, height);
	NSUInteger style = NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel;
	NSPanel *panel = [[NSPanel alloc] initWithContentRect:frame
		styleMask:style
		backing:NSBackingStoreBuffered
		defer:NO];
	panel.title = @"";
	panel.level = NSFloatingWindowLevel;
	panel.floatingPanel = YES;
	panel.hidesOnDeactivate = NO;
	panel.releasedWhenClosed = NO;
	panel.opaque = NO;
	panel.backgroundColor = [NSColor clearColor];
	panel.hasShadow = YES;
	panel.movableByWindowBackground = NO;
	panel.collectionBehavior =
		NSWindowCollectionBehaviorCanJoinAllSpaces |
		NSWindowCollectionBehaviorFullScreenAuxiliary |
		NSWindowCollectionBehaviorStationary;

	NSVisualEffectView *root = xiadownPermissionRootView(NSMakeRect(0, 0, width, height));
	panel.contentView = root;

	NSRect itemFrame = NSMakeRect(14, 84, width - 28, 56);
	XiaDownPermissionItemView *permissionItem = [[[XiaDownPermissionItemView alloc]
		initWithFrame:itemFrame
		appURL:appURL
		permissionName:permissionName
	] autorelease];
	[root addSubview:permissionItem];

	NSTextField *hintLabel = xiadownPermissionLabel(
		hint,
		NSMakeRect(18, 57, width - 36, 16),
		11,
		[NSColor tertiaryLabelColor],
		NSTextAlignmentCenter
	);
	hintLabel.font = [NSFont systemFontOfSize:11 weight:NSFontWeightMedium];
	[root addSubview:hintLabel];

	XiaDownPermissionDragSourceView *dragSource = [[[XiaDownPermissionDragSourceView alloc]
		initWithFrame:root.bounds
		dragRect:itemFrame
		appURL:appURL
	] autorelease];
	dragSource.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
	[root addSubview:dragSource positioned:NSWindowAbove relativeTo:permissionItem];

	NSButton *closeButton = [[[NSButton alloc] initWithFrame:NSMakeRect((width - 34) / 2, 10, 34, 34)] autorelease];
	closeButton.title = @"×";
	closeButton.bordered = NO;
	closeButton.wantsLayer = YES;
	xiadownApplyDreamIconButtonSurface(closeButton, 11);
	closeButton.font = [NSFont systemFontOfSize:16 weight:NSFontWeightSemibold];
	closeButton.contentTintColor = [NSColor tertiaryLabelColor];
	closeButton.target = [XiaDownPermissionPanelActions sharedActions];
	closeButton.action = @selector(closePermissionPanel:);
	[root addSubview:closeButton];

	xiadownPermissionPanel = panel;
	[panel orderFrontRegardless];
	xiadownStartPermissionPanelTracking();
}

int xiadownOpenPermissionGuide(const char *settings_url, const char *permission_name, const char *hint) {
	NSURL *appURL = [xiadownResolveAppBundleURL() retain];
	NSString *settingsURL = [xiadownStringFromCString(
		settings_url,
		@"x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"
	) retain];
	NSString *resolvedPermissionName = xiadownStringFromCString(permission_name, @"Screen & system audio recording");
	NSString *permissionName = [resolvedPermissionName retain];
	NSString *resolvedHint = xiadownStringFromCString(hint, @"Drag permissions to the list above");
	NSString *panelHint = [resolvedHint retain];
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			xiadownOpenSettingsURL(settingsURL);
			dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 350 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
				xiadownActivateSystemSettings();
				xiadownShowPermissionPanel(appURL, permissionName, panelHint);
				[appURL release];
				[settingsURL release];
				[permissionName release];
				[panelHint release];
			});
		}
	});
	return appURL == nil ? 0 : 1;
}
