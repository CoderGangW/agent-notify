package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// hookInput is the JSON Claude Code writes to a hook's stdin.
type hookInput struct {
	SessionID      string          `json:"session_id"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	Message        string          `json:"message"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	Prompt         string          `json:"prompt"`
	TranscriptPath string          `json:"transcript_path"`
	StopHookActive bool            `json:"stop_hook_active"`
	// Stop: the turn's final reply. The transcript is written async and
	// may not hold it yet when the hook fires (per the hooks docs).
	LastAssistantMessage string `json:"last_assistant_message"`
}

// toolDetail condenses a tool_input into a one-line "what is it doing"
// hint for the live session list. Field priority mirrors how specific
// each field is about the actual work.
func toolDetail(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var in struct {
		Description string `json:"description"` // Bash, Task: human-written summary
		Command     string `json:"command"`
		FilePath    string `json:"file_path"`
		Pattern     string `json:"pattern"`
		URL         string `json:"url"`
		Query       string `json:"query"`
		Path        string `json:"path"`
		Skill       string `json:"skill"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return ""
	}
	v := ""
	switch {
	case in.Description != "":
		v = in.Description
	case in.Command != "":
		v = in.Command
	case in.FilePath != "":
		v = filepath.Base(in.FilePath)
	case in.Pattern != "":
		v = in.Pattern
	case in.URL != "":
		v = in.URL
	case in.Query != "":
		v = in.Query
	case in.Skill != "":
		v = in.Skill
	case in.Path != "":
		v = filepath.Base(in.Path)
	}
	return condense(v, 90)
}

// runHook is invoked by Claude Code as `agent-notify hook`. It must be
// fast and must always exit 0 so it never blocks or fails the session.
func runHook() {
	defer os.Exit(0)

	// Cursor CLI reads Claude Code's settings hooks too (third-party
	// interop) and would run us with a Cursor-shaped payload; cursor-hook
	// handles those sessions natively.
	if os.Getenv("CURSOR_VERSION") != "" {
		return
	}

	data, err := readHookStdin()
	if err != nil {
		return
	}
	var in hookInput
	if json.Unmarshal(data, &in) != nil {
		return
	}
	// A Stop fired because a previous stop hook forced continuation;
	// notifying again would duplicate.
	if in.StopHookActive {
		return
	}

	hostBundle := ""
	if runtime.GOOS == "darwin" {
		hostBundle = os.Getenv("__CFBundleIdentifier")
	}
	mux := muxContext()

	// Live-status events. Tool events fire constantly and stay minimal;
	// the whole channel can be switched off in settings.
	if loadConfig().DisableLiveStatus {
		switch in.HookEventName {
		case "PreToolUse", "PostToolUse", "SessionEnd", "UserPromptSubmit":
			return
		}
	}
	// UserPromptSubmit is once per user message, so it can afford the same
	// title resolution the notifications use.
	switch in.HookEventName {
	case "PreToolUse", "PostToolUse", "SessionEnd":
		kind := map[string]string{
			"PreToolUse":  "pretool",
			"PostToolUse": "posttool",
			"SessionEnd":  "end",
		}[in.HookEventName]
		postSession(sessionUpdate{
			SessionID: in.SessionID, CWD: in.CWD, Kind: kind,
			Tool: in.ToolName, Detail: toolDetail(in.ToolInput),
			Activate: hostBundle,
		})
		return
	case "UserPromptSubmit":
		pInfo := transcriptInfo(in.TranscriptPath)
		pTitle := pInfo.Title
		if vs, _ := vscodeTitle(in.SessionID); vs != "" {
			pTitle = vs
		}
		if n := sessionName(in.SessionID); n != "" {
			pTitle = n
		}
		postSession(sessionUpdate{
			SessionID: in.SessionID, CWD: in.CWD, Kind: "prompt",
			Prompt: cleanPrompt(in.Prompt), Activate: hostBundle,
			Title: pTitle, Branch: pInfo.Branch, Model: pInfo.Model,
			Mux: mux,
		})
		return
	}

	kind := "done"
	if in.HookEventName == "Notification" {
		kind = "attention"
	}

	info := transcriptInfo(in.TranscriptPath)
	title := info.Title
	vsTitle, activate := vscodeTitle(in.SessionID)
	if vsTitle != "" {
		title = vsTitle
	}
	// No IDE match: the hook inherits the hosting app's bundle id from the
	// terminal Claude Code runs in, which is exactly what a click should focus.
	if activate == "" && runtime.GOOS == "darwin" {
		activate = os.Getenv("__CFBundleIdentifier")
	}
	// An explicitly user-set session name beats anything derived.
	if n := sessionName(in.SessionID); n != "" {
		title = n
	}

	ev := Event{
		SessionID: in.SessionID,
		CWD:       in.CWD,
		Kind:      kind,
		Source:    "claude",
		Branch:    info.Branch,
		Model:     info.Model,
		DurSec:    info.DurationSec,
		Title:     title,
		Activate:  activate,
		Mux:       mux,
		Message:   in.Message, // Notification events carry their own message
		Time:      time.Now(),
	}

	// Keep the live session view in sync: Stop = idle, Notification = waiting.
	sKind := "idle"
	if kind == "attention" {
		sKind = "waiting"
	}
	postSession(sessionUpdate{
		SessionID: in.SessionID, CWD: in.CWD, Kind: sKind,
		Activate: activate, Title: title, Branch: info.Branch, Model: info.Model,
		Mux: mux,
	})

	if kind == "done" && ev.Message == "" {
		last := in.LastAssistantMessage
		if last == "" { // older Claude Code without the field
			last = info.LastAssistant
		}
		ev.Message = condense(last, 180)
	}
	deliver(ev)
}

// cleanPrompt drops injected context blocks (<ide_opened_file>…,
// <system-reminder>…) so the live view shows what the user actually typed.
var injectedBlock = regexp.MustCompile(`(?s)<([a-zA-Z][\w-]*)>.*?</[a-zA-Z][\w-]*>`)

func cleanPrompt(s string) string {
	s = injectedBlock.ReplaceAllString(s, " ")
	// an unclosed opening tag at the end (truncated block) — drop its tail
	if i := strings.LastIndex(s, "<"); i >= 0 && !strings.Contains(s[i:], ">") {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

var utf8BOM = []byte("\xef\xbb\xbf")

// readHookStdin reads a hook's JSON payload. Windows PowerShell 5.1 hook
// wrappers (Cursor, Gemini) prefix a UTF-8 BOM that json.Unmarshal rejects.
func readHookStdin() ([]byte, error) {
	data, err := io.ReadAll(os.Stdin)
	return bytes.TrimPrefix(data, utf8BOM), err
}

// psHookCommand is the hook command for agents that run hooks through
// PowerShell on Windows (Gemini, Cursor): an unquoted path breaks on a
// profile like C:\Users\John Smith, a quoted one needs the call operator,
// and piping keeps PowerShell waiting on our GUI-subsystem exe and handing
// it the event JSON on stdin.
func psHookCommand(exe, sub string) string {
	return "& '" + strings.ReplaceAll(exe, "'", "''") + "' " + sub + " | Out-Null"
}

// repairHookFile reinstalls when path holds an entry of ours whose command
// isn't want — the pre-fix Windows form was unquoted and broke on paths
// with spaces. The user opted in earlier; without this the broken entry
// would stay until a manual re-setup. Windows only: elsewhere the command
// format never changed, so there's nothing to repoint.
func repairHookFile(path, want string, install func() error) {
	if runtime.GOOS != "windows" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || !containsOurCommand(string(data)) {
		return
	}
	var doc any
	if json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &doc) == nil && !jsonHasString(doc, want) {
		reinstallOnce(path, install)
	}
}

var reinstalled sync.Map // config path → true

// reinstallOnce runs install at most once per daemon run per config file:
// repairs are polled every agent-status refresh, and an install that keeps
// failing (or writing a path that never matches) must not loop.
func reinstallOnce(path string, install func() error) {
	if _, done := reinstalled.LoadOrStore(path, true); !done {
		_ = install()
	}
}

// jsonHasString reports whether s appears as a string value anywhere in v.
func jsonHasString(v any, s string) bool {
	switch t := v.(type) {
	case string:
		return t == s
	case []any:
		for _, e := range t {
			if jsonHasString(e, s) {
				return true
			}
		}
	case map[string]any:
		for _, e := range t {
			if jsonHasString(e, s) {
				return true
			}
		}
	}
	return false
}

func deliver(ev Event) {
	if postToDaemon(ev) {
		return
	}
	// Daemon not running: no notification (only the daemon notifies) —
	// just try to bring it back for the next event.
	reviveDaemon()
}

// reviveDaemon restarts a dead daemon, at most once per 5 minutes (the
// stamp throttle keeps a burst of hooks from spawn-storming).
func reviveDaemon() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	stamp := filepath.Join(home, ".claude-notify", "revive.stamp")
	if st, err := os.Stat(stamp); err == nil && time.Since(st.ModTime()) < 5*time.Minute {
		return
	}
	_ = os.MkdirAll(filepath.Dir(stamp), 0o755)
	_ = os.WriteFile(stamp, []byte(time.Now().Format(time.RFC3339)), 0o644)

	if runtime.GOOS == "darwin" {
		// Reload the job instead of kickstart: a daemon that exited 0
		// (tray quit, in-app update) satisfies the KeepAlive
		// SuccessfulExit=false semaphore, and a satisfied job ignores
		// kickstart on newer macOS. bootout+bootstrap resets it and
		// RunAtLoad brings the daemon up managed, with its log intact.
		plist := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
		if _, err := os.Stat(plist); err == nil {
			gui := fmt.Sprintf("gui/%d", os.Getuid())
			_ = exec.Command("launchctl", "bootout", gui+"/"+launchdLabel).Run()
			if exec.Command("launchctl", "bootstrap", gui, plist).Run() == nil {
				return
			}
		}
		// no plist (autostart off): the app bundle, when installed, still
		// runs the daemon on a plain no-arg open
		if _, err := os.Stat(appBundleBin); err == nil &&
			exec.Command("open", "-b", launchdLabel).Run() == nil {
			return
		}
	}
	// no launchd job (or not macOS): spawn the installed daemon detached
	bin := installDest()
	if bin == "" {
		return
	}
	cmd := exec.Command(bin, "daemon")
	if err := cmd.Start(); err == nil {
		_ = cmd.Process.Release()
	}
}

// hookClient talks to the daemon. Loopback connects are instant when it's
// up, but Windows retries a refused connect for ~2s instead of failing
// fast — the short dial timeout keeps a dead daemon from adding that to
// every synchronous tool hook.
var hookClient = &http.Client{
	Timeout:   700 * time.Millisecond,
	Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 150 * time.Millisecond}).DialContext},
}

// postSession best-effort delivers a live-status update; no daemon, no problem.
func postSession(u sessionUpdate) {
	buf, err := json.Marshal(u)
	if err != nil {
		return
	}
	resp, err := hookClient.Post(
		fmt.Sprintf("http://127.0.0.1:%d/session", daemonPort),
		"application/json", bytes.NewReader(buf))
	if err != nil {
		return
	}
	resp.Body.Close()
}

func postToDaemon(ev Event) bool {
	buf, err := json.Marshal(ev)
	if err != nil {
		return false
	}
	resp, err := hookClient.Post(
		fmt.Sprintf("http://127.0.0.1:%d/event", daemonPort),
		"application/json", bytes.NewReader(buf))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 300
}
