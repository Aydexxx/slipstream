package privatemode

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// The tunnel name doubles as the wintun adapter name, the UAPI pipe suffix, and
// (with amneziawg.exe's prefix) the Windows service name. amneziawg.exe derives
// it from the config *filename*, so the on-disk config MUST be tunnelName+".conf".
const (
	tunnelName  = "Slipstream"
	serviceName = "AmneziaWGTunnel$" + tunnelName
	// CREATE_NO_WINDOW — don't flash a console for amneziawg.exe invocations.
	createNoWindow = 0x08000000
)

// runAWG invokes amneziawg.exe hidden and returns combined output.
func runAWG(ctx context.Context, awgExe string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, awgExe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// installTunnel installs and starts the AmneziaWG tunnel service from confPath.
// The service (amneziawg.exe /service) owns the wintun adapter, the default
// route through the tunnel, tunnel DNS, and the endpoint /32 exclusion — and
// undoes all of it when uninstalled.
func installTunnel(ctx context.Context, awgExe, confPath string) error {
	out, err := runAWG(ctx, awgExe, "/installtunnelservice", confPath)
	if err != nil {
		return fmt.Errorf("install tunnel service: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

// uninstallTunnel stops and removes the tunnel service, which restores the
// original routing table and DNS. Safe to call when it isn't installed.
func uninstallTunnel(ctx context.Context, awgExe string) error {
	exists, _, _ := serviceState()
	if !exists {
		return nil
	}
	out, err := runAWG(ctx, awgExe, "/uninstalltunnelservice", tunnelName)
	if err != nil {
		return fmt.Errorf("uninstall tunnel service: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

// serviceState reports whether the tunnel service exists and whether it's
// running. Any manager/open error other than clear existence is reported so
// callers can log it; a non-existent service returns (false,false,nil).
func serviceState() (exists bool, running bool, err error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, false, fmt.Errorf("connect service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		// The overwhelmingly common case here is "service does not exist".
		return false, false, nil
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return true, false, fmt.Errorf("query service: %w", err)
	}
	return true, st.State == svc.Running, nil
}

// ServiceExists reports whether a Slipstream AmneziaWG tunnel service is
// currently installed (used for start-up crash recovery).
func ServiceExists() bool {
	exists, _, _ := serviceState()
	return exists
}

// waitServiceGone blocks until the service no longer exists or timeout elapses.
// Service removal is asynchronous, so this makes reinstall-after-uninstall safe.
func waitServiceGone(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if exists, _, _ := serviceState(); !exists {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tunnel service still present after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
