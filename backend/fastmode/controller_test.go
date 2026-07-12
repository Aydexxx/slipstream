package fastmode

import (
	"strings"
	"testing"

	"slipstream/backend/engine"
)

// newTestController builds a real Controller against temp directories. Only
// safe for tests that never reach real process/DNS I/O (the not-elevated
// refusal, and Stop/Shutdown on a controller that was never started).
func newTestController(t *testing.T) *Controller {
	t.Helper()
	em, err := engine.New(nil)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	c, err := New(Config{Engine: em, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("fastmode.New: %v", err)
	}
	return c
}

// Start must refuse before touching the engine, launching winws.exe, or
// changing state at all when not elevated - the "no admin" failure path.
func TestStartRefusesWithoutElevation(t *testing.T) {
	orig := isElevated
	isElevated = func() bool { return false }
	defer func() { isElevated = orig }()

	c := newTestController(t)
	err := c.Start(ModeFull, nil)
	if err == nil {
		t.Fatal("expected Start to refuse when not elevated")
	}
	if !strings.Contains(err.Error(), "Administrator") {
		t.Errorf("expected an Administrator-related error, got %v", err)
	}
	if got := c.Status(); got.State != StateStopped {
		t.Errorf("state should remain %q after a refused Start, got %q", StateStopped, got.State)
	}
}

// Stop on a Controller that was never started must be a safe, real no-op:
// no DNS backup exists, so dns.restore() returns at its first check without
// ever shelling out to netsh/PowerShell. This exercises the actual
// production Controller (not a fake), proving the idempotent-teardown
// guarantee holds even for the "nothing to do" case.
func TestStopOnFreshControllerIsSafeNoOp(t *testing.T) {
	c := newTestController(t)
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop on a fresh controller should be a clean no-op, got %v", err)
	}
	if got := c.Status(); got.State != StateStopped {
		t.Errorf("expected state %q, got %q", StateStopped, got.State)
	}
}

// Shutdown must be safe to call on a fresh Controller too - it's the
// unconditional app-exit backstop, so it needs to tolerate "nothing was ever
// started" without error or hang.
func TestShutdownOnFreshControllerIsSafeNoOp(t *testing.T) {
	c := newTestController(t)
	c.Shutdown() // must not panic or hang
	if got := c.Status(); got.State != StateStopped {
		t.Errorf("expected state %q after Shutdown, got %q", StateStopped, got.State)
	}
}

// RecoverPendingDNS is the crash-recovery entry point called at every launch
// - this proves its path-construction and pending() wiring are correct for
// the "clean prior shutdown" case without shelling out to netsh/PowerShell
// (the actual restore behavior is already covered by dnsManager.restore's
// recordingRunner-based tests in dns_test.go).
func TestRecoverPendingDNSNoBackupIsNoOp(t *testing.T) {
	if err := RecoverPendingDNS(t.TempDir(), nil); err != nil {
		t.Fatalf("RecoverPendingDNS with no backup should be a no-op, got %v", err)
	}
}

func TestClassifyLaunchError(t *testing.T) {
	c := newTestController(t)
	cases := []struct {
		name   string
		stderr string
		want   string
	}{
		{"access denied", "Error 5: Access is denied.", "Administrator"},
		{"driver signature blocked", "driver load failed: code 1275", "Secure Boot"},
		{"windivert failed to load", "WinDivert.dll: failed to open driver", "WinDivert driver failed to load"},
		{"already in use", "the device is already in use", "already in use"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ring := newRingBuffer(1024)
			ring.Write([]byte(tc.stderr))
			got := c.classifyLaunchError(errString("winws exited"), ring)
			if !strings.Contains(got.Error(), tc.want) {
				t.Errorf("classifyLaunchError(%q) = %q, want it to contain %q", tc.stderr, got.Error(), tc.want)
			}
		})
	}

	t.Run("no stderr falls back to the wrapped base error", func(t *testing.T) {
		got := c.classifyLaunchError(errString("winws exited immediately"), nil)
		if !strings.Contains(got.Error(), "Fast Mode engine failed to start") {
			t.Errorf("expected the generic fallback message, got %q", got.Error())
		}
	})
}

type errString string

func (e errString) Error() string { return string(e) }
