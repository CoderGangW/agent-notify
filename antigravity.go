package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Antigravity CLI (`agy`) integration — Google's successor to Gemini CLI
// for personal accounts. Hook spec verified against the embedded docs in
// the agy binary (2026-08 build):
//   - shared config: ~/.gemini/config/hooks.json (the antigravity-cli/
//     hooks.json path was an agy bug, fixed upstream)
//   - top-level keys are HOOK NAMES; events live inside each named hook
//   - events: PreToolUse / PostToolUse / PreInvocation / PostInvocation /
//     Stop — there is no Notification/attention event
//   - payload: camelCase JSON on stdin; stdout must be a JSON object
// We deliberately skip PreToolUse: its stdout contract requires a
// "decision" field, and both "allow" (auto-approves everything) and a
// parse-failure fallback are riskier than not observing tools at all.

// runAntigravityHook is invoked as `agent-notify antigravity-hook <event>`
// (the event name rides on argv — the payload doesn't carry it).
func runAntigravityHook() {
	// Every contract expects a JSON object on stdout; {} carries no
	// directives (no injectSteps, no decision, no force_continue).
	defer func() {
		fmt.Print("{}")
		os.Exit(0)
	}()
	if len(os.Args) < 3 {
		return
	}
	event := os.Args[2]
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var in struct {
		ConversationID    string   `json:"conversationId"`
		WorkspacePaths    []string `json:"workspacePaths"`
		ModelName         string   `json:"modelName"`
		InvocationNum     int      `json:"invocationNum"`     // PreInvocation
		TerminationReason string   `json:"terminationReason"` // Stop: model_stop | max_steps_exceeded | error
		Error             string   `json:"error"`
	}
	if json.Unmarshal(data, &in) != nil {
		return
	}
	cwd := ""
	if len(in.WorkspacePaths) > 0 {
		cwd = in.WorkspacePaths[0]
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
		u.SessionID, u.Source, u.CWD, u.Kind = in.ConversationID, "antigravity", cwd, kind
		u.Activate, u.Mux, u.Model = activate, mux, in.ModelName
		postSession(u)
	}

	switch event {
	case "PreInvocation":
		if !live {
			return
		}
		if in.InvocationNum <= 1 { // first model call of the turn
			session("prompt", sessionUpdate{})
		} else {
			session("posttool", sessionUpdate{}) // -> "working"
		}
	case "PostToolUse":
		if live {
			session("posttool", sessionUpdate{})
		}
	case "Stop":
		session("idle", sessionUpdate{})
		kind := "done"
		msg := ""
		if in.TerminationReason == "error" { // stopped mid-task — worth a look
			kind = "attention"
			msg = condense(in.Error, 180)
		}
		deliver(Event{
			SessionID: in.ConversationID, CWD: cwd, Kind: kind, Source: "antigravity",
			Title: condense(projectName(cwd), 60), Model: in.ModelName,
			Activate: activate, Mux: mux, Message: msg, Time: time.Now(),
		})
	}
}

// antigravityHooksPath is the shared hooks config agy actually loads.
func antigravityHooksPath() string { return homePath(".gemini", "config", "hooks.json") }

// antigravityLegacyPath got hooks written by agy's own /hooks bug (and by
// our first installer); agy never reads it.
func antigravityLegacyPath() string { return homePath(".gemini", "antigravity-cli", "hooks.json") }

func antigravityHooked() bool {
	data, _ := os.ReadFile(antigravityHooksPath())
	return containsOurCommand(string(data))
}

// antigravityHookSpec builds our named-hook entry. Tool events need the
// matcher+hooks wrapper; lifecycle events take flat handler lists.
func antigravityHookSpec(exe string) map[string]any {
	cmd := func(ev string) string { return fmt.Sprintf("%s antigravity-hook %s", exe, ev) }
	handler := func(ev string) map[string]any {
		return map[string]any{"type": "command", "command": cmd(ev), "timeout": 10}
	}
	return map[string]any{
		"PostToolUse":   []any{map[string]any{"matcher": "*", "hooks": []any{handler("PostToolUse")}}},
		"PreInvocation": []any{handler("PreInvocation")},
		"Stop":          []any{handler("Stop")},
	}
}

// installAntigravityHook writes our named hook into the shared
// hooks.json, preserving other named hooks, and scrubs the legacy file.
func installAntigravityHook() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)

	path := antigravityHooksPath()
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	doc["agent-notify"] = antigravityHookSpec(exe)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return err
	}
	cleanupAntigravityLegacy()
	return nil
}

// cleanupAntigravityLegacy removes our entries from the never-read legacy
// file; if nothing else is in it, the file goes too.
func cleanupAntigravityLegacy() {
	path := antigravityLegacyPath()
	data, err := os.ReadFile(path)
	if err != nil || !containsOurCommand(string(data)) {
		return
	}
	doc := map[string]any{}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	for key, v := range doc {
		buf, _ := json.Marshal(v)
		if containsOurCommand(string(buf)) {
			delete(doc, key)
		}
	}
	if len(doc) == 0 {
		_ = os.Remove(path)
		return
	}
	if out, err := json.MarshalIndent(doc, "", "  "); err == nil {
		_ = os.WriteFile(path, append(out, '\n'), 0o644)
	}
}
