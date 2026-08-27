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
	"runtime"
	"strings"
	"time"
)

// hookInput is the JSON Claude Code writes to a hook's stdin.
type hookInput struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	Message        string `json:"message"`
	TranscriptPath string `json:"transcript_path"`
	StopHookActive bool   `json:"stop_hook_active"`
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
		Title:     title,
		Activate:  activate,
		Message:   in.Message, // Notification events carry their own message
		Time:      time.Now(),
	}

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
	if os.Getenv("CLAUDE_NOTIFY_NO_AI") == "1" || strings.TrimSpace(report) == "" {
		return ""
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return ""
	}

	prompt := "Below are the last request and final report of a coding-agent session. Output a one-sentence summary (max 80 chars) for a desktop notification body, focused on what was completed or changed. Output nothing but the summary sentence. Write it in the same language as the report.\n\n[request]\n" +
		request + "\n\n[report]\n" + report

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, claude, "-p", "--model", "haiku")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = os.TempDir()
	cmd.Env = append(os.Environ(), "CLAUDE_NOTIFY_SUPPRESS=1")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return condense(string(out), 200)
}

func deliver(ev Event) {
	if postToDaemon(ev) {
		return
	}
	// Daemon not running: degrade to a direct OS notification.
	deliverNotification(ev)
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
