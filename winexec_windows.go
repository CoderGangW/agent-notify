//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
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

// cmdSafePath makes p usable unquoted inside a `cmd /c` string that a Go
// program builds (agy runs hooks that way): Go wraps the whole string in
// quotes and cmd strips them, so neither a bare path with spaces nor a
// quoted one survives. The 8.3 short form has no spaces — when the volume
// keeps short names (the system drive does by default).
func cmdSafePath(p string) string {
	if !strings.Contains(p, " ") {
		return p
	}
	long, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, syscall.MAX_PATH)
	n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return p
	}
	if s := syscall.UTF16ToString(buf[:n]); !strings.Contains(s, " ") {
		return s
	}
	return p
}

var (
	advapi32      = syscall.NewLazyDLL("advapi32.dll")
	procCredReadW = advapi32.NewProc("CredReadW")
	procCredFree  = advapi32.NewProc("CredFree")
)

// winCredential mirrors CREDENTIALW.
type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// credRead returns a generic credential's secret from Windows Credential
// Manager — where go-keyring-based CLIs (agy) keep tokens, under the
// target "<service>:<account>". nil when absent.
func credRead(target string) []byte {
	t, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return nil
	}
	var c *winCredential
	const credTypeGeneric = 1
	if r, _, _ := procCredReadW.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c))); r == 0 || c == nil {
		return nil
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(c)))
	return append([]byte(nil), unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize)...)
}
