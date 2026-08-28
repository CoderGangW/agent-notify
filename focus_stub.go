//go:build !windows

package main

// non-windows: toast/protocol/window focusing live in focus_windows.go;
// darwin has its own native path and linux falls back to openFolder.

func windowsToastNotify(ev Event, title, subtitle, body string) bool { return false }

func registerURLProtocol() {}

func focusNativeWindow(cwd, title string) bool { return false }
