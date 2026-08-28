package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// opencode integration: a generated JS plugin in
// ~/.config/opencode/plugins/ subscribes to opencode's event bus and
// POSTs daemon events directly — no Go hook subcommand needed. Kept to
// codex-level scope: turn-complete + attention notifications and
// coarse live state.

func opencodePluginPath() string {
	return homePath(".config", "opencode", "plugins", "agent-notify.js")
}

func opencodeHooked() bool { return fileExists(opencodePluginPath()) }

// installOpencodeHook writes the notify plugin. Every handler is
// defensive: a plugin exception must never break the user's session.
func installOpencodeHook() error {
	path := opencodePluginPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	js := fmt.Sprintf(`// agent-notify opencode plugin (generated — safe to delete;
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
`, daemonPort)
	return os.WriteFile(path, []byte(js), 0o644)
}
