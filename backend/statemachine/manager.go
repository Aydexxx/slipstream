package statemachine

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"slipstream/backend/fastmode"
	"slipstream/backend/privatemode"
)

// Config wires a Manager to its dependencies.
type Config struct {
	Log     *slog.Logger
	Fast    FastController
	Private PrivateController

	// StateDataDir is where settings.json (persisted last-mode/reconnect
	// preference) lives, e.g. %LocalAppData%\Slipstream\state.
	StateDataDir string
	// FastDataDir / PrivateDataDir / AmneziaWGPath are only needed by
	// Reconcile, which calls the existing package-level recovery functions.
	FastDataDir    string
	PrivateDataDir string
	AmneziaWGPath  string
}

// Manager is the single owner of cross-mode state. All exported methods are
// safe for concurrent use. All mode changes — Fast Mode, Private Mode, or
// back to Idle — go through it, never through the underlying controllers
// directly, so mutual exclusion can't be bypassed.
type Manager struct {
	log     *slog.Logger
	fast    FastController
	private PrivateController

	stateDataDir   string
	fastDataDir    string
	privateDataDir string
	amneziaWGPath  string

	emit   Emitter
	emitMu sync.RWMutex

	mu            sync.Mutex
	transitioning bool
	state         State
	subMode       SubMode
	lastErr       string
	since         time.Time
	settings      Settings
	lastFast      fastmode.Status
	lastPrivate   privatemode.Status

	shutdownOnce sync.Once
}

// New constructs a Manager, loads any persisted settings, and wires itself
// as the sole listener of both controllers' status callbacks.
func New(cfg Config) (*Manager, error) {
	if cfg.Fast == nil || cfg.Private == nil {
		return nil, fmt.Errorf("statemachine: both controllers are required")
	}
	if cfg.StateDataDir == "" {
		return nil, fmt.Errorf("statemachine: state data directory is required")
	}
	settings, err := loadSettings(cfg.StateDataDir)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		log:            cfg.Log,
		fast:           cfg.Fast,
		private:        cfg.Private,
		stateDataDir:   cfg.StateDataDir,
		fastDataDir:    cfg.FastDataDir,
		privateDataDir: cfg.PrivateDataDir,
		amneziaWGPath:  cfg.AmneziaWGPath,
		state:          StateIdle,
		settings:       settings,
		lastFast:       cfg.Fast.Status(),
		lastPrivate:    cfg.Private.Status(),
	}
	cfg.Fast.SetEmitter(m.onFastStatus)
	cfg.Private.SetEmitter(m.onPrivateStatus)
	return m, nil
}

// SetEmitter registers the unified status-change callback.
func (m *Manager) SetEmitter(e Emitter) {
	m.emitMu.Lock()
	m.emit = e
	m.emitMu.Unlock()
}

// Fast exposes the underlying Fast Mode controller for config-only
// operations (domain lists, presets) that aren't mode transitions and so
// don't need to go through the state machine.
func (m *Manager) Fast() FastController { return m.fast }

// Private exposes the underlying Private Mode controller for config-only
// operations (import/summary/delete) that aren't mode transitions.
func (m *Manager) Private() PrivateController { return m.private }

// LastFastSelection returns the Fast Mode sub-mode/strategy/domains last used
// successfully — the same values MaybeReconnectLastMode resumes with.
// Defaults to ModeFull, the default strategy, and no domains if Fast Mode has
// never been started. Used by one-click callers (the tray menu) that don't
// have a config UI of their own to ask the user for a sub-mode.
func (m *Manager) LastFastSelection() (fastmode.Mode, string, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mode := fastmode.Mode(m.settings.LastFastSubMode)
	if mode == "" {
		mode = fastmode.ModeFull
	}
	domains := make([]string, len(m.settings.LastFastDomains))
	copy(domains, m.settings.LastFastDomains)
	// An empty strategy is fine: Fast Mode's Start resolves it to the default.
	return mode, m.settings.LastFastStrategy, domains
}

// Status returns a snapshot of the unified state.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

func (m *Manager) snapshotLocked() Status {
	fs := m.lastFast
	ps := m.lastPrivate
	return Status{
		State:             m.state,
		SubMode:           m.subMode,
		Transitioning:     m.transitioning,
		Healthy:           m.state == StateFastActive || m.state == StatePrivateActive,
		Error:             m.lastErr,
		Since:             m.since,
		FastStatus:        &fs,
		PrivateStatus:     &ps,
		KillSwitchArmed:   m.private.KillSwitchArmed(),
		ReconnectOnLaunch: m.settings.ReconnectOnLaunch,
		LastFastStrategy:  m.settings.LastFastStrategy,
	}
}

func (m *Manager) emitStatus() {
	m.mu.Lock()
	s := m.snapshotLocked()
	m.mu.Unlock()
	m.emitMu.RLock()
	e := m.emit
	m.emitMu.RUnlock()
	if e != nil {
		e(s)
	}
}

// beginTransition claims the top-level transition lock so concurrent
// Request* calls can't race each other. It mirrors the transitioning guard
// each underlying controller already has, one level up. It emits the new
// transitioning=true status so the UI can show a pending state for the whole
// round-trip (not just the local action call).
func (m *Manager) beginTransition() error {
	m.mu.Lock()
	if m.transitioning {
		m.mu.Unlock()
		return fmt.Errorf("Still changing mode; try again in a moment")
	}
	m.transitioning = true
	m.mu.Unlock()
	m.emitStatus()
	return nil
}

// endTransition clears the transition lock and — crucially — emits the
// settled status. Without this final emit the last status the frontend ever
// saw would still carry transitioning=true (every emit during the transition
// snapshots it as true), leaving mode buttons stuck in their loading/disabled
// state forever. Emitted outside the lock since emitStatus takes m.mu itself.
func (m *Manager) endTransition() {
	m.mu.Lock()
	m.transitioning = false
	m.mu.Unlock()
	m.emitStatus()
}

// fail records a transition failure as StateError and emits it.
func (m *Manager) fail(err error) {
	m.mu.Lock()
	m.state = StateError
	m.subMode = SubModeNone
	m.lastErr = err.Error()
	m.mu.Unlock()
	if m.log != nil {
		m.log.Error("state machine transition failed", "error", err)
	}
	m.emitStatus()
}

// RequestFastMode switches to Fast Mode using the named desync strategy (the
// "how"; see fastmode/strategies.go) against the given target sub-mode (the
// "what"). It first fully tears down and verifies Private Mode if it was
// active. strategyID may be "" or unknown — Fast Mode resolves it to the
// default.
func (m *Manager) RequestFastMode(mode fastmode.Mode, strategyID string, domains []string) error {
	if err := m.beginTransition(); err != nil {
		return err
	}
	defer m.endTransition()

	if err := m.teardownOtherMode(SubModeFast); err != nil {
		m.fail(err)
		return err
	}

	// Claim the sub-mode before starting so onFastStatus attributes the
	// status callbacks Start() fires internally (Starting, then
	// Running/Failed) to Fast Mode instead of dropping them while subMode is
	// still stale.
	m.mu.Lock()
	m.subMode = SubModeFast
	m.state = StateFastActive
	m.mu.Unlock()

	if err := m.fast.Start(mode, strategyID, domains); err != nil {
		m.fail(err)
		return err
	}

	m.mu.Lock()
	m.lastErr = ""
	m.since = time.Now()
	m.settings.LastMode = SubModeFast
	m.settings.LastFastSubMode = string(mode)
	// Persist the strategy that actually ran, so the picker preselects it and
	// reconnect-on-launch resumes with it. The controller has resolved any
	// empty/stale ID by now; read it back from its status.
	if s := m.fast.Status().Strategy; s != "" {
		m.settings.LastFastStrategy = s
	} else {
		m.settings.LastFastStrategy = strategyID
	}
	m.settings.LastFastDomains = domains
	settingsToSave := m.settings
	m.mu.Unlock()
	m.persistSettings(settingsToSave)
	m.emitStatus()
	return nil
}

// RequestPrivateMode switches to Private Mode, first fully tearing down and
// verifying Fast Mode if it was active.
func (m *Manager) RequestPrivateMode() error {
	if err := m.beginTransition(); err != nil {
		return err
	}
	defer m.endTransition()

	if err := m.teardownOtherMode(SubModePrivate); err != nil {
		m.fail(err)
		return err
	}

	// Claim the sub-mode before connecting so onPrivateStatus attributes the
	// status callbacks Connect() fires internally to Private Mode instead of
	// dropping them while subMode is still stale.
	m.mu.Lock()
	m.subMode = SubModePrivate
	m.state = StatePrivateConnecting
	m.mu.Unlock()

	if err := m.private.Connect(); err != nil {
		m.fail(err)
		return err
	}

	m.mu.Lock()
	m.lastErr = ""
	m.since = time.Now()
	m.settings.LastMode = SubModePrivate
	settingsToSave := m.settings
	m.mu.Unlock()
	m.persistSettings(settingsToSave)
	m.emitStatus()
	return nil
}

// RequestIdle tears down whichever mode is active and verifies clean state.
func (m *Manager) RequestIdle() error {
	if m.log != nil {
		m.log.Info("request idle (stop) received")
	}
	if err := m.beginTransition(); err != nil {
		return err
	}
	defer m.endTransition()

	if err := m.teardownOtherMode(SubModeNone); err != nil {
		m.fail(err)
		return err
	}

	m.mu.Lock()
	m.subMode = SubModeNone
	m.state = StateIdle
	m.lastErr = ""
	m.since = time.Time{}
	m.settings.LastMode = SubModeNone
	settingsToSave := m.settings
	m.mu.Unlock()
	m.persistSettings(settingsToSave)
	m.emitStatus()
	return nil
}

// displayName renders a SubMode as the same Title Case name used throughout
// the rest of the app's user-facing copy ("Fast Mode"/"Private Mode"),
// rather than its raw lowercase string value.
func displayName(s SubMode) string {
	switch s {
	case SubModeFast:
		return "Fast Mode"
	case SubModePrivate:
		return "Private Mode"
	default:
		return "the active mode"
	}
}

// teardownOtherMode fully stops whichever mode is active and isn't target,
// then verifies the teardown actually left clean DNS/WFP state. Call sites
// must hold the transition lock (via beginTransition).
func (m *Manager) teardownOtherMode(target SubMode) error {
	m.mu.Lock()
	current := m.subMode
	m.mu.Unlock()

	if current == SubModeNone || current == target {
		return nil
	}

	var err error
	switch current {
	case SubModeFast:
		err = m.fast.Stop()
	case SubModePrivate:
		err = m.private.Disconnect()
	}
	if err != nil {
		return fmt.Errorf("tear down %s: %w", displayName(current), err)
	}
	return m.verifyClean()
}

// verifyClean checks live signals — not just the post-hoc state fields,
// which both controllers reset unconditionally even on a partially failed
// teardown — that the previous mode genuinely left no DNS override or WFP
// block behind before the next mode is allowed to start.
func (m *Manager) verifyClean() error {
	if m.fast.DNSBackupPending() {
		return fmt.Errorf("Fast Mode teardown incomplete: DNS override still pending restore")
	}
	if m.private.KillSwitchArmed() {
		return fmt.Errorf("Private Mode teardown incomplete: kill switch still armed")
	}
	return nil
}

// DisarmKillSwitch removes WFP leak-protection filters without tearing down
// the tunnel. Manual "restore internet" control.
func (m *Manager) DisarmKillSwitch() error {
	if err := m.private.DisarmKillSwitch(); err != nil {
		return err
	}
	m.emitStatus()
	return nil
}

// SetReconnectOnLaunch toggles and persists the reconnect-on-launch
// preference.
func (m *Manager) SetReconnectOnLaunch(v bool) error {
	m.mu.Lock()
	m.settings.ReconnectOnLaunch = v
	s := m.settings
	m.mu.Unlock()
	return m.persistSettings(s)
}

func (m *Manager) persistSettings(s Settings) error {
	if err := saveSettings(m.stateDataDir, s); err != nil {
		if m.log != nil {
			m.log.Error("failed to persist state machine settings", "error", err)
		}
		return err
	}
	return nil
}

// onFastStatus is Fast Mode's status callback. It only drives the top-level
// state while Fast Mode is the active sub-mode — an emission that arrives
// after a mode switch has already moved on (e.g. a late supervisor event
// from the just-stopped controller) is recorded for status detail but does
// not move the top-level state.
func (m *Manager) onFastStatus(s fastmode.Status) {
	m.mu.Lock()
	m.lastFast = s
	if m.subMode == SubModeFast {
		switch s.State {
		case fastmode.StateFailed:
			m.state = StateError
			m.lastErr = s.Error
		case fastmode.StateStopped:
			m.state = StateIdle
			m.subMode = SubModeNone
		default:
			m.state = StateFastActive
		}
	}
	m.mu.Unlock()
	m.emitStatus()
}

// onPrivateStatus is Private Mode's status callback. Same "only drive
// top-level state while active" rule as onFastStatus — this is what lets an
// async handshake drop (StateNoHandshake, reached via the monitor
// goroutine's giveUp(), not through a Request* call) correctly flip the
// top-level state to Error outside of any Request* call.
func (m *Manager) onPrivateStatus(s privatemode.Status) {
	m.mu.Lock()
	m.lastPrivate = s
	if m.subMode == SubModePrivate {
		switch s.State {
		case privatemode.StateConnecting:
			m.state = StatePrivateConnecting
		case privatemode.StateConnected:
			m.state = StatePrivateActive
		case privatemode.StateNoHandshake, privatemode.StateError:
			m.state = StateError
			m.lastErr = s.Error
		case privatemode.StateDisconnected:
			m.state = StateIdle
			m.subMode = SubModeNone
		}
	}
	m.mu.Unlock()
	m.emitStatus()
}

// Shutdown is the app-exit hook: tear both modes down (each controller's own
// Shutdown is itself an unconditional DNS/WFP-restore backstop) and persist
// settings.
// Shutdown is safe to call more than once (only the first call does
// anything) — it now has multiple independent triggers: a normal quit, the
// main.go safety-net defer, and a Windows session-end callback.
func (m *Manager) Shutdown() {
	m.shutdownOnce.Do(func() {
		m.fast.Shutdown()
		m.private.Shutdown()
		m.mu.Lock()
		s := m.settings
		m.mu.Unlock()
		m.persistSettings(s)
	})
}
