package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"
)

const maxMenuEvents = 10

type daemonState struct {
	mu     sync.Mutex
	events []Event // newest first
	items  []*systray.MenuItem
	done   int  // completed-task count since last clear
	muted  bool // suppress native notifications; events still listed
}

func runDaemon() {
	s := &daemonState{}
	systray.Run(s.onReady, nil)
}

func (s *daemonState) onReady() {
	s.muted = loadConfig().Muted
	systray.SetIcon(trayIcon())
	systray.SetTooltip("claude-notify")
	if runtime.GOOS == "darwin" {
		systray.SetTemplateIcon(trayIcon(), trayIcon())
	}

	header := systray.AddMenuItem("Claude 세션 알림", "")
	header.Disable()
	systray.AddSeparator()

	// Fixed slots for recent events; hidden until filled.
	s.items = make([]*systray.MenuItem, maxMenuEvents)
	for i := range s.items {
		s.items[i] = systray.AddMenuItem("", "클릭하면 프로젝트 폴더 열기")
		s.items[i].Hide()
		go s.watchClick(i)
	}

	systray.AddSeparator()
	mute := systray.AddMenuItemCheckbox("알림 켜기", "끄면 배너 알림 없이 목록에만 기록", !s.muted)
	clear := systray.AddMenuItem("목록 비우기", "")
	quit := systray.AddMenuItem("종료", "")

	go func() {
		for {
			select {
			case <-mute.ClickedCh:
				s.mu.Lock()
				s.muted = !s.muted
				muted := s.muted
				s.mu.Unlock()
				if muted {
					mute.Uncheck()
				} else {
					mute.Check()
				}
				saveConfig(config{Muted: muted})
			case <-clear.ClickedCh:
				s.mu.Lock()
				s.events = nil
				s.done = 0
				s.refreshLocked()
				s.mu.Unlock()
			case <-quit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	go s.serve()
}

func (s *daemonState) watchClick(i int) {
	for range s.items[i].ClickedCh {
		s.mu.Lock()
		var cwd string
		if i < len(s.events) {
			cwd = s.events[i].CWD
		}
		s.mu.Unlock()
		if cwd != "" {
			openFolder(cwd)
		}
	}
}

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

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", daemonPort))
	if err != nil {
		// Another daemon instance is already running.
		log.Printf("listen failed (already running?): %v", err)
		systray.Quit()
		return
	}
	log.Fatal(http.Serve(ln, mux))
}

func (s *daemonState) add(ev Event) {
	s.mu.Lock()
	muted := s.muted
	s.mu.Unlock()
	if !muted {
		deliverNotification(ev)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append([]Event{ev}, s.events...)
	if len(s.events) > maxMenuEvents {
		s.events = s.events[:maxMenuEvents]
	}
	if ev.Kind == "done" {
		s.done++
	}
	s.refreshLocked()
}

// refreshLocked syncs the tray menu with s.events; caller holds s.mu.
func (s *daemonState) refreshLocked() {
	for i, item := range s.items {
		if i < len(s.events) {
			ev := s.events[i]
			icon := "✅"
			if ev.Kind == "attention" {
				icon = "🔔"
			}
			item.SetTitle(fmt.Sprintf("%s %s · %s", icon, projectName(ev.CWD), ev.Time.Format("15:04")))
			item.Show()
		} else {
			item.Hide()
		}
	}
	if runtime.GOOS == "darwin" && s.done > 0 {
		systray.SetTitle(fmt.Sprintf("%d", s.done))
	} else if runtime.GOOS == "darwin" {
		systray.SetTitle("")
	}
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
