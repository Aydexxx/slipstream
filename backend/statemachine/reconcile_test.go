package statemachine

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"slipstream/backend/fastmode"
)

// MaybeReconnectLastMode only touches m.settings and the already-fake
// Fast/Private controllers, so it's fully safe to drive here — unlike
// Manager.Reconcile(), which calls fastmode.KillOrphanedProcesses,
// killswitch.Reconcile, and privatemode.RecoverLeftoverTunnel as real
// package-level functions (real taskkill / real WFP engine open / real
// service-manager queries), bypassing the fake interfaces entirely. That's a
// deliberate reuse of the existing recovery functions from when this package
// was built, not an oversight — but it does mean Reconcile() itself can't be
// unit-tested without the same live-system risk this test suite avoids
// everywhere else, so it isn't exercised here.

func TestMaybeReconnectLastModeNoOpWhenDisabled(t *testing.T) {
	h := newHarness(t)
	h.mgr.mu.Lock()
	h.mgr.settings = Settings{ReconnectOnLaunch: false, LastMode: SubModeFast}
	h.mgr.mu.Unlock()

	h.mgr.MaybeReconnectLastMode()

	for _, e := range h.log.snapshot() {
		if e == "fast:start" || e == "private:connect" {
			t.Fatalf("expected no reconnect attempt while disabled, got %v", h.log.snapshot())
		}
	}
}

func TestMaybeReconnectLastModeNoOpWhenNoLastMode(t *testing.T) {
	h := newHarness(t)
	h.mgr.mu.Lock()
	h.mgr.settings = Settings{ReconnectOnLaunch: true, LastMode: SubModeNone}
	h.mgr.mu.Unlock()

	h.mgr.MaybeReconnectLastMode()

	if len(h.log.snapshot()) != 0 {
		t.Fatalf("expected no calls with no last mode, got %v", h.log.snapshot())
	}
}

func TestMaybeReconnectLastModeResumesFastMode(t *testing.T) {
	h := newHarness(t)
	h.mgr.mu.Lock()
	h.mgr.settings = Settings{
		ReconnectOnLaunch: true,
		LastMode:          SubModeFast,
		LastFastSubMode:   string(fastmode.ModeDiscord),
		LastFastDomains:   []string{"example.com"},
	}
	h.mgr.mu.Unlock()

	h.mgr.MaybeReconnectLastMode()

	entries := h.log.snapshot()
	if indexOf(entries, "fast:start") == -1 {
		t.Fatalf("expected fast:start, got %v", entries)
	}
	if got := h.mgr.Status(); got.SubMode != SubModeFast {
		t.Errorf("expected SubModeFast after resume, got %v", got.SubMode)
	}
}

func TestMaybeReconnectLastModeResumesPrivateMode(t *testing.T) {
	h := newHarness(t)
	h.mgr.mu.Lock()
	h.mgr.settings = Settings{ReconnectOnLaunch: true, LastMode: SubModePrivate}
	h.mgr.mu.Unlock()

	h.mgr.MaybeReconnectLastMode()

	entries := h.log.snapshot()
	if indexOf(entries, "private:connect") == -1 {
		t.Fatalf("expected private:connect, got %v", entries)
	}
	if got := h.mgr.Status(); got.SubMode != SubModePrivate {
		t.Errorf("expected SubModePrivate after resume, got %v", got.SubMode)
	}
}

func TestMaybeReconnectLastModeLogsAndSwallowsFailure(t *testing.T) {
	h := newHarness(t)
	h.fast.mu.Lock()
	h.fast.startErr = errors.New("boom")
	h.fast.mu.Unlock()
	h.mgr.mu.Lock()
	h.mgr.settings = Settings{ReconnectOnLaunch: true, LastMode: SubModeFast}
	h.mgr.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	h.mgr.mu.Unlock()

	// Must not panic even though the resume attempt fails.
	h.mgr.MaybeReconnectLastMode()

	if got := h.mgr.Status(); got.State != StateError {
		t.Errorf("expected StateError after a failed resume, got %v", got.State)
	}
}
