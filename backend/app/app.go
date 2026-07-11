// Package app holds the Wails-bound App struct exposed to the frontend.
package app

import (
	"context"
	"log/slog"
	"runtime"

	"slipstream/backend/elevate"
	"slipstream/backend/engine"
	"slipstream/backend/version"
)

// App is the Go-side application struct bound to the frontend runtime.
type App struct {
	ctx    context.Context
	log    *slog.Logger
	engine *engine.Manager
}

// New creates the App struct.
func New(log *slog.Logger, em *engine.Manager) *App {
	return &App{log: log, engine: em}
}

// Startup is called by Wails once the frontend is ready.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.log.Info("frontend ready", "os", runtime.GOOS, "arch", runtime.GOARCH)
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
