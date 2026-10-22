//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideConsole keeps a console child from flashing a window: the daemon is
// a GUI-subsystem process, so every console child would otherwise get a
// brand-new visible console.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// detach on Windows: a hidden console child already runs independently
// of the parent's lifetime; no session escape needed.
func detach(cmd *exec.Cmd) { hideConsole(cmd) }
