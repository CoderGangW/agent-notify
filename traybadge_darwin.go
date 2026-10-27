//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework QuartzCore
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>

// Red count chip on the menu-bar icon. The icon itself is a template
// image (macOS tints it to the menu bar), which can't carry color, so
// the chip is a separate layer on the status-item button.

// Our status item's button. Wails keeps the NSStatusItem private, but
// status items live in NSStatusBarWindow instances owned by this app,
// and we only ever create one. The button sits at varying depth (macOS
// 27.2: NSStatusBarContentView > NSView > NSStatusBarButton), so search
// the whole subtree.
// ponytail: only the real item on the main display gets the chip — the
// system-drawn replicas on other displays' menu bars don't.
static NSStatusBarButton *findButton(NSView *v) {
	if ([v isKindOfClass:[NSStatusBarButton class]]) return (NSStatusBarButton *)v;
	for (NSView *sub in v.subviews) {
		NSStatusBarButton *b = findButton(sub);
		if (b != nil) return b;
	}
	return nil;
}

static NSStatusBarButton *trayButton(void) {
	for (NSWindow *w in [NSApp windows]) {
		if (![NSStringFromClass([w class]) containsString:@"StatusBarWindow"]) continue;
		NSStatusBarButton *b = findButton(w.contentView);
		if (b != nil) return b;
	}
	return nil;
}

static CALayer *badge;

static void anTrayBadge(int count) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSStatusBarButton *b = trayButton();
		if (b == nil) return;
		if (count <= 0) {
			[badge removeFromSuperlayer];
			badge = nil;
			return;
		}
		b.wantsLayer = YES;
		NSString *txt = count > 9 ? @"9+" : [NSString stringWithFormat:@"%d", count];
		NSDictionary *attrs = @{
			NSFontAttributeName : [NSFont systemFontOfSize:8 weight:NSFontWeightBold],
			NSForegroundColorAttributeName : NSColor.whiteColor,
		};
		NSSize ts = [txt sizeWithAttributes:attrs];
		CGFloat h = 11, w = MAX(h, ceil(ts.width) + 6);
		NSImage *img = [NSImage imageWithSize:NSMakeSize(w, h) flipped:NO
		                       drawingHandler:^BOOL(NSRect r) {
			[NSColor.systemRedColor setFill];
			[[NSBezierPath bezierPathWithRoundedRect:r xRadius:h / 2 yRadius:h / 2] fill];
			[txt drawAtPoint:NSMakePoint((w - ts.width) / 2, (h - ts.height) / 2) withAttributes:attrs];
			return YES;
		}];
		if (badge == nil) {
			badge = [CALayer layer];
			badge.zPosition = 10;
			[b.layer addSublayer:badge];
		}
		badge.contentsScale = b.window.backingScaleFactor ?: 2;
		badge.contents = img;
		// top-right of the (centered) icon, overlapping its corner
		NSRect bb = b.bounds;
		badge.frame = CGRectMake(NSMidX(bb) + 2, NSMaxY(bb) - h - 1, w, h);
	});
}
*/
import "C"

// nativeTrayBadge shows count on a red chip over the menu-bar icon (0 hides,
// two digits and up read "9+").
func nativeTrayBadge(count int) { C.anTrayBadge(C.int(count)) }
