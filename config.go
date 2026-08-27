package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// config holds daemon preferences persisted across restarts.
type config struct {
	Muted      bool   `json:"muted"`
	Lang       string `json:"lang,omitempty"`       // "", "auto", "en", "ko", "zh"
	DefaultTab string `json:"defaultTab,omitempty"` // "claude" (default) | "codex"
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
