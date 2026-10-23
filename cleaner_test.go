package main

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCleanerScanDelete(t *testing.T) {
	home := t.TempDir()
	write := func(p, text string) {
		t.Helper()
		p = filepath.Join(home, p)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exists := func(p string) bool { _, err := os.Lstat(filepath.Join(home, p)); return err == nil }
	sid, other, orphan := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"
	proj := ".claude/projects/-tmp-demo"
	write(proj+"/"+sid+".jsonl", `{"type":"user","cwd":"/tmp/demo","entrypoint":"claude-vscode","timestamp":"2026-10-01T10:00:00Z","message":{"content":"<command-name>/init</command-name>"}}
{"type":"user","message":{"content":[{"type":"text","text":"버그 고쳐줘"}]}}
{"type":"ai-title","aiTitle":"옛 제목"}
{"type":"ai-title","aiTitle":"버그 \"수정\" 세션"}
`)
	write(proj+"/"+sid+"/subagents/a.jsonl", "{}")
	write(proj+"/"+other+".jsonl", `{"type":"user","message":{"content":"남길 세션"}}`)
	write(".claude/file-history/"+sid+"/x", "x")
	write(".claude/session-env/"+sid+"/x", "x")
	write(".claude/file-history/"+orphan+"/x", "x")
	write(".claude/history.jsonl", `{"sessionId":"`+sid+`"}`+"\n"+`{"sessionId":"`+other+`"}`+"\n")
	hist := filepath.Join(home, ".claude/history.jsonl")
	if err := os.Chmod(hist, 0o600); err != nil {
		t.Fatal(err)
	}

	noEnv := func(string) string { return "" }
	items := map[string]cleanerSession{}
	for _, s := range cleanerScan("claude", home, noEnv, config{CleanerExcluded: []string{"/tmp/demo"}}, nil) {
		items[s.SID] = s
	}
	s := items[sid]
	if s.Title != `버그 "수정" 세션` || s.Source != "vscode" || s.Prompt != "버그 고쳐줘" || s.CWD != "/tmp/demo" {
		t.Fatalf("meta: %+v", s)
	}
	if len(s.Files) != 4 {
		t.Fatalf("files: %+v", s.Files)
	}
	if s.Excluded != "/tmp/demo" || s.deletable() {
		t.Fatalf("exclusion: %q", s.Excluded)
	}
	if items[other].Title != "남길 세션" || !items[orphan].Orphan {
		t.Fatalf("other/orphan: %+v %+v", items[other], items[orphan])
	}

	trashDir := filepath.Join(home, "Trash")
	trash := func(p string) error {
		if err := os.MkdirAll(trashDir, 0o755); err != nil {
			return err
		}
		return os.Rename(p, filepath.Join(trashDir, filepath.Base(p)))
	}
	if errs := cleanerDelete([]cleanerSession{s, items[orphan]}, false, home, trash); len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, p := range []string{proj + "/" + sid + ".jsonl", proj + "/" + sid, ".claude/file-history/" + sid, ".claude/file-history/" + orphan} {
		if exists(p) {
			t.Errorf("left behind: %s", p)
		}
	}
	if !exists(proj + "/" + other + ".jsonl") {
		t.Error("other session deleted")
	}
	if d, _ := os.ReadFile(hist); string(d) != `{"sessionId":"`+other+`"}`+"\n" {
		t.Errorf("history: %q", d)
	}
	if st, _ := os.Stat(hist); st.Mode().Perm() != 0o600 {
		t.Errorf("history mode: %v", st.Mode())
	}
	trashed, _ := os.ReadDir(trashDir)
	if len(trashed) != 1 {
		t.Fatalf("trash: %v", trashed)
	}
	root := filepath.Join("Trash", trashed[0].Name(), ".claude")
	if !exists(filepath.Join(root, "projects/-tmp-demo", sid+".jsonl")) || !exists(filepath.Join(root, "history-removed.jsonl")) {
		t.Error("trash layout")
	}
	if exists(".claude-notify/cleaner-staging") {
		t.Error("staging left behind")
	}

	if errs := cleanerDelete([]cleanerSession{items[other]}, true, home, trash); len(errs) > 0 {
		t.Fatal(errs)
	}
	if exists(proj) {
		t.Error("empty project folder left behind")
	}

	if excludingFolder([]string{"/a/b"}, "/a/b/c") != "/a/b" || excludingFolder([]string{"/a/b"}, "/a/b") != "/a/b" {
		t.Error("exclusion should cover the folder and its children")
	}
	if excludingFolder([]string{"/a/b"}, "/a/bc") != "" {
		t.Error("exclusion boundary")
	}
	if shellQuote("it's") != `'it'\''s'` {
		t.Error(shellQuote("it's"))
	}
}

func fakeTrash(t *testing.T, dir string) func(string) error {
	return func(p string) error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return os.Rename(p, filepath.Join(dir, filepath.Base(p)))
	}
}

func writeAll(t *testing.T, home string, files map[string]string) {
	t.Helper()
	for p, text := range files {
		p = filepath.Join(home, p)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanerAntigravity(t *testing.T) {
	home := t.TempDir()
	id, keep := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	cli := ".gemini/antigravity-cli/"
	writeAll(t, home, map[string]string{
		cli + "conversations/" + id + ".db":     "db",
		cli + "conversations/" + id + ".db-wal": "wal",
		cli + "brain/" + id + "/task.md":        "# Fix login\n- [ ] x",
		cli + "annotations/" + id + ".pbtxt":    `title:"Login \"bug\" fix"` + "\n",
		cli + "conversations/" + keep + ".db":   "db",
		cli + "brain/" + keep + "/task.md":      "# Keep me\n",
		cli + "history.jsonl": `{"display":"/model","timestamp":1787883076400,"workspace":"/w","conversationId":"` + id + `"}` + "\n" +
			`{"display":"로그인 고쳐줘","timestamp":1787883076500,"workspace":"/w","conversationId":"` + id + `"}` + "\n" +
			`{"display":"남김","timestamp":1787883076600,"workspace":"/k","conversationId":"` + keep + `"}` + "\n",
	})
	items := map[string]cleanerSession{}
	for _, s := range cleanerScan("antigravity", home, func(string) string { return "" }, config{}, nil) {
		items[s.SID] = s
	}
	s := items[id]
	if s.Title != `Login "bug" fix` || s.CWD != "/w" || len(s.Files) != 4 || s.Source != "cli" {
		t.Fatalf("scan: %+v", s)
	}
	if items[keep].Title != "Keep me" || items[keep].Prompt != "남김" {
		t.Fatalf("fallback title: %+v", items[keep])
	}
	if s.Resume[0] != "agy" || s.Resume[2] != id {
		t.Fatalf("resume: %v", s.Resume)
	}
	trash := filepath.Join(home, "Trash")
	if errs := cleanerDelete([]cleanerSession{s}, false, home, fakeTrash(t, trash)); len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, p := range []string{"conversations/" + id + ".db", "brain/" + id, "annotations/" + id + ".pbtxt"} {
		if _, err := os.Lstat(filepath.Join(home, cli, p)); err == nil {
			t.Errorf("left behind: %s", p)
		}
	}
	if d, _ := os.ReadFile(filepath.Join(home, cli, "history.jsonl")); strings.Contains(string(d), id) || !strings.Contains(string(d), keep) {
		t.Errorf("history: %s", d)
	}
	staged, _ := os.ReadDir(trash)
	if len(staged) != 1 {
		t.Fatalf("trash: %v", staged)
	}
	if _, err := os.Stat(filepath.Join(trash, staged[0].Name(), "antigravity-cli", "history-removed.jsonl")); err != nil {
		t.Error("history copy missing from trash")
	}
}

func TestCleanerOpencode(t *testing.T) {
	if sqliteBin() == "" {
		t.Skip("sqlite3 not installed")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".local/share/opencode")
	if err := os.MkdirAll(filepath.Join(dir, "storage/session_diff"), 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "opencode.db")
	err := sqliteExec(db,
		"CREATE TABLE session (id text PRIMARY KEY, parent_id text, directory text NOT NULL, title text NOT NULL, time_created integer NOT NULL, time_updated integer NOT NULL)",
		"CREATE TABLE message (id text PRIMARY KEY, session_id text NOT NULL, time_created integer NOT NULL, data text NOT NULL, FOREIGN KEY (session_id) REFERENCES session(id) ON DELETE CASCADE)",
		"CREATE TABLE part (id text PRIMARY KEY, message_id text NOT NULL, data text NOT NULL, FOREIGN KEY (message_id) REFERENCES message(id) ON DELETE CASCADE)",
		"CREATE TABLE todo (session_id text NOT NULL, content text, FOREIGN KEY (session_id) REFERENCES session(id) ON DELETE CASCADE)",
		`INSERT INTO session VALUES ('ses_parent01', NULL, '/w', 'Fix login', 1771917455495, 1772071922120),
			('ses_child001', 'ses_parent01', '/w', 'subagent', 1771917455495, 1771917455495),
			('ses_keep0001', NULL, '/k', 'Keep', 1771917455495, 1771917455495)`,
		`INSERT INTO message VALUES ('msg_1', 'ses_parent01', 1, '{"role":"user"}'), ('msg_2', 'ses_child001', 2, '{"role":"user"}'), ('msg_3', 'ses_keep0001', 3, '{"role":"user"}')`,
		`INSERT INTO part VALUES ('prt_1', 'msg_1', '{"type":"text","text":"로그인 고쳐줘"}'), ('prt_2', 'msg_2', '{"type":"text","text":"x"}'), ('prt_3', 'msg_3', '{"type":"text","text":"k"}')`,
		"INSERT INTO todo VALUES ('ses_parent01', 'a')",
	)
	if err != nil {
		t.Fatal(err)
	}
	writeAll(t, home, map[string]string{".local/share/opencode/storage/session_diff/ses_child001.json": "[]"})
	items := map[string]cleanerSession{}
	for _, s := range cleanerScan("opencode", home, func(string) string { return "" }, config{}, nil) {
		items[s.SID] = s
	}
	s := items["ses_parent01"]
	if len(items) != 2 || s.Title != "Fix login" || s.Prompt != "로그인 고쳐줘" || s.CWD != "/w" || len(s.Files) != 1 {
		t.Fatalf("scan: %+v", items)
	}
	trash := filepath.Join(home, "Trash")
	if errs := cleanerDelete([]cleanerSession{s}, false, home, fakeTrash(t, trash)); len(errs) > 0 {
		t.Fatal(errs)
	}
	var left []struct {
		N int `json:"n"`
	}
	if err := sqliteRows(db, "SELECT (SELECT COUNT(*) FROM session) + (SELECT COUNT(*) FROM message) + (SELECT COUNT(*) FROM part) + (SELECT COUNT(*) FROM todo) AS n", &left); err != nil || left[0].N != 3 {
		t.Fatalf("rows left: %v %v (want only ses_keep0001's 3)", left, err)
	}
	staged, _ := os.ReadDir(trash)
	export, err := os.ReadFile(filepath.Join(trash, staged[0].Name(), "opencode", "sessions-removed.json"))
	if err != nil || !strings.Contains(string(export), "ses_child001") || !strings.Contains(string(export), "prt_1") {
		t.Fatalf("export: %s %v", export, err)
	}
}

func scanMap(t *testing.T, agent, home string, env map[string]string) map[string]cleanerSession {
	t.Helper()
	out := map[string]cleanerSession{}
	for _, s := range cleanerScan(agent, home, func(k string) string { return env[k] }, config{}, nil) {
		out[s.SID] = s
	}
	return out
}

func TestCleanerCodex(t *testing.T) {
	home := t.TempDir()
	id, old := "019a0000-0000-7000-8000-000000000001", "019a0000-0000-7000-8000-000000000002"
	day := ".codex/sessions/2026/10/22/"
	writeAll(t, home, map[string]string{
		day + "rollout-2026-10-22T10-00-00-" + id + ".jsonl": `{"timestamp":"2026-10-22T01:00:00Z","type":"session_meta","payload":{"id":"` + id + `","cwd":"/w","source":"vscode","timestamp":"2026-10-22T01:00:00Z"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>x</environment_context>"}]}}
{"type":"event_msg","payload":{"type":"user_message","message":"## My request for Codex:\n로그인 고쳐줘"}}
`,
		day + "rollout-2026-10-22T11-00-00-" + id + "_r2.jsonl": `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/other"}}`,
		".codex/archived_sessions/rollout-2026-01-01T00-00-00-" + old + ".jsonl": `{"type":"session_meta","payload":{"id":"` + old + `","cwd":"/k","source":{"subagent":{"x":1}}}}
{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"text","text":"남김"}]}}}
`,
		".codex/session_index.jsonl":               `{"id":"` + id + `","thread_name":"Login fix","updated_at":"x"}` + "\n" + `{"id":"` + id + `","thread_name":"","updated_at":"y"}` + "\n",
		".codex/history.jsonl":                     `{"session_id":"` + id + `","ts":1,"text":"a"}` + "\n" + `{"session_id":"` + old + `","ts":2,"text":"b"}` + "\n",
		".codex/shell_snapshots/" + id + ".123.sh": "x",
	})
	items := scanMap(t, "codex", home, nil)
	s := items[id]
	if s.Title != "Login fix" || s.Prompt != "로그인 고쳐줘" || s.CWD != "/w" || s.Source != "vscode" || len(s.Files) != 3 {
		t.Fatalf("scan: %+v", s)
	}
	if o := items[old]; o.Title != "남김" || o.Source != "subagent" {
		t.Fatalf("archived: %+v", o)
	}
	if strings.Join(s.Resume, " ") != "codex resume "+id {
		t.Fatalf("resume: %v", s.Resume)
	}
	if errs := cleanerDelete([]cleanerSession{s}, true, home, nil); len(errs) > 0 {
		t.Fatal(errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex/sessions/2026")); err == nil {
		t.Error("empty date folders left behind")
	}
	if d, _ := os.ReadFile(filepath.Join(home, ".codex/history.jsonl")); strings.Contains(string(d), id) || !strings.Contains(string(d), old) {
		t.Errorf("history: %s", d)
	}
	if d, _ := os.ReadFile(filepath.Join(home, ".codex/session_index.jsonl")); strings.Contains(string(d), id) {
		t.Errorf("session_index: %s", d)
	}
}

func TestCleanerGemini(t *testing.T) {
	home := t.TempDir()
	id, legacy := "a1b2c3d4-0000-4000-8000-000000000001", "a1b2c3d4-0000-4000-8000-000000000002"
	p := ".gemini/tmp/myproj/"
	writeAll(t, home, map[string]string{
		p + ".project_root": "/w/myproj\n",
		p + "chats/session-2026-10-22T01-00-a1b2c3d4.jsonl": `{"sessionId":"` + id + `","projectHash":"h","startTime":"2026-10-22T01:00:00Z","lastUpdated":"x"}
{"id":"m1","timestamp":"t","type":"user","content":"/help"}
{"id":"m2","timestamp":"t","type":"user","content":[{"text":"테스트 고쳐줘"}]}
{"$set":{"summary":"Old"}}
{"$set":{"summary":"Fix tests"}}
`,
		p + "chats/session-2026-10-20T01-00-a1b2c3d4.json": `{"sessionId":"` + legacy + `","startTime":"2026-10-20T01:00:00Z","messages":[{"type":"user","content":"옛날"}]}`,
		p + "chats/" + id + "/sub.jsonl":                   "{}",
		p + "logs/session-" + id + ".jsonl":                "{}",
		p + "tool-outputs/session-" + id + "/a.txt":        "x",
		p + "logs.json": `[{"sessionId":"` + id + `","messageId":0,"type":"user","message":"a"},{"sessionId":"` + legacy + `","messageId":0,"type":"user","message":"b"}]`,
	})
	items := scanMap(t, "gemini", home, nil)
	s := items[id]
	if s.Title != "Fix tests" || s.Prompt != "테스트 고쳐줘" || s.CWD != "/w/myproj" || len(s.Files) != 4 {
		t.Fatalf("scan: %+v", s)
	}
	if items[legacy].Title != "옛날" {
		t.Fatalf("legacy: %+v", items[legacy])
	}
	trash := filepath.Join(home, "Trash")
	if errs := cleanerDelete([]cleanerSession{s}, false, home, fakeTrash(t, trash)); len(errs) > 0 {
		t.Fatal(errs)
	}
	if d, _ := os.ReadFile(filepath.Join(home, p, "logs.json")); strings.Contains(string(d), id) || !strings.Contains(string(d), legacy) {
		t.Errorf("logs.json: %s", d)
	}
	staged, _ := os.ReadDir(trash)
	if _, err := os.Stat(filepath.Join(trash, staged[0].Name(), ".gemini/tmp/myproj/logs-removed.json")); err != nil {
		t.Error("logs copy missing from trash")
	}
	if _, err := os.Stat(filepath.Join(trash, staged[0].Name(), ".gemini/tmp/myproj/chats", id, "sub.jsonl")); err != nil {
		t.Error("subagent chat missing from trash")
	}
}

func TestCleanerOpencodeV2(t *testing.T) {
	if sqliteBin() == "" {
		t.Skip("sqlite3 not installed")
	}
	home := t.TempDir()
	db := filepath.Join(home, "oc.db")
	err := sqliteExec(db,
		"CREATE TABLE session_v2 (id text PRIMARY KEY, parent_id text, directory text NOT NULL, slug text NOT NULL, title text, time_created integer, time_updated integer)",
		"CREATE TABLE session_message (id text PRIMARY KEY, session_id text NOT NULL, type text, seq integer, data text, FOREIGN KEY (session_id) REFERENCES session_v2(id) ON DELETE CASCADE)",
		"CREATE TABLE event (id text PRIMARY KEY, aggregate_id text)",
		"CREATE TABLE event_sequence (aggregate_id text PRIMARY KEY, seq integer)",
		`INSERT INTO session_v2 VALUES ('ses_parent01', NULL, '/w', 'quiet-river', NULL, 1, 1771917455495),
			('ses_child001', 'ses_parent01', '/w', 'sub', NULL, 1, 1), ('ses_grand001', 'ses_child001', '/w', 'subsub', NULL, 1, 1),
			('ses_keep0001', NULL, '/k', 'keep', 'Keep', 1, 1)`,
		`INSERT INTO session_message VALUES ('m1', 'ses_parent01', 'user', 1, '{"text":"로그인"}'), ('m2', 'ses_grand001', 'user', 1, '{"text":"x"}'), ('m3', 'ses_keep0001', 'user', 1, '{"text":"k"}')`,
		"INSERT INTO event VALUES ('e1', 'ses_parent01'), ('e2', 'ses_grand001'), ('e3', 'ses_keep0001')",
		"INSERT INTO event_sequence VALUES ('ses_parent01', 1), ('ses_keep0001', 1)",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", db)
	items := scanMap(t, "opencode", home, map[string]string{"OPENCODE_DB": db})
	s := items["ses_parent01"]
	if len(items) != 2 || s.Title != "quiet-river" || s.Prompt != "로그인" {
		t.Fatalf("scan: %+v", items)
	}
	if errs := cleanerDelete([]cleanerSession{s}, true, home, nil); len(errs) > 0 {
		t.Fatal(errs)
	}
	var left []struct {
		N int `json:"n"`
	}
	q := "SELECT (SELECT COUNT(*) FROM session_v2) + (SELECT COUNT(*) FROM session_message) + (SELECT COUNT(*) FROM event) + (SELECT COUNT(*) FROM event_sequence) AS n"
	if err := sqliteRows(db, q, &left); err != nil || left[0].N != 4 {
		t.Fatalf("rows left: %v %v (want only ses_keep0001's 4)", left, err)
	}
}

func TestCleanerCursor(t *testing.T) {
	if sqliteBin() == "" {
		t.Skip("sqlite3 not installed")
	}
	home := t.TempDir()
	sum := md5.Sum([]byte(home))
	dir := filepath.Join(home, ".cursor/chats", hex.EncodeToString(sum[:]), "c0ffee00-0000-4000-8000-000000000001")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := hex.EncodeToString([]byte(`{"agentId":"a","name":"Refactor auth","createdAt":1771917455495,"latestRootBlobId":"ab"}`))
	if err := sqliteExec(filepath.Join(dir, "store.db"), "CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)", "INSERT INTO meta VALUES ('0', '"+meta+"')"); err != nil {
		t.Fatal(err)
	}
	items := scanMap(t, "cursor", home, nil)
	s := items["c0ffee00-0000-4000-8000-000000000001"]
	if s.Title != "Refactor auth" || s.CWD != home || s.Empty || s.Resume[0] != "cursor-agent" {
		t.Fatalf("scan: %+v", s)
	}
	if errs := cleanerDelete([]cleanerSession{s}, true, home, nil); len(errs) > 0 {
		t.Fatal(errs)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("chat folder left behind")
	}
}

// renameCheck renames sid through its agent and checks the next scan shows
// the title while the session's mtime holds still.
func renameCheck(t *testing.T, agent, home, sid string) cleanerSession {
	t.Helper()
	s := scanMap(t, agent, home, nil)[sid]
	const title = `새 '이름' "q"`
	if err := cleanerAgents[agent].rename(s, title); err != nil {
		t.Fatal(err)
	}
	got := scanMap(t, agent, home, nil)[sid]
	if got.Title != title || !got.Mtime.Equal(s.Mtime) {
		t.Fatalf("after rename: %q %v (mtime was %v)", got.Title, got.Mtime, s.Mtime)
	}
	return got
}

func TestCleanerRenameDetail(t *testing.T) {
	fixed := []cleanerTurn{{User: "고쳐줘", Assistant: "고쳤어요"}}
	t.Run("claude", func(t *testing.T) {
		home := t.TempDir()
		sid := "11111111-1111-1111-1111-111111111111"
		lines := []string{`{"type":"user","cwd":"/w","gitBranch":"main","message":{"content":"하나"}}`,
			`{"type":"assistant","message":{"model":"claude-a","content":[{"type":"text","text":"하나 답"}]}}`}
		for _, q := range []string{"둘", "셋", "넷"} {
			lines = append(lines,
				`{"type":"user","message":{"content":"`+q+`"}}`,
				`{"type":"assistant","message":{"model":"claude-b","content":[{"type":"text","text":"보는 중"},{"type":"tool_use","id":"x"}]}}`,
				`{"type":"user","message":{"content":[{"type":"tool_result","content":"out"}]}}`,
				`{"type":"user","isSidechain":true,"message":{"content":"sub"}}`,
				`{"type":"user","isMeta":true,"message":{"content":"meta"}}`,
				`{"type":"user","message":{"content":"<local-command-stdout>x</local-command-stdout>"}}`,
				`{"type":"assistant","gitBranch":"dev","message":{"model":"claude-b","content":[{"type":"text","text":"`+q+` 답"}]}}`)
		}
		writeAll(t, home, map[string]string{
			".claude/projects/-w/" + sid + ".jsonl":             strings.Join(lines, "\n") + "\n",
			".claude/projects/-w/" + sid + "/custom-title.json": `{"customTitle":"옛"}`,
		})
		s := renameCheck(t, "claude", home, sid)
		if d, _ := os.ReadFile(filepath.Join(home, ".claude/projects/-w", sid, "custom-title.json")); !strings.Contains(string(d), "이름") {
			t.Errorf("custom-title.json: %s", d)
		}
		d := claudeDetail(s)
		want := []cleanerTurn{{User: "둘", Assistant: "둘 답"}, {User: "셋", Assistant: "셋 답"}, {User: "넷", Assistant: "넷 답"}}
		if !reflect.DeepEqual(d.Turns, want) || d.Model != "claude-b" || d.Branch != "dev" {
			t.Fatalf("detail: %+v", d)
		}
	})
	t.Run("codex", func(t *testing.T) {
		home := t.TempDir()
		id := "019a0000-0000-7000-8000-000000000001"
		writeAll(t, home, map[string]string{
			".codex/sessions/2026/10/22/rollout-2026-10-22T10-00-00-" + id + ".jsonl": `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/w","git":{"branch":"feat"}}}
{"type":"turn_context","payload":{"model":"gpt-x"}}
{"type":"event_msg","payload":{"type":"user_message","message":"## My request for Codex:\n고쳐줘"}}
{"type":"event_msg","payload":{"type":"agent_message","message":"보는 중"}}
{"type":"event_msg","payload":{"type":"agent_message","message":"고쳤어요"}}
`,
			".codex/session_index.jsonl": `{"id":"` + id + `","thread_name":"옛","updated_at":"x"}`, // no final newline
		})
		d := codexDetail(renameCheck(t, "codex", home, id))
		if !reflect.DeepEqual(d.Turns, fixed) || d.Model != "gpt-x" || d.Branch != "feat" {
			t.Fatalf("detail: %+v", d)
		}
	})
	t.Run("gemini", func(t *testing.T) {
		home := t.TempDir()
		id := "a1b2c3d4-0000-4000-8000-000000000001"
		writeAll(t, home, map[string]string{
			".gemini/tmp/p/.project_root": "/w\n",
			".gemini/tmp/p/chats/session-2026-10-22T01-00-a1b2c3d4.jsonl": `{"sessionId":"` + id + `","startTime":"t"}
{"id":"m1","type":"user","content":[{"text":"고쳐줘"}]}
{"id":"m2","type":"gemini","content":"보는 중","model":"gemini-x"}
{"id":"m2","type":"gemini","content":[{"text":"고쳤어요"}],"model":"gemini-x"}
{"$set":{"summary":"옛"}}
`,
		})
		d := geminiDetail(renameCheck(t, "gemini", home, id))
		if !reflect.DeepEqual(d.Turns, fixed) || d.Model != "gemini-x" {
			t.Fatalf("detail: %+v", d)
		}
	})
	t.Run("antigravity", func(t *testing.T) {
		home := t.TempDir()
		id, bare := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
		cli := ".gemini/antigravity-cli/"
		writeAll(t, home, map[string]string{
			cli + "conversations/" + id + ".db":   "db",
			cli + "annotations/" + id + ".pbtxt":  "title:\"옛\"\nlast_user_view_time:{seconds:1}\n",
			cli + "conversations/" + bare + ".db": "db",
			cli + "brain/" + id + "/.system_generated/logs/transcript.jsonl": `{"source":"USER_EXPLICIT","type":"USER_INPUT","content":"<USER_REQUEST>\n고쳐줘\n</USER_REQUEST>\n<ADDITIONAL_METADATA>x</ADDITIONAL_METADATA>"}
{"source":"MODEL","type":"PLANNER_RESPONSE","content":"보는 중"}
{"source":"MODEL","type":"PLANNER_RESPONSE"}
{"source":"MODEL","type":"PLANNER_RESPONSE","content":"고쳤어요"}
`,
		})
		s := renameCheck(t, "antigravity", home, id)
		renameCheck(t, "antigravity", home, bare) // no annotation yet
		if d, _ := os.ReadFile(filepath.Join(home, cli, "annotations", id+".pbtxt")); !strings.Contains(string(d), "last_user_view_time") {
			t.Errorf("annotation lost its other fields: %s", d)
		}
		if d := agyDetail(s); !reflect.DeepEqual(d.Turns, fixed) {
			t.Fatalf("detail: %+v", d)
		}
	})
	t.Run("opencode", func(t *testing.T) {
		if sqliteBin() == "" {
			t.Skip("sqlite3 not installed")
		}
		home := t.TempDir()
		dir := filepath.Join(home, ".local/share/opencode")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		err := sqliteExec(filepath.Join(dir, "opencode.db"),
			"CREATE TABLE session (id text PRIMARY KEY, parent_id text, directory text NOT NULL, title text NOT NULL, time_created integer NOT NULL, time_updated integer NOT NULL)",
			"CREATE TABLE message (id text PRIMARY KEY, session_id text NOT NULL, time_created integer NOT NULL, data text NOT NULL)",
			"CREATE TABLE part (id text PRIMARY KEY, message_id text NOT NULL, data text NOT NULL)",
			"INSERT INTO session VALUES ('ses_a0000001', NULL, '/w', 'Old', 1, 1)",
			`INSERT INTO message VALUES ('msg_1', 'ses_a0000001', 1, '{"role":"user"}'), ('msg_2', 'ses_a0000001', 2, '{"role":"assistant","modelID":"big"}')`,
			`INSERT INTO part VALUES ('prt_1', 'msg_1', '{"type":"text","text":"고쳐줘"}'), ('prt_2', 'msg_1', '{"type":"text","text":"ctx","synthetic":true}'),
				('prt_3', 'msg_2', '{"type":"reasoning","text":"음"}'), ('prt_4', 'msg_2', '{"type":"text","text":"고쳤어요"}')`,
		)
		if err != nil {
			t.Fatal(err)
		}
		d := opencodeDetail(renameCheck(t, "opencode", home, "ses_a0000001"))
		if !reflect.DeepEqual(d.Turns, fixed) || d.Model != "big" {
			t.Fatalf("detail: %+v", d)
		}
	})
}
