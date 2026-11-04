//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework ServiceManagement
#include <stdlib.h>
#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

// SMAppServiceStatus for a legacy LaunchAgent plist (-1 before macOS 13):
// 0 notRegistered, 1 enabled, 2 requiresApproval (switched off under
// Login Items → Allow in the Background), 3 notFound.
static int legacyAgentStatus(const char *path) {
	if (@available(macOS 13.0, *)) {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		return (int)[SMAppService statusForLegacyURL:url];
	}
	return -1;
}

static void openLoginItemsPane(void) {
	if (@available(macOS 13.0, *)) {
		[SMAppService openSystemSettingsLoginItems];
	}
}
*/
import "C"

import "unsafe"

const loginItemOff = 2 // SMAppServiceStatusRequiresApproval

func legacyAgentStatus(path string) int {
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	return int(C.legacyAgentStatus(cs))
}

func openLoginItems() { C.openLoginItemsPane() }
