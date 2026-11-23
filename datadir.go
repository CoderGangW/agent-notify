package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// Per-user data (config, stats, logs, stamps) lives in ~/.agent-notify.
// Releases before the rename used ~/.claude-notify; the first process of
// a new build — daemon or hook, whichever runs first after an update —
// moves it over and leaves a link at the old path, so a still-running
// old daemon (Windows/Linux keep it until next start) writes into the
// new folder instead of resurrecting the old one.
const (
	dataDirName   = ".agent-notify"
	legacyDirName = ".claude-notify"
)

var migrateOnce sync.Once

// dataPath joins parts under the data dir of the current user.
func dataPath(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(append([]string{dataDirFor(home)}, parts...)...)
}

// dataDirFor resolves the data dir under home, migrating once per process.
// If the move failed (a file locked on Windows), keep using the legacy
// folder rather than starting an empty one — creating the new folder
// first would strand the user's config and stats.
func dataDirFor(home string) string {
	migrateOnce.Do(func() { migrateDataDir(home) })
	dir := filepath.Join(home, dataDirName)
	if _, err := os.Stat(dir); err != nil {
		if legacy := filepath.Join(home, legacyDirName); isRealDir(legacy) {
			return legacy
		}
	}
	return dir
}

// migrateDataDir moves ~/.claude-notify to ~/.agent-notify and links the
// old name to the new one. Idempotent; racing processes are fine — the
// rename is atomic, the loser sees the new folder already there.
// ponytail: when both exist as real folders the new one wins and the old
// is left alone; merge file-by-file if that ever happens in practice.
func migrateDataDir(home string) {
	legacy := filepath.Join(home, legacyDirName)
	dir := filepath.Join(home, dataDirName)
	if !isRealDir(legacy) {
		return
	}
	if _, err := os.Stat(dir); err == nil {
		return
	}
	if os.Rename(legacy, dir) != nil {
		return
	}
	if runtime.GOOS == "windows" {
		// directory symlinks need admin/dev mode on Windows; a junction doesn't
		cmd := exec.Command("cmd", "/c", "mklink", "/J", legacy, dir)
		hideConsole(cmd)
		_ = cmd.Run()
		return
	}
	_ = os.Symlink(dir, legacy)
}

func isRealDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.IsDir()
}
