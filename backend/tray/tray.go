// Package tray puts Slipstream in the Windows notification area: a dynamic
// icon reflecting live state, one-click mode toggles, and Quit. The window
// is secondary — this is the primary always-available surface.
//
// getlantern/systray pins whatever goroutine first runs its package init()
// (always the process's original main goroutine, per Go's init() semantics)
// to its OS thread and expects Run() to execute there — it has no per-call
// LockOSThread of its own. So Tray.Run must be called from that original
// goroutine, not a spawned one; see main.go for how wails.Run is moved to a
// goroutine instead to make room for this.
package tray

import (
	"log/slog"
	"sync"

	"github.com/getlantern/systray"

	"slipstream/backend/autostart"
	"slipstream/backend/statemachine"
)

// Config wires a Tray to its dependencies.
type Config struct {
	Log     *slog.Logger
	Manager *statemachine.Manager
	AppName string
	ExePath string

	// ShowWindow brings the (possibly hidden) main window to front.
	ShowWindow func()
	// Quit performs a full application quit (expected to trigger the normal
	// Wails shutdown path, which tears the state machine down).
	Quit func()
	// ResetAndQuit restores all network/system state (belt-and-suspenders,
	// beyond the active mode) and then quits. Optional; the item is hidden
	// when nil.
	ResetAndQuit func()
}

// Tray owns the tray icon and menu.
type Tray struct {
	cfg Config

	mu       sync.Mutex
	ready    bool
	last     statemachine.Status
	haveLast bool

	mOpen      *systray.MenuItem
	mOff       *systray.MenuItem
	mFast      *systray.MenuItem
	mPrivate   *systray.MenuItem
	mAutostart *systray.MenuItem
	mReconnect *systray.MenuItem
	mReset     *systray.MenuItem
	mQuit      *systray.MenuItem
}

// New constructs a Tray. Call Run to actually show it.
func New(cfg Config) *Tray {
	return &Tray{cfg: cfg}
}

// Run blocks until Quit is invoked (from the menu, or externally via
// systray.Quit()). Must run on the process's original goroutine — see the
// package doc comment.
func (t *Tray) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *Tray) onReady() {
	systray.SetIcon(offIcon)
	systray.SetTooltip("Slipstream — Off")

	t.mOpen = systray.AddMenuItem("Open Window", "Show the Slipstream window")
	systray.AddSeparator()
	t.mOff = systray.AddMenuItemCheckbox("Off", "Turn everything off", true)
	t.mFast = systray.AddMenuItemCheckbox("Fast Mode", "Defeat DPI without a tunnel", false)
	t.mPrivate = systray.AddMenuItemCheckbox("Private Mode", "Full obfuscated tunnel", false)
	systray.AddSeparator()

	autostartEnabled, err := autostart.IsEnabled(t.cfg.AppName)
	if err != nil && t.cfg.Log != nil {
		t.cfg.Log.Warn("tray: failed to read autostart state", "error", err)
	}
	t.mAutostart = systray.AddMenuItemCheckbox("Start with Windows", "Launch Slipstream at sign-in", autostartEnabled)

	reconnect := false
	if t.cfg.Manager != nil {
		reconnect = t.cfg.Manager.Status().ReconnectOnLaunch
	}
	t.mReconnect = systray.AddMenuItemCheckbox("Reconnect Last Mode on Launch", "Resume the last mode automatically", reconnect)

	systray.AddSeparator()
	t.mReset = systray.AddMenuItem("Reset & Quit", "Restore all network settings, then quit")
	if t.cfg.ResetAndQuit == nil {
		t.mReset.Hide()
	}
	t.mQuit = systray.AddMenuItem("Quit", "Quit Slipstream")

	// Publish readiness. The mutex here also safely publishes the menu-item
	// pointer assignments above to other goroutines (UpdateStatus,
	// handleClicks) that synchronize on the same mutex before reading them —
	// they only proceed once they observe ready == true.
	t.mu.Lock()
	t.ready = true
	last, have := t.last, t.haveLast
	t.mu.Unlock()

	if have {
		t.applyStatus(last)
	} else if t.cfg.Manager != nil {
		t.applyStatus(t.cfg.Manager.Status())
	}

	go t.handleClicks()
}

// Stop ends the tray's message loop (causing Run to return), for use when
// the rest of the app has already exited through some other path (e.g. the
// window's Quit) and the tray now needs to follow.
func (t *Tray) Stop() {
	systray.Quit()
}

func (t *Tray) onExit() {
	// No cleanup here: actual network teardown happens through the state
	// machine's own Shutdown, triggered independently of the tray.
}

// handleClicks dispatches menu clicks. Mode-toggle actions run in their own
// goroutine so a slow Start/Connect can't delay the tray from responding to
// the next click (in particular, Quit must always be immediately responsive).
func (t *Tray) handleClicks() {
	for {
		select {
		case <-t.mOpen.ClickedCh:
			if t.cfg.ShowWindow != nil {
				t.cfg.ShowWindow()
			}

		case <-t.mOff.ClickedCh:
			go func() {
				if err := t.cfg.Manager.RequestIdle(); err != nil && t.cfg.Log != nil {
					t.cfg.Log.Error("tray: turn off failed", "error", err)
				}
			}()

		case <-t.mFast.ClickedCh:
			go func() {
				mode, strategy, domains := t.cfg.Manager.LastFastSelection()
				if err := t.cfg.Manager.RequestFastMode(mode, strategy, domains); err != nil && t.cfg.Log != nil {
					t.cfg.Log.Error("tray: start fast mode failed", "error", err)
				}
			}()

		case <-t.mPrivate.ClickedCh:
			go func() {
				if err := t.cfg.Manager.RequestPrivateMode(); err != nil && t.cfg.Log != nil {
					t.cfg.Log.Error("tray: connect private mode failed", "error", err)
				}
			}()

		case <-t.mAutostart.ClickedCh:
			go t.toggleAutostart()

		case <-t.mReconnect.ClickedCh:
			go t.toggleReconnect()

		case <-t.mReset.ClickedCh:
			if t.cfg.ResetAndQuit != nil {
				t.cfg.ResetAndQuit()
			}
			return

		case <-t.mQuit.ClickedCh:
			if t.cfg.Quit != nil {
				t.cfg.Quit()
			}
			return
		}
	}
}

func (t *Tray) toggleAutostart() {
	enabled := t.mAutostart.Checked()
	var err error
	if enabled {
		err = autostart.Disable(t.cfg.AppName)
	} else {
		err = autostart.Enable(t.cfg.AppName, t.cfg.ExePath, "--autostart")
	}
	if err != nil {
		if t.cfg.Log != nil {
			t.cfg.Log.Error("tray: toggle autostart failed", "error", err)
		}
		return
	}
	if enabled {
		t.mAutostart.Uncheck()
	} else {
		t.mAutostart.Check()
	}
}

func (t *Tray) toggleReconnect() {
	if t.cfg.Manager == nil {
		return
	}
	enabled := t.mReconnect.Checked()
	if err := t.cfg.Manager.SetReconnectOnLaunch(!enabled); err != nil {
		if t.cfg.Log != nil {
			t.cfg.Log.Error("tray: toggle reconnect-on-launch failed", "error", err)
		}
		return
	}
	if enabled {
		t.mReconnect.Uncheck()
	} else {
		t.mReconnect.Check()
	}
}

// UpdateStatus applies a fresh status snapshot to the icon, tooltip, and
// mode checkboxes. Safe to call before onReady has finished (e.g. a status
// event racing tray startup) — it's cached and applied once ready.
func (t *Tray) UpdateStatus(s statemachine.Status) {
	t.mu.Lock()
	t.last, t.haveLast = s, true
	ready := t.ready
	t.mu.Unlock()
	if !ready {
		return
	}
	t.applyStatus(s)
}

func (t *Tray) applyStatus(s statemachine.Status) {
	systray.SetIcon(iconFor(s))
	systray.SetTooltip(tooltipFor(s))

	setChecked(t.mOff, s.SubMode == statemachine.SubModeNone)
	setChecked(t.mFast, s.SubMode == statemachine.SubModeFast)
	setChecked(t.mPrivate, s.SubMode == statemachine.SubModePrivate)
	setChecked(t.mReconnect, s.ReconnectOnLaunch)
}

func setChecked(item *systray.MenuItem, checked bool) {
	if item == nil {
		return
	}
	if checked {
		item.Check()
	} else {
		item.Uncheck()
	}
}

// iconFor picks the tray icon for a status snapshot. Any error takes
// priority; next, a kill switch that's armed but not backed by a healthy
// connection means traffic is currently fail-closed-blocked — worth flagging
// distinctly even before it becomes an outright error (e.g. while
// reconnecting after a drop).
func iconFor(s statemachine.Status) []byte {
	switch {
	case s.State == statemachine.StateError:
		return alertIcon
	case s.KillSwitchArmed && s.State != statemachine.StatePrivateActive:
		return alertIcon
	case s.State == statemachine.StatePrivateActive:
		return privateIcon
	case s.State == statemachine.StateFastActive:
		return fastIcon
	default:
		return offIcon
	}
}

func tooltipFor(s statemachine.Status) string {
	switch {
	case s.State == statemachine.StateError:
		return "Slipstream — Error"
	case s.KillSwitchArmed && s.State != statemachine.StatePrivateActive:
		return "Slipstream — Kill switch engaged"
	case s.State == statemachine.StatePrivateActive:
		return "Slipstream — Private Mode (connected)"
	case s.State == statemachine.StateFastActive:
		return "Slipstream — Fast Mode"
	default:
		return "Slipstream — Off"
	}
}
