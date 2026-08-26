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
// ideBundleID maps a VSCode-family product name to its macOS bundle id,
// used to focus the right IDE when the notification is clicked.
var ideBundleID = map[string]string{
	"Code":            "com.microsoft.VSCode",
	"Code - Insiders": "com.microsoft.VSCodeInsiders",
	"VSCodium":        "com.vscodium",
	"Cursor":          "com.todesktop.230313mzl4w4u92",
	"Windsurf":        "com.exafunction.windsurf",
}

func vscodeTitle(sessionID string) (title, bundleID string) {
	if sessionID == "" {
		return "", ""
	}
	files := claudeExtensionLogs()
	needle := []byte(`"sessionId":"` + sessionID + `"`)
	for _, lf := range files { // newest log first
		if t := lastTitleInLog(lf.path, needle); t != "" {
			return t, ideBundleID[lf.product]
		}
	}
	return "", ""
}

type extensionLog struct {
	path    string
	product string
	mtime   time.Time
}

func claudeExtensionLogs() []extensionLog {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	type base struct{ dir, product string }
	var bases []base
	products := []string{"Code", "Code - Insiders", "VSCodium", "Cursor", "Windsurf"}
	switch runtime.GOOS {
	case "darwin":
		for _, p := range products {
			bases = append(bases, base{filepath.Join(home, "Library", "Application Support", p, "logs"), p})
		}
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			for _, p := range products {
				bases = append(bases, base{filepath.Join(appdata, p, "logs"), p})
			}
		}
	default:
		for _, p := range products {
			bases = append(bases, base{filepath.Join(home, ".config", p, "logs"), p})
		}
	}

	var found []extensionLog
	cutoff := time.Now().Add(-48 * time.Hour)
	for _, b := range bases {
		matches, _ := filepath.Glob(filepath.Join(b.dir, "*", "window*", "exthost", "Anthropic.claude-code", "*.log"))
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil || st.ModTime().Before(cutoff) {
				continue
			}
			found = append(found, extensionLog{m, b.product, st.ModTime()})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mtime.After(found[j].mtime) })
	if len(found) > 5 {
		found = found[:5]
	}
	return found
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
