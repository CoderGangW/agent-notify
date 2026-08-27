package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
