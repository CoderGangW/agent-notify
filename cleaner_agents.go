package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Session sources for the agents other than Claude (cleaner_claude.go).
// Formats checked against each project's source / installed builds in
// 2026-10; the closed ones (Cursor, Antigravity) degrade to fewer details
// rather than failing when their layout moves.

// sideIndex maps session id → entries named <uuid>… in root's dirs.
func sideIndex(root string, dirs ...string) map[string][]string {
	side := map[string][]string{}
	for _, d := range dirs {
		ents, _ := os.ReadDir(filepath.Join(root, d))
		for _, e := range ents {
			if n := e.Name(); len(n) >= 36 && cleanerUUID.MatchString(n[:36]) {
				side[n[:36]] = append(side[n[:36]], filepath.Join(root, d, n))
			}
		}
	}
	return side
}

// existing keeps the paths that are there.
func existing(paths ...string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Lstat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// headTail reads up to n bytes from each end of a file (the whole file
// when it's small).
func headTail(path string, n int64) (head, tail []byte) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil
	}
	if st.Size() <= 2*n {
		head, _ = io.ReadAll(f)
		return head, head
	}
	head = make([]byte, n)
	k, _ := io.ReadFull(f, head)
	tail = make([]byte, n)
	m, _ := f.ReadAt(tail, st.Size()-n)
	return head[:k], tail[:m]
}

// ---------- SQLite via the sqlite3 CLI ----------
// ponytail: shells out rather than linking SQLite into every build; macOS
// ships sqlite3 and most Linux installs do, Windows usually doesn't — those
// agents show a notice there. Link a pure-Go driver if that matters.

var errNoSqlite = errors.New("sqlite3 not found")

func sqliteBin() string { return findCLI("sqlite3") }

// sqliteRows runs a read-only query and decodes the -json output into out.
func sqliteRows(db, query string, out any) error {
	bin := sqliteBin()
	if bin == "" {
		return errNoSqlite
	}
	cmd := exec.Command(bin, "-readonly", "-json", db, query)
	hideConsole(cmd)
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil // no rows: -json prints nothing
	}
	return json.Unmarshal(b, out)
}

// sqliteExec runs statements on one connection, waiting out a busy writer.
func sqliteExec(db string, stmts ...string) error {
	bin := sqliteBin()
	if bin == "" {
		return errNoSqlite
	}
	cmd := exec.Command(bin, append([]string{db, ".timeout 5000"}, stmts...)...)
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)) + " " + err.Error())
	}
	return nil
}

func sqliteTables(db string) map[string]bool {
	var rows []struct {
		Name string `json:"name"`
	}
	_ = sqliteRows(db, "SELECT name FROM sqlite_master WHERE type = 'table'", &rows)
	out := map[string]bool{}
	for _, r := range rows {
		out[r.Name] = true
	}
	return out
}

// sqlIDs renders ids as an SQL list; anything that isn't a plain id is
// dropped, so no quote can reach the statement.
func sqlIDs(ids []string) string {
	q := make([]string, 0, len(ids))
	for _, id := range ids {
		if cleanerID.MatchString(id) {
			q = append(q, "'"+id+"'")
		}
	}
	if len(q) == 0 {
		return "''"
	}
	return strings.Join(q, ",")
}

func msTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func needSqlite(db string) string {
	if _, err := os.Stat(db); err == nil && sqliteBin() == "" {
		return "cl.needSqlite"
	}
	return ""
}

// purgeByRoot strips each root's JSONL index of the sessions' lines and
// keeps the removed lines for the trash copy.
func purgeByRoot(items []cleanerSession, file, key string, saved map[string][]byte) []string {
	var errs []string
	byRoot := map[string]map[string]bool{}
	for _, it := range items {
		if byRoot[it.Root] == nil {
			byRoot[it.Root] = map[string]bool{}
		}
		byRoot[it.Root][it.SID] = true
	}
	for root, ids := range byRoot {
		p := filepath.Join(root, file)
		removed, err := stripJSONLines(p, key, ids)
		if err != nil {
			errs = append(errs, p+": "+err.Error())
		} else if len(removed) > 0 {
			saved[filepath.Join(filepath.Base(root), strings.TrimSuffix(file, ".jsonl")+"-removed.jsonl")] = removed
		}
	}
	return errs
}

// ---------- Codex ----------
//
//	sessions/YYYY/MM/DD/rollout-<ts>-<thread>[_<rollout>].jsonl[.zst]
//	archived_sessions/rollout-….jsonl     (flat)
//	session_index.jsonl                  thread names (id, thread_name)
//	history.jsonl                        prompt history (session_id)
//	shell_snapshots/<thread>.*
//	thread-writer-locks/<thread>.lock    flock held while a Codex writes it
//
// ponytail: the state_*.sqlite thread rows stay behind; Codex skips rows
// whose rollout is gone, and a restore from the trash matches them again.

var codexRollout = regexp.MustCompile(`^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:_[^.]*)?\.jsonl(?:\.zst)?$`)

func codexHome(home string, env func(string) string) string {
	if d := env("CODEX_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".codex")
}

// codexNames: the last non-empty thread name per id.
func codexNames(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var e struct {
			ID   string `json:"id"`
			Name string `json:"thread_name"`
		}
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Name != "" {
			out[e.ID] = e.Name
		}
	}
	return out
}

// codexPrompt drops the wrapper some front ends put before the request.
func codexPrompt(s string) string {
	if i := strings.Index(s, "## My request for Codex:"); i >= 0 {
		s = s[i+len("## My request for Codex:"):]
	}
	return strings.TrimSpace(s)
}

func readCodexMeta(path string) transcriptMeta {
	var m transcriptMeta
	if strings.HasSuffix(path, ".zst") {
		return m // compressed: the name and size still list it
	}
	head, _ := headTail(path, 1<<20)
	for _, line := range bytes.Split(head, []byte("\n")) {
		switch {
		case bytes.Contains(line, []byte(`"session_meta"`)) && m.id == "":
			var e struct {
				Payload struct {
					ID        string          `json:"id"`
					CWD       string          `json:"cwd"`
					Timestamp string          `json:"timestamp"`
					Source    json.RawMessage `json:"source"`
				} `json:"payload"`
			}
			if json.Unmarshal(line, &e) != nil {
				continue
			}
			m.id, m.cwd, m.started = e.Payload.ID, e.Payload.CWD, e.Payload.Timestamp
			// "cli" | "vscode" | "exec" | "mcp" | {"subagent":…} | {"custom":"atlas"}
			if json.Unmarshal(e.Payload.Source, &m.source) != nil {
				var obj map[string]json.RawMessage
				_ = json.Unmarshal(e.Payload.Source, &obj)
				for k, v := range obj {
					m.source = k
					var s string
					if k == "custom" && json.Unmarshal(v, &s) == nil {
						m.source = s
					}
				}
			}
		case m.prompt == "" && bytes.Contains(line, []byte(`"user_message"`)):
			var e struct {
				Payload struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"payload"`
			}
			if json.Unmarshal(line, &e) == nil && e.Payload.Type == "user_message" {
				m.prompt = codexPrompt(e.Payload.Message)
			}
		case m.prompt == "" && bytes.Contains(line, []byte(`"UserMessage"`)):
			var e struct {
				Payload struct {
					Item struct {
						Type    string `json:"type"`
						Content []struct {
							Text string `json:"text"`
						} `json:"content"`
					} `json:"item"`
				} `json:"payload"`
			}
			if json.Unmarshal(line, &e) == nil && e.Payload.Item.Type == "UserMessage" {
				var parts []string
				for _, c := range e.Payload.Item.Content {
					parts = append(parts, c.Text)
				}
				m.prompt = codexPrompt(strings.Join(parts, " "))
			}
		}
		if m.id != "" && m.prompt != "" {
			break
		}
	}
	return m
}

func scanCodex(home string, env func(string) string) []cleanerSession {
	root := codexHome(home, env)
	names := codexNames(filepath.Join(root, "session_index.jsonl"))
	locks := filepath.Join(root, "thread-writer-locks")
	_, err := os.Stat(locks)
	lockable := err == nil // older Codex builds have no writer locks
	side := sideIndex(root, "shell_snapshots")
	byID := map[string]*cleanerSession{}
	var order []string
	visit := func(path string, d fs.DirEntry) {
		mt := codexRollout.FindStringSubmatch(d.Name())
		if mt == nil {
			return
		}
		info, err := d.Info()
		if err != nil {
			return
		}
		id := mt[1]
		s := byID[id]
		if s == nil {
			s = &cleanerSession{ID: id, SID: id, Root: root}
			byID[id] = s
			order = append(order, id)
		}
		// a thread's extra rollouts (reverts) share its id; the first one
		// with a session_meta tells where and how it started
		if m := cleanerMeta(path, info, readCodexMeta); s.CWD == "" && s.Prompt == "" {
			s.CWD, s.Source, s.Started, s.Prompt = m.cwd, m.source, m.started, m.prompt
		}
		s.addFiles(path)
	}
	_ = filepath.WalkDir(filepath.Join(root, "sessions"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			visit(p, d)
		}
		return nil
	})
	ents, _ := os.ReadDir(filepath.Join(root, "archived_sessions"))
	for _, e := range ents {
		visit(filepath.Join(root, "archived_sessions", e.Name()), e)
	}
	items := make([]cleanerSession, 0, len(order))
	for _, id := range order {
		s := byID[id]
		s.addFiles(side[id]...)
		if lockable {
			s.Running, s.liveKnown = fileLocked(filepath.Join(locks, id+".lock")), true
		}
		s.Title = names[id]
		s.Empty = s.Title == "" && s.Prompt == ""
		if s.Title == "" {
			s.Title = clip(oneLine(s.Prompt), 120)
		}
		s.Project = s.CWD
		items = append(items, *s)
	}
	return items
}

func purgeCodex(items []cleanerSession) (map[string][]byte, []string) {
	saved := map[string][]byte{}
	errs := purgeByRoot(items, "history.jsonl", "session_id", saved)
	errs = append(errs, purgeByRoot(items, "session_index.jsonl", "id", saved)...)
	// emptied day/month/year folders
	for _, it := range items {
		for _, f := range it.Files {
			for d := filepath.Dir(f.Path); strings.HasPrefix(d, filepath.Join(it.Root, "sessions")+string(filepath.Separator)); d = filepath.Dir(d) {
				if os.Remove(d) != nil {
					break
				}
			}
		}
	}
	return saved, errs
}

// ---------- Gemini CLI ----------
//
//	tmp/<project>/chats/session-<utc>-<id8>.jsonl   ({"$set":{summary}} = title)
//	tmp/<project>/chats/session-….json              legacy, may sit beside a .jsonl copy
//	tmp/<project>/chats/<id>/                       subagent chats
//	tmp/<project>/logs/session-<id>.jsonl, tool-outputs/session-<id>/, <id>/
//	tmp/<project>/logs.json                         prompt log (array, sessionId)
//
// <project> is a slug named in .project_root / projects.json, or a sha256
// of the root for folders not migrated yet (path unknown then).

func geminiDir(home string, env func(string) string) string {
	if d := env("GEMINI_CLI_HOME"); d != "" {
		return filepath.Join(d, ".gemini")
	}
	return filepath.Join(home, ".gemini")
}

func geminiUserText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		var parts []struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(raw, &parts)
		var b []string
		for _, p := range parts {
			b = append(b, p.Text)
		}
		s = strings.Join(b, " ")
	}
	s = strings.TrimSpace(s)
	for _, skip := range []string{"/", "?", "<session_context>", "<hook_context>"} {
		if strings.HasPrefix(s, skip) {
			return ""
		}
	}
	return s
}

type geminiMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

func readGeminiMeta(path string) transcriptMeta {
	var m transcriptMeta
	if strings.HasSuffix(path, ".json") {
		data, err := os.ReadFile(path)
		if err != nil {
			return m
		}
		var r struct {
			SessionID string          `json:"sessionId"`
			StartTime string          `json:"startTime"`
			Summary   string          `json:"summary"`
			Messages  []geminiMessage `json:"messages"`
		}
		if json.Unmarshal(data, &r) != nil {
			return m
		}
		m.id, m.started, m.title = r.SessionID, r.StartTime, r.Summary
		for _, msg := range r.Messages {
			if msg.Type == "user" {
				if m.prompt = geminiUserText(msg.Content); m.prompt != "" {
					break
				}
			}
		}
		return m
	}
	head, tail := headTail(path, 1<<20)
	for i, line := range bytes.Split(head, []byte("\n")) {
		if i == 0 {
			var meta struct {
				SessionID string `json:"sessionId"`
				StartTime string `json:"startTime"`
			}
			_ = json.Unmarshal(line, &meta)
			m.id, m.started = meta.SessionID, meta.StartTime
			continue
		}
		if !bytes.Contains(line, []byte(`"type":"user"`)) {
			continue
		}
		var msg geminiMessage
		if json.Unmarshal(line, &msg) == nil && msg.Type == "user" {
			if m.prompt = geminiUserText(msg.Content); m.prompt != "" {
				break
			}
		}
	}
	// the latest {"$set":{"summary":…}} names the session
	lines := bytes.Split(tail, []byte("\n"))
	for i := len(lines) - 1; i >= 0 && m.title == ""; i-- {
		if bytes.HasPrefix(lines[i], []byte(`{"$set"`)) && bytes.Contains(lines[i], []byte(`"summary"`)) {
			var set struct {
				Set struct {
					Summary string `json:"summary"`
				} `json:"$set"`
			}
			if json.Unmarshal(lines[i], &set) == nil {
				m.title = set.Set.Summary
			}
		}
	}
	return m
}

func scanGemini(home string, env func(string) string) []cleanerSession {
	root := geminiDir(home, env)
	var reg struct {
		Projects map[string]string `json:"projects"` // path → slug
	}
	if d, err := os.ReadFile(filepath.Join(root, "projects.json")); err == nil {
		_ = json.Unmarshal(d, &reg)
	}
	slugPath := map[string]string{}
	for p, slug := range reg.Projects {
		slugPath[slug] = p
	}
	items := []cleanerSession{}
	dirs, _ := os.ReadDir(filepath.Join(root, "tmp"))
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "bin" {
			continue
		}
		pdir := filepath.Join(root, "tmp", d.Name())
		proj := slugPath[d.Name()]
		if b, err := os.ReadFile(filepath.Join(pdir, ".project_root")); err == nil {
			proj = strings.TrimSpace(string(b))
		}
		byID := map[string]*cleanerSession{}
		var order []string
		chats, _ := os.ReadDir(filepath.Join(pdir, "chats"))
		for _, e := range chats {
			n := e.Name()
			if e.IsDir() || !strings.HasPrefix(n, "session-") || !(strings.HasSuffix(n, ".jsonl") || strings.HasSuffix(n, ".json")) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			path := filepath.Join(pdir, "chats", n)
			m := cleanerMeta(path, info, readGeminiMeta)
			if m.id == "" {
				continue
			}
			s := byID[m.id]
			if s == nil {
				s = &cleanerSession{ID: m.id, SID: m.id, Root: root, Source: "cli", CWD: proj, Project: proj}
				if proj == "" {
					s.Project = d.Name()
				}
				byID[m.id] = s
				order = append(order, m.id)
			}
			// a resumed legacy .json leaves a .jsonl copy: the copy is current
			if s.Title == "" || strings.HasSuffix(n, ".jsonl") {
				if m.title != "" {
					s.Title = m.title
				}
				if m.prompt != "" {
					s.Prompt = m.prompt
				}
				s.Started = m.started
			}
			s.addFiles(path)
		}
		for _, id := range order {
			s := byID[id]
			s.addFiles(existing(
				filepath.Join(pdir, "chats", id),
				filepath.Join(pdir, "logs", "session-"+id+".jsonl"),
				filepath.Join(pdir, "tool-outputs", "session-"+id),
				filepath.Join(pdir, id),
			)...)
			s.Empty = s.Title == "" && s.Prompt == ""
			if s.Title == "" {
				s.Title = clip(oneLine(s.Prompt), 120)
			}
			items = append(items, *s)
		}
	}
	return items
}

// purgeGemini drops the sessions from each project's logs.json (a JSON
// array Gemini's own delete leaves alone).
func purgeGemini(items []cleanerSession) (map[string][]byte, []string) {
	saved := map[string][]byte{}
	var errs []string
	byLog := map[string]map[string]bool{}
	for _, it := range items {
		for _, f := range it.Files {
			if filepath.Base(filepath.Dir(f.Path)) == "chats" {
				p := filepath.Join(filepath.Dir(filepath.Dir(f.Path)), "logs.json")
				if byLog[p] == nil {
					byLog[p] = map[string]bool{}
				}
				byLog[p][it.SID] = true
			}
		}
	}
	for p, ids := range byLog {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var entries []map[string]any
		if json.Unmarshal(data, &entries) != nil {
			continue
		}
		var keep, removed []map[string]any
		for _, e := range entries {
			if id, _ := e["sessionId"].(string); ids[id] {
				removed = append(removed, e)
			} else {
				keep = append(keep, e)
			}
		}
		if len(removed) == 0 {
			continue
		}
		if keep == nil {
			keep = []map[string]any{}
		}
		out, _ := json.MarshalIndent(keep, "", "  ")
		if err := replaceFile(p, out); err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		rel, _ := filepath.Rel(filepath.Dir(items[0].Root), p)
		b, _ := json.MarshalIndent(removed, "", "  ")
		saved[strings.TrimSuffix(rel, ".json")+"-removed.json"] = b
	}
	return saved, errs
}

// ---------- Antigravity: agy CLI and the IDE keep the same layout ----------
//
//	conversations/<id>.db|.pb   the conversation
//	brain/<id>/                 plans, task lists, transcript logs
//	annotations/<id>.pbtxt      title
//	presence/<id>.lock
//	history.jsonl               prompt history (conversationId)
//	cache/conversation_metadata.json, conversation_summaries.db (SQLite)

var agyRoots = []struct{ dir, source string }{{"antigravity-cli", "cli"}, {"antigravity", "desktop"}}

var (
	pbTitle    = regexp.MustCompile(`(?m)^title:\s*("(?:[^"\\]|\\.)*")`)
	agyRequest = regexp.MustCompile(`(?s)<USER_REQUEST>\s*(.*?)\s*</USER_REQUEST>`)
)

type agyHist struct{ prompt, cwd, started string }

// agyHistory: first prompt, workspace and time per conversation.
func agyHistory(path string) map[string]agyHist {
	out := map[string]agyHist{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		var e struct {
			Display        string `json:"display"`
			Timestamp      int64  `json:"timestamp"`
			Workspace      string `json:"workspace"`
			ConversationID string `json:"conversationId"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.ConversationID == "" {
			continue
		}
		h, seen := out[e.ConversationID]
		if !seen {
			h.cwd = e.Workspace
			if t := msTime(e.Timestamp); !t.IsZero() {
				h.started = t.Format(time.RFC3339)
			}
		}
		if h.prompt == "" && !strings.HasPrefix(e.Display, "/") { // slash commands aren't prompts
			h.prompt = e.Display
		}
		out[e.ConversationID] = h
	}
	return out
}

// agyWorkspaces: first workspace per conversation from agy's metadata
// cache (plain JSON, unlike the summaries db).
func agyWorkspaces(root string) map[string]string {
	var c struct {
		Conversations map[string]struct {
			Summary struct {
				WorkspaceURIs []string `json:"WorkspaceURIs"`
			} `json:"summary"`
		} `json:"conversations"`
	}
	if d, err := os.ReadFile(filepath.Join(root, "cache", "conversation_metadata.json")); err == nil {
		_ = json.Unmarshal(d, &c)
	}
	out := map[string]string{}
	for id, v := range c.Conversations {
		if len(v.Summary.WorkspaceURIs) > 0 {
			if u, err := url.Parse(v.Summary.WorkspaceURIs[0]); err == nil && u.Scheme == "file" {
				out[id] = u.Path
			}
		}
	}
	return out
}

// agyTitle: the annotation's title, else the brain's task-list heading.
func agyTitle(root, id string) string {
	if d, err := os.ReadFile(filepath.Join(root, "annotations", id+".pbtxt")); err == nil {
		if m := pbTitle.FindSubmatch(d); m != nil {
			if t, err := strconv.Unquote(string(m[1])); err == nil && t != "" {
				return t
			}
		}
	}
	if d, err := os.ReadFile(filepath.Join(root, "brain", id, "task.md")); err == nil {
		for _, line := range strings.SplitN(string(d), "\n", 20) {
			if strings.HasPrefix(line, "# ") {
				return strings.TrimSpace(line[2:])
			}
		}
	}
	return ""
}

// agyTranscriptPrompt: the first explicit user input in the brain's log.
func agyTranscriptPrompt(root, id string) string {
	head, _ := headTail(filepath.Join(root, "brain", id, ".system_generated", "logs", "transcript.jsonl"), 256<<10)
	for _, line := range bytes.Split(head, []byte("\n")) {
		var e struct {
			Source  string `json:"source"`
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if json.Unmarshal(line, &e) != nil || e.Source != "USER_EXPLICIT" || e.Type != "USER_INPUT" {
			continue
		}
		if m := agyRequest.FindStringSubmatch(e.Content); m != nil {
			return m[1]
		}
		return strings.TrimSpace(e.Content)
	}
	return ""
}

func scanAntigravity(home string, _ func(string) string) []cleanerSession {
	items := []cleanerSession{}
	ws := agyWorkspaces(filepath.Join(home, ".gemini", "antigravity-cli"))
	for _, r := range agyRoots {
		root := filepath.Join(home, ".gemini", r.dir)
		side := sideIndex(root, "conversations", "brain", "annotations", "html_artifacts", "presence")
		hist := agyHistory(filepath.Join(root, "history.jsonl"))
		ents, _ := os.ReadDir(filepath.Join(root, "conversations"))
		seen := map[string]bool{}
		for _, e := range ents {
			n := e.Name()
			if len(n) < 36 || !cleanerUUID.MatchString(n[:36]) || seen[n[:36]] {
				continue
			}
			id := n[:36]
			seen[id] = true
			h := hist[id]
			s := cleanerSession{
				ID: r.dir + ":" + id, SID: id, Root: root, Source: r.source,
				Prompt: h.prompt, CWD: h.cwd, Started: h.started, Title: agyTitle(root, id),
			}
			if s.Prompt == "" {
				s.Prompt = agyTranscriptPrompt(root, id)
			}
			if s.CWD == "" {
				s.CWD = ws[id]
			}
			s.Empty = s.Title == "" && s.Prompt == ""
			if s.Title == "" {
				s.Title = clip(oneLine(s.Prompt), 120)
			}
			s.Project = s.CWD
			s.addFiles(side[id]...)
			items = append(items, s)
		}
	}
	return items
}

func purgeAntigravity(items []cleanerSession) (map[string][]byte, []string) {
	saved := map[string][]byte{}
	errs := purgeByRoot(items, "history.jsonl", "conversationId", saved)
	ids := map[string]bool{}
	var all []string
	for _, it := range items {
		ids[it.SID] = true
		all = append(all, it.SID)
	}
	cli := filepath.Join(filepath.Dir(items[0].Root), "antigravity-cli")
	// metadata cache: drop the entries so nothing lists a ghost
	if p := filepath.Join(cli, "cache", "conversation_metadata.json"); fileExists(p) {
		var c map[string]json.RawMessage
		var convs map[string]json.RawMessage
		if d, err := os.ReadFile(p); err == nil && json.Unmarshal(d, &c) == nil && json.Unmarshal(c["conversations"], &convs) == nil {
			gone := map[string]json.RawMessage{}
			for id := range convs {
				if ids[id] {
					gone[id] = convs[id]
					delete(convs, id)
				}
			}
			if len(gone) > 0 {
				c["conversations"], _ = json.Marshal(convs)
				out, _ := json.Marshal(c)
				if err := replaceFile(p, out); err != nil {
					errs = append(errs, p+": "+err.Error())
				}
				saved[filepath.Join("antigravity-cli", "conversation_metadata-removed.json")], _ = json.MarshalIndent(gone, "", "  ")
			}
		}
	}
	// summary index rows: export them for the trash copy, then drop them
	db := filepath.Join(cli, "conversation_summaries.db")
	if !fileExists(db) || sqliteBin() == "" {
		return saved, errs
	}
	where := " WHERE conversation_id IN (" + sqlIDs(all) + ")"
	var rows []map[string]any
	if err := sqliteRows(db, "SELECT * FROM conversation_summaries"+where, &rows); err != nil {
		return saved, append(errs, db+": "+err.Error())
	}
	if len(rows) == 0 {
		return saved, errs
	}
	saved[filepath.Join("antigravity-cli", "conversation_summaries-removed.json")], _ = json.MarshalIndent(rows, "", "  ")
	if err := sqliteExec(db, "DELETE FROM conversation_summaries"+where); err != nil {
		errs = append(errs, db+": "+err.Error())
	}
	return saved, errs
}

// ---------- opencode: sessions are rows in opencode.db ----------
// v2 (opencode 2.x) uses session_v2 + session_message and an event log
// the foreign-key cascade doesn't reach; v1 used session/message/part.
// A session's subagent runs are child sessions (parent_id) and go with it.

func opencodeDB(home string, env func(string) string) string {
	if p := env("OPENCODE_DB"); p != "" {
		return p
	}
	if d := env("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "opencode", "opencode.db")
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

type ocSchema struct {
	session string            // session table
	list    string            // one row per top-level session
	export  map[string]string // table → rows to save, with {fam}
	deletes []string          // with {fam}; the session row last
}

// family is a session plus all its descendants.
func ocFamily(table, ids string) string {
	return "(WITH RECURSIVE fam(id) AS (SELECT id FROM " + table + " WHERE id IN (" + ids +
		") UNION SELECT c.id FROM " + table + " c JOIN fam ON c.parent_id = fam.id) SELECT id FROM fam)"
}

var ocV1 = ocSchema{
	session: "session",
	list: `SELECT s.id, s.directory, s.title, s.time_created AS created, s.time_updated AS updated,
 (SELECT COUNT(*) FROM message m WHERE m.session_id IN {kids}) AS messages,
 (SELECT COALESCE(SUM(LENGTH(m.data)), 0) FROM message m WHERE m.session_id IN {kids})
 + (SELECT COALESCE(SUM(LENGTH(p.data)), 0) FROM part p JOIN message m ON p.message_id = m.id
    WHERE m.session_id IN {kids}) AS bytes,
 (SELECT GROUP_CONCAT(c.id) FROM session c WHERE c.parent_id = s.id) AS children,
 (SELECT json_extract(p.data, '$.text') FROM part p JOIN message m ON p.message_id = m.id
    WHERE m.session_id = s.id AND json_extract(m.data, '$.role') = 'user'
      AND json_extract(p.data, '$.type') = 'text'
    ORDER BY m.time_created, p.id LIMIT 1) AS prompt
FROM session s WHERE s.parent_id IS NULL`,
	export: map[string]string{
		"session":       "SELECT * FROM session WHERE id IN {fam}",
		"message":       "SELECT * FROM message WHERE session_id IN {fam}",
		"part":          "SELECT p.* FROM part p JOIN message m ON p.message_id = m.id WHERE m.session_id IN {fam}",
		"todo":          "SELECT * FROM todo WHERE session_id IN {fam}",
		"session_share": "SELECT * FROM session_share WHERE session_id IN {fam}",
	},
	deletes: []string{"DELETE FROM session WHERE id IN {fam}"},
}

var ocV2 = ocSchema{
	session: "session_v2",
	list: `SELECT s.id, s.directory, COALESCE(NULLIF(s.title, ''), s.slug, '') AS title,
 s.time_created AS created, s.time_updated AS updated,
 (SELECT COUNT(*) FROM session_message m WHERE m.session_id IN {kids}) AS messages,
 (SELECT COALESCE(SUM(LENGTH(m.data)), 0) FROM session_message m WHERE m.session_id IN {kids}) AS bytes,
 (SELECT GROUP_CONCAT(c.id) FROM session_v2 c WHERE c.parent_id = s.id) AS children,
 (SELECT json_extract(m.data, '$.text') FROM session_message m
    WHERE m.session_id = s.id AND m.type = 'user' ORDER BY m.seq LIMIT 1) AS prompt
FROM session_v2 s WHERE s.parent_id IS NULL`,
	export: map[string]string{
		"session_v2":        "SELECT * FROM session_v2 WHERE id IN {fam}",
		"session_message":   "SELECT * FROM session_message WHERE session_id IN {fam}",
		"session_pending":   "SELECT * FROM session_pending WHERE session_id IN {fam}",
		"session_inbox":     "SELECT * FROM session_inbox WHERE session_id IN {fam}",
		"instruction_entry": "SELECT * FROM instruction_entry WHERE session_id IN {fam}",
		"instruction_state": "SELECT * FROM instruction_state WHERE session_id IN {fam}",
		"event":             "SELECT * FROM event WHERE aggregate_id IN {fam}",
		"event_sequence":    "SELECT * FROM event_sequence WHERE aggregate_id IN {fam}",
	},
	// opencode's own remove: the event log first, then the cascade
	deletes: []string{
		"DELETE FROM event WHERE aggregate_id IN {fam}",
		"DELETE FROM event_sequence WHERE aggregate_id IN {fam}",
		"DELETE FROM session_v2 WHERE id IN {fam}",
	},
}

func opencodeSchema(db string) (ocSchema, map[string]bool) {
	tables := sqliteTables(db)
	if tables["session_v2"] {
		return ocV2, tables
	}
	return ocV1, tables
}

type ocSession struct {
	ID        string  `json:"id"`
	Directory string  `json:"directory"`
	Title     string  `json:"title"`
	Created   int64   `json:"created"`
	Updated   int64   `json:"updated"`
	Messages  int64   `json:"messages"`
	Bytes     int64   `json:"bytes"`
	Children  *string `json:"children"`
	Prompt    *string `json:"prompt"`
}

func scanOpencode(home string, env func(string) string) []cleanerSession {
	items := []cleanerSession{}
	db := opencodeDB(home, env)
	if !fileExists(db) {
		return items
	}
	schema, _ := opencodeSchema(db)
	kids := "(SELECT c.id FROM " + schema.session + " c WHERE c.id = s.id OR c.parent_id = s.id)"
	var rows []ocSession
	if sqliteRows(db, strings.ReplaceAll(schema.list, "{kids}", kids), &rows) != nil {
		return items
	}
	dir := filepath.Dir(db)
	for _, r := range rows {
		s := cleanerSession{
			ID: r.ID, SID: r.ID, Root: dir, Title: r.Title, CWD: r.Directory, Project: r.Directory,
			Source: "cli", Size: r.Bytes, Empty: r.Messages == 0,
		}
		if r.Prompt != nil {
			s.Prompt = *r.Prompt
		}
		if t := msTime(r.Created); !t.IsZero() {
			s.Started = t.Format(time.RFC3339)
		}
		ids := []string{r.ID}
		if r.Children != nil {
			ids = append(ids, strings.Split(*r.Children, ",")...)
		}
		for _, id := range ids { // v1 left per-session diffs on disk
			s.addFiles(existing(filepath.Join(dir, "storage", "session_diff", id+".json"))...)
		}
		s.Mtime = msTime(r.Updated) // file mtimes don't speak for the session
		items = append(items, s)
	}
	return items
}

func purgeOpencode(items []cleanerSession) (map[string][]byte, []string) {
	saved := map[string][]byte{}
	db := filepath.Join(items[0].Root, "opencode.db")
	if p := os.Getenv("OPENCODE_DB"); p != "" {
		db = p
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.SID
	}
	schema, tables := opencodeSchema(db)
	fam := ocFamily(schema.session, sqlIDs(ids))
	// save every row the delete will take, for the trash copy
	export := map[string]any{}
	for table, q := range schema.export {
		if !tables[table] {
			continue
		}
		var rows []map[string]any
		if err := sqliteRows(db, strings.ReplaceAll(q, "{fam}", fam), &rows); err != nil {
			return saved, []string{db + ": " + err.Error()} // no copy, no delete
		}
		export[table] = rows
	}
	saved[filepath.Join("opencode", "sessions-removed.json")], _ = json.MarshalIndent(export, "", "  ")
	stmts := []string{"PRAGMA foreign_keys = ON"} // the cascade needs it per connection
	for _, d := range schema.deletes {
		if table := strings.Fields(d)[2]; tables[table] {
			stmts = append(stmts, strings.ReplaceAll(d, "{fam}", fam))
		}
	}
	if err := sqliteExec(db, stmts...); err != nil {
		return saved, []string{db + ": " + err.Error()}
	}
	return saved, nil
}

// ---------- Cursor CLI ----------
//
//	chats/<md5(workspace)>/<chatId>/store.db   SQLite; meta '0' = hex(JSON)
//
// The workspace hash is one-way: paths other agents' sessions mention
// (and the home folder) are hashed to name it back.

func cursorDir(home string, env func(string) string) string {
	if d := env("CURSOR_CONFIG_DIR"); d != "" {
		return d
	}
	if d := env("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "cursor")
	}
	return filepath.Join(home, ".cursor")
}

func readCursorMeta(db string) transcriptMeta {
	var m transcriptMeta
	var rows []struct {
		Value string `json:"value"`
	}
	if sqliteRows(db, "SELECT value FROM meta WHERE key = '0'", &rows) != nil || len(rows) == 0 {
		return m
	}
	raw, err := hex.DecodeString(rows[0].Value)
	if err != nil {
		raw = []byte(rows[0].Value)
	}
	var meta struct {
		Name       string `json:"name"`
		CreatedAt  int64  `json:"createdAt"`
		LatestRoot string `json:"latestRootBlobId"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		return m
	}
	if meta.Name != "New Agent" { // the default name says nothing
		m.title = meta.Name
	}
	if t := msTime(meta.CreatedAt); !t.IsZero() {
		m.started = t.Format(time.RFC3339)
	}
	m.empty = meta.LatestRoot == ""
	return m
}

// cursorWorkspaces maps md5(path) → path for every folder this app has
// seen a session in.
func cursorWorkspaces(home string) map[string]string {
	out := map[string]string{}
	add := func(p string) {
		if p != "" {
			sum := md5.Sum([]byte(p))
			out[hex.EncodeToString(sum[:])] = p
		}
	}
	add(home)
	for _, agent := range cleaner.cache {
		for _, c := range agent {
			add(c.meta.cwd)
		}
	}
	for _, it := range cleaner.last {
		add(it.CWD)
	}
	return out
}

func scanCursor(home string, env func(string) string) []cleanerSession {
	root := cursorDir(home, env)
	ws := cursorWorkspaces(home)
	items := []cleanerSession{}
	hashes, _ := os.ReadDir(filepath.Join(root, "chats"))
	for _, h := range hashes {
		chats, _ := os.ReadDir(filepath.Join(root, "chats", h.Name()))
		for _, c := range chats {
			dir := filepath.Join(root, "chats", h.Name(), c.Name())
			db := filepath.Join(dir, "store.db")
			info, err := os.Stat(db)
			if !c.IsDir() || err != nil {
				continue
			}
			m := cleanerMeta(db, info, readCursorMeta)
			s := cleanerSession{
				ID: c.Name(), SID: c.Name(), Root: root, Source: "cli", Title: m.title,
				Started: m.started, CWD: ws[h.Name()], Project: ws[h.Name()], Empty: m.empty,
			}
			if s.Project == "" {
				s.Project = h.Name()
			}
			s.addFiles(dir)
			items = append(items, s)
		}
	}
	return items
}
