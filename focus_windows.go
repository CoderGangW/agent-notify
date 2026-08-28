//go:build windows

package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"git.sr.ht/~jackmordaunt/go-toast"
	"golang.org/x/sys/windows/registry"
)

// Windows click-to-focus. Toasts activate the agent-notify: URL protocol,
// which relaunches this binary ("openurl") to POST /api/focus-notify at
// the daemon; the daemon then raises the session's window via win32.

// windowsToastNotify posts a toast whose click focuses the session.
// beeep's toast has no activation wiring at all, so we call go-toast
// directly. Returns false so the caller can fall back to beeep.
func windowsToastNotify(ev Event, title, subtitle, body string) bool {
	if subtitle != "" {
		body = subtitle + "\n" + body
	}
	n := toast.Notification{
		AppID: "agent-notify",
		Title: title,
		Body:  body,
		Icon:  toastIconPath(),
	}
	if ev.SessionID != "" {
		n.ActivationType = toast.Protocol
		n.ActivationArguments = "agent-notify:focus?session=" + url.QueryEscape(ev.SessionID)
	}
	return n.Push() == nil
}

// toastIconPath materializes the embedded app icon; toasts only take a
// filesystem path. Best-effort — an empty path just means no icon.
func toastIconPath() string {
	p := configPath()
	if p == "" {
		return ""
	}
	p = filepath.Join(filepath.Dir(p), "icon.png")
	if _, err := os.Stat(p); err != nil {
		if os.MkdirAll(filepath.Dir(p), 0o755) != nil ||
			os.WriteFile(p, iconLogo, 0o644) != nil {
			return ""
		}
	}
	return p
}

// registerURLProtocol claims the agent-notify: scheme for this binary in
// HKCU (no admin needed) so toast clicks can call back into the daemon.
func registerURLProtocol() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	root, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\agent-notify`, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer root.Close()
	_ = root.SetStringValue("", "URL:agent-notify")
	_ = root.SetStringValue("URL Protocol", "")
	cmd, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\agent-notify\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer cmd.Close()
	_ = cmd.SetStringValue("", `"`+exe+`" openurl "%1"`)
}

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procEnumWindows       = user32.NewProc("EnumWindows")
	procGetWindowTextW    = user32.NewProc("GetWindowTextW")
	procGetWindowTextLenW = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible   = user32.NewProc("IsWindowVisible")
	procIsIconic          = user32.NewProc("IsIconic")
	procShowWindow        = user32.NewProc("ShowWindow")
	procSetForeground     = user32.NewProc("SetForegroundWindow")
	procGetForeground     = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadID = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput = user32.NewProc("AttachThreadInput")
	procBringWindowToTop  = user32.NewProc("BringWindowToTop")
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procCurrentThreadID   = kernel32.NewProc("GetCurrentThreadId")
)

// ideMarkers tag window titles of apps we can address precisely; a match
// on one of these plus the folder name beats a bare substring hit.
var ideMarkers = []string{
	"visual studio code", "vscodium", "cursor", "windsurf",
	"windows terminal", "powershell", "command prompt", "wezterm",
	"alacritty", "hyper", "tabby", "conemu", "cmder",
}

// enumState carries needles and the running best match through the
// EnumWindows callback; the callback itself must be created exactly once
// (syscall.NewCallback allocations are permanent, capped per process).
var enumState struct {
	sync.Mutex
	folder, sess string
	best         uintptr
	bestScore    int
}

var enumCallback = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
	if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
		return 1 // continue enumerating
	}
	n, _, _ := procGetWindowTextLenW.Call(hwnd)
	if n == 0 {
		return 1
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	wt := strings.ToLower(syscall.UTF16ToString(buf))
	if wt == "" || strings.Contains(wt, "agent-notify") {
		return 1 // never focus our own window
	}
	score := 0
	if enumState.sess != "" && strings.Contains(wt, enumState.sess) {
		score += 3
	}
	if enumState.folder != "" && strings.Contains(wt, enumState.folder) {
		score += 2
	}
	if score > 0 {
		for _, m := range ideMarkers {
			if strings.Contains(wt, m) {
				score++
				break
			}
		}
	}
	if score > enumState.bestScore {
		enumState.bestScore = score
		enumState.best = hwnd
	}
	return 1
})

// focusNativeWindow finds the top-level window whose title mentions the
// session (project folder name or session title) and raises it.
func focusNativeWindow(cwd, title string) bool {
	folder := ""
	if cwd != "" {
		folder = strings.ToLower(filepath.Base(cwd))
	}
	sess := strings.ToLower(strings.TrimSpace(title))
	if len(sess) < 4 {
		sess = "" // too short to be a meaningful needle
	}
	if folder == "" && sess == "" {
		return false
	}

	enumState.Lock()
	defer enumState.Unlock()
	enumState.folder, enumState.sess = folder, sess
	enumState.best, enumState.bestScore = 0, 0
	procEnumWindows.Call(enumCallback, 0)
	if enumState.best == 0 {
		return false
	}
	forceForeground(enumState.best)
	return true
}

// forceForeground raises hwnd even though the daemon is a background
// process: attaching to the foreground window's input thread borrows its
// right to change focus. If Windows still refuses, the taskbar button
// flashes instead — visible, just not stolen.
func forceForeground(hwnd uintptr) {
	if ic, _, _ := procIsIconic.Call(hwnd); ic != 0 {
		procShowWindow.Call(hwnd, 9) // SW_RESTORE
	}
	fg, _, _ := procGetForeground.Call()
	if fg != 0 && fg != hwnd {
		var pid uint32
		fgThread, _, _ := procGetWindowThreadID.Call(fg, uintptr(unsafe.Pointer(&pid)))
		cur, _, _ := procCurrentThreadID.Call()
		if fgThread != cur {
			procAttachThreadInput.Call(cur, fgThread, 1)
			defer procAttachThreadInput.Call(cur, fgThread, 0)
		}
	}
	procBringWindowToTop.Call(hwnd)
	procSetForeground.Call(hwnd)
}
