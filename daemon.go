package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	muted         bool // suppress native notifications; events still listed
	pendingUpdate releaseInfo
	app           *application.App
	tray          *application.SystemTray
	window        application.Window
}

func evSource(ev Event) string {
	if ev.Source == "codex" {
		return "codex"
	}
	return "claude"
}

// unreadLocked returns per-source unacknowledged counts; caller holds mu.
func (s *daemonState) unreadLocked() (claude, codex int) {
	for _, ev := range s.events {
		if ev.Read {
			continue
		}
		if evSource(ev) == "codex" {
			codex++
		} else {
			claude++
		}
	}
	return
}

func runDaemon() {
	s := &daemonState{muted: loadConfig().Muted}

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
	// Any click SHOWS the window — never toggle. Toggling races with
	// HideOnFocusLost (the tray click first blurs and hides the window,
	// then the toggle reads it as visible and hides it "again"), which
	// made left clicks feel dead. Closing is Esc / clicking elsewhere.
	// Any click SHOWS the window — never toggle, and bind every button:
	// macOS can deliver status-item left clicks as right-button events
	// (observed on macOS 27), which used to fall into the menuless
	// right-click path and feel dead.
	s.window = window
	tray.OnClick(s.showWindow)
	tray.OnRightClick(s.showWindow)
	tray.OnDoubleClick(s.showWindow)

	go s.serve()
	go firstRunSetup() // double-clicked .app installs its own hooks
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
		s.add(ev)
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
	muted := s.muted
	s.events = append([]Event{ev}, s.events...)
	if len(s.events) > maxEvents {
		s.events = s.events[:maxEvents]
	}
	s.mu.Unlock()

	if !muted {
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
	uc, ux := s.unreadLocked()
	s.mu.Unlock()
	label := ""
	if uc+ux > 0 {
		label = strconv.Itoa(uc + ux)
	}
	s.tray.SetLabel(label)
}

// setupStatus reports what first-run setup accomplished, for the welcome
// screen's checklist. Cached briefly — it hits a few files.
type setupStatus struct {
	Hooks            bool `json:"hooks"`
	Autostart        bool `json:"autostart"`
	TerminalNotifier bool `json:"terminalNotifier"`
	ClaudeCLI        bool `json:"claudeCLI"`
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
	mux.Handle("/i18n/", http.FileServer(http.FS(i18nFS)))
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		cfg := loadConfig()
		s.mu.Lock()
		uc, ux := s.unreadLocked()
		resp := struct {
			Events      []Event        `json:"events"`
			Sessions    []sessionInfo  `json:"sessions"`
			Unread      map[string]int `json:"unread"` // per-source unacknowledged counts
			Muted       bool           `json:"muted"`
			Lang        string         `json:"lang"`        // resolved UI language
			LangSetting string         `json:"langSetting"` // raw config value
			DefaultTab  string         `json:"defaultTab"`
			Version     string         `json:"version"`
			Settings    config         `json:"settings"`
			Setup       setupStatus    `json:"setup"`
			UpdateAvail string         `json:"updateAvail"` // version waiting for a user-approved install
			Usage       usageReport    `json:"usage"`
			Limits      limitsReport   `json:"limits"`
		}{Version: version, Settings: cfg, Setup: computeSetup(),
			Events: s.events, Sessions: s.sessionListLocked(),
			Unread: map[string]int{"claude": uc, "codex": ux}, Muted: s.muted,
			Lang: resolveLang(cfg.Lang), LangSetting: cfg.Lang, DefaultTab: cfg.DefaultTab}
		if s.pendingUpdate.Available {
			resp.UpdateAvail = s.pendingUpdate.Latest
		}
		s.mu.Unlock()
		resp.Usage = usage.report()
		resp.Limits = limits.report()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/mute", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.muted = !s.muted
		muted := s.muted
		s.mu.Unlock()
		c := loadConfig()
		c.Muted = muted
		saveConfig(c)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/tab", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tab string `json:"tab"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || (req.Tab != "claude" && req.Tab != "codex") {
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
		focusTarget(ev.Activate, ev.Mux, ev.CWD)
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
		focusTarget(info.Activate, info.Mux, info.CWD)
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
			Muted             *bool   `json:"muted"`
			Lang              *string `json:"lang"`
			DefaultTab        *string `json:"defaultTab"`
			DisableAutoUpdate *bool   `json:"disableAutoUpdate"`
			DisableAISummary  *bool   `json:"disableAISummary"`
			DisableLiveStatus *bool   `json:"disableLiveStatus"`
			Theme             *string `json:"theme"`
			Autostart         *bool   `json:"autostart"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		c := loadConfig()
		if req.Muted != nil {
			c.Muted = *req.Muted
			s.mu.Lock()
			s.muted = *req.Muted
			s.mu.Unlock()
		}
		if req.Lang != nil {
			switch *req.Lang {
			case "", "auto", "en", "ko", "zh":
				c.Lang = *req.Lang
				setLang(resolveLang(*req.Lang))
			}
		}
		if req.DefaultTab != nil && (*req.DefaultTab == "claude" || *req.DefaultTab == "codex") {
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
	mux.HandleFunc("/api/github", func(w http.ResponseWriter, r *http.Request) {
		openFolder("https://github.com/CoderGangW/agent-notify") // open/xdg-open handle URLs too
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		go func() {
			time.Sleep(400 * time.Millisecond) // let the response flush
			if runtime.GOOS == "darwin" && os.Getppid() == 1 {
				os.Exit(1) // launchd KeepAlive respawns us
			}
			// unmanaged: hand off to a detached replacement, then quit;
			// its listen-retry loop waits for this port to free up
			if bin := installDest(); bin != "" {
				cmd := exec.Command(bin, "daemon")
				if err := cmd.Start(); err == nil {
					_ = cmd.Process.Release()
				}
			}
			os.Exit(0)
		}()
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
func focusTarget(activate string, mux muxRef, cwd string) {
	muxFocus(mux)
	if activate != "" && runtime.GOOS == "darwin" {
		// VSCode-family apps focus the window that already has the folder
		// open when handed its path — window-level precision for free.
		if cwd != "" && ideBundles[activate] {
			_ = exec.Command("open", "-b", activate, cwd).Start()
			return
		}
		_ = exec.Command("open", "-b", activate).Start()
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
