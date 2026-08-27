package main

import "time"

// daemonPort is the fixed localhost port the tray daemon listens on.
// Hooks POST events here; if the daemon is down they fall back to a
// direct OS notification.
const daemonPort = 49517

// Event is what a hook sends to the daemon.
type Event struct {
	SessionID string    `json:"session_id"`
	CWD       string    `json:"cwd"`
	Kind      string    `json:"kind"`     // "done" | "attention"
	Source    string    `json:"source"`   // "claude" | "codex" ("" = claude, pre-0.2 events)
	Title     string    `json:"title"`    // session title, if found
	Activate  string    `json:"activate"` // macOS bundle id to focus on click
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}
