package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func settingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot find home directory:", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func loadSettings(path string) map[string]any {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintf(os.Stderr, "%s is not valid JSON: %v\n", path, err)
			os.Exit(1)
		}
		backup := path + ".bak-claude-notify"
		if err := os.WriteFile(backup, data, 0o644); err == nil {
			fmt.Printf(T("install.backup")+"\n", backup)
		}
	}
	return settings
}

func saveSettings(path string, settings map[string]any) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// isOurCommand reports whether a hook command string belongs to claude-notify.
func isOurCommand(cmd string) bool {
	return (strings.Contains(cmd, "agent-notify") || strings.Contains(cmd, "claude-notify")) &&
		strings.HasSuffix(strings.TrimSpace(cmd), " hook")
}

func hookEntries(hooks map[string]any, event string) []any {
	arr, _ := hooks[event].([]any)
	return arr
}

// installBinary copies the running executable into a stable location.
// launchd refuses to run binaries out of TCC-protected folders (Desktop/
// Documents/Downloads) — the process hangs in dyld before main — and a dev
// build path would break the install when the working copy moves anyway.
func installBinary(exe string) string {
	var dir string
	home, err := os.UserHomeDir()
	if err != nil {
		return exe
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" && filepath.Separator == '\\' {
		dir = filepath.Join(localAppData, "agent-notify")
	} else {
		dir = filepath.Join(home, ".local", "bin")
	}
	dest := filepath.Join(dir, filepath.Base(exe))
	if exe == dest {
		return exe
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return exe
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return exe
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return exe
	}
	// Atomic swap: safe even while the old binary at dest is running.
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return exe
	}
	fmt.Printf(T("install.binary")+"\n", dest)
	// drop the pre-rename binary so stale copies don't linger
	_ = os.Remove(filepath.Join(dir, "claude-notify"))
	_ = os.Remove(filepath.Join(dir, "claude-notify.exe"))
	return dest
}

func runInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)
	command := fmt.Sprintf("%q hook", exe)

	path := settingsPath()
	settings := loadSettings(path)

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	added, updated := 0, 0
	for _, event := range []string{"Stop", "Notification"} {
		entries := hookEntries(hooks, event)
		found := false
		for _, e := range entries {
			m, _ := e.(map[string]any)
			inner, _ := m["hooks"].([]any)
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				if !isOurCommand(cmd) {
					continue
				}
				found = true
				if cmd != command { // repoint an old binary path at this one
					hm["command"] = command
					updated++
				}
			}
		}
		if !found {
			entries = append(entries, map[string]any{
				"hooks": []any{map[string]any{"type": "command", "command": command}},
			})
			hooks[event] = entries
			added++
		}
	}

	if added == 0 && updated == 0 {
		fmt.Printf(T("install.already")+"\n", path)
	} else {
		saveSettings(path, settings)
		fmt.Printf(T("install.hooks")+"\n", added, updated, path)
		fmt.Printf(T("install.command")+"\n", command)
	}

	if err := installAutostart(exe); err != nil {
		fmt.Printf(T("install.autostartFail")+"\n", err)
	} else {
		fmt.Println(T("install.autostartOK"))
	}
}

func runUninstall() {
	if err := uninstallAutostart(); err == nil {
		fmt.Println(T("uninstall.autostart"))
	}

	path := settingsPath()
	settings := loadSettings(path)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println(T("uninstall.none"))
		return
	}

	removed := 0
	for _, event := range []string{"Stop", "Notification"} {
		entries := hookEntries(hooks, event)
		var kept []any
		for _, e := range entries {
			m, _ := e.(map[string]any)
			inner, _ := m["hooks"].([]any)
			var keptInner []any
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				if isOurCommand(cmd) {
					removed++
					continue
				}
				keptInner = append(keptInner, h)
			}
			if len(keptInner) > 0 {
				m["hooks"] = keptInner
				kept = append(kept, m)
			}
		}
		if len(kept) > 0 {
			hooks[event] = kept
		} else {
			delete(hooks, event)
		}
	}

	if removed == 0 {
		fmt.Println(T("uninstall.none"))
		return
	}
	saveSettings(path, settings)
	fmt.Printf(T("uninstall.removed")+"\n", removed, path)
}
