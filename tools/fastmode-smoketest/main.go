//go:build ignore

// fastmode-smoketest drives the real backend/fastmode package directly
// (Start -> wait -> Stop) so its DNS hijack/restore behavior can be smoke
// tested without needing the full Wails app or a censored network to prove
// the mechanism against a real blocked site. See docs/E2E-TESTING.md #1.
//
// Must run in an ELEVATED terminal (Fast Mode requires Administrator) and
// must NOT run while the real Slipstream app is also running - both would
// fight over the same DNS backup file and winws.exe.
//
// Usage (from the repo root, elevated PowerShell):
//
//	go run tools/fastmode-smoketest/main.go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"slipstream/backend/elevate"
	"slipstream/backend/engine"
	"slipstream/backend/fastmode"
)

func main() {
	if !elevate.IsElevated() {
		fmt.Println("This must run in an elevated (Administrator) terminal — Fast Mode needs it.")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	em, err := engine.New(logger)
	if err != nil {
		fatal("engine.New", err)
	}
	if err := em.EnsureExtracted(); err != nil {
		fatal("EnsureExtracted", err)
	}

	// Uses the same %LocalAppData%\Slipstream data directories the real app
	// uses, so this exercises the actual production DNS-backup path — not an
	// isolated copy. That's deliberate (a faithful smoke test), but it's why
	// this must not run concurrently with the real app.
	root := filepath.Join(os.Getenv("LOCALAPPDATA"), "Slipstream")
	c, err := fastmode.New(fastmode.Config{
		Log:     logger,
		Engine:  em,
		DataDir: filepath.Join(root, "fastmode"),
		LogDir:  filepath.Join(root, "logs"),
	})
	if err != nil {
		fatal("fastmode.New", err)
	}

	printDNS("BEFORE")

	fmt.Println("\nStarting Fast Mode (Full)...")
	if err := c.Start(fastmode.ModeFull, "", nil); err != nil {
		fatal("Start", err)
	}
	fmt.Println("Fast Mode started. Waiting 5s for DNS to settle...")
	time.Sleep(5 * time.Second)

	printDNS("DURING (Fast Mode active)")

	fmt.Println("\nPress Enter to stop Fast Mode and restore DNS...")
	fmt.Scanln()

	fmt.Println("Stopping Fast Mode...")
	if err := c.Stop(); err != nil {
		fmt.Println("Stop reported an error (see above) — DNS restore may be incomplete:", err)
	} else {
		fmt.Println("Fast Mode stopped cleanly.")
	}

	printDNS("AFTER")

	fmt.Println("\nCompare BEFORE and AFTER above — they must match exactly.")
	fmt.Println("If they don't, do NOT close this terminal; run:")
	fmt.Println(`  netsh interface ipv4 set dnsservers name="<your adapter>" dhcp`)
	fmt.Println("to manually restore DNS, then file a bug.")
}

func printDNS(label string) {
	fmt.Printf("\n--- DNS (%s) ---\n", label)
	out, err := exec.Command("netsh", "interface", "ipv4", "show", "dnsservers").CombinedOutput()
	if err != nil {
		fmt.Println("failed to query DNS:", err)
		return
	}
	fmt.Println(string(out))
}

func fatal(step string, err error) {
	fmt.Printf("%s failed: %v\n", step, err)
	os.Exit(1)
}
