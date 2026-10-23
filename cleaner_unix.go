//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

// pidAlive: signal 0 probes without signalling; EPERM still means alive.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false // 0 / negatives would address process groups
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// fileLocked reports whether another process holds an exclusive flock on
// path (Codex's thread writer lock — Rust's File::lock is flock here).
// ponytail: the probe holds the lock for an instant; a Codex opening that
// very thread in the same microsecond could see it taken.
func fileLocked(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK)
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}
