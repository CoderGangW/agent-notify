package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Agent registry: every coding agent the app can track. "claude" is the
// native integration (hooks installed by runInstall); the others follow
// the codex.go adapter pattern — a CLI-side hook subcommand posting to
// the daemon, wired in by an installer that edits the agent's config.

type agentDef struct {
	ID   string // event Source / tab id
	Name string // display name
	Beta bool
	Bin  string // CLI binary for install detection

	InstallCmd string // shown to the user; we never run it
	LoginCmd   string // run in the user's terminal on request
	Site       string // docs / download page

	// loggedIn reports whether CLI credentials exist. nil = unknown,
	// treated as logged in (never block a working setup on a bad guess).
	loggedIn func() bool
	// hooked reports whether our notify hook is wired into the agent's
	// config. nil = the claude native path (setupStatus covers it).
	hooked func() bool
	// installHook wires the notify hook into the agent's config; nil for
	// agents whose hook install is handled elsewhere.
	installHook func() error
}

// agentPublic is the per-agent status exposed to the window.
type agentPublic struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Beta       bool   `json:"beta"`
	Enabled    bool   `json:"enabled"` // user-picked tab (settings)
	Installed  bool   `json:"installed"`
	LoggedIn   bool   `json:"loggedIn"`
	Hooked     bool   `json:"hooked"`
	InstallCmd string `json:"installCmd"`
	LoginCmd   string `json:"loginCmd"`
	Site       string `json:"site"`
}

// agentListEnabled stamps the user's tab selection onto the (cached)
// status list — Enabled must reflect the config instantly, so it lives
// outside the cache.
func agentListEnabled(c config) []agentPublic {
	on := map[string]bool{}
	for _, id := range c.enabledAgents() {
		on[id] = true
	}
	list := agentList()
	out := make([]agentPublic, len(list))
	for i, a := range list {
		a.Enabled = on[a.ID]
		out[i] = a
	}
	return out
}

func homePath(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(append([]string{home}, parts...)...)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

var agentDefs = []agentDef{
	{
		ID: "claude", Name: "Claude", Bin: "claude",
		InstallCmd: "npm install -g @anthropic-ai/claude-code",
		LoginCmd:   "claude /login",
		Site:       "https://claude.com/claude-code",
		loggedIn: func() bool {
			return fileExists(homePath(".claude", ".credentials.json")) ||
				fileExists(homePath(".claude.json")) // API-key / config login
		},
	},
	{
		ID: "codex", Name: "Codex", Bin: "codex",
		InstallCmd:  "npm install -g @openai/codex",
		LoginCmd:    "codex login",
		Site:        "https://developers.openai.com/codex/cli",
		loggedIn:    func() bool { return fileExists(homePath(".codex", "auth.json")) },
		hooked:      codexHooked,
		installHook: installCodexHook,
	},
	{
		// Personal-account OAuth was cut off 2026-06-18 (migrated to
		// Antigravity); the CLI still works on API-key / Vertex /
		// enterprise auth, so the tab stays for those users.
		ID: "gemini", Name: "Gemini", Beta: true, Bin: "gemini",
		InstallCmd: "npm install -g @google/gemini-cli",
		LoginCmd:   "gemini", // no auth subcommand: first interactive run opens the picker
		Site:       "https://geminicli.com",
		loggedIn: func() bool {
			// oauth_creds.json alone no longer proves a working login —
			// personal OAuth is dead — so only count API-key style auth
			// (plus Vertex ADC via GOOGLE_APPLICATION_CREDENTIALS).
			return os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" ||
				os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" ||
				fileExists(homePath(".gemini", ".env"))
		},
		hooked:      geminiHooked,
		installHook: installGeminiHook,
	},
	{
		ID: "antigravity", Name: "Antigravity", Beta: true, Bin: "agy",
		InstallCmd: "curl -fsSL https://antigravity.google/cli/install.sh | bash",
		LoginCmd:   "agy", // first run opens the browser login
		Site:       "https://antigravity.google",
		// credentials live in the OS keyring — no file to probe, so
		// loggedIn stays nil (treated as logged in)
		hooked:      antigravityHooked,
		installHook: installAntigravityHook,
	},
	{
		ID: "opencode", Name: "opencode", Beta: true, Bin: "opencode",
		InstallCmd: "npm install -g opencode-ai",
		LoginCmd:   "opencode auth login",
		Site:       "https://opencode.ai",
		loggedIn: func() bool {
			// any provider credential counts; env-var-only setups are
			// invisible, so absence of the file is only a soft signal
			st, err := os.Stat(homePath(".local", "share", "opencode", "auth.json"))
			return err == nil && st.Size() > 2 // "{}" = empty store
		},
		hooked:      opencodeHooked,
		installHook: installOpencodeHook,
	},
	{
		ID: "cursor", Name: "Cursor", Beta: true, Bin: "cursor-agent",
		InstallCmd:  "curl https://cursor.com/install -fsS | bash",
		LoginCmd:    "cursor-agent login",
		Site:        "https://cursor.com/docs/cli",
		loggedIn:    cursorLoggedIn,
		hooked:      cursorHooked,
		installHook: installCursorHook,
	},
}

func agentByID(id string) *agentDef {
	for i := range agentDefs {
		if agentDefs[i].ID == id {
			return &agentDefs[i]
		}
	}
	return nil
}

func validAgent(id string) bool { return agentByID(id) != nil }

// agentList computes per-agent setup status. Cached briefly — each call
// stats a handful of files and walks PATH.
var (
	agentsMu      sync.Mutex
	agentsCached  []agentPublic
	agentsChecked time.Time
)

func agentList() []agentPublic {
	agentsMu.Lock()
	defer agentsMu.Unlock()
	if time.Since(agentsChecked) < 15*time.Second {
		return agentsCached
	}
	out := make([]agentPublic, 0, len(agentDefs))
	for _, a := range agentDefs {
		p := agentPublic{
			ID: a.ID, Name: a.Name, Beta: a.Beta,
			InstallCmd: a.InstallCmd, LoginCmd: a.LoginCmd, Site: a.Site,
			Installed: findCLI(a.Bin) != "",
			LoggedIn:  true,
			Hooked:    true,
		}
		if a.loggedIn != nil {
			p.LoggedIn = a.loggedIn()
		}
		if a.hooked != nil {
			p.Hooked = a.hooked()
		}
		out = append(out, p)
	}
	agentsCached, agentsChecked = out, time.Now()
	return out
}

// invalidateAgentCache forces the next agentList to re-probe (after a
// hook install or login).
func invalidateAgentCache() {
	agentsMu.Lock()
	agentsChecked = time.Time{}
	agentsMu.Unlock()
}

// openLoginTerminal runs the agent's login command in the user's
// terminal — login flows are interactive (browser round-trip, prompts),
// so the daemon can't run them headless.
func openLoginTerminal(cmd string) error {
	switch runtime.GOOS {
	case "darwin":
		osascript(`tell application "Terminal"
	activate
	do script "` + cmd + `"
end tell`)
		return nil
	case "windows":
		return exec.Command("cmd", "/c", "start", "cmd", "/k", cmd).Start()
	default:
		for _, term := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xterm"} {
			if bin := findCLI(term); bin != "" {
				return exec.Command(bin, "-e", cmd).Start()
			}
		}
		return os.ErrNotExist
	}
}
