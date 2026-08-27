//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdlib.h>
#include "notify_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"unsafe"
)

// Native UNUserNotificationCenter delivery: notifications posted by the
// bundled daemon carry the app's own icon, and clicks come back through
// the delegate so they get the same exact-surface focus as row clicks.
// Unbundled runs (dev binary, hook fallback) can't use it — the callers
// fall through to terminal-notifier / beeep.

var nativeNotifyReady = false

// setupNativeNotify is called once at daemon start.
func setupNativeNotify() {
	if bool(C.unBundled()) {
		C.unSetup()
		nativeNotifyReady = true
	}
}

// notifyPayload is what a notification click needs to focus the session.
type notifyPayload struct {
	Activate string `json:"activate"`
	CWD      string `json:"cwd"`
	Surface  string `json:"surface"`
	Mux      muxRef `json:"mux"`
}

func nativeNotify(ev Event, title, subtitle, body string) bool {
	if !nativeNotifyReady {
		return false
	}
	pl, _ := json.Marshal(notifyPayload{
		Activate: ev.Activate, CWD: ev.CWD, Surface: ev.Surface, Mux: ev.Mux,
	})
	ident := "claude-notify"
	if ev.SessionID != "" {
		ident += "-" + ev.SessionID // same id replaces, like -group did
	}
	cs := func(s string) *C.char { return C.CString(s) }
	cIdent, cTitle, cSub, cBody, cPl := cs(ident), cs(title), cs(subtitle), cs(body), cs(string(pl))
	defer func() {
		for _, p := range []*C.char{cIdent, cTitle, cSub, cBody, cPl} {
			C.free(unsafe.Pointer(p))
		}
	}()
	C.unNotify(cIdent, cTitle, cSub, cBody, cPl)
	return true
}

//export goNotificationClicked
func goNotificationClicked(p *C.char) {
	var pl notifyPayload
	if json.Unmarshal([]byte(C.GoString(p)), &pl) != nil {
		return
	}
	// focusTarget shells out (osascript / open) — never block the
	// notification callback thread
	go focusTarget(pl.Activate, pl.Mux, pl.Surface, pl.CWD)
}
