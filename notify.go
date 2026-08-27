package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gen2brain/beeep"
)

// deliverNotification shows one native notification for an event.
//
// On macOS it prefers terminal-notifier when installed: -activate makes a
// click focus the session's IDE — osascript notifications belong to
// Script Editor, so a click opens Script Editor instead. (-sender icon
// faking died in terminal-notifier 3.0 / the UserNotifications framework.)
// Everything else (and the fallback) is beeep.
func deliverNotification(ev Event) {
	title, body := notificationText(ev)
	subtitle := shortPath(ev.CWD)

	if runtime.GOOS == "darwin" {
		if tn := findTerminalNotifier(); tn != "" {
			args := []string{"-title", title, "-message", body}
			if subtitle != "" {
				args = append(args, "-subtitle", subtitle)
			}
			if ev.SessionID != "" {
				args = append(args, "-group", "claude-notify-"+ev.SessionID)
			}
			if ev.Activate != "" {
				args = append(args, "-activate", ev.Activate)
			}
			if exec.Command(tn, args...).Run() == nil {
				return
			}
		}
	}

	beeep.AppName = "agent-notify"
	if subtitle != "" {
		body = subtitle + " — " + body
	}
	_ = beeep.Notify(title, body, "")
}

// findTerminalNotifier checks PATH plus the usual Homebrew locations,
// since a login-item daemon may not inherit a brew-aware PATH.
func findTerminalNotifier() string {
	if p, err := exec.LookPath("terminal-notifier"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/terminal-notifier", "/usr/local/bin/terminal-notifier"} {
		if _, err := os.Stat(p); err == nil {
			return p
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
