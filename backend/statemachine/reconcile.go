package statemachine

import (
	"slipstream/backend/fastmode"
)

// Reconcile is called once at start-up, before the UI comes up. It detects
// and cleans any leftover Fast Mode state from a previous run that crashed or
// was hard-killed: an orphaned winws.exe and a pending DNS restore.
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

	m.mu.Lock()
	m.state = StateIdle
	m.subMode = SubModeNone
	m.mu.Unlock()
}

// MaybeReconnectLastMode resumes the user's last mode if they opted into
// reconnect-on-launch. It blocks until the reconnect attempt finishes
// (Start can take several seconds), so callers that don't want to hold up
// the UI should invoke it in a goroutine.
func (m *Manager) MaybeReconnectLastMode() {
	m.mu.Lock()
	s := m.settings
	m.mu.Unlock()
	if !s.ReconnectOnLaunch || s.LastMode == SubModeNone {
		return
	}

	if s.LastMode == SubModeFast {
		if err := m.RequestFastMode(fastmode.Mode(s.LastFastSubMode), s.LastFastStrategy, s.LastFastDomains); err != nil && m.log != nil {
			m.log.Error("reconnect-on-launch failed", "lastMode", s.LastMode, "error", err)
		}
	}
}
