// Package elevate checks and requests Windows Administrator privileges.
//
// The bundled exe manifest already requests requireAdministrator, so the
// OS shell (Explorer double-click, shortcuts, etc.) elevates automatically.
// This package is a defense-in-depth fallback for launch paths that skip
// manifest processing (e.g. `go run`, `wails dev`, or a bare CreateProcess).
package elevate

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process token has administrator privileges.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RelaunchElevated re-executes the current process with the "runas" verb,
// which triggers the Windows UAC elevation prompt. The caller is expected
// to exit after calling this, since the elevated instance runs separately.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	args := strings.Join(os.Args[1:], " ")

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exePtr, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	argPtr, err := syscall.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	cwdPtr, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		return err
	}

	const swNormal = 1
	if err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, swNormal); err != nil {
		return fmt.Errorf("ShellExecute runas: %w", err)
	}
	return nil
}
