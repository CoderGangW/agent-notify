package main

import (
	"sort"
	"time"
)

// Live session tracking. Lightweight hook events (UserPromptSubmit,
// PreToolUse, PostToolUse, SessionEnd) POST sessionUpdates; Stop and
// Notification enrich the same session with title/branch/model. The
// daemon keeps a small state machine per session id.

// sessionUpdate is what a hook sends to /session.
type sessionUpdate struct {
	SessionID string `json:"session_id"`
	Source    string `json:"source"` // "claude" (default) | "codex"
	CWD       string `json:"cwd"`
	Kind      string `json:"kind"` // prompt | pretool | posttool | idle | waiting | end
	Tool      string `json:"tool"`
	Prompt    string `json:"prompt"`
	Activate  string `json:"activate"`
	Title     string `json:"title"`
	Branch    string `json:"branch"`
	Model     string `json:"model"`
	Mux       muxRef `json:"mux"`
}

// sessionInfo is the daemon-side state exposed to the window.
type sessionInfo struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	CWD       string    `json:"cwd"`
	Title     string    `json:"title"` // session title (same chain as events)
	Task      string    `json:"task"`  // current prompt excerpt
	State     string    `json:"state"` // working | tool | waiting | idle
	Tool      string    `json:"tool"`  // current tool while state == tool
	Branch    string    `json:"branch"`
	Model     string    `json:"model"`
	Activate  string    `json:"activate"`
	Mux       muxRef    `json:"mux"`
	TurnStart time.Time `json:"turnStart"` // start of the current turn
	LastSeen  time.Time `json:"lastSeen"`
}

func (s *daemonState) applySessionUpdate(u sessionUpdate) {
	if u.SessionID == "" {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = map[string]*sessionInfo{}
	}
	if u.Kind == "end" {
		delete(s.sessions, u.SessionID)
		return
	}
	info := s.sessions[u.SessionID]
	if info == nil {
		src := u.Source
		if src == "" {
			src = "claude"
		}
		info = &sessionInfo{ID: u.SessionID, Source: src, TurnStart: now}
		s.sessions[u.SessionID] = info
	}
	info.LastSeen = now
	if u.CWD != "" {
		info.CWD = u.CWD
	}
	if u.Activate != "" {
		info.Activate = u.Activate
	}
	if u.Title != "" {
		info.Title = u.Title
	}
	if u.Branch != "" {
		info.Branch = u.Branch
	}
	if u.Model != "" {
		info.Model = u.Model
	}
	if u.Mux.Pane != "" {
		info.Mux = u.Mux
	}
	switch u.Kind {
	case "prompt":
		info.State = "working"
		info.Tool = ""
		info.TurnStart = now
		info.Task = condense(u.Prompt, 500)
	case "pretool":
		info.State = "tool"
		info.Tool = u.Tool
	case "posttool":
		info.State = "working"
		info.Tool = ""
	case "waiting":
		info.State = "waiting"
		info.Tool = ""
	case "idle":
		info.State = "idle"
		info.Tool = ""
	}
}

// sessionListLocked returns active sessions, busiest first; caller holds
// mu. Sessions silent for 2h are evicted (crashed or force-quit CLIs
// never send SessionEnd).
func (s *daemonState) sessionListLocked() []sessionInfo {
	cutoff := time.Now().Add(-2 * time.Hour)
	var out []sessionInfo
	for id, info := range s.sessions {
		if info.LastSeen.Before(cutoff) {
			delete(s.sessions, id)
			continue
		}
		out = append(out, *info)
	}
	rank := map[string]int{"tool": 0, "working": 1, "waiting": 2, "idle": 3}
	sort.Slice(out, func(i, j int) bool {
		if rank[out[i].State] != rank[out[j].State] {
			return rank[out[i].State] < rank[out[j].State]
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}
