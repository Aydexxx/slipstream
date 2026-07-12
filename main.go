package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"slipstream/backend/app"
	"slipstream/backend/cleanup"
	"slipstream/backend/elevate"
	"slipstream/backend/engine"
	"slipstream/backend/fastmode"
	"slipstream/backend/logging"
	"slipstream/backend/privatemode"
	"slipstream/backend/sessionwatch"
	"slipstream/backend/singleinstance"
	"slipstream/backend/statemachine"
	"slipstream/backend/tray"
	"slipstream/backend/version"
)

//go:embed all:frontend/dist
var assets embed.FS

const appName = "Slipstream"

// Fixed GUID-suffixed name so it can't collide with an unrelated app's mutex.
const mutexName = "Slipstream-8f21c9d4-4b7a-4e3e-9c2b-3f0a6e6b9d10"

func main() {
	if !elevate.IsElevated() {
		if err := elevate.RelaunchElevated(); err != nil {
			fmt.Println("failed to relaunch elevated:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Uninstall stages run before any logging/single-instance setup, so they
	// never open handles to (or lock) the files and directories they are about
	// to delete. Elevation above re-passes os.Args, so these flags survive the
	// UAC relaunch. Finalize is checked first: it runs from the %TEMP% copy and
	// must win even if both flags were somehow present.
	if cleanup.HasFlag(os.Args[1:], cleanup.FlagUninstallFinalize) {
		cleanup.RunUninstallFinalize(os.Args, appName)
		return
	}
	if cleanup.HasFlag(os.Args[1:], cleanup.FlagUninstall) {
		if err := cleanup.RunUninstallBootstrap(); err != nil {
			fmt.Println("failed to start uninstaller:", err)
			os.Exit(1)
		}
		return
	}

	logger, closeLog, err := logging.Init(appName)
	if err != nil {
		fmt.Println("failed to init logging:", err)
		os.Exit(1)
	}
	defer closeLog()

	guard, alreadyRunning, err := singleinstance.Acquire(mutexName)
	if err != nil {
		logger.Error("failed to acquire single-instance guard", "error", err)
		os.Exit(1)
	}
	if alreadyRunning {
		logger.Warn("another instance is already running, exiting")
		singleinstance.NotifyAlreadyRunning(appName)
		os.Exit(0)
	}
	defer guard.Release()

	logger.Info("starting slipstream", "version", version.Version, "commit", version.GitCommit, "buildDate", version.BuildDate)

	exePath, err := os.Executable()
	if err != nil {
		// Non-fatal: only the "start with Windows" toggle needs this.
		logger.Warn("failed to resolve own executable path; autostart will be unavailable", "error", err)
	}
	// Launched via the Run key (see backend/autostart): start hidden in the
	// tray rather than popping the window up on every login.
	startHidden := slices.Contains(os.Args[1:], "--autostart")

	engineManager, err := engine.New(logger)
	if err != nil {
		logger.Error("failed to init engine manager", "error", err)
		os.Exit(1)
	}
	if err := engineManager.EnsureExtracted(); err != nil {
		logger.Error("failed to extract engine binaries", "error", err)
		os.Exit(1)
	}

	// Per-user data/log locations, siblings of the engine dir.
	slipstreamRoot := filepath.Join(os.Getenv("LOCALAPPDATA"), appName)
	fastDataDir := filepath.Join(slipstreamRoot, "fastmode")
	fastLogDir := filepath.Join(slipstreamRoot, "logs")
	privateDataDir := filepath.Join(slipstreamRoot, "private")
	stateDataDir := filepath.Join(slipstreamRoot, "state")

	fastController, err := fastmode.New(fastmode.Config{
		Log:     logger,
		Engine:  engineManager,
		DataDir: fastDataDir,
		LogDir:  fastLogDir,
	})
	if err != nil {
		logger.Error("failed to init fast mode controller", "error", err)
		os.Exit(1)
	}

	privateController, err := privatemode.New(privatemode.Options{
		Log:     logger,
		Engine:  engineManager,
		DataDir: privateDataDir,
	})
	if err != nil {
		logger.Error("failed to init private mode controller", "error", err)
		os.Exit(1)
	}

	sm, err := statemachine.New(statemachine.Config{
		Log:            logger,
		Fast:           fastController,
		Private:        privateController,
		StateDataDir:   stateDataDir,
		FastDataDir:    fastDataDir,
		PrivateDataDir: privateDataDir,
		AmneziaWGPath:  engineManager.AmneziaWGPath(),
	})
	if err != nil {
		logger.Error("failed to init state machine", "error", err)
		os.Exit(1)
	}

	// Crash-safe backstop, run before the UI comes up: detect and clean any
	// leftover state from a previous run that crashed or was hard-killed —
	// an orphaned winws.exe, a pending DNS restore, leftover WFP kill-switch
	// filters (restored first, since that's what gets the user back online),
	// and a leftover AmneziaWG tunnel service.
	sm.Reconcile()

	// Catches Windows logging off or shutting down while a mode is active,
	// so DNS/routes/filters are restored before the OS kills the process
	// rather than being left dirty until the next launch's Reconcile.
	stopSessionWatch, err := sessionwatch.Watch(sm.Shutdown)
	if err != nil {
		logger.Error("failed to start session-end watcher; a logoff/shutdown mid-session may leave network state dirty until next launch", "error", err)
	} else {
		defer stopSessionWatch()
	}

	// Final safety net: whatever happens below, tear down whichever mode is
	// active (restore DNS + routing + WFP) on the way out. Shutdown is
	// idempotent, so this is harmless even after Wails' own OnShutdown (or
	// the session-end watcher above) already ran it.
	defer sm.Shutdown()

	application := app.New(logger, engineManager, sm, logging.LogDir(appName), appName, exePath)

	// stopTray safely calls Stop() on whichever Tray has been published so
	// far (nil before it's built) — shared between the wails goroutine below
	// and the "wails already exited" fallback further down, so a Tray
	// published after wails.Run() has already returned still gets stopped
	// instead of blocking Run() forever.
	var trayMu sync.Mutex
	var trayHandle *tray.Tray
	stopTray := func() {
		trayMu.Lock()
		t := trayHandle
		trayMu.Unlock()
		if t != nil {
			t.Stop()
		}
	}
	publishTray := func(t *tray.Tray) {
		trayMu.Lock()
		trayHandle = t
		trayMu.Unlock()
	}

	wailsExited := make(chan struct{})

	// getlantern/systray pins the process's original goroutine (via a
	// package-level init()) to its OS thread and expects its Run() to
	// execute there — see backend/tray's package doc comment. Wails has no
	// such requirement (its winc package locks whichever OS thread it needs
	// per call site), so wails.Run moves to a goroutine to leave the
	// original goroutine free for the tray below.
	go func() {
		err := wails.Run(&options.App{
			Title:  appName,
			Width:  1024,
			Height: 768,
			AssetServer: &assetserver.Options{
				Assets: assets,
			},
			BackgroundColour:  &options.RGBA{R: 27, G: 38, B: 54, A: 1},
			OnStartup:         application.Startup,
			OnShutdown:        application.Shutdown,
			HideWindowOnClose: true,
			StartHidden:       startHidden,
			Bind: []interface{}{
				application,
			},
		})
		if err != nil {
			logger.Error("wails run failed", "error", err)
		}
		close(wailsExited)
		stopTray()
	}()

	// Wait for Startup to hand back the frontend context, but don't hang
	// forever if wails.Run fails before Startup ever runs.
	var ctx context.Context
	select {
	case ctx = <-application.Ready():
	case <-wailsExited:
		logger.Error("wails exited before startup completed; shutting down")
		return
	}

	trayHandle = tray.New(tray.Config{
		Log:     logger,
		Manager: sm,
		AppName: appName,
		ExePath: exePath,
		ShowWindow: func() {
			wailsruntime.WindowShow(ctx)
			wailsruntime.WindowUnminimise(ctx)
		},
		Quit: func() {
			wailsruntime.Quit(ctx)
		},
		ResetAndQuit: application.ResetAndQuit,
	})
	publishTray(trayHandle)
	application.SetTrayUpdater(trayHandle.UpdateStatus)

	select {
	case <-wailsExited:
		// wails.Run already returned (before the Tray above existed to be
		// stopped) — stop it now instead of blocking Run() forever.
		trayHandle.Stop()
	default:
	}
	trayHandle.Run() // blocks until Stop() (above) or the tray's own Quit item
}
