// Package app holds the Wails-bound App struct exposed to the frontend.
package app

import (
	"context"
	"log/slog"
	"os/exec"
	"runtime"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"slipstream/backend/autostart"
	"slipstream/backend/cleanup"
	"slipstream/backend/elevate"
	"slipstream/backend/engine"
	"slipstream/backend/fastmode"
	"slipstream/backend/privatemode"
	"slipstream/backend/statemachine"
	"slipstream/backend/version"
)

// App is the Go-side application struct bound to the frontend runtime.
type App struct {
	ctx     context.Context
	log     *slog.Logger
	engine  *engine.Manager
	sm      *statemachine.Manager
	logDir  string
	appName string
	exePath string

	// ready carries the frontend context out to main.go once Startup has
	// run — main.go needs it to build the tray's ShowWindow/Quit actions,
	// but can't get it any other way since the tray is constructed outside
	// the Wails lifecycle. Buffered so Startup never blocks sending it.
	ready chan context.Context

	trayMu      sync.Mutex
	trayUpdater func(statemachine.Status)
}

// New creates the App struct. appName/exePath are used for the "start with
// Windows" autostart toggle.
func New(log *slog.Logger, em *engine.Manager, sm *statemachine.Manager, logDir, appName, exePath string) *App {
	return &App{
		log:     log,
		engine:  em,
		sm:      sm,
		logDir:  logDir,
		appName: appName,
		exePath: exePath,
		ready:   make(chan context.Context, 1),
	}
}

// Ready receives the frontend context once, after Startup has run. Exposed
// as a channel (rather than a blocking method) so main.go can select on it
// alongside "wails.Run already returned" — otherwise a Wails startup failure
// before Startup ever fires would hang main() forever waiting for a context
// that's never coming.
func (a *App) Ready() <-chan context.Context {
	return a.ready
}

// SetTrayUpdater registers the callback that pushes status snapshots to the
// tray icon/menu. Safe to call at any time; nil-safe if never called (the
// tray simply won't reflect live status, e.g. if it failed to start).
func (a *App) SetTrayUpdater(fn func(statemachine.Status)) {
	a.trayMu.Lock()
	a.trayUpdater = fn
	a.trayMu.Unlock()
}

// Startup is called by Wails once the frontend is ready.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.log.Info("frontend ready", "os", runtime.GOOS, "arch", runtime.GOARCH)

	// Push the unified state to the frontend as it changes, and to the tray
	// (once SetTrayUpdater has been called — nil until then). This is the
	// single event the frontend needs: state, sub-mode, connection health,
	// kill-switch status, and human-readable errors all live on one Status.
	a.sm.SetEmitter(func(s statemachine.Status) {
		wailsruntime.EventsEmit(ctx, "state:status", s)
		a.trayMu.Lock()
		upd := a.trayUpdater
		a.trayMu.Unlock()
		if upd != nil {
			upd(s)
		}
	})

	// Resume the user's last mode, if they opted in. Connect/Start can take
	// several seconds, so this must not block the frontend becoming ready.
	go a.sm.MaybeReconnectLastMode()

	a.ready <- ctx
}

// Shutdown is called by Wails when the app is closing. It tears down
// whichever mode is active — restoring DNS, routing, and WFP state — via the
// state machine, which is itself backed by each controller's own
// unconditional Shutdown backstop. See main.go for the crash-safe
// RecoverPendingDNS / killswitch.Reconcile / RecoverLeftoverTunnel path run
// at the next launch in case this one doesn't get to run, and the Windows
// session-end (logoff/shutdown) backstop.
func (a *App) Shutdown(ctx context.Context) {
	a.sm.Shutdown()
}

// GetVersion returns the running application version.
func (a *App) GetVersion() string {
	return version.Version
}

// Ping is a minimal round-trip check for the Go<->frontend bridge.
func (a *App) Ping() string {
	return "pong"
}

// IsElevated reports whether the process is running with administrator privileges.
func (a *App) IsElevated() bool {
	return elevate.IsElevated()
}

// VerifyFastModeEngine checks the extracted zapret/WinDivert files against
// the hardcoded hash manifest. A non-nil error means Fast Mode must not run.
func (a *App) VerifyFastModeEngine() error {
	return a.engine.Verify(engine.ModeFast)
}

// VerifyPrivateModeEngine checks the extracted AmneziaWG/wintun files
// against the hardcoded hash manifest. A non-nil error means Private Mode
// must not run.
func (a *App) VerifyPrivateModeEngine() error {
	return a.engine.Verify(engine.ModePrivate)
}

// --- unified state machine bindings ---

// RequestFastMode switches to Fast Mode. mode is "full", "discord", or
// "custom"; domains is only used for "custom" (the resolved list of hosts to
// unblock). If Private Mode is active, it is fully torn down and verified
// clean first.
func (a *App) RequestFastMode(mode string, domains []string) error {
	return a.sm.RequestFastMode(fastmode.Mode(mode), domains)
}

// RequestPrivateMode switches to Private Mode. If Fast Mode is active, it is
// fully torn down and verified clean first.
func (a *App) RequestPrivateMode() error {
	return a.sm.RequestPrivateMode()
}

// RequestIdle tears down whichever mode is active and verifies clean
// DNS/WFP state.
func (a *App) RequestIdle() error {
	return a.sm.RequestIdle()
}

// State returns a snapshot of the unified state: coarse state, active
// sub-mode, connection health, kill-switch status, and any human-readable
// error — the single source of truth for the frontend.
func (a *App) State() statemachine.Status {
	return a.sm.Status()
}

// DisarmKillSwitch is the manual "restore internet" control: it removes the
// leak-protection filters immediately without tearing down the tunnel. Use
// it to recover connectivity if a dropped tunnel has left the machine failed
// closed.
func (a *App) DisarmKillSwitch() error {
	return a.sm.DisarmKillSwitch()
}

// SetReconnectOnLaunch toggles whether Slipstream should resume the last
// active mode automatically on the next launch.
func (a *App) SetReconnectOnLaunch(enabled bool) error {
	return a.sm.SetReconnectOnLaunch(enabled)
}

// --- Fast Mode config bindings ---

// FastModePresets returns the bundled preset domain groups keyed by name.
func (a *App) FastModePresets() map[string][]string {
	return a.sm.Fast().Presets()
}

// GetCustomDomains returns the user's persisted custom domain list.
func (a *App) GetCustomDomains() ([]string, error) {
	return a.sm.Fast().LoadCustomDomains()
}

// SaveCustomDomains persists the user's custom domain list.
func (a *App) SaveCustomDomains(domains []string) error {
	return a.sm.Fast().SaveCustomDomains(domains)
}

// --- Private Mode config bindings ---

// ImportPrivateConfig validates and stores (DPAPI-encrypted) the user's
// AmneziaWG config, returning a key-free summary of it.
func (a *App) ImportPrivateConfig(raw string) (privatemode.Summary, error) {
	return a.sm.Private().ImportConfig(raw)
}

// HasPrivateConfig reports whether an AmneziaWG config has been imported.
func (a *App) HasPrivateConfig() bool {
	return a.sm.Private().HasConfig()
}

// PrivateConfigSummary returns the key-free summary of the imported config.
func (a *App) PrivateConfigSummary() (privatemode.Summary, error) {
	return a.sm.Private().ConfigSummary()
}

// DeletePrivateConfig removes the stored AmneziaWG config (must be disconnected).
func (a *App) DeletePrivateConfig() error {
	return a.sm.Private().DeleteConfig()
}

// GetExternalIP reports the address Private Mode's traffic currently appears
// to come from. It only succeeds while genuinely connected with the kill
// switch armed and a fresh handshake — see privatemode.Controller.ExternalIP.
func (a *App) GetExternalIP() (string, error) {
	return a.sm.Private().ExternalIP(a.ctx)
}

// --- misc bindings ---

// OpenLogsFolder opens the directory containing Slipstream's rotating log
// files in Windows Explorer.
func (a *App) OpenLogsFolder() error {
	return exec.Command("explorer.exe", a.logDir).Start()
}

// GetAutoStartEnabled reports whether Slipstream is registered to launch at
// sign-in via the HKCU Run key.
func (a *App) GetAutoStartEnabled() (bool, error) {
	return autostart.IsEnabled(a.appName)
}

// ResetAndQuit tears the active mode down and then runs a full network/system
// state restore (DNS, DoH template, WFP kill switch, tunnel + WinDivert
// services, orphaned processes, leftover plaintext key) before quitting. It
// deletes no user files — it's the "put my networking back exactly as it was,
// then close" control. Bound to the frontend and reused as the tray's Reset &
// Quit action (main.go passes this method as the tray callback).
func (a *App) ResetAndQuit() {
	a.sm.Shutdown()
	results := cleanup.RestoreNetworkState(cleanup.DefaultDeps(a.appName, a.exePath, a.log))
	if cleanup.HasFailures(results) && a.log != nil {
		a.log.Warn("reset completed with residual network state; see step errors above")
	}
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
}

// Uninstall launches the standalone self-deleting uninstaller (a detached
// "slipstream.exe --uninstall") and then quits this instance so the
// uninstaller can restore networking and delete every trace — the
// %LocalAppData%\Slipstream tree, the autostart Run key, shortcuts, driver
// artifacts, and finally the app itself.
func (a *App) Uninstall() error {
	if err := cleanup.SpawnUninstaller(a.exePath); err != nil {
		if a.log != nil {
			a.log.Error("failed to launch uninstaller", "error", err)
		}
		return err
	}
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
	return nil
}

// SetAutoStart enables or disables launching Slipstream at sign-in. Enabling
// launches unelevated (the Run key can't elevate); Slipstream's own
// self-elevation takes over from there, at the cost of a UAC prompt on every
// boot.
func (a *App) SetAutoStart(enabled bool) error {
	if enabled {
		return autostart.Enable(a.appName, a.exePath, "--autostart")
	}
	return autostart.Disable(a.appName)
}
