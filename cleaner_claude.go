package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Claude Code sessions (CLI, VS Code, SDK, Desktop) under ~/.claude,
// ~/.claude-* and $CLAUDE_CONFIG_DIR. One session is:
//
//	projects/<proj>/<id>.jsonl   transcript
//	projects/<proj>/<id>/        subagents, tool-results, custom-title.json
//	file-history/<id>/           edit checkpoints
//	session-env/<id>/, debug/<id>.txt, todos/<id>*.json
//	history.jsonl lines with that sessionId (prompt history)

var cleanerSources = map[string]string{
	"cli": "cli", "claude-vscode": "vscode", "claude-desktop": "desktop",
	"sdk-cli": "sdk", "sdk-ts": "sdk", "sdk-py": "sdk",
}

// cleanerRoots: every config dir that holds a projects/ folder.
func cleanerRoots(home string, env func(string) string) []string {
	seen := map[string]bool{filepath.Join(home, ".claude"): true}
	if ents, err := os.ReadDir(home); err == nil {
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), ".claude-") {
				seen[filepath.Join(home, e.Name())] = true
			}
		}
	}
	if d := env("CLAUDE_CONFIG_DIR"); d != "" {
		if strings.HasPrefix(d, "~/") {
			d = filepath.Join(home, d[2:])
		}
		seen[filepath.Clean(d)] = true
	}
	var out []string
	for p := range seen {
		if st, err := os.Stat(filepath.Join(p, "projects")); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// firstPrompt is the first real user message: no meta, no subagent
// traffic, no injected <command-name>/<system-reminder> text.
func firstPrompt(head []byte) string {
	marker := []byte(`"type":"user"`)
	lines := bytes.SplitN(head, []byte("\n"), 501)
	for _, line := range lines[:min(len(lines), 500)] {
		if !bytes.Contains(line, marker) {
			continue
		}
		var e rawEntry
		if json.Unmarshal(line, &e) != nil || e.IsMeta || e.IsSidechain {
			continue
		}
		if t := strings.TrimSpace(contentText(e.Message.Content)); t != "" && !strings.HasPrefix(t, "<") {
			return t
		}
	}
	return ""
}

// readTranscriptMeta reads only the first 1MB (first prompt, cwd) and the
// last 4MB (latest title) of a transcript.
func readTranscriptMeta(path string) transcriptMeta {
	const headN, tailN = 1 << 20, 4 << 20
	var m transcriptMeta
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return m
	}
	var head, tail []byte
	if st.Size() <= headN+tailN {
		head, _ = io.ReadAll(f)
		tail = head
	} else {
		head = make([]byte, headN)
		n, _ := io.ReadFull(f, head)
		head = head[:n]
		tail = make([]byte, tailN)
		n, _ = f.ReadAt(tail, st.Size()-tailN)
		tail = tail[:n]
	}
	title := jsonStr(tail, "customTitle", true)
	if title == "" {
		title = jsonStr(head, "customTitle", true)
	}
	if title == "" {
		if d, err := os.ReadFile(filepath.Join(strings.TrimSuffix(path, ".jsonl"), "custom-title.json")); err == nil {
			var o struct {
				CustomTitle string `json:"customTitle"`
			}
			_ = json.Unmarshal(d, &o)
			title = o.CustomTitle
		}
	}
	m.prompt = firstPrompt(head)
	for _, alt := range []string{jsonStr(tail, "aiTitle", true), jsonStr(tail, "summary", true), m.prompt} {
		if title == "" {
			title = alt
		}
	}
	m.title = clip(oneLine(title), 120)
	m.cwd = jsonStr(head, "cwd", false)
	entry := jsonStr(head, "entrypoint", false)
	m.source = entry
	if s, ok := cleanerSources[entry]; ok {
		m.source = s
	}
	m.started = jsonStr(head, "timestamp", false)
	return m
}

// runningSessions: ids of live sessions, from the pid files Claude Code
// keeps in <root>/sessions.
func runningSessions(root string) map[string]bool {
	out := map[string]bool{}
	ents, _ := os.ReadDir(filepath.Join(root, "sessions"))
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		d, err := os.ReadFile(filepath.Join(root, "sessions", e.Name()))
		if err != nil {
			continue
		}
		var o struct {
			PID       int    `json:"pid"`
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(d, &o) == nil && o.SessionID != "" && pidAlive(o.PID) {
			out[o.SessionID] = true
		}
	}
	return out
}

func scanClaude(home string, env func(string) string) []cleanerSession {
	items := []cleanerSession{}
	for _, root := range cleanerRoots(home, env) {
		running := runningSessions(root)
		_, err := os.Stat(filepath.Join(root, "sessions"))
		pidFiles := err == nil
		side := sideIndex(root, "file-history", "session-env", "debug", "todos")
		projects, _ := os.ReadDir(filepath.Join(root, "projects"))
		for _, p := range projects {
			if !p.IsDir() {
				continue
			}
			dir := filepath.Join(root, "projects", p.Name())
			ents, _ := os.ReadDir(dir)
			for _, e := range ents {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
					continue
				}
				t := filepath.Join(dir, e.Name())
				sid := strings.TrimSuffix(e.Name(), ".jsonl")
				info, err := e.Info()
				if err != nil {
					continue
				}
				m := cleanerMeta(t, info, readTranscriptMeta)
				paths := []string{t}
				if _, err := os.Lstat(strings.TrimSuffix(t, ".jsonl")); err == nil {
					paths = append(paths, strings.TrimSuffix(t, ".jsonl"))
				}
				paths = append(paths, side[sid]...)
				delete(side, sid)
				s := cleanerSession{
					ID: t, SID: sid, Root: root, Title: m.title, Prompt: m.prompt,
					CWD: m.cwd, Project: m.cwd, Source: m.source, Started: m.started,
					Mtime: info.ModTime(), Running: running[sid], Empty: m.title == "", liveKnown: pidFiles,
				}
				if s.Project == "" {
					s.Project = p.Name()
				}
				s.addFiles(paths...)
				items = append(items, s)
			}
		}
		// side data whose transcript is gone
		for sid, paths := range side {
			s := cleanerSession{ID: "orphan:" + root + ":" + sid, SID: sid, Root: root, Orphan: true, Running: running[sid], liveKnown: pidFiles}
			s.addFiles(paths...)
			items = append(items, s)
		}
	}
	return items
}

// renameClaude does what /rename does: a custom-title entry at the end of
// the transcript, where the latest one wins. The custom-title.json some
// front ends keep beside it is rewritten in place, so its folder's mtime
// (part of the session's) stays.
func renameClaude(s cleanerSession, title string) error {
	err := keepMtime(s.ID, s.Mtime, func() error {
		return appendJSONLine(s.ID, struct {
			Type        string `json:"type"`
			SessionID   string `json:"sessionId"`
			CustomTitle string `json:"customTitle"`
		}{"custom-title", s.SID, title})
	})
	side := filepath.Join(strings.TrimSuffix(s.ID, ".jsonl"), "custom-title.json")
	if err == nil && fileExists(side) {
		data, _ := json.Marshal(map[string]string{"customTitle": title})
		err = keepMtime(side, s.Mtime, func() error { return os.WriteFile(side, data, 0o600) })
	}
	return err
}

// claudeDetail reads the transcript's last 4MB. Turns start at real
// prompts: not meta, subagent traffic, tool results or injected <…> text.
func claudeDetail(s cleanerSession) cleanerDetail {
	var d cleanerDetail
	var l turnLog
	_, tail := headTail(s.ID, 4<<20)
	for _, line := range bytes.Split(tail, []byte("\n")) {
		var e rawEntry
		if json.Unmarshal(line, &e) != nil || e.IsMeta || e.IsSidechain || e.IsCompact {
			continue
		}
		if e.GitBranch != "" {
			d.Branch = e.GitBranch
		}
		t := strings.TrimSpace(contentText(e.Message.Content))
		switch {
		case e.Type == "user" && !strings.HasPrefix(t, "<") && !strings.HasPrefix(t, "[Request interrupted"):
			l.user(t)
		case e.Type == "assistant" && e.Message.Model != "<synthetic>":
			if e.Message.Model != "" {
				d.Model = e.Message.Model
			}
			l.reply(t)
		}
	}
	d.Turns = l.last()
	return d
}

// purgeClaude drops the sessions' history.jsonl lines and their emptied
// project folders.
func purgeClaude(items []cleanerSession) (map[string][]byte, []string) {
	for _, it := range items {
		if !it.Orphan {
			_ = os.Remove(filepath.Dir(it.ID)) // the project folder, only once it's empty
		}
	}
	saved := map[string][]byte{}
	return saved, purgeByRoot(items, "history.jsonl", "sessionId", saved)
}
