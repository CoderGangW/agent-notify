package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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

// asyncPayload travels from the fast hook process to the detached
// summarizer child via a temp file.
type asyncPayload struct {
	Event   Event  `json:"event"`
	Request string `json:"request"`
	Report  string `json:"report"`
}

// runHook is invoked by Claude Code as `agent-notify hook`. It must be
// fast and must always exit 0 so it never blocks or fails the session.
// AI summarization is handed to a detached child process.
func runHook() {
	defer os.Exit(0)

	// Set when we invoke `claude -p` ourselves: that run's own Stop hook
	// must not notify (or recurse).
	if os.Getenv("CLAUDE_NOTIFY_SUPPRESS") == "1" {
		return
	}
	// Cursor CLI reads Claude Code's settings hooks too (third-party
	// interop) and would run us with a Cursor-shaped payload; cursor-hook
	// handles those sessions natively.
	if os.Getenv("CURSOR_VERSION") != "" {
		return
	}

	data, err := io.ReadAll(os.Stdin)
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
		if spawnSummarizer(asyncPayload{Event: ev, Request: info.LastUser, Report: info.LastAssistant}) {
			return
		}
		ev.Message = condense(info.LastAssistant, 180)
	}
	deliver(ev)
}

func spawnSummarizer(p asyncPayload) bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	buf, err := json.Marshal(p)
	if err != nil {
		return false
	}
	tmp, err := os.CreateTemp("", "claude-notify-*.json")
	if err != nil {
		return false
	}
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return false
	}
	tmp.Close()

	cmd := exec.Command(exe, "summarize-notify", tmp.Name())
	if err := cmd.Start(); err != nil {
		os.Remove(tmp.Name())
		return false
	}
	_ = cmd.Process.Release() // orphan it; the hook exits immediately
	return true
}

// runSummarizeNotify is the detached child: summarize with `claude -p`,
// fall back to a raw excerpt, then deliver.
func runSummarizeNotify() {
	if len(os.Args) < 3 {
		return
	}
	path := os.Args[2]
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var p asyncPayload
	if json.Unmarshal(data, &p) != nil {
		return
	}
	ev := p.Event
	if s := aiSummarize(p.Request, p.Report); s != "" {
		ev.Message = s
	} else {
		ev.Message = condense(p.Report, 180)
	}
	deliver(ev)
}

// aiSummarize asks the claude CLI (haiku, the user's existing auth) for a
// one-line notification body. Empty string on any failure.
func aiSummarize(request, report string) string {
	if os.Getenv("CLAUDE_NOTIFY_NO_AI") == "1" || loadConfig().DisableAISummary ||
		strings.TrimSpace(report) == "" {
		return ""
	}
	claude := findCLI("claude")
	if claude == "" {
		return ""
	}

	// The summary language follows the app's UI language setting.
	langName := map[string]string{
		"en": "English",
		"ko": "Korean",
		"zh": "Simplified Chinese",
	}[resolveLang(loadConfig().Lang)]

	prompt := "Below are the last request and final report of a coding-agent session. Output a one-sentence summary (max 80 chars) for a desktop notification body, focused on what was completed or changed. Output nothing but the summary sentence. Write the summary in " + langName + ".\n\n[request]\n" +
		request + "\n\n[report]\n" + report

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, claude, "-p", "--model", "haiku")
	hideConsole(cmd) // GUI-subsystem parent: a console child would flash a window
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = os.TempDir()
	cmd.Env = append(os.Environ(), "CLAUDE_NOTIFY_SUPPRESS=1")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return condense(string(out), 200)
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

func deliver(ev Event) {
	if postToDaemon(ev) {
		return
	}
	// Daemon not running: degrade to a direct OS notification, and try to
	// bring the daemon back for the next event.
	deliverNotification(ev)
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

// postSession best-effort delivers a live-status update; no daemon, no problem.
func postSession(u sessionUpdate) {
	buf, err := json.Marshal(u)
	if err != nil {
		return
	}
	client := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Post(
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
	client := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Post(
		fmt.Sprintf("http://127.0.0.1:%d/event", daemonPort),
		"application/json", bytes.NewReader(buf))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 300
}
