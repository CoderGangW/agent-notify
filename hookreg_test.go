package main

import (
	"path/filepath"
	"testing"
)

func TestRegisterHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	exe := filepath.Join(home, "bin", "agent-notify")
	if a, u := registerHooks(exe); a != 6 || u != 0 {
		t.Fatalf("first register: added %d updated %d, want 6 0", a, u)
	}
	if a, u := registerHooks(exe); a != 0 || u != 0 {
		t.Fatalf("second register not idempotent: added %d updated %d", a, u)
	}
	if a, u := registerHooks(exe + "-new"); a != 0 || u != 6 {
		t.Fatalf("repoint: added %d updated %d, want 0 6", a, u)
	}
	// Windows writes exec form; it must still count as ours
	if !isOurHook(map[string]any{"command": `C:\x\agent-notify.exe`, "args": []any{"hook"}}) {
		t.Fatal("exec-form hook not recognized")
	}
	if isOurHook(map[string]any{"command": `C:\x\agent-notify.exe`, "args": []any{"daemon"}}) {
		t.Fatal("non-hook exec form recognized as ours")
	}
}
