//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

// Move to the Trash the way Finder does (restorable via "Put Back").
static char *anTrash(const char *path) {
	@autoreleasepool {
		NSURL *u = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSError *err = nil;
		if ([[NSFileManager defaultManager] trashItemAtURL:u resultingItemURL:nil error:&err]) {
			return NULL;
		}
		return strdup(err.localizedDescription.UTF8String ?: "trash failed");
	}
}

// Regular while a full window is open (Dock icon, Cmd-Tab), accessory
// (menu-bar only) otherwise.
static void anSetRegular(bool regular) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:regular ? NSApplicationActivationPolicyRegular
		                                   : NSApplicationActivationPolicyAccessory];
		if (regular) {
			[NSApp activateIgnoringOtherApps:YES];
		}
	});
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unsafe"
)

func moveToTrash(path string) error {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	if e := C.anTrash(cp); e != nil {
		defer C.free(unsafe.Pointer(e))
		return errors.New(C.GoString(e))
	}
	return nil
}

func setDockPresence(on bool) { C.anSetRegular(C.bool(on)) }

func revealPath(p string) { _ = exec.Command("open", "-R", p).Start() }

// resumeInTerminal opens the default terminal app on a throwaway
// .command script — no Automation permission needed, unlike scripting
// Terminal. The script deletes itself first.
func resumeInTerminal(cwd string, argv []string) error {
	f, err := os.CreateTemp("", "agent-notify-resume-*.command")
	if err != nil {
		return err
	}
	q := make([]string, len(argv))
	for i, a := range argv {
		q[i] = shellQuote(a)
	}
	fmt.Fprintf(f, "#!/bin/zsh -l\nrm -f -- \"$0\"\ncd %s && %s\n", shellQuote(cwd), strings.Join(q, " "))
	f.Close()
	if err := os.Chmod(f.Name(), 0o700); err != nil {
		return err
	}
	return exec.Command("open", f.Name()).Start()
}
