//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework QuartzCore
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>

// Animate the window to a new height with the top edge pinned. NSWindow
// frames are bottom-left origin, so the origin shifts to keep the top
// where it is. Core Animation drives the frame via the animator proxy —
// setFrame:animate: steps on the main runloop and judders whenever the
// webview relayout blocks a step; the CA transaction stays smooth.
static void anAnimateHeight(void *nsWindow, int target) {
	NSWindow *w = (__bridge NSWindow *)nsWindow;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSRect f = [w frame];
		CGFloat topY = f.origin.y + f.size.height;
		f.size.height = (CGFloat)target;
		f.origin.y = topY - (CGFloat)target;
		[NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx) {
			ctx.duration = 0.26;
			ctx.timingFunction =
			    [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
			[[w animator] setFrame:f display:YES];
		}];
	});
}
*/
import "C"

import "unsafe"

// nativeAnimateHeight animates via AppKit; false = caller must fall back.
func nativeAnimateHeight(ptr unsafe.Pointer, target int) bool {
	if ptr == nil {
		return false
	}
	C.anAnimateHeight(ptr, C.int(target))
	return true
}
