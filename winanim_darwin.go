//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework QuartzCore
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>
#include <stdbool.h>

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

// Window alpha, set synchronously (the show that follows must see it).
static void anSetAlpha(void *nsWindow, double alpha) {
	NSWindow *w = (__bridge NSWindow *)nsWindow;
	void (^apply)(void) = ^{ [w setAlphaValue:alpha]; };
	if ([NSThread isMainThread]) {
		apply();
	} else {
		dispatch_sync(dispatch_get_main_queue(), apply);
	}
}

// Native alpha fade. easeIn: slow start (hides), else slow end (shows).
static void anFadeAlpha(void *nsWindow, double alpha, double secs, bool easeIn) {
	NSWindow *w = (__bridge NSWindow *)nsWindow;
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSAnimationContext runAnimationGroup:^(NSAnimationContext *ctx) {
			ctx.duration = secs;
			ctx.timingFunction = [CAMediaTimingFunction
			    functionWithName:(easeIn ? kCAMediaTimingFunctionEaseIn
			                             : kCAMediaTimingFunctionEaseOut)];
			[[w animator] setAlphaValue:alpha];
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

// nativeSetAlpha / nativeFadeAlpha drive the window's own opacity — the
// fade half of the show/hide animation (the scale half is page CSS).
func nativeSetAlpha(ptr unsafe.Pointer, a float64) {
	if ptr != nil {
		C.anSetAlpha(ptr, C.double(a))
	}
}

func nativeFadeAlpha(ptr unsafe.Pointer, a, secs float64, easeIn bool) {
	if ptr != nil {
		C.anFadeAlpha(ptr, C.double(a), C.double(secs), C.bool(easeIn))
	}
}
