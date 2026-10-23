package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Session cleaner: one window listing every session an agent left on disk,
// with filters, protect/exclude, resume, and delete — a session goes
// together with everything hanging off it (side folders, prompt-history
// lines, index rows). Each agent plugs in a scanner, a purge for its
// shared index files, and a resume command (cleaner_<agent>.go); deleting,
// staging for the trash, and the protections live here.
//
// The routes live on the webview asset handler only, never on the
// localhost hook port, so nothing outside the app's own windows can delete.

type cleanerFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Dir  bool   `json:"dir"`
}

type cleanerSession struct {
	ID       string        `json:"id"` // unique per agent: transcript path, db id, "orphan:<root>:<sid>"
	Agent    string        `json:"agent"`
	SID      string        `json:"sid"` // the agent's own session id
	Root     string        `json:"root"`
	Title    string        `json:"title"` // "" = empty session / orphan; the page labels it
	Prompt   string        `json:"prompt"`
	CWD      string        `json:"cwd"`
	Project  string        `json:"project"` // cwd, else whatever names the project; "" for orphans
	Source   string        `json:"source"`  // cli | vscode | desktop | sdk | raw origin | ""
	Started  string        `json:"started"` // first entry's timestamp, may be ""
	Mtime    time.Time     `json:"mtime"`
	Files    []cleanerFile `json:"files"`
	Size     int64         `json:"size"`
	Running  bool          `json:"running"`
	Empty    bool          `json:"empty"`
	Orphan   bool          `json:"orphan"`
	Pinned   bool          `json:"pinned"`
	Excluded string        `json:"excluded,omitempty"` // exclusion folder covering this session
	Resume   []string      `json:"resume,omitempty"`   // argv that reopens it in a terminal

	// the scanner told running from not (pid file, writer lock), so the
	// hook-based guess isn't needed
	liveKnown bool
}

// cleanerAgent is what differs per agent.
type cleanerAgent struct {
	scan func(home string, env func(string) string) []cleanerSession
	// purge runs after the files moved: it drops the sessions from shared
	// index files and returns what it took out (staging-relative path →
	// bytes) so the trash copy can restore it
	purge  func(items []cleanerSession) (map[string][]byte, []string)
	resume func(s cleanerSession) []string // nil when it can't be resumed
	// rename stores a title where the agent itself keeps it, so its own
	// session picker shows it too; nil when it can't
	rename func(s cleanerSession, title string) error
	// detail reads what the inspector shows on demand: the last few
	// exchanges, model, branch; nil when it can't
	detail func(s cleanerSession) cleanerDetail
	// unavailable explains (i18n key) why the agent can't be listed here
	unavailable func() string
}

var cleanerAgents = map[string]cleanerAgent{
	"claude": {scan: scanClaude, purge: purgeClaude, rename: renameClaude, detail: claudeDetail,
		resume: func(s cleanerSession) []string { return []string{"claude", "--resume", s.SID} }},
	"antigravity": {scan: scanAntigravity, purge: purgeAntigravity, rename: renameAntigravity, detail: agyDetail,
		resume: func(s cleanerSession) []string {
			if s.Source != "cli" {
				return nil // IDE conversations reopen in the IDE
			}
			return []string{"agy", "--conversation", s.SID}
		}},
	"codex": {scan: scanCodex, purge: purgeCodex, rename: renameCodex, detail: codexDetail,
		resume: func(s cleanerSession) []string { return []string{"codex", "resume", s.SID} }},
	// Gemini looks the id up within the project it's started in (cwd)
	"gemini": {scan: scanGemini, purge: purgeGemini, rename: renameGemini, detail: geminiDetail,
		resume: func(s cleanerSession) []string { return []string{"gemini", "--resume", s.SID} }},
	"cursor": {scan: scanCursor,
		resume: func(s cleanerSession) []string { return []string{"cursor-agent", "--resume", s.SID} }},
	"opencode": {scan: scanOpencode, purge: purgeOpencode, rename: renameOpencode, detail: opencodeDetail,
		resume: func(s cleanerSession) []string { return []string{"opencode", "--session", s.SID} },
		unavailable: func() string {
			home, _ := os.UserHomeDir()
			return needSqlite(opencodeDB(home, os.Getenv))
		}},
}

var (
	cleanerUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	// ids that may go on a terminal command line unquoted
	cleanerID = regexp.MustCompile(`^[A-Za-z0-9_-]{6,128}$`)
)

func (s *cleanerSession) addFiles(paths ...string) {
	for _, p := range paths {
		f, mod := cleanerEntry(p)
		s.Files = append(s.Files, f)
		s.Size += f.Size
		if mod.After(s.Mtime) {
			s.Mtime = mod
		}
	}
}

func cleanerEntry(p string) (cleanerFile, time.Time) {
	st, err := os.Lstat(p)
	if err != nil {
		return cleanerFile{Path: p}, time.Time{}
	}
	f := cleanerFile{Path: p, Size: st.Size(), Dir: st.IsDir()}
	if f.Dir {
		f.Size = 0
		_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				if i, err := d.Info(); err == nil {
					f.Size += i.Size()
				}
			}
			return nil
		})
	}
	return f, st.ModTime()
}

// jsonStr pulls the string value of "key":"…" straight out of compact
// JSONL bytes — a byte search instead of decoding every line.
func jsonStr(d []byte, key string, last bool) string {
	pat := []byte(`"` + key + `":"`)
	i := bytes.Index(d, pat)
	if last {
		i = bytes.LastIndex(d, pat)
	}
	if i < 0 {
		return ""
	}
	start := i + len(pat) - 1 // the opening quote
	j := start + 1
	for j < len(d) && d[j] != '"' {
		if d[j] == '\\' {
			j++
		}
		j++
	}
	if j >= len(d) {
		return ""
	}
	var s string
	if json.Unmarshal(d[start:j+1], &s) != nil {
		return ""
	}
	return s
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

type transcriptMeta struct {
	id, title, prompt, cwd, source, started string
	empty                                   bool // the agent marks it as never used
}

// Metadata cache, per agent: 800 Claude sessions are ~700MB of JSONL, so
// only files whose size or mtime moved get re-parsed.
type metaCacheEntry struct {
	size int64
	mod  time.Time
	meta transcriptMeta
}

var cleaner struct {
	mu    sync.Mutex
	cache map[string]map[string]metaCacheEntry // agent → path → entry
	next  map[string]metaCacheEntry            // filled by the scan in progress
	cur   string                               // agent being scanned
	last  map[string]cleanerSession            // last scan by id, for resume/reveal

	winMu sync.Mutex
	win   application.Window
}

// cleanerMeta returns path's parsed metadata from the cache, re-parsing
// only when it changed. Scanners call it while cleanerScan holds mu.
func cleanerMeta(path string, info fs.FileInfo, parse func(string) transcriptMeta) transcriptMeta {
	c, ok := cleaner.cache[cleaner.cur][path]
	if !ok || c.size != info.Size() || !c.mod.Equal(info.ModTime()) {
		c = metaCacheEntry{size: info.Size(), mod: info.ModTime(), meta: parse(path)}
	}
	cleaner.next[path] = c
	return c.meta
}

// cleanerScan lists an agent's sessions with protections applied. live
// holds session ids the daemon's hooks saw active lately.
func cleanerScan(agent, home string, env func(string) string, cfg config, live map[string]bool) []cleanerSession {
	a, ok := cleanerAgents[agent]
	if !ok || (a.unavailable != nil && a.unavailable() != "") {
		return []cleanerSession{}
	}
	cleaner.mu.Lock()
	defer cleaner.mu.Unlock()
	if cleaner.cache == nil {
		cleaner.cache = map[string]map[string]metaCacheEntry{}
	}
	cleaner.cur, cleaner.next = agent, map[string]metaCacheEntry{}
	items := a.scan(home, env)
	cleaner.cache[agent] = cleaner.next
	pinned := map[string]bool{}
	for _, id := range cfg.CleanerPinned {
		pinned[id] = true
	}
	cleaner.last = map[string]cleanerSession{}
	for i := range items {
		it := &items[i]
		it.Agent = agent
		it.Prompt = clip(it.Prompt, 500)
		// without a pid file or lock to ask, what the hooks saw lately;
		// and for everyone, a transcript written in the last two minutes
		if (!it.liveKnown && live[it.SID]) || time.Since(it.Mtime) < 2*time.Minute {
			it.Running = true
		}
		it.Pinned = pinned[it.SID]
		it.Excluded = excludingFolder(cfg.CleanerExcluded, it.Project)
		if a.resume != nil && !it.Orphan && it.CWD != "" && cleanerID.MatchString(it.SID) {
			it.Resume = a.resume(*it)
		}
		cleaner.last[it.ID] = *it
	}
	return items
}

// excludingFolder returns the exclusion folder covering p (p itself or a
// parent of it), or "".
func excludingFolder(excluded []string, p string) string {
	if p == "" {
		return ""
	}
	for _, e := range excluded {
		if pathsEqual(e, p) || pathContains(e, p) {
			return e
		}
	}
	return ""
}

func (s cleanerSession) deletable() bool { return !s.Running && !s.Pinned && s.Excluded == "" }

// cleanerDelete removes the sessions' files and index entries and returns
// what failed. Unless permanent, everything is first gathered into one
// staging folder that mirrors the original layout, and that folder goes to
// the trash as a single restorable item.
func cleanerDelete(items []cleanerSession, permanent bool, home string, trash func(string) error) []string {
	staging := filepath.Join(home, ".claude-notify", "cleaner-staging",
		"sessions-"+time.Now().Format("20060102-150405"))
	var errs []string
	fail := func(p string, err error) { errs = append(errs, p+": "+err.Error()) }
	byAgent := map[string][]cleanerSession{}
	for _, it := range items {
		for _, f := range it.Files {
			var err error
			if permanent {
				err = os.RemoveAll(f.Path)
			} else if rel, rerr := filepath.Rel(it.Root, f.Path); rerr != nil {
				err = rerr
			} else {
				dest := filepath.Join(staging, filepath.Base(it.Root), rel)
				if err = os.MkdirAll(filepath.Dir(dest), 0o700); err == nil {
					if err = os.Rename(f.Path, dest); err != nil && !errors.Is(err, fs.ErrNotExist) {
						err = trash(f.Path) // other volume (a symlinked root): trash it on its own
					}
				}
			}
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				fail(f.Path, err)
			}
		}
		byAgent[it.Agent] = append(byAgent[it.Agent], it)
	}
	for agent, list := range byAgent {
		purge := cleanerAgents[agent].purge
		if purge == nil {
			continue
		}
		saved, perrs := purge(list)
		errs = append(errs, perrs...)
		if permanent {
			continue
		}
		for rel, data := range saved {
			dest := filepath.Join(staging, rel)
			err := os.MkdirAll(filepath.Dir(dest), 0o700)
			if err == nil {
				err = os.WriteFile(dest, data, 0o600)
			}
			if err != nil {
				fail(dest, err)
			}
		}
	}
	if _, err := os.Stat(staging); err == nil {
		if err := trash(staging); err != nil {
			errs = append(errs, fmt.Sprintf(T("cl.err.trash"), staging, err))
		}
		_ = os.Remove(filepath.Dir(staging))
	}
	return errs
}

// stripJSONLines rewrites a JSONL file without the lines whose key holds
// one of ids, keeping its permissions (history files are 0600), and
// returns the removed lines.
// ponytail: a session appending between the read and the rename loses
// that one line; take a file lock if it ever shows up in practice.
func stripJSONLines(path, key string, ids map[string]bool) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var keep, removed bytes.Buffer
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var e map[string]json.RawMessage
		var id string
		buf := &keep
		if json.Unmarshal(line, &e) == nil && json.Unmarshal(e[key], &id) == nil && ids[id] {
			buf = &removed
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if removed.Len() == 0 {
		return nil, nil
	}
	return removed.Bytes(), replaceFile(path, keep.Bytes())
}

// cleanerDetail is what one session's files say beyond the list row.
type cleanerDetail struct {
	Turns  []cleanerTurn `json:"turns"`
	Model  string        `json:"model,omitempty"`  // the latest one
	Branch string        `json:"branch,omitempty"` // git branch it ran on
}

// cleanerTurn is one exchange: a prompt and the agent's final reply to it.
type cleanerTurn struct {
	User      string `json:"user"`
	Assistant string `json:"assistant"`
}

// turnLog folds a session's messages, oldest first, into turns.
type turnLog []cleanerTurn

func (l *turnLog) user(t string) {
	if t = strings.TrimSpace(t); t != "" {
		*l = append(*l, cleanerTurn{User: clip(t, 2000)})
	}
}

// reply keeps the turn's latest reply: the last one answers the prompt,
// the ones before it narrate the work. A reply before any prompt (a tail
// that starts mid-turn) is dropped.
func (l *turnLog) reply(t string) {
	if t = strings.TrimSpace(t); t != "" && len(*l) > 0 {
		(*l)[len(*l)-1].Assistant = clip(t, 4000)
	}
}

func (l turnLog) last() []cleanerTurn { return l[max(0, len(l)-3):] }

// appendJSONLine adds v to a JSONL file as one line, first ending the
// previous line if the file doesn't.
func appendJSONLine(path string, v any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > 0 {
		end := make([]byte, 1)
		if _, err := f.ReadAt(end, st.Size()-1); err == nil && end[0] != '\n' {
			line = append([]byte("\n"), line...)
		}
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// keepMtime runs write, then puts path's mtime back (mod when the file is
// new): a rename isn't activity, and a fresh mtime would sort the session
// to the top and read as running for two minutes.
func keepMtime(path string, mod time.Time, write func() error) error {
	if st, err := os.Stat(path); err == nil {
		mod = st.ModTime()
	}
	if err := write(); err != nil {
		return err
	}
	return os.Chtimes(path, time.Now(), mod)
}

// replaceFile swaps in new content atomically, keeping the file's mode.
func replaceFile(path string, data []byte) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp := path + ".agent-notify-tmp"
	if err := os.WriteFile(tmp, data, st.Mode().Perm()); err != nil {
		return err
	}
	_ = os.Chmod(tmp, st.Mode().Perm()) // WriteFile's mode is masked by umask
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// cleanerLog appends what was deleted to ~/.claude-notify/cleaner.log, for
// tracing and recovery after an accident.
func cleanerLog(home string, items []cleanerSession, permanent bool) {
	mode := "trash"
	if permanent {
		mode = "permanent"
	}
	var b strings.Builder
	agent := ""
	if len(items) > 0 {
		agent = items[0].Agent
	}
	fmt.Fprintf(&b, "[%s] %s %s %d\n", time.Now().Format(time.RFC3339), agent, mode, len(items))
	for _, it := range items {
		fmt.Fprintf(&b, "  %s  %s  %s\n", it.SID, it.Project, it.Title)
	}
	f, err := os.OpenFile(filepath.Join(home, ".claude-notify", "cleaner.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.WriteString(f, b.String())
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// toggleList removes vals from list, then appends them when on.
func toggleList(list, vals []string, on bool) []string {
	drop := map[string]bool{}
	for _, v := range vals {
		drop[v] = true
	}
	out := []string{}
	for _, v := range list {
		if !drop[v] {
			out = append(out, v)
		}
	}
	if on {
		out = append(out, vals...)
	}
	return out
}

func cleanerTitle(agent string) string {
	name := agent
	if a := agentByID(agent); a != nil {
		name = a.Name
	}
	return strings.ReplaceAll(T("cl.title"), "{agent}", name)
}

// liveSessionIDs: sessions the hooks reported within the last 10 minutes.
// Agents that never send an end event stay "running" that long after
// their last turn.
func (s *daemonState) liveSessionIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]bool{}
	cutoff := time.Now().Add(-10 * time.Minute)
	for id, info := range s.sessions {
		if info.LastSeen.After(cutoff) {
			out[id] = true
		}
	}
	return out
}

// openCleaner shows the session window for agent; an open window switches
// over instead of a second one appearing. tour opens it on the onboarding
// tour's second leg (mock sessions, cleaner-tour.js).
func (s *daemonState) openCleaner(agent string, tour bool) {
	if _, ok := cleanerAgents[agent]; !ok {
		agent = "claude"
	}
	url := "/cleaner.html?agent=" + agent
	if tour {
		url += "&tour=1"
	}
	cleaner.winMu.Lock()
	defer cleaner.winMu.Unlock()
	setDockPresence(true) // a full window should be reachable from Cmd-Tab
	if cleaner.win != nil {
		cleaner.win.SetTitle(cleanerTitle(agent))
		if tour {
			cleaner.win.ExecJS(fmt.Sprintf("location.replace(%q)", url))
		} else {
			cleaner.win.ExecJS(fmt.Sprintf("window.switchAgent && switchAgent(%q)", agent))
		}
		cleaner.win.Show().Focus()
		return
	}
	// native backdrop in the page's color, so the first frame doesn't flash
	bg := application.NewRGB(20, 18, 16)
	if th := loadConfig().Theme; th == "light" || (th != "dark" && !s.app.Env.IsDarkMode()) {
		bg = application.NewRGB(243, 239, 233)
	}
	w := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "agent-notify-sessions",
		Title:            cleanerTitle(agent),
		Width:            1280,
		Height:           760,
		MinWidth:         880,
		MinHeight:        480,
		BackgroundColour: bg,
		URL:              url,
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		cleaner.winMu.Lock()
		cleaner.win = nil
		cleaner.winMu.Unlock()
		setDockPresence(false)
	})
	cleaner.win = w
	w.Show().Focus()
}

// endCleanerTour closes the session window the tour borrowed and brings
// the dashboard back, as if the tour had never left it.
func (s *daemonState) endCleanerTour() {
	cleaner.winMu.Lock()
	w := cleaner.win
	cleaner.winMu.Unlock()
	if w == nil {
		s.showWindow()
		return
	}
	nativeFadeAlpha(w.NativeWindow(), 0, 0.26, true) // macOS only; elsewhere it just closes
	time.AfterFunc(290*time.Millisecond, func() {
		w.Close() // fires WindowClosing, which takes winMu — never call it holding the lock
		s.showWindow()
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *daemonState) cleanerRoutes(mux *http.ServeMux) {
	home, _ := os.UserHomeDir()
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		if json.NewDecoder(r.Body).Decode(v) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return false
		}
		return true
	}
	lookup := func(id string) (cleanerSession, bool) {
		cleaner.mu.Lock()
		defer cleaner.mu.Unlock()
		it, ok := cleaner.last[id]
		return it, ok
	}
	// only folders the scan reported: `open` on an .app "folder" launches it
	knownProject := func(p string) bool {
		cleaner.mu.Lock()
		defer cleaner.mu.Unlock()
		for _, it := range cleaner.last {
			if !it.Orphan && it.Project == p {
				return true
			}
		}
		return false
	}
	// theme baked in like menu.html: the page paints in the right colors
	// before its first fetch
	mux.HandleFunc("/cleaner.html", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(frontendFS, "frontend/cleaner.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, strings.ReplaceAll(string(data), "{{THEME}}", loadConfig().Theme))
	})
	mux.HandleFunc("/api/cleaner/open", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Agent string `json:"agent"`
			Tour  bool   `json:"tour"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		s.openCleaner(req.Agent, req.Tour)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/cleaner/tour-end", func(w http.ResponseWriter, r *http.Request) {
		s.endCleanerTour()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/cleaner/state", func(w http.ResponseWriter, r *http.Request) {
		cfg := loadConfig()
		agent := r.URL.Query().Get("agent")
		a := cleanerAgents[agent]
		notice := ""
		if a.unavailable != nil {
			notice = a.unavailable()
		}
		name := agent
		if a := agentByID(agent); a != nil {
			name = a.Name
		}
		writeJSON(w, map[string]any{
			"agent": agent, "name": name, "notice": notice,
			"can":      map[string]bool{"rename": a.rename != nil, "detail": a.detail != nil},
			"sessions": cleanerScan(agent, home, os.Getenv, cfg, s.liveSessionIDs()),
			"prefs": map[string]any{
				"excluded": cfg.CleanerExcluded, "hideExcluded": cfg.CleanerHideExcluded,
				"oldDays": cfg.cleanerOldDays(), "largeMB": cfg.cleanerLargeMB(),
			},
			"lang": resolveLang(cfg.Lang), "theme": cfg.Theme, "home": home, "os": runtime.GOOS,
		})
	})
	mux.HandleFunc("/api/cleaner/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Agent     string   `json:"agent"`
			IDs       []string `json:"ids"`
			Permanent bool     `json:"permanent"`
		}
		if !decode(w, r, &req) {
			return
		}
		want := map[string]bool{}
		for _, id := range req.IDs {
			want[id] = true
		}
		// decide on a fresh scan: a session may have started since the page loaded
		var targets []cleanerSession
		skipped := 0
		for _, it := range cleanerScan(req.Agent, home, os.Getenv, loadConfig(), s.liveSessionIDs()) {
			switch {
			case !want[it.ID]:
			case !it.deletable():
				skipped++
			default:
				targets = append(targets, it)
			}
		}
		cleanerLog(home, targets, req.Permanent)
		errs := cleanerDelete(targets, req.Permanent, home, moveToTrash)
		writeJSON(w, map[string]any{"deleted": len(targets), "skipped": skipped, "errors": errs})
	})
	mux.HandleFunc("/api/cleaner/prefs", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Pin          []string `json:"pin"`     // session ids
			Exclude      []string `json:"exclude"` // folders
			On           bool     `json:"on"`
			HideExcluded *bool    `json:"hideExcluded"`
			OldDays      *int     `json:"oldDays"`
			LargeMB      *int     `json:"largeMB"`
		}
		if !decode(w, r, &req) {
			return
		}
		c := loadConfig()
		if len(req.Pin) > 0 {
			c.CleanerPinned = toggleList(c.CleanerPinned, req.Pin, req.On)
		}
		if len(req.Exclude) > 0 {
			c.CleanerExcluded = toggleList(c.CleanerExcluded, req.Exclude, req.On)
		}
		if req.HideExcluded != nil {
			c.CleanerHideExcluded = *req.HideExcluded
		}
		if req.OldDays != nil && *req.OldDays >= 1 && *req.OldDays <= 3650 {
			c.CleanerOldDays = *req.OldDays
		}
		if req.LargeMB != nil && *req.LargeMB >= 1 && *req.LargeMB <= 100000 {
			c.CleanerLargeMB = *req.LargeMB
		}
		saveConfig(c)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/cleaner/rename", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Agent string `json:"agent"`
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		if !decode(w, r, &req) {
			return
		}
		rename := cleanerAgents[req.Agent].rename
		title := clip(oneLine(req.Title), 200)
		if rename == nil || title == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// decide on a fresh scan, like delete: the session may be live again
		for _, it := range cleanerScan(req.Agent, home, os.Getenv, loadConfig(), s.liveSessionIDs()) {
			if it.ID != req.ID {
				continue
			}
			// a live agent writes the title it holds back over ours
			if it.Running || it.Orphan {
				http.Error(w, "not renamable", http.StatusConflict)
			} else if err := rename(it, title); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/cleaner/detail", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if !decode(w, r, &req) {
			return
		}
		var d cleanerDetail
		if it, ok := lookup(req.ID); ok && !it.Orphan {
			if read := cleanerAgents[it.Agent].detail; read != nil {
				d = read(it)
			}
		}
		if d.Turns == nil {
			d.Turns = []cleanerTurn{}
		}
		writeJSON(w, d)
	})
	mux.HandleFunc("/api/cleaner/resume", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if !decode(w, r, &req) {
			return
		}
		it, ok := lookup(req.ID)
		if st, err := os.Stat(it.CWD); !ok || it.Running || len(it.Resume) == 0 || err != nil || !st.IsDir() {
			http.Error(w, "not resumable", http.StatusConflict)
			return
		}
		if err := resumeInTerminal(it.CWD, it.Resume); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/cleaner/reveal", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID   string `json:"id"`   // reveal the session's first file
			Path string `json:"path"` // or open a project folder
		}
		if !decode(w, r, &req) {
			return
		}
		if it, ok := lookup(req.ID); ok && len(it.Files) > 0 {
			revealPath(it.Files[0].Path)
		} else if st, err := os.Stat(req.Path); knownProject(req.Path) && err == nil && st.IsDir() {
			openFolder(req.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
