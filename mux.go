package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Terminal-multiplexer awareness. Every supported multiplexer stamps its
// child processes with well-known environment variables (names are part
// of each tool's contract, identical across installs — only the values
// differ per pane). Hooks capture them; clicking an event or session
// replays them as a focus command before the hosting app is activated.

// muxRef identifies a pane/window inside a multiplexer.
type muxRef struct {
	Kind string `json:"kind"` // tmux | zellij | screen | cmux | wezterm | kitty | iterm
	Sess string `json:"sess"` // socket path / session name / workspace id
	Pane string `json:"pane"` // pane / window / surface id
}

// muxContext reads the calling environment. Inner multiplexers (tmux,
// zellij, screen) win over the hosting terminal's own pane identity —
// they're the more precise jump target, and the app-level focus still
// comes from the bundle id.
func muxContext() muxRef {
	if env := os.Getenv("TMUX"); env != "" {
		return muxRef{Kind: "tmux", Sess: strings.SplitN(env, ",", 2)[0], Pane: os.Getenv("TMUX_PANE")}
	}
	if os.Getenv("ZELLIJ") != "" {
		return muxRef{Kind: "zellij", Sess: os.Getenv("ZELLIJ_SESSION_NAME"), Pane: os.Getenv("ZELLIJ_PANE_ID")}
	}
	if sty := os.Getenv("STY"); sty != "" {
		return muxRef{Kind: "screen", Sess: sty, Pane: os.Getenv("WINDOW")}
	}
	if ws := os.Getenv("CMUX_WORKSPACE_ID"); ws != "" {
		// verified against cmux: panes are CMUX_PANEL_ID, tabs CMUX_SURFACE_ID
		pane := os.Getenv("CMUX_PANEL_ID")
		if pane == "" {
			pane = os.Getenv("CMUX_SURFACE_ID")
		}
		return muxRef{Kind: "cmux", Sess: ws, Pane: pane}
	}
	if pane := os.Getenv("WEZTERM_PANE"); pane != "" {
		return muxRef{Kind: "wezterm", Sess: os.Getenv("WEZTERM_UNIX_SOCKET"), Pane: pane}
	}
	if id := os.Getenv("KITTY_WINDOW_ID"); id != "" {
		return muxRef{Kind: "kitty", Sess: os.Getenv("KITTY_LISTEN_ON"), Pane: id}
	}
	if id := os.Getenv("ITERM_SESSION_ID"); id != "" {
		// "w0t0p0:UUID" — the UUID is the stable session id
		if i := strings.IndexByte(id, ':'); i >= 0 {
			id = id[i+1:]
		}
		return muxRef{Kind: "iterm", Pane: id}
	}
	return muxRef{}
}

// muxFocus best-effort selects the pane/window; every failure is silent
// (a detached or gone multiplexer just means the app-level focus is all
// the user gets).
func muxFocus(m muxRef) {
	if m.Kind == "" || m.Pane == "" {
		return
	}
	run := func(name string, args ...string) {
		if bin := findCLI(name); bin != "" {
			_ = exec.Command(bin, args...).Run()
		}
	}
	switch m.Kind {
	case "tmux":
		args := []string{}
		if m.Sess != "" {
			args = []string{"-S", m.Sess}
		}
		run("tmux", append(args, "select-window", "-t", m.Pane)...)
		run("tmux", append(args, "select-pane", "-t", m.Pane)...)
		run("tmux", append(args, "switch-client", "-t", m.Pane)...)
	case "zellij":
		if m.Sess != "" {
			// needs zellij >= the focus-pane-with-id action; older versions
			// just ignore it and the app-level focus still lands
			run("zellij", "--session", m.Sess, "action", "focus-pane-with-id", m.Pane)
		}
	case "screen":
		if m.Sess != "" {
			run("screen", "-S", m.Sess, "-X", "select", m.Pane)
		}
	case "cmux":
		run("cmux", "workspace", "select", m.Sess)
		run("cmux", "focus-pane", "--pane", m.Pane, "--workspace", m.Sess)
	case "wezterm":
		run("wezterm", "cli", "activate-pane", "--pane-id", m.Pane)
	case "kitty":
		if m.Sess != "" {
			run("kitten", "@", "--to", m.Sess, "focus-window", "--match", "id:"+m.Pane)
		}
	case "iterm":
		script := fmt.Sprintf(`tell application "iTerm2"
	repeat with w in windows
		repeat with tb in tabs of w
			repeat with s in sessions of tb
				if unique ID of s is "%s" then
					select s
					select tb
					select w
				end if
			end repeat
		end repeat
	end repeat
end tell`, m.Pane)
		_ = exec.Command("osascript", "-e", script).Run()
	}
}

// ---- Ghostty (macOS) ----
// Ghostty has no per-surface env var yet (ghostty-org/ghostty#10603 is
// still open), but 1.3+ ships a real AppleScript object model:
// application > windows > tabs > terminals, each terminal with a stable
// id and working directory. The daemon captures the focused terminal's id
// the moment a prompt is submitted — the one moment the session's surface
// is guaranteed frontmost — and clicking replays `focus` on that exact
// terminal. https://ghostty.org/docs/features/applescript

const ghosttyBundle = "com.mitchellh.ghostty"

// terminal ids are UUIDs in practice; accept hex-and-dashes only so the
// value can be spliced into a script without any escaping concerns
var ghosttyIDRe = regexp.MustCompile(`^[0-9A-Fa-f-]{1,64}$`)

// osascript runs a script with a hard timeout (a pending Automation
// permission dialog must not wedge the daemon) and returns trimmed stdout.
func osascript(script string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// asQuote escapes a string for an AppleScript literal (strconv.Quote is
// wrong here: it \u-escapes non-ASCII, which AppleScript doesn't parse).
func asQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// ghosttyCaptureSurface returns the id of the focused Ghostty terminal,
// or "" when Ghostty isn't running/frontmost (stale capture is worse
// than none). `application id ... is running` does not launch the app.
func ghosttyCaptureSurface() string {
	id := osascript(`if application id "` + ghosttyBundle + `" is running then
	tell application id "` + ghosttyBundle + `"
		if frontmost then
			try
				return (id of focused terminal of selected tab of front window) as text
			end try
		end if
	end tell
end if
return ""`)
	if ghosttyIDRe.MatchString(id) {
		return id
	}
	return ""
}

// ghosttyFocus selects the captured surface; without one it falls back to
// the terminal whose working directory equals the session cwd. Returns
// true when a terminal was focused (Ghostty's focus command also raises
// and activates its window).
func ghosttyFocus(surface, cwd string) bool {
	focusBy := func(filter string) bool {
		return osascript(`if application id "`+ghosttyBundle+`" is running then
	tell application id "`+ghosttyBundle+`"
		set m to every terminal whose `+filter+`
		if m is not {} then
			focus item 1 of m
			return "ok"
		end if
	end tell
end if
return ""`) == "ok"
	}
	if ghosttyIDRe.MatchString(surface) && focusBy(`id is "`+surface+`"`) {
		return true
	}
	return cwd != "" && focusBy("working directory is "+asQuote(cwd))
}
