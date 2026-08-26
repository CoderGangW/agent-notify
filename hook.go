package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gen2brain/beeep"
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

// runHook is invoked by Claude Code as `claude-notify hook`. It must be
// fast and must always exit 0 so it never blocks or fails the session.
func runHook() {
	defer os.Exit(0)

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

	title, summary := transcriptInfo(in.TranscriptPath)
	// The VSCode extension's generated session title, when we can find it.
	if n := vscodeTitle(in.SessionID); n != "" {
		title = n
	}
	// An explicitly user-set session name beats anything derived.
	if n := sessionName(in.SessionID); n != "" {
		title = n
	}
	message := in.Message // Notification events carry their own message
	if message == "" {
		message = summary // Stop events: Claude's last reply = work summary
	}

	ev := Event{
		SessionID: in.SessionID,
		CWD:       in.CWD,
		Kind:      kind,
		Title:     title,
		Message:   message,
		Time:      time.Now(),
	}

	if postToDaemon(ev) {
		return
	}
	// Daemon not running: degrade to a direct OS notification.
	beeep.AppName = "claude-notify"
	title, body := notificationText(ev)
	_ = beeep.Notify(title, body, "")
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
