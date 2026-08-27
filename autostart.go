package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const launchdLabel = "com.codergangw.claude-notify"

// installAutostart registers the daemon to start at login and starts it
// now where the mechanism supports it.
const appBundleBin = "/Applications/agent-notify.app/Contents/MacOS/agent-notify"

func installAutostart(exe string, start bool) error {
	switch runtime.GOOS {
	case "darwin":
		// Prefer the app bundle: Login Items then shows the app's name and
		// icon instead of falling back to the macOS account name the way
		// it does for bare unsigned binaries.
		if _, err := os.Stat(appBundleBin); err == nil {
			exe = appBundleBin
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir := filepath.Join(home, "Library", "LaunchAgents")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		plist := filepath.Join(dir, launchdLabel+".plist")
		logPath := filepath.Join(home, ".claude-notify", "daemon.log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
		// KeepAlive on crash only: quitting from the tray (exit 0) stays quit.
		content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key><true/>
	<key>KeepAlive</key>
	<dict><key>SuccessfulExit</key><false/></dict>
	<key>StandardOutPath</key><string>%s</string>
	<key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, launchdLabel, exe, logPath, logPath)
		if err := os.WriteFile(plist, []byte(content), 0o644); err != nil {
			return err
		}
		target := fmt.Sprintf("gui/%d", os.Getuid())
		service := target + "/" + launchdLabel
		if !start {
			// The plist loads at next login; the calling process is already
			// the running daemon, so bootstrapping now would just collide.
			return nil
		}
		if exec.Command("launchctl", "print", service).Run() == nil {
			_ = exec.Command("launchctl", "bootout", service).Run()
			// bootout is asynchronous; wait until the label is gone
			for i := 0; i < 20; i++ {
				if exec.Command("launchctl", "print", service).Run() != nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		err = exec.Command("launchctl", "bootstrap", target, plist).Run()
		if err != nil { // transient EIO right after bootout — one retry
			time.Sleep(time.Second)
			err = exec.Command("launchctl", "bootstrap", target, plist).Run()
		}
		return err

	case "windows":
		return exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "agent-notify", "/t", "REG_SZ",
			"/d", fmt.Sprintf(`"%s" daemon`, exe), "/f").Run()

	default: // Linux desktop environments honor XDG autostart
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir := filepath.Join(home, ".config", "autostart")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=agent-notify
Exec="%s" daemon
X-GNOME-Autostart-enabled=true
`, exe)
		if err := os.WriteFile(filepath.Join(dir, "agent-notify.desktop"), []byte(content), 0o644); err != nil {
			return err
		}
		installLinuxMenuEntry(home, exe)
		return nil
	}
}

func uninstallAutostart() error {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		target := fmt.Sprintf("gui/%d", os.Getuid())
		_ = exec.Command("launchctl", "bootout", target+"/"+launchdLabel).Run()
		return os.Remove(filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"))

	case "windows":
		return exec.Command("reg", "delete",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "agent-notify", "/f").Run()

	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(home, ".local", "share", "applications", "agent-notify.desktop"))
		_ = os.Remove(filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "agent-notify.png"))
		return os.Remove(filepath.Join(home, ".config", "autostart", "agent-notify.desktop"))
	}
}

// installLinuxMenuEntry registers the app-menu launcher and hicolor icon;
// best-effort, a headless box just skips it.
func installLinuxMenuEntry(home, exe string) {
	iconDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps")
	if err := os.MkdirAll(iconDir, 0o755); err == nil {
		_ = os.WriteFile(filepath.Join(iconDir, "agent-notify.png"), iconLogo, 0o644)
	}
	appDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=agent-notify
Comment=Coding-agent session notifications
Exec="%s" daemon
Icon=agent-notify
Terminal=false
Categories=Utility;
`, exe)
	_ = os.WriteFile(filepath.Join(appDir, "agent-notify.desktop"), []byte(content), 0o644)
}

// removeAutostartFiles unregisters login autostart WITHOUT touching the
// running daemon (uninstallAutostart boots the launchd job out, which
// would kill the process serving the settings toggle).
func removeAutostartFiles() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	switch runtime.GOOS {
	case "darwin":
		_ = os.Remove(filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"))
	case "windows":
		_ = exec.Command("reg", "delete",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "agent-notify", "/f").Run()
	default:
		_ = os.Remove(filepath.Join(home, ".config", "autostart", "agent-notify.desktop"))
	}
}

// autostartHealthy reports whether the login entry exists and points at a
// binary that is still there — daemons then leave it alone instead of
// rewriting it every start (an older bundle build used to undo the
// bundle-path preference that way).
func autostartHealthy() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	switch runtime.GOOS {
	case "darwin":
		data, err := os.ReadFile(filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"))
		if err != nil {
			return false
		}
		s := string(data)
		i := strings.Index(s, "<array>")
		j := strings.Index(s, "</string>")
		if i < 0 || j < 0 || j < i {
			return false
		}
		prog := s[i:j]
		if k := strings.LastIndex(prog, "<string>"); k >= 0 {
			prog = prog[k+len("<string>"):]
		}
		_, err = os.Stat(prog)
		return err == nil
	case "windows":
		return true // registry entry never goes stale by path swap
	default:
		_, err := os.Stat(filepath.Join(home, ".config", "autostart", "agent-notify.desktop"))
		return err == nil
	}
}
