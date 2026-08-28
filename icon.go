package main

import (
	_ "embed"
	"runtime"
)

// Tray icons are embedded at build time — swap the files in assets/ and
// rebuild to change them. icon_mac.png should stay monochrome: macOS
// treats it as a template image and tints it to match the menu bar.
var (
	//go:embed assets/icon.png
	iconColor []byte
	//go:embed assets/icon_mac.png
	iconMac []byte
	//go:embed assets/logo.png
	iconLogo []byte // 256px badge, installed as the Linux hicolor app icon
)

func trayIcon() []byte {
	switch runtime.GOOS {
	case "darwin":
		return iconMac
	case "windows":
		// Wails' CreateIconFromResourceEx wants raw PNG bytes (RT_ICON
		// style), not an ICO file — an ICONDIR-wrapped payload fails and
		// leaves the tray blank. Pre-scale so Windows doesn't shrink the
		// full-size art badly.
		if small := scalePNG(iconColor, 32); small != nil {
			return small
		}
		return iconColor
	default:
		return iconColor
	}
}
