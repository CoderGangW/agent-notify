package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Codex CLI integration: ~/.codex/config.toml `notify` runs a program with
// one JSON argument per event. `agent-notify codex-hook` adapts that to a
// daemon event, so Codex turns land in the same tray/window/notifications.

// runCodexHook is invoked by Codex as `agent-notify codex-hook <json>`.
func runCodexHook() {
	defer os.Exit(0)
	if len(os.Args) < 3 {
		return
	}
	var n struct {
		Type          string   `json:"type"`
		InputMessages []string `json:"input-messages"`
		Last          string   `json:"last-assistant-message"`
	}
	if json.Unmarshal([]byte(os.Args[2]), &n) != nil {
		return
	}
	if n.Type != "agent-turn-complete" {
		return
	}

	title := ""
	if len(n.InputMessages) > 0 {
		title = condense(n.InputMessages[0], 60)
	}
	if title == "" {
		title = "Codex"
	}
	cwd, _ := os.Getwd() // notify runs in the session's working directory
	activate := ""
	if runtime.GOOS == "darwin" {
		activate = os.Getenv("__CFBundleIdentifier")
	}
	mux := muxContext()
	deliver(Event{
		CWD:      cwd,
		Kind:     "done",
		Source:   "codex",
		Title:    title,
		Activate: activate,
		Mux:      mux,
		Message:  condense(n.Last, 180),
		Time:     time.Now(),
	})
}

// codexHooked reports whether ~/.codex/config.toml already runs our
// notify hook.
func codexHooked() bool {
	data, _ := os.ReadFile(homePath(".codex", "config.toml"))
	return strings.Contains(string(data), "notify") &&
		(strings.Contains(string(data), "claude-notify") || strings.Contains(string(data), "agent-notify"))
}

// installCodexHook wires the notify hook into ~/.codex/config.toml.
func installCodexHook() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)
	line := fmt.Sprintf("notify = [%q, \"codex-hook\"]", exe)

	path := filepath.Join(home, ".codex", "config.toml")
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "notify") {
		if strings.Contains(string(data), "claude-notify") || strings.Contains(string(data), "agent-notify") {
			// our own entry from an older binary path/name: repoint it
			lines := strings.Split(string(data), "\n")
			for i, l := range lines {
				if strings.HasPrefix(strings.TrimSpace(l), "notify") {
					lines[i] = line
				}
			}
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
		return fmt.Errorf(T("codex.already")+"\n  %s", path, line)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out := string(data)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += line + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}

// runInstallCodex is the CLI entry (`agent-notify install-codex`).
func runInstallCodex() {
	if err := installCodexHook(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf(T("codex.installed")+"\n", homePath(".codex", "config.toml"))
}
