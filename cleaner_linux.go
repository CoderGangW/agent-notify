//go:build linux

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// moveToTrash follows the freedesktop.org Trash spec: the item moves to
// $XDG_DATA_HOME/Trash/files and a .trashinfo records where it came from,
// so file managers can restore it.
func moveToTrash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		data = filepath.Join(home, ".local", "share")
	}
	files, info := filepath.Join(data, "Trash", "files"), filepath.Join(data, "Trash", "info")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(info, 0o700); err != nil {
		return err
	}
	base := filepath.Base(abs)
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s.%d", base, i)
		}
		ti := filepath.Join(info, name+".trashinfo")
		f, err := os.OpenFile(ti, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			(&url.URL{Path: abs}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
		f.Close()
		if err := os.Rename(abs, filepath.Join(files, name)); err != nil {
			_ = os.Remove(ti)
			return err
		}
		return nil
	}
}

func setDockPresence(bool) {}

func revealPath(p string) { openFolder(filepath.Dir(p)) }

// argv is a binary name plus flags and a validated id: nothing to quote.
func resumeInTerminal(cwd string, argv []string) error {
	for _, term := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xterm"} {
		if bin := findCLI(term); bin != "" {
			cmd := exec.Command(bin, "-e", strings.Join(argv, " "))
			cmd.Dir = cwd
			return cmd.Start()
		}
	}
	return os.ErrNotExist
}
