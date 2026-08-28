//go:build darwin

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// File access is what lets a click hand the project path to the IDE:
// focusTarget's `open -b <bundle> <cwd>` only matches an existing window
// when the daemon can actually read <cwd>, and TCC hides Desktop/
// Documents/Downloads from it until the user consents. Full Disk Access
// has no request API at all — the folder prompts are the only consent
// dialogs we can fire ourselves; FDA can only be granted by hand in
// Privacy & Security.

// diskAccessStatus probes file access without ever triggering a TCC
// prompt: 1 granted (FDA, or every project folder readable), 0 denied,
// 2 not determined (folder prompts not fired yet), -1 unavailable.
// The FDA probe reads files only FDA can see (same set
// node-mac-permissions uses); a hit on any of them proves the grant.
func diskAccessStatus() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return -1
	}
	probes := []string{
		filepath.Join(home, "Library", "Safari", "Bookmarks.plist"),
		filepath.Join(home, "Library", "Application Support", "com.apple.TCC", "TCC.db"),
		filepath.Join(home, "Library", "Safari", "CloudTabs.db"),
		"/Library/Preferences/com.apple.TimeMachine.plist",
	}
	for _, p := range probes {
		if f, err := os.Open(p); err == nil {
			f.Close()
			return 1
		}
	}
	// No FDA. The folder-level grants cover the usual project homes, but
	// reading a folder while its consent is undecided BLOCKS on the
	// dialog — so the silent probe is only safe once the prompts fired.
	if !loadConfig().FolderPermsAsked {
		return 2
	}
	for _, d := range userFolders(home) {
		if _, err := os.ReadDir(d); errors.Is(err, fs.ErrPermission) {
			return 0
		}
	}
	return 1
}

// requestFolderAccess fires the native Desktop/Documents/Downloads
// consent dialogs (first access while undecided prompts; each ReadDir
// blocks until the user answers, so call only from a goroutine or a
// request handler) and records that they were asked, which unlocks the
// silent folder probe above. Decided folders never re-prompt.
func requestFolderAccess() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for _, d := range userFolders(home) {
		_, _ = os.ReadDir(d)
	}
	c := loadConfig()
	c.FolderPermsAsked = true
	saveConfig(c)
}

func userFolders(home string) []string {
	return []string{
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Downloads"),
	}
}
