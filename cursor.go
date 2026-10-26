package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Cursor CLI integration: ~/.cursor/hooks.json runs commands with one
// JSON object on stdin. `agent-notify cursor-hook` adapts the events we
// subscribe to (beforeSubmitPrompt, stop, sessionEnd) into daemon
// events. Cursor has no permission-request event, so no "attention"
// notifications yet.

var cursorHookEvents = []string{"beforeSubmitPrompt", "stop", "sessionEnd"}

// cursorEvents is what we register on this OS. Windows skips
// beforeSubmitPrompt: launched from Git Bash, Cursor runs its PowerShell
// hook wrapper through bash, which crashes with exit 2 — Cursor's "block"
// code — so every prompt would be refused (forum.cursor.com/t/168129).
// stop/sessionEnd just fail there, harmlessly.
func cursorEvents() []string {
	if runtime.GOOS == "windows" {
		return cursorHookEvents[1:]
	}
	return cursorHookEvents
}

// cursorHookCommand: Cursor runs hooks through PowerShell on Windows.
func cursorHookCommand(exe string) string {
	if runtime.GOOS == "windows" {
		return psHookCommand(exe, "cursor-hook")
	}
	return fmt.Sprintf("%s cursor-hook", exe)
}

func cursorRepairHook() {
	if runtime.GOOS != "windows" {
		return
	}
	var doc struct {
		Hooks map[string][]any `json:"hooks"`
	}
	data, err := os.ReadFile(cursorHooksPath())
	if err != nil || json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &doc) != nil {
		return
	}
	for _, e := range doc.Hooks["beforeSubmitPrompt"] {
		if buf, _ := json.Marshal(e); containsOurCommand(string(buf)) {
			reinstallOnce(cursorHooksPath(), installCursorHook) // drops it
			return
		}
	}
	repairHookFile(cursorHooksPath(), cursorHookCommand(installDest()), installCursorHook)
}

// runCursorHook is invoked by Cursor CLI. Print nothing to stdout
// (Cursor parses it as a hook response) and always exit 0.
func runCursorHook() {
	defer os.Exit(0)
	data, err := readHookStdin()
	if err != nil {
		return
	}
	var in struct {
		ConversationID string   `json:"conversation_id"`
		HookEventName  string   `json:"hook_event_name"`
		Model          string   `json:"model"`
		WorkspaceRoots []string `json:"workspace_roots"`
		Prompt         string   `json:"prompt"`
		Status         string   `json:"status"` // stop: completed | aborted | error
	}
	if json.Unmarshal(data, &in) != nil {
		return
	}
	cwd := ""
	if len(in.WorkspaceRoots) > 0 {
		cwd = in.WorkspaceRoots[0]
		// Windows roots can arrive as "/d:/code/proj/" — make it D:\code\proj
		// so it matches other agents' cwds and shortPath's home prefix
		if runtime.GOOS == "windows" && cwd != "" {
			if len(cwd) > 2 && cwd[0] == '/' && cwd[2] == ':' {
				cwd = cwd[1:]
			}
			if len(cwd) > 1 && cwd[1] == ':' {
				cwd = strings.ToUpper(cwd[:1]) + cwd[1:]
			}
			cwd = filepath.Clean(cwd)
		}
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	activate := ""
	if runtime.GOOS == "darwin" {
		activate = os.Getenv("__CFBundleIdentifier")
	}
	mux := muxContext()
	live := !loadConfig().DisableLiveStatus

	session := func(kind string, u sessionUpdate) {
		u.SessionID, u.Source, u.CWD, u.Kind = in.ConversationID, "cursor", cwd, kind
		u.Activate, u.Mux, u.Model = activate, mux, in.Model
		postSession(u)
	}

	switch in.HookEventName {
	case "beforeSubmitPrompt":
		if live {
			session("prompt", sessionUpdate{Prompt: cleanPrompt(in.Prompt)})
		}
	case "sessionEnd":
		if live {
			session("end", sessionUpdate{})
		}
	case "stop":
		session("idle", sessionUpdate{})
		if in.Status == "aborted" { // user interrupted — nothing to announce
			return
		}
		deliver(Event{
			SessionID: in.ConversationID, CWD: cwd, Kind: "done", Source: "cursor",
			Title: condense(projectName(cwd), 60), Model: in.Model,
			Activate: activate, Mux: mux, Time: time.Now(),
		})
	}
}

func cursorHooksPath() string { return homePath(".cursor", "hooks.json") }

func cursorHooked() bool {
	data, _ := os.ReadFile(cursorHooksPath())
	return containsOurCommand(string(data))
}

// installCursorHook merges our entries into ~/.cursor/hooks.json,
// preserving everything else.
func installCursorHook() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)
	command := cursorHookCommand(exe)

	path := cursorHooksPath()
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if _, ok := doc["version"]; !ok {
		doc["version"] = 1
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	// drop our entries everywhere first: events we no longer register on
	// this OS (Windows beforeSubmitPrompt) must not keep a stale copy
	for ev, v := range hooks {
		entries, ok := v.([]any)
		if !ok {
			continue // not an event list; leave it alone
		}
		var kept []any
		for _, e := range entries {
			if buf, _ := json.Marshal(e); !containsOurCommand(string(buf)) {
				kept = append(kept, e)
			}
		}
		if len(kept) > 0 {
			hooks[ev] = kept
		} else {
			delete(hooks, ev)
		}
	}
	for _, ev := range cursorEvents() {
		entries, _ := hooks[ev].([]any)
		hooks[ev] = append(entries, map[string]any{"command": command})
	}
	doc["hooks"] = hooks

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

// cursorLoggedIn probes Cursor's credential store: macOS keeps tokens in
// the Keychain; Linux/Windows use an auth.json.
func cursorLoggedIn() bool {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("security", "find-generic-password",
			"-a", "cursor-user", "-s", "cursor-access-token").Run() == nil
	case "windows":
		return fileExists(filepath.Join(os.Getenv("APPDATA"), "Cursor", "auth.json"))
	default:
		if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
			return fileExists(filepath.Join(x, "cursor", "auth.json"))
		}
		return fileExists(homePath(".config", "cursor", "auth.json"))
	}
}
