// Package autostart registers (or unregisters) Slipstream to launch when the
// user signs in, via the per-user HKCU Run key. This launches unelevated —
// Slipstream's own self-elevation (backend/elevate) takes it from there, at
// the cost of a UAC prompt on every boot; that trade-off is inherent to using
// the Run key rather than a privileged scheduled task.
package autostart

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// Enable writes a Run key value named appName that launches exePath, with
// any extraArgs appended to the command line. Idempotent: overwrites any
// existing value for appName (e.g. after the exe moves).
func Enable(appName, exePath string, extraArgs ...string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("Couldn't access the Windows startup entry: %w", err)
	}
	defer key.Close()

	cmd := quoteArg(exePath)
	for _, a := range extraArgs {
		cmd += " " + quoteArg(a)
	}
	if err := key.SetStringValue(appName, cmd); err != nil {
		return fmt.Errorf("Couldn't update the Windows startup entry: %w", err)
	}
	return nil
}

// Disable removes the Run key value for appName. Safe to call when not
// present (no-op).
func Disable(appName string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("Couldn't access the Windows startup entry: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("Couldn't remove the Windows startup entry: %w", err)
	}
	return nil
}

// IsEnabled reports whether a Run key value exists for appName.
func IsEnabled(appName string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("Couldn't access the Windows startup entry: %w", err)
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("Couldn't read the Windows startup entry: %w", err)
	}
	return true, nil
}

// quoteArg wraps a command-line argument in double quotes, matching Windows'
// own conventions for a Run key command line (needed for paths with spaces).
func quoteArg(s string) string {
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
