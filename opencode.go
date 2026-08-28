package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// opencode integration: a generated JS plugin in
// ~/.config/opencode/plugins/ subscribes to opencode's event bus and
// POSTs daemon events directly — no Go hook subcommand needed. Beyond
// the codex-level notifications, the plugin also aggregates the local
// opencode.db (opencode runs on Bun, which ships sqlite) and posts
// token/cost usage the window renders on the opencode tab.

// opencodePluginVersion is embedded in the generated file; bumping it
// makes opencodeHooked() report false so the plugin gets rewritten.
const opencodePluginVersion = "v2"

func opencodePluginPath() string {
	return homePath(".config", "opencode", "plugins", "agent-notify.js")
}

func opencodeHooked() bool {
	data, err := os.ReadFile(opencodePluginPath())
	return err == nil && bytes.Contains(data, []byte("agent-notify plugin "+opencodePluginVersion))
}

// upgradeOpencodePlugin silently rewrites an outdated generated plugin.
// The file is ours ("safe to delete"), so overwriting never loses user
// content; a missing file is left alone — installing is the user's call.
func upgradeOpencodePlugin() {
	if fileExists(opencodePluginPath()) && !opencodeHooked() {
		_ = installOpencodeHook()
	}
}

// installOpencodeHook writes the notify plugin. Every handler is
// defensive: a plugin exception must never break the user's session.
func installOpencodeHook() error {
	path := opencodePluginPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	js := fmt.Sprintf(`// agent-notify plugin %s (generated — safe to delete;
// re-created from the agent-notify app's Codex-style hook installer)
export const AgentNotify = async ({ directory }) => {
  const base = "http://127.0.0.1:%d";
  const post = (path, body) => {
    try {
      return fetch(base + path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      }).catch(() => {});
    } catch (_) {}
  };
  const session = (id, kind) =>
    post("/session", { session_id: String(id || ""), source: "opencode", cwd: directory, kind });
  const notify = (id, kind) =>
    post("/event", { session_id: String(id || ""), source: "opencode", cwd: directory, kind });

  // ---- usage sync: aggregate the local opencode.db and hand the daemon
  // per-day/per-model token totals plus the latest sessions. bun:sqlite is
  // built into opencode's runtime; on any failure (non-Bun runtime, DB
  // busy) we skip silently and retry on the next idle.
  let lastSync = 0;
  const syncUsage = async () => {
    const now = Date.now();
    if (now - lastSync < 60000) return;
    lastSync = now;
    try {
      const { Database } = await import("bun:sqlite");
      const home = process.env.HOME || process.env.USERPROFILE || "";
      const roots = [];
      if (process.env.XDG_DATA_HOME) roots.push(process.env.XDG_DATA_HOME + "/opencode");
      if (home) roots.push(home + "/.local/share/opencode");
      let db = null;
      for (const r of roots) {
        try { db = new Database(r + "/opencode.db", { readonly: true }); break; } catch (_) {}
      }
      if (!db) return;
      try {
        const days = db.query(
          "SELECT coalesce(json_extract(data,'$.modelID'),'') model, " +
          "date(coalesce(json_extract(data,'$.time.completed'), time_created)/1000,'unixepoch','localtime') day, " +
          "sum(coalesce(json_extract(data,'$.tokens.input'),0)) input, " +
          "sum(coalesce(json_extract(data,'$.tokens.output'),0)) output, " +
          "sum(coalesce(json_extract(data,'$.tokens.reasoning'),0)) reasoning, " +
          "sum(coalesce(json_extract(data,'$.tokens.cache.read'),0)) cacheRead, " +
          "sum(coalesce(json_extract(data,'$.tokens.cache.write'),0)) cacheWrite, " +
          "sum(coalesce(json_extract(data,'$.cost'),0)) cost " +
          "FROM message WHERE json_extract(data,'$.role')='assistant' " +
          "AND time_created >= (strftime('%%s','now') - 35*86400)*1000 " +
          "GROUP BY model, day").all();
        const sessions = db.query(
          "SELECT s.id id, s.title title, s.directory dir, s.time_updated updated, " +
          "(SELECT sum(coalesce(json_extract(m.data,'$.tokens.input'),0)+coalesce(json_extract(m.data,'$.tokens.output'),0)) " +
          " FROM message m WHERE m.session_id = s.id) tokens " +
          "FROM session s WHERE s.parent_id IS NULL " +
          "ORDER BY s.time_updated DESC LIMIT 6").all();
        post("/opencode-usage", { days, sessions });
      } finally {
        try { db.close(); } catch (_) {}
      }
    } catch (_) {}
  };
  setTimeout(syncUsage, 3000); // after opencode finishes DB migration

  const lastIdle = {}; // current builds emit both session.idle and session.status(idle)
  return {
    event: async ({ event }) => {
      try {
        const p = (event && event.properties) || {};
        const id = p.sessionID || (p.info && p.info.id) || "";
        if (!id) return;
        const idle =
          event.type === "session.idle" ||
          (event.type === "session.status" && p.status && p.status.type === "idle");
        if (idle) {
          const now = Date.now();
          if (now - (lastIdle[id] || 0) < 3000) return;
          lastIdle[id] = now;
          session(id, "idle");
          notify(id, "done");
          syncUsage();
          return;
        }
        if (event.type === "session.status" && p.status &&
            (p.status.type === "busy" || p.status.type === "retry")) {
          session(id, "posttool"); // -> "working" in the live view
          return;
        }
        if (event.type === "permission.asked" || event.type === "permission.updated") {
          session(id, "waiting");
          notify(id, "attention");
          return;
        }
        if (event.type === "session.deleted") session(id, "end");
      } catch (_) {}
    },
  };
};
`, opencodePluginVersion, daemonPort)
	return os.WriteFile(path, []byte(js), 0o644)
}

// ---- usage data posted by the plugin ----

type ocDayRow struct {
	Model      string  `json:"model"`
	Day        string  `json:"day"` // "2006-01-02", local time
	Input      int64   `json:"input"`
	Output     int64   `json:"output"`
	Reasoning  int64   `json:"reasoning"`
	CacheRead  int64   `json:"cacheRead"`
	CacheWrite int64   `json:"cacheWrite"`
	Cost       float64 `json:"cost"`
}

type ocSessionRow struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Dir     string `json:"dir"`
	Updated int64  `json:"updated"` // ms since epoch
	Tokens  int64  `json:"tokens"`
}

type ocUsagePayload struct {
	Days     []ocDayRow     `json:"days"`
	Sessions []ocSessionRow `json:"sessions"`
}

// ocUsageReport is the per-request aggregate served to the window.
// Fields mirror the Claude usage card so the frontend renders the same
// stat grid; sessions add the DB-backed history the plugin sends.
type ocUsageReport struct {
	HasData      bool           `json:"hasData"`
	Today        usageTotals    `json:"today"`
	Week         usageTotals    `json:"week"`
	WeekCost     float64        `json:"weekCost"`
	TodayByModel []modelUsage   `json:"todayByModel"`
	Sessions     []ocSessionRow `json:"sessions"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// ocUsageStore holds the last payload; persisted so the tab still shows
// data when opencode isn't running (tokens only accrue while it is, so
// the cache can't go stale — at worst it predates a plugin install).
type ocUsageStore struct {
	mu      sync.Mutex
	data    ocUsagePayload
	updated time.Time
	loaded  bool
}

var ocUsage = &ocUsageStore{}

func ocUsagePath() string { return homePath(".claude-notify", "opencode-usage.json") }

type ocUsageDisk struct {
	ocUsagePayload
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *ocUsageStore) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := os.ReadFile(ocUsagePath())
	if err != nil {
		return
	}
	var d ocUsageDisk
	if json.Unmarshal(data, &d) == nil {
		s.data, s.updated = d.ocUsagePayload, d.UpdatedAt
	}
}

func (s *ocUsageStore) ingest(p ocUsagePayload) {
	// cap defensively so a rogue poster can't bloat the state file
	if len(p.Days) > 400 {
		p.Days = p.Days[:400]
	}
	if len(p.Sessions) > 10 {
		p.Sessions = p.Sessions[:10]
	}
	s.mu.Lock()
	s.loadLocked()
	s.data, s.updated = p, time.Now()
	s.mu.Unlock()
	if path := ocUsagePath(); path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if data, err := json.MarshalIndent(ocUsageDisk{p, time.Now()}, "", " "); err == nil {
			_ = os.WriteFile(path, data, 0o644)
		}
	}
}

func (s *ocUsageStore) report() ocUsageReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()

	r := ocUsageReport{UpdatedAt: s.updated}
	if len(s.data.Days) == 0 && len(s.data.Sessions) == 0 {
		return r
	}
	r.HasData = true
	r.Sessions = s.data.Sessions

	now := time.Now()
	today := now.Format("2006-01-02")
	weekStart := now.AddDate(0, 0, -6).Format("2006-01-02") // ISO dates compare lexically
	byModel := map[string]*modelUsage{}
	for _, d := range s.data.Days {
		if d.Day < weekStart {
			continue
		}
		out := d.Output + d.Reasoning // reasoning bills as output
		r.Week.Input += d.Input
		r.Week.Output += out
		r.Week.CacheRead += d.CacheRead
		r.Week.CacheWrite += d.CacheWrite
		r.WeekCost += d.Cost
		if d.Day != today {
			continue
		}
		r.Today.Input += d.Input
		r.Today.Output += out
		r.Today.CacheRead += d.CacheRead
		r.Today.CacheWrite += d.CacheWrite
		m := byModel[d.Model]
		if m == nil {
			m = &modelUsage{Model: d.Model}
			byModel[d.Model] = m
		}
		m.Input += d.Input
		m.Output += out
		m.CacheRead += d.CacheRead
		m.CacheWrite += d.CacheWrite
	}
	for _, m := range byModel {
		r.TodayByModel = append(r.TodayByModel, *m)
	}
	sort.Slice(r.TodayByModel, func(i, j int) bool {
		a, b := r.TodayByModel[i], r.TodayByModel[j]
		return a.Input+a.Output > b.Input+b.Output
	})
	return r
}
