package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Gemini CLI integration: ~/.gemini/settings.json `hooks` runs commands
// with a Claude-Code-compatible JSON payload on stdin — only the event
// names differ. `agent-notify gemini-hook` adapts them to daemon events
// and live-session updates.

// geminiHookEvents lists the lifecycle events we subscribe to.
var geminiHookEvents = []string{
	"BeforeAgent", "AfterAgent", "Notification",
	"BeforeTool", "AfterTool", "SessionEnd",
}

// runGeminiHook is invoked by Gemini CLI for every subscribed event. It
// must be fast, print nothing to stdout (Gemini parses stdout as JSON),
// and always exit 0.
func runGeminiHook() {
	defer os.Exit(0)
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var in struct {
		SessionID      string          `json:"session_id"`
		CWD            string          `json:"cwd"`
		HookEventName  string          `json:"hook_event_name"`
		Prompt         string          `json:"prompt"`
		PromptResponse string          `json:"prompt_response"`
		Message        string          `json:"message"`
		ToolName       string          `json:"tool_name"`
		ToolInput      json.RawMessage `json:"tool_input"`
		StopHookActive bool            `json:"stop_hook_active"`
	}
	if json.Unmarshal(data, &in) != nil {
		return
	}
	if in.StopHookActive {
		return
	}
	if in.CWD == "" {
		in.CWD, _ = os.Getwd()
	}

	activate := ""
	if runtime.GOOS == "darwin" {
		activate = os.Getenv("__CFBundleIdentifier")
	}
	mux := muxContext()
	live := !loadConfig().DisableLiveStatus

	session := func(kind string, u sessionUpdate) {
		u.SessionID, u.Source, u.CWD, u.Kind = in.SessionID, "gemini", in.CWD, kind
		u.Activate, u.Mux = activate, mux
		postSession(u)
	}

	switch in.HookEventName {
	case "BeforeAgent":
		if live {
			session("prompt", sessionUpdate{Prompt: cleanPrompt(in.Prompt)})
		}
	case "BeforeTool":
		if live {
			session("pretool", sessionUpdate{Tool: in.ToolName, Detail: toolDetail(in.ToolInput)})
		}
	case "AfterTool":
		if live {
			session("posttool", sessionUpdate{})
		}
	case "SessionEnd":
		if live {
			session("end", sessionUpdate{})
		}
	case "AfterAgent":
		session("idle", sessionUpdate{})
		title := condense(in.Prompt, 60)
		if title == "" {
			title = "Gemini"
		}
		deliver(Event{
			SessionID: in.SessionID, CWD: in.CWD, Kind: "done", Source: "gemini",
			Title: title, Activate: activate, Mux: mux,
			Message: condense(in.PromptResponse, 180), Time: time.Now(),
		})
	case "Notification":
		session("waiting", sessionUpdate{})
		deliver(Event{
			SessionID: in.SessionID, CWD: in.CWD, Kind: "attention", Source: "gemini",
			Title: condense(projectName(in.CWD), 60), Activate: activate, Mux: mux,
			Message: condense(in.Message, 180), Time: time.Now(),
		})
	}
}

// geminiSettingsPath is ~/.gemini/settings.json (user scope).
func geminiSettingsPath() string { return homePath(".gemini", "settings.json") }

// geminiHooked reports whether our hook command is present in the user
// settings.
func geminiHooked() bool {
	data, _ := os.ReadFile(geminiSettingsPath())
	return containsOurCommand(string(data))
}

func containsOurCommand(s string) bool {
	return strings.Contains(s, "agent-notify") || strings.Contains(s, "claude-notify")
}

// installGeminiHook merges our hook entries into ~/.gemini/settings.json,
// preserving everything else in the file.
func installGeminiHook() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)
	command := fmt.Sprintf("%s gemini-hook", exe)

	path := geminiSettingsPath()
	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	entry := map[string]any{
		"hooks": []any{map[string]any{
			"type": "command", "command": command, "name": "agent-notify",
		}},
	}
	for _, ev := range geminiHookEvents {
		groups, _ := hooks[ev].([]any)
		// repoint an older entry of ours; otherwise append
		replaced := false
		for gi, g := range groups {
			buf, _ := json.Marshal(g)
			if containsOurCommand(string(buf)) {
				groups[gi] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			groups = append(groups, entry)
		}
		hooks[ev] = groups
	}
	settings["hooks"] = hooks

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
