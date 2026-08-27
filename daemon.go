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
	mu     sync.Mutex
	events []Event // newest first
	done   int     // completed-task count since last clear
	muted  bool    // suppress native notifications; events still listed
	app    *application.App
	tray   *application.SystemTray
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
	// Left click and right click both toggle the window; there is no menu.
	tray.OnClick(tray.ToggleWindow)
	tray.OnRightClick(tray.ToggleWindow)

	go s.serve()
	go firstRunSetup() // double-clicked .app installs its own hooks
	go startAutoUpdate()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// serve accepts events from hook processes on the fixed localhost port.
func (s *daemonState) serve() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
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
	if ev.Kind == "done" {
		s.done++
	}
	s.mu.Unlock()

	if !muted {
		deliverNotification(ev)
	}
	s.refreshBadge()
}

// refreshBadge mirrors the completed count next to the macOS tray icon.
func (s *daemonState) refreshBadge() {
	if runtime.GOOS != "darwin" {
		return
	}
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	label := ""
	if done > 0 {
		label = strconv.Itoa(done)
	}
	s.tray.SetLabel(label)
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
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		cfg := loadConfig()
		s.mu.Lock()
		resp := struct {
			Events      []Event      `json:"events"`
			Done        int          `json:"done"`
			Muted       bool         `json:"muted"`
			Lang        string       `json:"lang"`        // resolved UI language
			LangSetting string       `json:"langSetting"` // raw config value
			DefaultTab  string       `json:"defaultTab"`
			Usage       usageReport  `json:"usage"`
			Limits      limitsReport `json:"limits"`
		}{Events: s.events, Done: s.done, Muted: s.muted,
			Lang: resolveLang(cfg.Lang), LangSetting: cfg.Lang, DefaultTab: cfg.DefaultTab}
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
		s.done = 0
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
			ev = s.events[req.Index]
		}
		s.mu.Unlock()
		// Prefer focusing the window the session ran in (IDE or terminal);
		// fall back to opening the project folder.
		if ev.Activate != "" && runtime.GOOS == "darwin" {
			_ = exec.Command("open", "-b", ev.Activate).Start()
		} else if ev.CWD != "" {
			openFolder(ev.CWD)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		go s.app.Quit()
	})
	return mux
}

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
