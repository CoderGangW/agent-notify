package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend
var frontendFS embed.FS

const maxEvents = 50

type daemonState struct {
	mu            sync.Mutex
	events        []Event // newest first
	sessions      map[string]*sessionInfo
	notifyMode    string // "on" | "alerts" | "quiet" | "silent" (config.notifyMode)
	pendingUpdate releaseInfo
	app           *application.App
	tray          *application.SystemTray
	window        application.Window
	menuWindow    application.Window // custom right-click tray menu
}

func evSource(ev Event) string {
	if ev.Source == "" {
		return "claude" // pre-0.2 events carried no source
	}
	return ev.Source
}

// unreadLocked returns per-agent unacknowledged counts; caller holds mu.
func (s *daemonState) unreadLocked() map[string]int {
	unread := map[string]int{}
	for _, a := range agentDefs {
		unread[a.ID] = 0
	}
	for _, ev := range s.events {
		if !ev.Read {
			unread[evSource(ev)]++
		}
	}
	return unread
}

func runDaemon() {
	// fresh install (no config yet): start with no agents chosen — the
	// window shows the pick-your-agents state and the tour runs on mock
	// data. Upgraders (config without the agents key) keep the legacy
	// claude+codex pair via enabledAgents().
	if _, err := os.Stat(configPath()); os.IsNotExist(err) {
		c := loadConfig()
		c.Agents = []string{}
		saveConfig(c)
	}
	s := &daemonState{notifyMode: loadConfig().notifyMode()}
	setupNativeNotify() // no-op unless running from the .app bundle
	registerURLProtocol() // Windows: agent-notify: scheme for toast clicks (no-op elsewhere)

	// First launch of the bundled app: fire the OS consent prompts up
	// front — notification banners, then the Desktop/Documents/Downloads
	// file access that click-to-focus needs. Each TCC dialog blocks its
	// syscall until answered, hence the goroutine; macOS never re-prompts
	// once decided, and unbundled dev runs (notifPermStatus -1) skip so
	// the grants don't land on the hosting terminal instead of the app.
	if c := loadConfig(); !c.PermsRequested && notifPermStatus() != -1 {
		c.PermsRequested = true
		saveConfig(c)
		go func() {
			if notifPermStatus() == 2 {
				notifPermRequest()
			}
			requestFolderAccess()
			setupMu.Lock()
			setupChecked = time.Time{} // welcome checklist repolls fresh
			setupMu.Unlock()
		}()
	}

	app := application.New(application.Options{
		Name:        "agent-notify",
		Description: "Coding-agent session notifications",
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Assets: application.AssetOptions{Handler: s.assetHandler()},
	})
	s.app = app

	tray := app.SystemTray.New()
	s.tray = tray
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIcon())
	} else {
		tray.SetIcon(trayIcon())
	}
	tray.SetTooltip("agent-notify")

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "agent-notify",
		Title:           "agent-notify",
		Width:           400,
		Height:          600,
		Frameless:       true,
		AlwaysOnTop:     true,
		Hidden:          true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
		BackgroundColour: application.NewRGB(20, 18, 16),
		URL:              "/",
	})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})
	tray.AttachWindow(window).WindowOffset(8)
	// Left click SHOWS the window — never toggle. Toggling races with
	// HideOnFocusLost (the tray click first blurs and hides the window,
	// then the toggle reads it as visible and hides it "again"), which
	// made left clicks feel dead. Closing is Esc / clicking elsewhere.
	// Right click opens OUR menu window (webview): native NSMenuItem
	// images stopped rendering on recent macOS, so the context menu is a
	// small page we draw ourselves — logo included.
	s.window = window
	s.menuWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "agent-notify-menu",
		Title:           "agent-notify",
		Width:           184,
		Height:          172,
		Frameless:       true,
		AlwaysOnTop:     true,
		Hidden:          true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
		BackgroundColour: application.NewRGB(20, 18, 16),
		URL:              "/menu.html",
	})
	s.menuWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s.menuWindow.Hide()
		e.Cancel()
	})
	tray.OnClick(s.showWindow)
	tray.OnDoubleClick(s.showWindow)
	tray.OnRightClick(s.showTrayMenu)

	go s.serve()
	go firstRunSetup() // double-clicked .app installs its own hooks
	go upgradeOpencodePlugin()
	go s.autoUpdateLoop()
	// after an in-window update the daemon respawns: bring the window back
	if home, err := os.UserHomeDir(); err == nil {
		stamp := filepath.Join(home, ".claude-notify", "reopen")
		if _, err := os.Stat(stamp); err == nil {
			_ = os.Remove(stamp)
			go func() {
				time.Sleep(800 * time.Millisecond)
				s.showWindow()
			}()
		}
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// animateHeight eases the window to the target height. On macOS AppKit
// animates the frame natively (one smooth pass, top edge pinned); other
// platforms step SetSize. A newer call supersedes a running stepper via
// the generation counter.
var (
	animMu  sync.Mutex
	animGen int
)

func (s *daemonState) animateHeight(target int) {
	animMu.Lock()
	animGen++
	gen := animGen
	animMu.Unlock()

	width, start := s.window.Size()
	if start == target {
		return
	}

	// Growing must not push the window out of the work area. Windows
	// anchors its tray at the bottom, so there the BOTTOM edge stays put
	// and the window grows upward; elsewhere the top stays pinned and we
	// only shift up when the bottom would cross the taskbar/dock line.
	x, y := s.window.Position()
	newY := y
	if runtime.GOOS == "windows" {
		newY = y + (start - target)
	}
	if scr, err := s.window.GetScreen(); err == nil && scr != nil && scr.WorkArea.Height > 0 {
		if bottom := scr.WorkArea.Y + scr.WorkArea.Height; newY+target > bottom {
			newY = bottom - target
		}
		if newY < scr.WorkArea.Y {
			newY = scr.WorkArea.Y
		}
	}

	// tray.PositionWindow reads the native frame directly, so no wails-side
	// geometry sync is needed after the native animation
	if newY == y && nativeAnimateHeight(s.window.NativeWindow(), target) {
		return
	}
	go func() {
		const dur = 240 * time.Millisecond
		t0 := time.Now()
		for {
			animMu.Lock()
			cancelled := gen != animGen
			animMu.Unlock()
			if cancelled {
				return
			}
			p := float64(time.Since(t0)) / float64(dur)
			if p >= 1 {
				s.window.SetPosition(x, newY)
				s.window.SetSize(width, target)
				return
			}
			e := 1 - (1-p)*(1-p)*(1-p) // ease-out cubic
			if newY != y {
				s.window.SetPosition(x, y+int(float64(newY-y)*e))
			}
			s.window.SetSize(width, start+int(float64(target-start)*e))
			time.Sleep(16 * time.Millisecond)
		}
	}()
}

func (s *daemonState) showWindow() {
	if s.window == nil {
		return
	}
	_ = s.tray.PositionWindow(s.window, 8)
	s.window.Show().Focus()
}

// serve accepts events from hook processes on the fixed localhost port.
func (s *daemonState) serve() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/show", func(w http.ResponseWriter, r *http.Request) {
		s.showWindow()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var u sessionUpdate
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// prompt submit is the one moment the session's Ghostty surface is
		// guaranteed focused — capture its id now (outside the state lock)
		if u.Kind == "prompt" && u.Activate == ghosttyBundle && u.Mux.Kind == "" {
			u.Surface = ghosttyCaptureSurface()
		}
		s.applySessionUpdate(u)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var ev Event
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ev.Time.IsZero() {
			ev.Time = time.Now()
		}
		// events outlive their session: pin the Ghostty surface id now
		if ev.Surface == "" && ev.SessionID != "" {
			s.mu.Lock()
			if info := s.sessions[ev.SessionID]; info != nil {
				ev.Surface = info.Surface
			}
			s.mu.Unlock()
		}
		s.add(ev)
		w.WriteHeader(http.StatusNoContent)
	})
	// token/cost aggregates posted by the opencode plugin (see opencode.go)
	mux.HandleFunc("/opencode-usage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var p ocUsagePayload
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ocUsage.ingest(p)
		w.WriteHeader(http.StatusNoContent)
	})

	// Retry briefly: on reinstall/restart the previous instance may still
	// hold the port for a moment. Persistent failure = real second instance.
	var ln net.Listener
	var err error
	for i := 0; i < 10; i++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", daemonPort))
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.Printf("listen failed (already running?): %v", err)
		os.Exit(1)
	}
	log.Fatal(http.Serve(ln, mux))
}

func (s *daemonState) add(ev Event) {
	s.mu.Lock()
	mode := s.notifyMode
	s.events = append([]Event{ev}, s.events...)
	if len(s.events) > maxEvents {
		s.events = s.events[:maxEvents]
	}
	s.mu.Unlock()

	if modeNotifies(mode) {
		deliverNotification(ev)
	}
	s.refreshBadge()
}

// refreshBadge mirrors the unread count next to the macOS tray icon.
func (s *daemonState) refreshBadge() {
	if runtime.GOOS != "darwin" {
		return
	}
	s.mu.Lock()
	unread := s.unreadLocked()
	mode := s.notifyMode
	s.mu.Unlock()
	total := 0
	for _, n := range unread {
		total += n
	}
	label := ""
	if total > 0 && modeBadges(mode) {
		label = strconv.Itoa(total)
	}
	s.tray.SetLabel(label)
}

// setupStatus reports what first-run setup accomplished, for the welcome
// screen's checklist. Cached briefly — it hits a few files.
type setupStatus struct {
	Hooks            bool `json:"hooks"` // in-tab claude guide reads this
	Autostart        bool `json:"autostart"`
	TerminalNotifier bool `json:"terminalNotifier"`
	ClaudeCLI        bool `json:"claudeCLI"`
	// OS permissions (welcome checklist): 1 granted, 0 denied,
	// 2 not determined, -1 unsupported on this platform/build
	NotifPerm  int `json:"notifPerm"`
	Automation int `json:"automation"`
	DiskAccess int `json:"diskAccess"` // file access for click-to-focus (FDA or folder grants)
}

var (
	setupMu      sync.Mutex
	setupCached  setupStatus
	setupChecked time.Time
)

func computeSetup() setupStatus {
	setupMu.Lock()
	defer setupMu.Unlock()
	if time.Since(setupChecked) < 30*time.Second {
		return setupCached
	}
	var st setupStatus
	// parse instead of substring-matching: the quoted command is escaped
	// inside the JSON file, so raw Contains checks miss it
	if data, err := os.ReadFile(settingsPath()); err == nil {
		var settings struct {
			Hooks map[string][]struct {
				Hooks []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"hooks"`
		}
		if json.Unmarshal(data, &settings) == nil {
			for _, entries := range settings.Hooks {
				for _, e := range entries {
					for _, h := range e.Hooks {
						if isOurCommand(h.Command) {
							st.Hooks = true
						}
					}
				}
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case "darwin":
			_, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"))
			st.Autostart = err == nil
		case "windows":
			st.Autostart = true // registry write never fails silently in install
		default:
			_, err := os.Stat(filepath.Join(home, ".config", "autostart", "agent-notify.desktop"))
			st.Autostart = err == nil
		}
	}
	st.TerminalNotifier = runtime.GOOS != "darwin" || findTerminalNotifier() != ""
	st.ClaudeCLI = findCLI("claude") != ""
	st.NotifPerm = notifPermStatus()
	st.Automation = automationStatus(false) // never prompts from a probe
	st.DiskAccess = diskAccessStatus()      // silent too: skips folder reads until their prompts fired
	setupCached, setupChecked = st, time.Now()
	return st
}

// assetHandler serves the embedded window UI plus its local JSON API.
func (s *daemonState) assetHandler() http.Handler {
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		panic(err)
	}
	static := http.FileServer(http.FS(sub))

	mux := http.NewServeMux()
	mux.Handle("/", static)
	// menu.html is served pre-baked: version, localized labels, and theme
	// land in the markup so the tray menu paints complete on first load,
	// with no client-side fetch in the way
	mux.HandleFunc("/menu.html", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "menu.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		page := string(data)
		for tok, val := range map[string]string{
			"{{L_VERSION}}": fmt.Sprintf(T("menu.version"), "v"+version),
			"{{L_UPDATE}}":  T("update.check"),
			"{{L_RESTART}}": T("tip.restart"),
			"{{L_QUIT}}":    T("tip.quit"),
			"{{THEME}}":     loadConfig().Theme,
		} {
			page = strings.ReplaceAll(page, tok, val)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, page)
	})
	mux.Handle("/i18n/", http.FileServer(http.FS(i18nFS)))
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		// the welcome checklist polls with fresh=1 so a grant made in
		// System Settings shows up within one poll, not one cache TTL
		if r.URL.Query().Get("fresh") == "1" {
			setupMu.Lock()
			setupChecked = time.Time{}
			setupMu.Unlock()
		}
		cfg := loadConfig()
		s.mu.Lock()
		resp := struct {
			Events      []Event        `json:"events"`
			Sessions    []sessionInfo  `json:"sessions"`
			Unread      map[string]int `json:"unread"`      // per-agent unacknowledged counts
			Agents      []agentPublic  `json:"agents"`      // supported agents + setup status
			Muted       bool           `json:"muted"`       // legacy: OS notifications off
			NotifyMode  string         `json:"notifyMode"`  // resolved 4-level mode
			Lang        string         `json:"lang"`        // resolved UI language
			LangSetting string         `json:"langSetting"` // raw config value
			DefaultTab  string         `json:"defaultTab"`
			Version     string         `json:"version"`
			Settings    config         `json:"settings"`
			Setup       setupStatus    `json:"setup"`
			UpdateAvail string         `json:"updateAvail"` // version waiting for a user-approved install
			Usage       usageReport    `json:"usage"`
			Limits      limitsReport   `json:"limits"`
			AgyQuota    agyQuotaReport `json:"agyQuota"`
			OcUsage     ocUsageReport  `json:"ocUsage"`
			Dev         bool           `json:"dev"` // running via tools/dev.sh
		}{Version: version, Settings: cfg, Setup: computeSetup(),
			Dev:    os.Getenv("AGENT_NOTIFY_DEV") != "",
			Events: s.events, Sessions: s.sessionListLocked(),
			Unread: s.unreadLocked(), Agents: agentListEnabled(cfg),
			Muted: !modeNotifies(s.notifyMode), NotifyMode: s.notifyMode,
			Lang: resolveLang(cfg.Lang), LangSetting: cfg.Lang, DefaultTab: cfg.DefaultTab}
		if s.pendingUpdate.Available {
			resp.UpdateAvail = s.pendingUpdate.Latest
		}
		s.mu.Unlock()
		resp.Usage = usage.report()
		resp.Limits = limits.report()
		// Antigravity quota needs a keyring read + token refresh, so only
		// fetch it when that tab is actually in play and agy is installed.
		if agentEnabled(cfg, "antigravity") && findCLI("agy") != "" {
			resp.AgyQuota = agyQuota.report()
		}
		if agentEnabled(cfg, "opencode") {
			resp.OcUsage = ocUsage.report()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	// header bell: cycle on → alerts → quiet → silent → on
	mux.HandleFunc("/api/mute", func(w http.ResponseWriter, r *http.Request) {
		next := map[string]string{"on": "alerts", "alerts": "quiet", "quiet": "silent", "silent": "on"}
		s.mu.Lock()
		mode := next[s.notifyMode]
		if mode == "" {
			mode = "on"
		}
		s.notifyMode = mode
		s.mu.Unlock()
		c := loadConfig()
		c.NotifyMode = mode
		c.Muted = !modeNotifies(mode) // keep the legacy field roughly in sync
		saveConfig(c)
		s.refreshBadge()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/tab", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tab string `json:"tab"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || !validAgent(req.Tab) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		c := loadConfig()
		c.DefaultTab = req.Tab
		saveConfig(c)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/lang", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Lang string `json:"lang"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch req.Lang {
		case "", "auto", "en", "ko", "zh":
		default:
			http.Error(w, "unsupported", http.StatusBadRequest)
			return
		}
		c := loadConfig()
		c.Lang = req.Lang
		saveConfig(c)
		setLang(resolveLang(req.Lang)) // notifications switch immediately
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/clear", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.events = nil
		s.mu.Unlock()
		s.refreshBadge()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/read-all", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Source string `json:"source"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		for i := range s.events {
			if evSource(s.events[i]) == req.Source {
				s.events[i].Read = true
			}
		}
		s.mu.Unlock()
		s.refreshBadge()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/open", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Index int `json:"index"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		var ev Event
		if req.Index >= 0 && req.Index < len(s.events) {
			s.events[req.Index].Read = true // interacting acknowledges it
			ev = s.events[req.Index]
		}
		s.mu.Unlock()
		s.refreshBadge()
		focusTarget(ev.Activate, ev.Mux, ev.Surface, ev.CWD, ev.Title)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/folder", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Index *int   `json:"index"` // event by position …
			ID    string `json:"id"`    // … or live session by id
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var cwd string
		s.mu.Lock()
		if req.Index != nil && *req.Index >= 0 && *req.Index < len(s.events) {
			s.events[*req.Index].Read = true // interacting acknowledges it
			cwd = s.events[*req.Index].CWD
		} else if req.ID != "" {
			if info := s.sessions[req.ID]; info != nil {
				cwd = info.CWD
			}
		}
		s.mu.Unlock()
		s.refreshBadge()
		if cwd != "" {
			openFolder(cwd)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/focus-session", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		var info sessionInfo
		if p := s.sessions[req.ID]; p != nil {
			info = *p
		}
		s.mu.Unlock()
		focusTarget(info.Activate, info.Mux, info.Surface, info.CWD, info.Title)
		w.WriteHeader(http.StatusNoContent)
	})
	// focus-notify serves Windows toast clicks: the agent-notify: protocol
	// relaunches the binary, which calls back here with the session id.
	// Finished sessions may already be gone from the live map, so fall
	// back to the newest event that carries the same id.
	mux.HandleFunc("/api/focus-notify", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		var info sessionInfo
		if p := s.sessions[id]; p != nil {
			info = *p
		} else {
			for _, ev := range s.events { // newest first
				if ev.SessionID == id {
					info = sessionInfo{Activate: ev.Activate, CWD: ev.CWD,
						Surface: ev.Surface, Mux: ev.Mux, Title: ev.Title}
					break
				}
			}
		}
		s.mu.Unlock()
		focusTarget(info.Activate, info.Mux, info.Surface, info.CWD, info.Title)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/agent-hook", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		a := agentByID(req.ID)
		if a == nil || a.installHook == nil {
			http.Error(w, "unknown agent", http.StatusBadRequest)
			return
		}
		if err := a.installHook(); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		invalidateAgentCache()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/agent-login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		a := agentByID(req.ID)
		if a == nil || a.LoginCmd == "" {
			http.Error(w, "unknown agent", http.StatusBadRequest)
			return
		}
		if err := openLoginTerminal(a.LoginCmd); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		invalidateAgentCache()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/hide-session", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		if p := s.sessions[req.ID]; p != nil {
			p.Hidden = true
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/update-check", func(w http.ResponseWriter, r *http.Request) {
		info, err := checkUpdate()
		resp := map[string]any{"current": version, "latest": info.Latest, "available": info.Available}
		if err != nil {
			resp["error"] = err.Error()
		}
		s.mu.Lock()
		s.pendingUpdate = info
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/update-apply", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		info := s.pendingUpdate
		s.mu.Unlock()
		if !info.Available {
			var err error
			info, err = checkUpdate()
			if err != nil || !info.Available {
				http.Error(w, "no update available", http.StatusConflict)
				return
			}
		}
		// the window is open (the user clicked the button) — reopen it
		// after the post-update respawn, ulio-style
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.WriteFile(filepath.Join(home, ".claude-notify", "reopen"), nil, 0o644)
		}
		restarted, err := applyUpdate(info, true)
		resp := map[string]any{"restarted": restarted, "version": info.Latest}
		if err != nil {
			resp["error"] = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Muted             *bool     `json:"muted"`      // legacy on/off
			NotifyMode        *string   `json:"notifyMode"` // 4-level mode
			Lang              *string   `json:"lang"`
			DefaultTab        *string   `json:"defaultTab"`
			DisableAutoUpdate *bool     `json:"disableAutoUpdate"`
			DisableAISummary  *bool     `json:"disableAISummary"`
			DisableLiveStatus *bool     `json:"disableLiveStatus"`
			Theme             *string   `json:"theme"`
			Autostart         *bool     `json:"autostart"`
			Agents            *[]string `json:"agents"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		c := loadConfig()
		if req.Muted != nil { // legacy toggle: map onto the 4-level mode
			m := "on"
			if *req.Muted {
				m = "quiet"
			}
			req.NotifyMode = &m
		}
		if req.NotifyMode != nil {
			switch *req.NotifyMode {
			case "on", "alerts", "quiet", "silent":
				c.NotifyMode = *req.NotifyMode
				c.Muted = !modeNotifies(*req.NotifyMode)
				s.mu.Lock()
				s.notifyMode = *req.NotifyMode
				s.mu.Unlock()
				defer s.refreshBadge()
			}
		}
		if req.Lang != nil {
			switch *req.Lang {
			case "", "auto", "en", "ko", "zh":
				c.Lang = *req.Lang
				setLang(resolveLang(*req.Lang))
			}
		}
		if req.DefaultTab != nil && validAgent(*req.DefaultTab) {
			c.DefaultTab = *req.DefaultTab
		}
		if req.DisableAutoUpdate != nil {
			c.DisableAutoUpdate = *req.DisableAutoUpdate
		}
		if req.DisableAISummary != nil {
			c.DisableAISummary = *req.DisableAISummary
		}
		if req.DisableLiveStatus != nil {
			c.DisableLiveStatus = *req.DisableLiveStatus
		}
		if req.Theme != nil {
			switch *req.Theme {
			case "", "auto", "light", "dark":
				c.Theme = *req.Theme
			}
		}
		if req.Agents != nil {
			list := []string{} // empty is allowed: "no agents chosen" state
			for _, id := range *req.Agents {
				if validAgent(id) {
					list = append(list, id)
				}
			}
			c.Agents = list
			// keep the default tab reachable
			if c.DefaultTab != "" {
				ok := false
				for _, id := range list {
					if id == c.DefaultTab {
						ok = true
					}
				}
				if !ok {
					if len(list) > 0 {
						c.DefaultTab = list[0]
					} else {
						c.DefaultTab = ""
					}
				}
			}
		}
		if req.Autostart != nil {
			c.DisableAutostart = !*req.Autostart
			if *req.Autostart {
				if bin := installDest(); bin != "" {
					_ = installAutostart(bin, false)
				}
			} else {
				removeAutostartFiles()
			}
			setupMu.Lock()
			setupChecked = time.Time{} // welcome checklist re-checks
			setupMu.Unlock()
		}
		saveConfig(c)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(usage.stats())
	})
	mux.HandleFunc("/api/setup-fix", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Item string `json:"item"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var err error
		switch req.Item {
		case "hooks":
			bin := installDest()
			if _, statErr := os.Stat(bin); statErr != nil {
				bin, _ = os.Executable()
			}
			registerHooks(fmt.Sprintf("%q hook", bin))
		case "autostart":
			bin := installDest()
			if _, statErr := os.Stat(bin); statErr != nil {
				bin, _ = os.Executable()
			}
			c := loadConfig()
			c.DisableAutostart = false
			saveConfig(c)
			err = installAutostart(bin, false)
		case "notifier":
			brew := findCLI("brew")
			if brew == "" {
				err = fmt.Errorf("homebrew not found")
			} else {
				err = exec.Command(brew, "install", "terminal-notifier").Run()
			}
		case "cli":
			// official Anthropic installer, explicitly user-initiated
			err = exec.Command("/bin/bash", "-c",
				"curl -fsSL https://claude.ai/install.sh | bash").Run()
		case "notifperm":
			// prompts while undecided; once denied only System Settings can
			// flip it back, so open the Notifications pane
			if notifPermStatus() == 0 {
				_ = exec.Command("open",
					"x-apple.systempreferences:com.apple.Notifications-Settings.extension").Start()
			} else {
				notifPermRequest()
			}
		case "automation":
			// ask=true fires the consent dialog on first request; a hard
			// denial needs the Privacy > Automation pane
			if automationStatus(true) == 0 {
				_ = exec.Command("open",
					"x-apple.systempreferences:com.apple.preference.security?Privacy_Automation").Start()
			}
		case "diskaccess":
			// folder dialogs fire only while undecided; once asked, the
			// only remaining lever is Full Disk Access (no request API —
			// the user has to flip it in Privacy & Security by hand)
			if loadConfig().FolderPermsAsked {
				_ = exec.Command("open",
					"x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles").Start()
			} else {
				requestFolderAccess()
			}
		default:
			http.Error(w, "unknown item", http.StatusBadRequest)
			return
		}
		setupMu.Lock()
		setupChecked = time.Time{} // force a fresh checklist
		setupMu.Unlock()
		resp := map[string]any{"ok": err == nil}
		if err != nil {
			resp["error"] = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/github", func(w http.ResponseWriter, r *http.Request) {
		openFolder("https://github.com/CoderGangW/agent-notify") // open/xdg-open handle URLs too
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		s.restartSelf()
	})
	// taller window for browsing long session/event lists; anchored at the
	// top so it grows downward from the tray
	mux.HandleFunc("/api/expand", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Expanded bool `json:"expanded"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if s.window != nil {
			h := 600
			if req.Expanded {
				h = 820
			}
			s.animateHeight(h)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// the tray menu page reports its rendered content size; clamp and fit
	mux.HandleFunc("/api/menu-size", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			W int `json:"w"`
			H int `json:"h"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		clamp := func(v, lo, hi int) int {
			if v < lo {
				return lo
			}
			if v > hi {
				return hi
			}
			return v
		}
		if s.menuWindow != nil {
			s.menuWindow.SetSize(clamp(req.W, 150, 320), clamp(req.H, 120, 400))
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		go s.app.Quit()
	})
	return mux
}

// focusTarget jumps to a session: select its exact multiplexer pane
// first, then bring the hosting app forward; with neither, open the
// project folder.
func focusTarget(activate string, mux muxRef, surface, cwd, title string) {
	muxFocus(mux)
	if activate != "" && runtime.GOOS == "darwin" {
		// VSCode-family apps focus the window that already has the folder
		// open when handed its path — window-level precision for free.
		// Only when the daemon can actually read the path though: a
		// TCC-hidden cwd (no file access grant) makes `open` hand the
		// IDE a folder it can't match, which spawns a fresh window —
		// plain activation is the better failure.
		if cwd != "" && ideBundles[activate] {
			if _, err := os.Stat(cwd); err == nil {
				_ = exec.Command("open", "-b", activate, cwd).Start()
				return
			}
		}
		// Ghostty: jump to the exact surface via its AppleScript API
		// (focus also activates the window — no open -b needed then)
		if activate == ghosttyBundle && ghosttyFocus(surface, cwd) {
			return
		}
		_ = exec.Command("open", "-b", activate).Start()
		return
	}
	// Windows: no bundle ids — find the IDE/terminal window whose title
	// mentions the project or session and raise it via win32.
	if focusNativeWindow(cwd, title) {
		return
	}
	if cwd != "" {
		openFolder(cwd)
	}
}

// ideBundles is the reverse view of ideBundleID: bundle ids that accept a
// folder path for window targeting.
var ideBundles = func() map[string]bool {
	m := map[string]bool{}
	for _, id := range ideBundleID {
		m[id] = true
	}
	return m
}()

func openFolder(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}
