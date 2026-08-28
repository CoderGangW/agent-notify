package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Cursor CLI integration: ~/.cursor/hooks.json runs commands with one
// JSON object on stdin. `agent-notify cursor-hook` adapts the events we
// subscribe to (beforeSubmitPrompt, stop, sessionEnd) into daemon
// events. Cursor has no permission-request event, so no "attention"
// notifications yet.

var cursorHookEvents = []string{"beforeSubmitPrompt", "stop", "sessionEnd"}

// runCursorHook is invoked by Cursor CLI. Print nothing to stdout
// (Cursor parses it as a hook response) and always exit 0.
func runCursorHook() {
	defer os.Exit(0)
	data, err := io.ReadAll(os.Stdin)
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
	command := fmt.Sprintf("%s cursor-hook", exe)

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
	for _, ev := range cursorHookEvents {
		entries, _ := hooks[ev].([]any)
		replaced := false
		for i, e := range entries {
			buf, _ := json.Marshal(e)
			if containsOurCommand(string(buf)) {
				entries[i] = map[string]any{"command": command}
				replaced = true
				break
			}
		}
		if !replaced {
			entries = append(entries, map[string]any{"command": command})
		}
		hooks[ev] = entries
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
