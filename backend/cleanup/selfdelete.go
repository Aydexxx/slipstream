package cleanup

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Uninstall command-line flags. The whole app self-elevates at launch and
// re-passes os.Args across the UAC relaunch, so these survive elevation.
const (
	// FlagUninstall is the bootstrap stage: copy self to %TEMP% and relaunch
	// the copy in finalize mode. Run from the install location.
	FlagUninstall = "--uninstall"
	// FlagUninstallFinalize is the second stage, run from the temp copy: wait
	// for the app to exit, purge everything, then delete the temp copy.
	FlagUninstallFinalize = "--uninstall-finalize"

	tempCopyName = "slipstream-uninstall.exe"

	// Windows process-creation flags (not in syscall for older Go): keep the
	// helper detached and windowless so it outlives us with no console flash.
	createNoWindow  = 0x08000000
	detachedProcess = 0x00000008
)

// HasFlag reports whether args contains flag.
func HasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// finalizeArg returns the original-exe path passed after
// FlagUninstallFinalize, or "" if absent.
func finalizeArg(args []string) string {
	for i, a := range args {
		if a == FlagUninstallFinalize && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// RunUninstallBootstrap copies the running exe to %TEMP% and relaunches that
// copy in finalize mode, passing the original exe's path. The copy is what
// deletes the original install location and app data — the running exe can't
// delete itself, and the live app locks its own files, so the copy-out is
// what makes a clean self-delete possible. The caller must exit after this.
func RunUninstallBootstrap() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve own path: %w", err)
	}
	tempCopy := filepath.Join(os.TempDir(), tempCopyName)
	if err := copyFile(self, tempCopy); err != nil {
		return fmt.Errorf("copy uninstaller to temp: %w", err)
	}

	cmd := exec.Command(tempCopy, FlagUninstallFinalize, self)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow | detachedProcess}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch finalize uninstaller: %w", err)
	}
	return cmd.Process.Release()
}

// RunUninstallFinalize runs from the temp copy. It waits for the app to exit,
// purges every trace, then schedules deletion of the temp copy itself. Logs to
// %TEMP% because the normal log directory is one of the things being deleted.
func RunUninstallFinalize(args []string, appName string) {
	log := newTempLogger()
	log.Info("uninstall finalize starting")

	origExe := finalizeArg(args)
	waitForAppExit(appName, log)

	results := PurgeTraces(DefaultDeps(appName, origExe, log))
	for _, s := range results {
		if s.Err != nil {
			log.Warn("residual after purge", "step", s.Name, "error", s.Err)
		}
	}
	if HasFailures(results) {
		log.Warn("uninstall completed with residual items; see entries above")
	} else {
		log.Info("uninstall completed cleanly; no traces remain")
	}

	if err := scheduleSelfDelete(); err != nil {
		log.Warn("could not schedule self-delete of the uninstaller helper", "error", err)
	}
}

// SpawnUninstaller launches "<exePath> --uninstall" detached, for the in-app
// Uninstall button. The caller (the running app) should quit right after, so
// the finalize stage can delete its files.
func SpawnUninstaller(exePath string) error {
	cmd := exec.Command(exePath, FlagUninstall)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow | detachedProcess}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn uninstaller: %w", err)
	}
	return cmd.Process.Release()
}

// waitForAppExit polls for the app image to disappear, then force-kills any
// straggler as a backstop. The app image is "<appName>.exe"; our temp copy is
// slipstream-uninstall.exe (a different image name), so the taskkill backstop
// can never target this process.
func waitForAppExit(appName string, log *slog.Logger) {
	image := strings.ToLower(appName) + ".exe"
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if !imageRunning(image) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Warn("app still running after grace period; force-terminating", "image", image)
	_ = exec.Command("taskkill", "/IM", image, "/F").Run()
	time.Sleep(1 * time.Second) // let file handles release
}

// imageRunning reports whether a process with the given image name is running.
func imageRunning(image string) bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image, "/NH").Output()
	if err != nil {
		return false // if we can't tell, don't block the uninstall
	}
	return strings.Contains(strings.ToLower(string(out)), image)
}

// scheduleSelfDelete spawns a detached cmd that waits a moment (so this process
// can exit and release the lock on its own image) and then deletes the temp
// copy. CmdLine is set explicitly so the shell operators survive intact.
func scheduleSelfDelete() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	// ping as a portable sleep (~2s), then delete our own image.
	line := fmt.Sprintf(`cmd /c ping 127.0.0.1 -n 3 >nul & del /f /q "%s"`, self)
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow | detachedProcess, CmdLine: line}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// newTempLogger writes uninstall diagnostics to %TEMP%\slipstream-uninstall.log
// (the normal log dir is being deleted). Falls back to stderr if the file
// can't be opened.
func newTempLogger() *slog.Logger {
	path := filepath.Join(os.TempDir(), "slipstream-uninstall.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
