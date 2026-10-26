package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gen2brain/beeep"
)

// deliverNotification shows one native notification for an event. Only
// the daemon calls it: with the daemon down, hooks stay silent.
//
// macOS posts through UNUserNotificationCenter, which needs the .app
// bundle — a bare-binary daemon shows nothing (no terminal-notifier or
// osascript fallback; beeep would shell out to both). Windows uses its
// own toast with click-to-focus; beeep (D-Bus on Linux) covers the rest.
func deliverNotification(ev Event) {
	title, body := notificationText(ev)
	subtitle := shortPath(ev.CWD)

	switch runtime.GOOS {
	case "darwin":
		nativeNotify(ev, title, subtitle, body)
		return
	case "windows":
		if windowsToastNotify(ev, title, subtitle, body) {
			return // toast click focuses the session via the agent-notify: protocol
		}
	}

	beeep.AppName = "agent-notify"
	if subtitle != "" {
		body = subtitle + " — " + body
	}
	_ = beeep.Notify(title, body, "")
}

// findCLI resolves a command via PATH plus the usual install locations —
// a launchd daemon's PATH is just /usr/bin:/bin and misses homebrew and
// ~/.local/bin; a Windows daemon's PATH is frozen at login, so a CLI
// installed afterwards only shows up through its install dir.
func findCLI(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		// an old Claude Desktop registers a Claude.exe execution alias that
		// outranks the CLI on PATH — running it opens the desktop app
		if !(runtime.GOOS == "windows" && name == "claude" &&
			strings.Contains(strings.ToLower(p), `\windowsapps\`)) {
			return p
		}
	}
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append([]string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".claude", "local"),
			filepath.Join(home, ".opencode", "bin")}, dirs...) // opencode's install script
	}
	exts := []string{""}
	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		dirs = append(dirs,
			filepath.Join(os.Getenv("APPDATA"), "npm"),                 // npm -g shims
			filepath.Join(local, "Programs", "OpenAI", "Codex", "bin"), // Codex installer
			filepath.Join(local, "agy", "bin"),                         // Antigravity installer
			filepath.Join(local, "cursor-agent"))                       // Cursor installer (.cmd shims)
		exts = []string{".exe", ".cmd"}
	}
	for _, d := range dirs {
		for _, ext := range exts {
			p := filepath.Join(d, name+ext)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

func notificationText(ev Event) (title, body string) {
	name := ev.Title
	if name == "" {
		name = projectName(ev.CWD)
	}
	switch ev.Kind {
	case "attention":
		title = "🔔 " + name
	default:
		title = "✅ " + name
	}
	body = ev.Message
	if body == "" {
		if ev.Kind == "attention" {
			body = T("notif.attention")
		} else {
			body = T("notif.done")
		}
	}
	return title, body
}

func projectName(cwd string) string {
	if cwd == "" {
		return "claude"
	}
	return filepath.Base(cwd)
}

// shortPath renders a cwd as a compact home-relative path for display.
func shortPath(cwd string) string {
	if cwd == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(cwd, home) {
		cwd = "~" + strings.TrimPrefix(cwd, home)
	}
	parts := strings.Split(cwd, string(filepath.Separator))
	if len(parts) > 4 {
		parts = append([]string{parts[0], "…"}, parts[len(parts)-2:]...)
		return strings.Join(parts, string(filepath.Separator))
	}
	return cwd
}
