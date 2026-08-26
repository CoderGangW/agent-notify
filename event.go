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
	Kind      string    `json:"kind"`  // "done" | "attention"
	Title     string    `json:"title"` // session title from the transcript, if found
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}
