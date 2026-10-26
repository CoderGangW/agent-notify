//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func hideConsole(cmd *exec.Cmd) {}

// detach puts a child in its own session so it survives the parent's
// exit: launchd SIGKILLs the job's process group when the daemon exits
// (AbandonProcessGroup defaults to false), which used to kill the
// relaunch helper mid-sleep — the "app never came back after an update".
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func cmdSafePath(p string) string { return p }

func credRead(string) []byte { return nil }
