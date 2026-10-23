//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A Codex thread holding its writer lock is running; once released it isn't,
// even though the hooks saw it moments ago.
func TestCleanerCodexWriterLock(t *testing.T) {
	home := t.TempDir()
	id := "019a0000-0000-7000-8000-000000000009"
	rollout := ".codex/sessions/2026/10/22/rollout-2026-10-22T10-00-00-" + id + ".jsonl"
	lock := ".codex/thread-writer-locks/" + id + ".lock"
	writeAll(t, home, map[string]string{
		rollout: `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/w"}}`,
		lock:    "",
	})
	old := time.Now().Add(-time.Hour) // past the "just written" window
	if err := os.Chtimes(filepath.Join(home, rollout), old, old); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(home, lock))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{id: true}
	scan := func() cleanerSession {
		return cleanerScan("codex", home, func(string) string { return "" }, config{}, live)[0]
	}
	if !scan().Running {
		t.Fatal("locked thread should be running")
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	if scan().Running {
		t.Fatal("unlocked thread should not be running, whatever the hooks said")
	}
}
