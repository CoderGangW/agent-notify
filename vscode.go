package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/url"
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

var ideProducts = []string{"Code", "Code - Insiders", "VSCodium", "Cursor", "Windsurf"}

// ideUserDataDir is the VSCode-family user-data root for a product
// (logs, User/globalStorage, User/workspaceStorage live under it).
func ideUserDataDir(home, product string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", product)
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, product)
		}
		return ""
	default:
		return filepath.Join(home, ".config", product)
	}
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
	for _, p := range ideProducts {
		if d := ideUserDataDir(home, p); d != "" {
			bases = append(bases, base{filepath.Join(d, "logs"), p})
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

// vscodeOpenTarget maps a session's cwd onto the path VSCode needs to
// focus the window that already shows it. `open -b <ide> <path>` only
// reuses a window when <path> is exactly that window's root — a folder
// that's a member of a multi-root .code-workspace, or a subfolder of an
// open folder, spawns a fresh window instead. So: read the IDE's live
// window list (globalStorage/storage.json → windowsState.openedWindows),
// find the window whose root contains cwd, and return that root — the
// .code-workspace file for workspace windows, the folder for plain ones.
// Exact folder windows win over workspaces, then the deepest root. ""
// means no open window contains cwd (caller keeps handing over cwd).
func vscodeOpenTarget(cwd, bundleID string) string {
	product := ""
	for p, id := range ideBundleID {
		if id == bundleID {
			product = p
		}
	}
	home, err := os.UserHomeDir()
	if product == "" || err != nil {
		return ""
	}
	dir := ideUserDataDir(home, product)
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "User", "globalStorage", "storage.json"))
	if err != nil {
		return ""
	}
	var st struct {
		WindowsState struct {
			OpenedWindows []ideWindow `json:"openedWindows"`
			LastActive    *ideWindow  `json:"lastActiveWindow"`
		} `json:"windowsState"`
	}
	if json.Unmarshal(data, &st) != nil {
		return ""
	}
	wins := st.WindowsState.OpenedWindows
	if st.WindowsState.LastActive != nil {
		wins = append(wins, *st.WindowsState.LastActive)
	}
	cwd = canonPath(cwd)
	best, bestDepth, bestExact := "", -1, false
	consider := func(root, target string) {
		root = canonPath(root)
		exact := pathsEqual(root, cwd)
		if !exact && !pathContains(root, cwd) {
			return
		}
		depth := strings.Count(root, string(filepath.Separator))
		if (exact && !bestExact) || (exact == bestExact && depth > bestDepth) {
			best, bestDepth, bestExact = target, depth, exact
		}
	}
	for _, w := range wins {
		if f := fileURIPath(w.Folder); f != "" {
			consider(f, f)
		}
		ws := fileURIPath(w.Workspace.ConfigURIPath)
		if ws == "" {
			continue
		}
		for _, f := range workspaceFolders(ws) {
			consider(f, ws)
		}
	}
	return best
}

type ideWindow struct {
	Folder    string `json:"folder"`
	Workspace struct {
		ConfigURIPath string `json:"configURIPath"`
	} `json:"workspaceIdentifier"`
}

// workspaceFolders lists the absolute folder roots of a .code-workspace
// file (JSON with comments; relative paths resolve against the file).
func workspaceFolders(wsFile string) []string {
	data, err := os.ReadFile(wsFile)
	if err != nil {
		return nil
	}
	var ws struct {
		Folders []struct {
			Path string `json:"path"`
			URI  string `json:"uri"`
		} `json:"folders"`
	}
	if json.Unmarshal(stripJSONC(data), &ws) != nil {
		return nil
	}
	base := filepath.Dir(wsFile)
	var out []string
	for _, f := range ws.Folders {
		switch {
		case f.Path != "":
			p := f.Path
			if !filepath.IsAbs(p) {
				p = filepath.Join(base, p)
			}
			out = append(out, p)
		case f.URI != "":
			if p := fileURIPath(f.URI); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// stripJSONC drops // and /* */ comments plus trailing commas so a
// VSCode settings-style file parses with encoding/json.
func stripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
		case c == ',':
			// trailing comma: skip if only whitespace precedes a closer
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// fileURIPath turns a file:// URI into a native path ("" for other
// schemes, e.g. vscode-remote://).
func fileURIPath(uri string) string {
	if uri == "" {
		return ""
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	if runtime.GOOS == "windows" && len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // /c:/Users/... → c:/Users/...
	}
	return filepath.FromSlash(p)
}

// canonPath cleans and resolves symlinks (/tmp vs /private/tmp) so open
// window roots and hook cwds compare as the same tree.
func canonPath(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func pathsEqual(a, b string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// pathContains reports whether sub lies strictly inside root.
func pathContains(root, sub string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		root, sub = strings.ToLower(root), strings.ToLower(sub)
	}
	rel, err := filepath.Rel(root, sub)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
