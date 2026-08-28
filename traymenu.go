package main

import (
	"os"
	"os/exec"
	"runtime"
	"time"
)

// Custom tray context menu: right-click opens a small webview window
// (frontend/menu.html) positioned at the tray icon. A native NSMenu
// can't render our logo on recent macOS (menu-item images are dropped),
// so the menu is a page we draw ourselves. It hides on focus loss / Esc
// like the dashboard.

func (s *daemonState) showTrayMenu() {
	if s.menuWindow == nil {
		return
	}
	_ = s.tray.PositionWindow(s.menuWindow, 8)
	s.menuWindow.Show().Focus()
}

// restartSelf hands the daemon off to a fresh process (tray menu and
// /api/restart share it).
func (s *daemonState) restartSelf() {
	go func() {
		time.Sleep(400 * time.Millisecond) // let any HTTP response flush
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
}
