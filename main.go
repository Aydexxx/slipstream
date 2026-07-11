package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"slipstream/backend/app"
	"slipstream/backend/elevate"
	"slipstream/backend/engine"
	"slipstream/backend/logging"
	"slipstream/backend/singleinstance"
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

	logger.Info("starting slipstream")

	engineManager, err := engine.New(logger)
	if err != nil {
		logger.Error("failed to init engine manager", "error", err)
		os.Exit(1)
	}
	if err := engineManager.EnsureExtracted(); err != nil {
		logger.Error("failed to extract engine binaries", "error", err)
		os.Exit(1)
	}

	application := app.New(logger, engineManager)

	err = wails.Run(&options.App{
		Title:  appName,
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        application.Startup,
		Bind: []interface{}{
			application,
		},
	})

	if err != nil {
		logger.Error("wails run failed", "error", err)
	}
}
