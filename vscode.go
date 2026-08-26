package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// vscodeTitle recovers the session title the VSCode extension generated
// ("derived title from message 1"). The extension keeps it only in webview
// memory, but every rename_session / update_session_state message is
// echoed into the extension host log, so the freshest matching log line
// is the current title.
func vscodeTitle(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	files := claudeExtensionLogs()
	needle := []byte(`"sessionId":"` + sessionID + `"`)
	for _, path := range files { // newest log first
		if t := lastTitleInLog(path, needle); t != "" {
			return t
		}
	}
	return ""
}

func claudeExtensionLogs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var bases []string
	products := []string{"Code", "Code - Insiders", "VSCodium", "Cursor", "Windsurf"}
	switch runtime.GOOS {
	case "darwin":
		for _, p := range products {
			bases = append(bases, filepath.Join(home, "Library", "Application Support", p, "logs"))
		}
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			for _, p := range products {
				bases = append(bases, filepath.Join(appdata, p, "logs"))
			}
		}
	default:
		for _, p := range products {
			bases = append(bases, filepath.Join(home, ".config", p, "logs"))
		}
	}

	type logFile struct {
		path  string
		mtime time.Time
	}
	var found []logFile
	cutoff := time.Now().Add(-48 * time.Hour)
	for _, base := range bases {
		matches, _ := filepath.Glob(filepath.Join(base, "*", "window*", "exthost", "Anthropic.claude-code", "*.log"))
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil || st.ModTime().Before(cutoff) {
				continue
			}
			found = append(found, logFile{m, st.ModTime()})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mtime.After(found[j].mtime) })
	if len(found) > 5 {
		found = found[:5]
	}
	paths := make([]string, len(found))
	for i, f := range found {
		paths[i] = f.path
	}
	return paths
}

// lastTitleInLog returns the title from the last webview message line in
// the log that mentions the session and carries a title.
func lastTitleInLog(path string, needle []byte) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var title string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // webview lines can be long
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, needle) || !bytes.Contains(line, []byte(`"title":"`)) {
			continue
		}
		i := bytes.IndexByte(line, '{')
		if i < 0 {
			continue
		}
		var msg struct {
			Request struct {
				Type      string `json:"type"`
				SessionID string `json:"sessionId"`
				Title     string `json:"title"`
			} `json:"request"`
		}
		if json.Unmarshal(line[i:], &msg) != nil {
			continue
		}
		r := msg.Request
		if (r.Type == "rename_session" || r.Type == "update_session_state") &&
			strings.TrimSpace(r.Title) != "" {
			title = r.Title // keep scanning: last one wins
		}
	}
	return title
}
