//go:build darwin

package main

import (
	"bytes"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// The app icon rides along in the binary. In-app updates swap only the
// bundle's executable (update.go applyUpdate), so without this the
// Resources/appicon.icns from the very first install would never change.
//
//go:embed assets/appicon.icns
var appIconICNS []byte

const lsregister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

// syncBundleIcon brings the installed bundle's icon up to the one this
// build carries. Only a production binary does it (the bundle copy or the
// CLI install) — a dev build must never rewrite and re-sign the real app.
func syncBundleIcon(self string) {
	if self != appBundleBin && self != installDest() {
		return
	}
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(appBundleBin))) // …/agent-notify.app
	icns := filepath.Join(bundle, "Contents", "Resources", "appicon.icns")
	cur, err := os.ReadFile(icns)
	if err != nil || bytes.Equal(cur, appIconICNS) {
		return // no bundle installed, or already current
	}
	if os.WriteFile(icns, appIconICNS, 0o644) != nil {
		return
	}
	// a changed resource breaks the ad-hoc seal: re-sign like applyUpdate
	_ = exec.Command("codesign", "--force", "-s", "-",
		"--identifier", "com.codergangw.claude-notify", bundle).Run()
	// Finder/Dock cache icons per bundle: bump it and re-register
	now := time.Now()
	_ = os.Chtimes(bundle, now, now)
	_ = exec.Command(lsregister, "-f", bundle).Run()
}
