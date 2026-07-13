package statemachine

import (
	"path/filepath"

	"slipstream/backend/fastmode"
	"slipstream/backend/killswitch"
	"slipstream/backend/privatemode"
)

// Reconcile is called once at start-up, before the UI comes up. It detects
// and cleans any leftover state from a previous run that crashed or was
// hard-killed: an orphaned winws.exe, a pending DNS restore, leftover WFP
// kill-switch filters, and a leftover AmneziaWG tunnel service.
//
// Order matters and mirrors what main.go did before this package existed:
// orphaned processes and DNS first (Fast Mode leaves no network block, so
// these are independent of the kill switch), then the kill switch itself
// (restores internet first, before touching the tunnel service, in case the
// service is slow to remove), then the tunnel service.
//
// Every step is best-effort and log-and-continue, matching the previous
// main.go behaviour: a failure to reconcile one piece must not prevent the
// app from starting and attempting the others.
func (m *Manager) Reconcile() {
	if err := fastmode.KillOrphanedProcesses(m.log); err != nil && m.log != nil {
		m.log.Error("failed to kill orphaned winws.exe from a previous run", "error", err)
	}
	if err := fastmode.RecoverPendingDNS(m.fastDataDir, m.log); err != nil && m.log != nil {
		m.log.Error("failed to recover pending DNS from a previous run", "error", err)
	}
	markerPath := filepath.Join(m.privateDataDir, "killswitch.marker")
	if err := killswitch.Reconcile(markerPath, m.log); err != nil && m.log != nil {
		m.log.Error("failed to reconcile leftover kill switch from a previous run", "error", err)
	}
	if err := privatemode.RecoverLeftoverTunnel(m.amneziaWGPath, m.log); err != nil && m.log != nil {
		m.log.Error("failed to remove leftover tunnel from a previous run", "error", err)
	}
	// A hard kill mid-connect can leave the plaintext tunnel config (with the
	// private key) on disk; shred it so it never outlives the session that
	// wrote it.
	privatemode.ShredLeftoverPlaintextConfig(m.privateDataDir, m.log)

	m.mu.Lock()
	m.state = StateIdle
	m.subMode = SubModeNone
	m.mu.Unlock()
}

// MaybeReconnectLastMode resumes the user's last mode if they opted into
// reconnect-on-launch. It blocks until the reconnect attempt finishes
// (Connect/Start can take several seconds), so callers that don't want to
// hold up the UI should invoke it in a goroutine.
func (m *Manager) MaybeReconnectLastMode() {
	m.mu.Lock()
	s := m.settings
	m.mu.Unlock()
	if !s.ReconnectOnLaunch || s.LastMode == SubModeNone {
		return
	}

	var err error
	switch s.LastMode {
	case SubModeFast:
		err = m.RequestFastMode(fastmode.Mode(s.LastFastSubMode), s.LastFastStrategy, s.LastFastDomains)
	case SubModePrivate:
		err = m.RequestPrivateMode()
	}
	if err != nil && m.log != nil {
		m.log.Error("reconnect-on-launch failed", "lastMode", s.LastMode, "error", err)
	}
}
