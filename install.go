package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// claudeHookEvents are the Claude Code events we register.
var claudeHookEvents = []string{"Stop", "Notification", "UserPromptSubmit", "PreToolUse", "PostToolUse", "SessionEnd"}

// claudeHookEntry is our settings.json hook for exe on event.
//
// Windows gets exec form (command + args, no shell, Claude Code ≥2.1.139):
// without Git for Windows, Claude Code runs shell-form hooks in
// PowerShell, where a quoted path followed by an argument is a parse
// error — and since 2.1.299 a failing PreToolUse hook blocks every tool
// call. The two events that can block go further, as http hooks
// (≥2.1.63): Cursor CLI imports Claude hooks and, launched from Git Bash,
// crashes its own hook wrapper with exit 2 — "block" for prompts and tool
// calls — but it skips entries without a command. They also spare a
// process spawn on every tool call. A failed http hook never blocks.
func claudeHookEntry(exe, event string) map[string]any {
	if runtime.GOOS == "windows" {
		if event == "UserPromptSubmit" || event == "PreToolUse" {
			return map[string]any{"type": "http", "url": claudeHookURL(), "timeout": 5}
		}
		return map[string]any{"type": "command", "command": exe, "args": []any{"hook"}}
	}
	return map[string]any{"type": "command", "command": fmt.Sprintf("%q hook", exe)}
}

func claudeHookURL() string { return fmt.Sprintf("http://127.0.0.1:%d/claude-hook", daemonPort) }

// isOurHook reports whether a settings.json hook (shell, exec or http
// form) is ours.
func isOurHook(hm map[string]any) bool {
	cmd, _ := hm["command"].(string)
	url, _ := hm["url"].(string)
	var args []string
	for _, a := range func() []any { v, _ := hm["args"].([]any); return v }() {
		s, _ := a.(string)
		args = append(args, s)
	}
	return isOurHookFields(cmd, args, url)
}

func isOurHookFields(cmd string, args []string, url string) bool {
	if strings.HasPrefix(url, "http://127.0.0.1:") && strings.HasSuffix(url, "/claude-hook") {
		return true
	}
	return isOurCommand(strings.TrimSpace(cmd + " " + strings.Join(args, " ")))
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
	// Always the canonical name: a browser-downloaded asset arrives as
	// "agent-notify-windows-amd64 (1).exe", and hooks registered under that
	// name would outlive every in-app update (which writes agent-notify.exe).
	dest := installDest()
	if dest == "" {
		return exe
	}
	dir := filepath.Dir(dest)
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
	// Atomic swap. Windows can't replace a running exe (the daemon, during
	// a re-install), only rename it away — otherwise the swap fails and
	// hooks get registered at the download path instead.
	if runtime.GOOS == "windows" {
		_ = os.Remove(dest + ".old")
		_ = os.Rename(dest, dest+".old")
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		_ = os.Rename(dest+".old", dest) // Windows: put the old one back
		return exe
	}
	// The source may carry the app bundle's Apple Development signature,
	// which AMFI refuses (SIGKILL) for the standalone copy — re-sign ad hoc.
	if runtime.GOOS == "darwin" {
		_ = exec.Command("codesign", "--force", "-s", "-",
			"--identifier", "com.codergangw.claude-notify", dest).Run()
	}
	fmt.Printf(T("install.binary")+"\n", dest)
	// drop stale copies (the pre-rename name, release-asset names) so
	// nothing keeps resolving to an old version; a running exe just fails
	// to delete on Windows, which is fine
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		p := filepath.Join(dir, n)
		if p == dest || p == exe || e.IsDir() {
			continue
		}
		if strings.HasPrefix(n, "claude-notify") || strings.HasPrefix(n, "agent-notify-") {
			_ = os.Remove(p)
		}
	}
	return dest
}

// registerHooks points our hooks at exe, returning how many entries were
// added or repointed (0,0 = already correct).
func registerHooks(exe string) (added, updated int) {
	path := settingsPath()
	settings := loadSettings(path)

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	for _, event := range claudeHookEvents {
		want := claudeHookEntry(exe, event)
		wantJSON, _ := json.Marshal(want)
		entries := hookEntries(hooks, event)
		found := false
		for _, e := range entries {
			m, _ := e.(map[string]any)
			inner, _ := m["hooks"].([]any)
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				if !isOurHook(hm) {
					continue
				}
				found = true
				// repoint an old binary path or hook form at this one
				if b, _ := json.Marshal(hm); string(b) != string(wantJSON) {
					for k := range hm {
						delete(hm, k)
					}
					for k, v := range want {
						hm[k] = v
					}
					updated++
				}
			}
		}
		if !found {
			entries = append(entries, map[string]any{
				"hooks": []any{want},
			})
			hooks[event] = entries
			added++
		}
	}
	if added > 0 || updated > 0 {
		saveSettings(path, settings)
	}
	return added, updated
}

func runInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)

	added, updated := registerHooks(exe)
	if added == 0 && updated == 0 {
		fmt.Printf(T("install.already")+"\n", settingsPath())
	} else {
		fmt.Printf(T("install.hooks")+"\n", added, updated, settingsPath())
		fmt.Printf(T("install.command")+"\n", fmt.Sprintf("%q hook", exe))
	}

	// explicit install = explicit opt back in
	if c := loadConfig(); c.DisableAutostart {
		c.DisableAutostart = false
		saveConfig(c)
	}
	if err := installAutostart(exe, true); err != nil {
		fmt.Printf(T("install.autostartFail")+"\n", err)
	} else {
		fmt.Println(T("install.autostartOK"))
	}
}

// firstRunSetup makes double-clicking the .app (or bare binary) enough:
// copy the binary to its stable path, register hooks, and write the login
// autostart — WITHOUT starting a second daemon; the caller is the daemon.
// Fast no-op when everything already points at this install.
func firstRunSetup() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	syncBundleIcon(exe)
	installed := installBinary(exe)
	// Claude hooks are NOT auto-registered anymore: agent wiring is the
	// user's explicit choice via the in-tab setup guide (/api/setup-fix).
	// EXCEPT when hooks are already ours from a previous version — then
	// keep them pointing at the freshly installed binary.
	if computeSetup().Hooks {
		added, updated := registerHooks(installed) // also migrates Windows entries to exec form
		if added > 0 || updated > 0 {
			fmt.Printf(T("install.hooks")+"\n", added, updated, settingsPath())
		}
	}
	if !loadConfig().DisableAutostart && !autostartHealthy() {
		_ = installAutostart(installed, false)
	}
}

func runUninstall() {
	if err := uninstallAutostart(); err == nil {
		fmt.Println(T("uninstall.autostart"))
	}
	if uninstallCodexHook() {
		fmt.Printf(T("codex.uninstalled")+"\n", codexConfigPath())
	}
	if uninstallCodexHooks() {
		fmt.Printf(T("codex.uninstalled")+"\n", codexHooksPath())
	}

	path := settingsPath()
	settings := loadSettings(path)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println(T("uninstall.none"))
		return
	}

	removed := 0
	for _, event := range claudeHookEvents {
		entries := hookEntries(hooks, event)
		var kept []any
		for _, e := range entries {
			m, _ := e.(map[string]any)
			inner, _ := m["hooks"].([]any)
			var keptInner []any
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				if isOurHook(hm) {
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
