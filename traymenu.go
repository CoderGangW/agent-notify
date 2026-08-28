package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Native tray context menu: left-click opens the dashboard window,
// right-click shows this menu (app info · update · restart · quit).
// With a menu set and no OnRightClick handler, Wails shows the menu via
// native tracking on right-click and still routes left-click to OnClick.

// buildTrayMenu assembles the menu once; mutable items live on
// daemonState so the update flow can relabel them.
func (s *daemonState) buildTrayMenu() *application.Menu {
	menu := application.NewMenu()

	name := menu.Add("agent-notify")
	if small := scalePNG(iconLogo, 18); small != nil {
		name.SetBitmap(small) // NSMenuItem renders images at natural size
	}
	name.SetEnabled(false)

	menu.Add(fmt.Sprintf(T("menu.version"), "v"+version)).SetEnabled(false)
	if st, err := os.Stat(installDest()); err == nil {
		menu.Add(fmt.Sprintf(T("menu.updated"), st.ModTime().Format("2006-01-02"))).SetEnabled(false)
	}
	menu.AddSeparator()

	s.updMenuItem = menu.Add(T("update.check"))
	s.updMenuItem.OnClick(func(*application.Context) { s.trayUpdateClicked() })

	menu.Add(T("tip.restart")).OnClick(func(*application.Context) { s.restartSelf() })
	menu.AddSeparator()
	menu.Add(T("tip.quit")).OnClick(func(*application.Context) { go s.app.Quit() })

	s.trayMenu = menu
	return menu
}

// rebuildTrayMenu re-creates the menu with current-language labels
// (called after the UI language changes).
func (s *daemonState) rebuildTrayMenu() {
	if s.tray != nil {
		s.tray.SetMenu(s.buildTrayMenu())
	}
}

// refreshTrayMenu pushes label changes into the cached native menu —
// macOS shows the menu on mouse-down before any Go handler runs, so the
// cache must be current ahead of the click.
func (s *daemonState) refreshTrayMenu() {
	if s.trayMenu != nil {
		s.tray.SetMenu(s.trayMenu)
	}
}

func (s *daemonState) setUpdItem(label string, enabled bool) {
	s.updMenuItem.SetLabel(label)
	s.updMenuItem.SetEnabled(enabled)
	s.refreshTrayMenu()
}

// trayUpdateClicked drives the two-step update flow on one menu item:
// "check" -> "vX.Y.Z available" -> apply (or "up to date" and revert).
func (s *daemonState) trayUpdateClicked() {
	s.mu.Lock()
	pending := s.pendingUpdate
	s.mu.Unlock()

	if pending.Available { // second click: install
		s.setUpdItem(T("update.applying"), false)
		go func() {
			if _, err := applyUpdate(pending, true); err != nil {
				s.setUpdItem(T("update.fail"), false)
				s.revertUpdItemAfter(4 * time.Second)
			}
			// on success the daemon restarts; nothing to relabel
		}()
		return
	}

	s.setUpdItem(T("update.checking"), false)
	go func() {
		info, err := checkUpdate()
		if err != nil {
			s.setUpdItem(T("update.fail"), false)
			s.revertUpdItemAfter(4 * time.Second)
			return
		}
		s.mu.Lock()
		s.pendingUpdate = info
		s.mu.Unlock()
		if info.Available {
			s.setUpdItem(strings.Replace(T("update.available"), "{v}", info.Latest, 1), true)
			return
		}
		s.setUpdItem(T("update.latest"), false)
		s.revertUpdItemAfter(4 * time.Second)
	}()
}

func (s *daemonState) revertUpdItemAfter(d time.Duration) {
	go func() {
		time.Sleep(d)
		s.setUpdItem(T("update.check"), true)
	}()
}

// scalePNG box-averages a PNG down to size×size (menu items show images
// at their natural pixel size, and the embedded logo is 256px).
func scalePNG(data []byte, size int) []byte {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// average the source block this output pixel covers
			x0, x1 := b.Min.X+x*b.Dx()/size, b.Min.X+(x+1)*b.Dx()/size
			y0, y1 := b.Min.Y+y*b.Dy()/size, b.Min.Y+(y+1)*b.Dy()/size
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					bl += uint64(pb)
					a += uint64(pa)
					n++
				}
			}
			if n == 0 {
				continue
			}
			out.SetRGBA(x, y, color.RGBA{
				R: uint8(r / n >> 8), G: uint8(g / n >> 8),
				B: uint8(bl / n >> 8), A: uint8(a / n >> 8),
			})
		}
	}
	var buf bytes.Buffer
	if png.Encode(&buf, out) != nil {
		return nil
	}
	return buf.Bytes()
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
