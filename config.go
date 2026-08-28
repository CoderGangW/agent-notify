package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// config holds daemon preferences persisted across restarts.
type config struct {
	Muted             bool     `json:"muted"`
	Lang              string   `json:"lang,omitempty"`       // "", "auto", "en", "ko", "zh"
	DefaultTab        string   `json:"defaultTab,omitempty"` // "claude" (default) | "codex"
	DisableAutoUpdate bool     `json:"disableAutoUpdate,omitempty"`
	DisableAISummary  bool     `json:"disableAISummary,omitempty"`  // skip the haiku one-liner
	DisableLiveStatus bool     `json:"disableLiveStatus,omitempty"` // skip live session updates
	Theme             string   `json:"theme,omitempty"`             // "auto" (default) | "light" | "dark"
	DisableAutostart  bool     `json:"disableAutostart,omitempty"`  // don't (re)register login autostart
	Agents            []string `json:"agents"`                      // enabled agent tabs; nil = legacy default pair, [] = none chosen
	NotifyMode        string   `json:"notifyMode,omitempty"`        // "on" | "alerts" | "quiet" | "silent"; "" = on
}

// notifyMode resolves the 4-level notification mode:
//
//	on:     OS notifications + unread badges
//	alerts: OS notifications, no badges (알림만)
//	quiet:  no OS notifications, badges kept (조용히)
//	silent: nothing (무음)
//
// Legacy Muted=true maps to quiet — the old mute suppressed OS
// notifications but kept badge counts.
func (c config) notifyMode() string {
	switch c.NotifyMode {
	case "on", "alerts", "quiet", "silent":
		return c.NotifyMode
	}
	if c.Muted {
		return "quiet"
	}
	return "on"
}

func modeNotifies(m string) bool { return m == "on" || m == "alerts" }
func modeBadges(m string) bool   { return m == "on" || m == "quiet" }

// agentEnabled reports whether the given tab is in the user's set.
func agentEnabled(c config, id string) bool {
	for _, a := range c.enabledAgents() {
		if a == id {
			return true
		}
	}
	return false
}

// enabledAgents resolves the configured tab set. Nil config means the
// historical default pair; an empty result never happens — at least one
// tab must exist for the window to make sense.
// enabledAgents resolves the tab set. nil (key absent — pre-selection
// configs) keeps the historical default pair; an explicit empty list
// means the user deselected everything, which the window renders as the
// "pick your agents" empty state.
func (c config) enabledAgents() []string {
	if c.Agents == nil {
		return []string{"claude", "codex"}
	}
	out := []string{}
	for _, id := range c.Agents {
		if validAgent(id) {
			out = append(out, id)
		}
	}
	return out
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude-notify", "config.json")
}

func loadConfig() config {
	var c config
	path := configPath()
	if path == "" {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

func saveConfig(c config) {
	path := configPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
