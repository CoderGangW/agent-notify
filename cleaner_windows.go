//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSHFileOperationW = windows.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW")

// SHFILEOPSTRUCTW (natural alignment on 64-bit)
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

// moveToTrash sends the item to the Recycle Bin.
func moveToTrash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	from, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0) // pFrom is a double-NUL-terminated list
	op := shFileOpStruct{
		wFunc:  3, // FO_DELETE
		pFrom:  &from[0],
		fFlags: 0x40 | 0x10 | 0x4 | 0x400, // ALLOWUNDO | NOCONFIRMATION | SILENT | NOERRORUI
	}
	if r, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op))); r != 0 {
		return fmt.Errorf("SHFileOperation 0x%x", r)
	}
	if op.fAnyOperationsAborted != 0 {
		return errors.New("recycle aborted")
	}
	return nil
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return true
	}
	return code == 259 // STILL_ACTIVE
}

func setDockPresence(bool) {}

// revealPath selects the file in Explorer. The raw command line keeps
// Go's argument quoting away from explorer's own /select, parser.
func revealPath(p string) {
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + p + `"`}
	_ = cmd.Start()
}

func resumeInTerminal(cwd string, argv []string) error {
	// cwd goes through cmd.Dir, never the command line, so & ^ | in a
	// path can't reach cmd.exe; argv is a binary, flags and a validated id
	cmd := exec.Command("cmd", append([]string{"/c", "start", "", "cmd", "/k"}, argv...)...)
	cmd.Dir = cwd
	return cmd.Start()
}

// fileLocked reports whether another process holds a lock on path's first
// byte (Rust's File::lock takes the whole file via LockFileEx).
func fileLocked(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol); err != nil {
		return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
	}
	_ = windows.UnlockFileEx(h, 0, 1, 0, ol)
	return false
}
