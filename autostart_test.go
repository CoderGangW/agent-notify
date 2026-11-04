//go:build darwin

package main

import (
	"os"
	"strings"
	"testing"
)

// autostartHealthy once parsed Label's </string> as the program path, so
// it always reported unhealthy and every start rewrote the plist.
func TestAutostartHealthyPlist(t *testing.T) {
	if _, err := os.Stat(appBundleBin); err != nil {
		t.Skip("needs /Applications/agent-notify.app")
	}
	t.Setenv("HOME", t.TempDir())
	if err := installAutostart(appBundleBin, false); err != nil {
		t.Fatal(err)
	}
	if !autostartHealthy() {
		t.Fatal("freshly written plist reported unhealthy")
	}
	p := homePath("Library", "LaunchAgents", launchdLabel+".plist")
	data, _ := os.ReadFile(p)
	old := strings.Replace(string(data), "AssociatedBundleIdentifiers", "Gone", 1)
	if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if autostartHealthy() {
		t.Fatal("plist without AssociatedBundleIdentifiers must be rewritten")
	}
}
