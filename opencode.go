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
const opencodePluginVersion = "v4"

// opencodePluginPath follows opencode's global config dir:
// $OPENCODE_CONFIG_DIR, else ($XDG_CONFIG_HOME or ~/.config)/opencode.
func opencodePluginPath() string {
	dir := os.Getenv("OPENCODE_CONFIG_DIR")
	if dir == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = homePath(".config")
		}
		dir = filepath.Join(base, "opencode")
	}
	return filepath.Join(dir, "plugins", "agent-notify.js")
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
//
// One file serves both opencode generations: v1 (≥1.3.4) calls
// default.server, v2 (OpenCode 2, @opencode/cli) calls default.setup and
// ignores the extra key. v2 renamed the lifecycle events (payload in
// e.data) and moved sessions to session_v2/session_message; it also runs
// under Node in some builds, so SQLite comes from bun:sqlite or
// node:sqlite, whichever exists.
func installOpencodeHook() error {
	path := opencodePluginPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	js := fmt.Sprintf(`// agent-notify plugin %s (generated — safe to delete;
// re-created from the agent-notify app's Codex-style hook installer)
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

// ---- usage sync: aggregate the local opencode.db and hand the daemon
// per-day/per-model token totals plus the latest sessions. On any failure
// (DB busy, unknown runtime) skip silently and retry on the next turn.
const SQL = {
  v1: {
    days:
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
      "GROUP BY model, day",
    sessions:
      "SELECT s.id id, s.title title, s.directory dir, s.time_updated updated, " +
      "(SELECT sum(coalesce(json_extract(m.data,'$.tokens.input'),0)+coalesce(json_extract(m.data,'$.tokens.output'),0)) " +
      " FROM message m WHERE m.session_id = s.id) tokens " +
      "FROM session s WHERE s.parent_id IS NULL " +
      "ORDER BY s.time_updated DESC LIMIT 6",
  },
  v2: {
    // fork copies carry msg_<evt>_<seq> ids — the instr filter skips them
    days:
      "SELECT coalesce(json_extract(data,'$.model.id'),'') model, " +
      "date(coalesce(json_extract(data,'$.time.completed'), time_created)/1000,'unixepoch','localtime') day, " +
      "sum(coalesce(json_extract(data,'$.tokens.input'),0)) input, " +
      "sum(coalesce(json_extract(data,'$.tokens.output'),0)) output, " +
      "sum(coalesce(json_extract(data,'$.tokens.reasoning'),0)) reasoning, " +
      "sum(coalesce(json_extract(data,'$.tokens.cache.read'),0)) cacheRead, " +
      "sum(coalesce(json_extract(data,'$.tokens.cache.write'),0)) cacheWrite, " +
      "sum(coalesce(json_extract(data,'$.cost'),0)) cost " +
      "FROM session_message WHERE type IN ('assistant','compaction') AND instr(substr(id,5),'_')=0 " +
      "AND time_created >= (strftime('%%s','now') - 35*86400)*1000 " +
      "GROUP BY model, day",
    sessions:
      "SELECT id, coalesce(title,'') title, directory dir, time_updated updated, " +
      "tokens_input+tokens_output tokens " +
      "FROM session_v2 WHERE parent_id IS NULL ORDER BY time_updated DESC LIMIT 6",
  },
};
const openDB = async () => {
  const env = process.env;
  const home = (await import("node:os")).homedir(); // what opencode uses (USERPROFILE on Windows)
  const data = env.XDG_DATA_HOME ? env.XDG_DATA_HOME + "/opencode" : home + "/.local/share/opencode";
  let file = env.OPENCODE_DB || "opencode.db";
  if (!/^([a-zA-Z]:)?[\\/]/.test(file)) file = data + "/" + file; // relative = under the data dir
  try {
    const { Database } = await import("bun:sqlite");
    const db = new Database(file, { readonly: true });
    return { all: (q) => db.query(q).all(), close: () => db.close() };
  } catch (_) {}
  try {
    const { DatabaseSync } = await import("node:sqlite");
    const db = new DatabaseSync(file, { readOnly: true });
    return { all: (q) => db.prepare(q).all(), close: () => db.close() };
  } catch (_) {}
  return null;
};
let lastSync = 0;
const syncUsage = async (gen) => {
  const now = Date.now();
  if (now - lastSync < 60000) return;
  lastSync = now;
  try {
    const db = await openDB();
    if (!db) return;
    try {
      post("/opencode-usage", { days: db.all(SQL[gen].days), sessions: db.all(SQL[gen].sessions) });
    } finally {
      try { db.close(); } catch (_) {}
    }
  } catch (_) {}
};

// ---- opencode v1: named-event hooks, payload in event.properties
const server = async ({ directory }) => {
  // titles arrive via session.updated; the terminal shows "OC | <title>",
  // which is what a Windows notification click matches to find the window
  const titles = {};
  const session = (id, kind) =>
    post("/session", { session_id: String(id || ""), source: "opencode", cwd: directory, kind, title: titles[id] || "" });
  const notify = (id, kind) =>
    post("/event", { session_id: String(id || ""), source: "opencode", cwd: directory, kind, title: titles[id] || "" });
  setTimeout(() => syncUsage("v1"), 3000); // after opencode finishes DB migration

  const lastIdle = {}; // current builds emit both session.idle and session.status(idle)
  return {
    event: async ({ event }) => {
      try {
        const p = (event && event.properties) || {};
        const id = p.sessionID || (p.info && p.info.id) || "";
        if (!id) return;
        if (event.type === "session.updated" && p.info && p.info.title) {
          titles[id] = p.info.title;
          return;
        }
        const idle =
          event.type === "session.idle" ||
          (event.type === "session.status" && p.status && p.status.type === "idle");
        if (idle) {
          const now = Date.now();
          if (now - (lastIdle[id] || 0) < 3000) return;
          lastIdle[id] = now;
          session(id, "idle");
          notify(id, "done");
          syncUsage("v1");
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

// ---- OpenCode 2: setup(ctx) with an event stream, payload in e.data.
// One instance runs per directory inside a shared service and may see
// every directory's events, so each reports only its own sessions.
const setup = async (ctx) => {
  const norm = (p) => {
    p = String(p || "").replace(/\\/g, "/").replace(/\/+$/, "");
    return process.platform === "win32" ? p.toLowerCase() : p; // v2 stores C:/x on Windows
  };
  const here = norm(ctx && ctx.location && ctx.location.directory);
  const info = new Map(); // sessionID -> { title, dir, child }
  const lookup = async (id) => {
    if (info.has(id)) return info.get(id);
    let v = { title: "", dir: (ctx.location && ctx.location.directory) || "", child: false };
    try {
      const s = await ctx.session.get({ sessionID: id });
      v = { title: s.title || "", dir: (s.location && s.location.directory) || v.dir, child: !!s.parentID };
    } catch (_) {}
    info.set(id, v);
    return v;
  };
  const send = async (id, kind, ev) => {
    if (!id) return;
    const s = await lookup(id);
    if (s.child || (here && norm(s.dir) !== here)) return; // subagents stay quiet
    const b = { session_id: id, source: "opencode", cwd: s.dir, title: s.title };
    post("/session", { ...b, kind });
    if (ev) post("/event", { ...b, kind: ev });
  };
  const ac = new AbortController();
  (async () => { // never awaited: setup must return for activation to finish
    while (!ac.signal.aborted) {
      try {
        for await (const e of ctx.event.subscribe({ signal: ac.signal })) {
          try {
            const d = (e && e.data) || {};
            const id = d.sessionID;
            switch (e.type) {
              case "session.created":
                if (id) info.set(id, { title: d.title || "", dir: (d.location && d.location.directory) || "", child: !!d.parentID });
                break;
              case "session.renamed":
                if (id && info.has(id) && d.title) info.get(id).title = d.title;
                break;
              case "session.execution.started":
              case "session.retry.scheduled":
                send(id, "posttool"); // -> "working" in the live view
                break;
              case "session.execution.succeeded":
                send(id, "idle", "done");
                syncUsage("v2");
                break;
              case "session.execution.failed":
                send(id, "idle", "attention");
                syncUsage("v2");
                break;
              case "session.execution.interrupted":
                if (d.reason !== "shutdown") send(id, "idle"); // the user stopped it — nothing to announce
                break;
              case "permission.asked":
                send(id, "waiting", "attention");
                break;
              case "session.deleted":
                send(id, "end");
                info.delete(id);
                break;
            }
          } catch (_) {}
        }
      } catch (_) {}
      if (!ac.signal.aborted) await new Promise((r) => setTimeout(r, 1000)); // stream ended: resubscribe
    }
  })();
  setTimeout(() => syncUsage("v2"), 3000);
  return () => ac.abort();
};

export default { id: "agent-notify", server, setup };
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
