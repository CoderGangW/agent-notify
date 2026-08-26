package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func settingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot find home directory:", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func loadSettings(path string) map[string]any {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintf(os.Stderr, "%s is not valid JSON: %v\n", path, err)
			os.Exit(1)
		}
		backup := path + ".bak-claude-notify"
		if err := os.WriteFile(backup, data, 0o644); err == nil {
			fmt.Println("백업 저장:", backup)
		}
	}
	return settings
}

func saveSettings(path string, settings map[string]any) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// isOurCommand reports whether a hook command string belongs to claude-notify.
func isOurCommand(cmd string) bool {
	return strings.Contains(cmd, "claude-notify") && strings.HasSuffix(strings.TrimSpace(cmd), " hook")
}

func hookEntries(hooks map[string]any, event string) []any {
	arr, _ := hooks[event].([]any)
	return arr
}

func hasOurHook(entries []any) bool {
	for _, e := range entries {
		m, _ := e.(map[string]any)
		inner, _ := m["hooks"].([]any)
		for _, h := range inner {
			hm, _ := h.(map[string]any)
			cmd, _ := hm["command"].(string)
			if isOurCommand(cmd) {
				return true
			}
		}
	}
	return false
}

func runInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	command := fmt.Sprintf("%q hook", exe)

	path := settingsPath()
	settings := loadSettings(path)

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	added := 0
	for _, event := range []string{"Stop", "Notification"} {
		entries := hookEntries(hooks, event)
		if hasOurHook(entries) {
			continue
		}
		entries = append(entries, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": command}},
		})
		hooks[event] = entries
		added++
	}

	if added == 0 {
		fmt.Println("hook 이미 설치되어 있음:", path)
	} else {
		saveSettings(path, settings)
		fmt.Printf("hook %d개 등록 완료: %s\n", added, path)
		fmt.Println("등록된 명령:", command)
	}

	if err := installAutostart(exe); err != nil {
		fmt.Println("자동 시작 등록 실패 (수동 실행 필요):", err)
	} else {
		fmt.Println("로그인 시 자동 시작 등록 완료 (데몬 지금 시작됨)")
	}
}

func runUninstall() {
	if err := uninstallAutostart(); err == nil {
		fmt.Println("자동 시작 등록 제거 완료")
	}

	path := settingsPath()
	settings := loadSettings(path)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println("등록된 hook 없음")
		return
	}

	removed := 0
	for _, event := range []string{"Stop", "Notification"} {
		entries := hookEntries(hooks, event)
		var kept []any
		for _, e := range entries {
			m, _ := e.(map[string]any)
			inner, _ := m["hooks"].([]any)
			var keptInner []any
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				if isOurCommand(cmd) {
					removed++
					continue
				}
				keptInner = append(keptInner, h)
			}
			if len(keptInner) > 0 {
				m["hooks"] = keptInner
				kept = append(kept, m)
			}
		}
		if len(kept) > 0 {
			hooks[event] = kept
		} else {
			delete(hooks, event)
		}
	}

	if removed == 0 {
		fmt.Println("등록된 hook 없음")
		return
	}
	saveSettings(path, settings)
	fmt.Printf("hook %d개 제거 완료: %s\n", removed, path)
}
