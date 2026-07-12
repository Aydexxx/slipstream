// Package cleanup is the single source of truth for undoing every persistent
// change Slipstream makes. It powers three things: the in-app "Reset & Quit"
// (network-state restore only) and "Uninstall" actions, and the standalone
// self-deleting uninstaller (see selfdelete.go). Every reversal here reuses an
// existing, individually-tested primitive from the mode packages; this package
// only orchestrates them and adds the filesystem/registry/shortcut purge.
//
// Everything is best-effort: a single failing step never aborts the rest, and
// all steps are collected into a result list so the caller can log exactly
// what did and didn't come clean. Network-state restore always runs before any
// destructive file/registry deletion, so a mid-purge failure still leaves the
// user's networking intact.
package cleanup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"slipstream/backend/autostart"
	"slipstream/backend/fastmode"
	"slipstream/backend/killswitch"
	"slipstream/backend/privatemode"
)

// Deps carries the paths and identifiers the cleanup steps need. Build it with
// DefaultDeps for the standard %LocalAppData%\Slipstream layout.
type Deps struct {
	Log            *slog.Logger
	AppName        string // "Slipstream"
	RootDir        string // %LocalAppData%\Slipstream
	FastDataDir    string // RootDir\fastmode
	PrivateDataDir string // RootDir\private
	AmneziaWGPath  string // RootDir\engine\private\amneziawg.exe
	InstallExePath string // the installed exe, for removing its install dir (may be "")
}

// DefaultDeps computes the standard on-disk layout from LOCALAPPDATA.
func DefaultDeps(appName, installExePath string, log *slog.Logger) Deps {
	root := filepath.Join(os.Getenv("LOCALAPPDATA"), appName)
	return Deps{
		Log:            log,
		AppName:        appName,
		RootDir:        root,
		FastDataDir:    filepath.Join(root, "fastmode"),
		PrivateDataDir: filepath.Join(root, "private"),
		AmneziaWGPath:  filepath.Join(root, "engine", "private", "amneziawg.exe"),
		InstallExePath: installExePath,
	}
}

// StepResult records the outcome of one cleanup step.
type StepResult struct {
	Name string
	Err  error
}

// stepRunner accumulates results and logs failures as they happen.
type stepRunner struct {
	log     *slog.Logger
	results []StepResult
}

func (r *stepRunner) do(name string, fn func() error) {
	err := fn()
	r.results = append(r.results, StepResult{Name: name, Err: err})
	if err != nil && r.log != nil {
		r.log.Error("cleanup step failed", "step", name, "error", err)
	} else if err == nil && r.log != nil {
		r.log.Info("cleanup step done", "step", name)
	}
}

// RestoreNetworkState reverses every network/system-state change (DNS, DoH,
// WFP kill switch, tunnel service, WinDivert driver service, orphaned
// processes, leftover plaintext key). It deletes no user files. This is what
// "Reset & Quit" runs, and it is a superset of the startup Reconcile. The
// order restores connectivity first (kill switch / DNS) before touching the
// slower service/driver removals.
func RestoreNetworkState(d Deps) []StepResult {
	r := &stepRunner{log: d.Log}

	r.do("kill orphaned winws.exe", func() error { return fastmode.KillOrphanedProcesses(d.Log) })
	r.do("restore DNS", func() error { return fastmode.RecoverPendingDNS(d.FastDataDir, d.Log) })
	r.do("remove global DoH template", func() error { fastmode.RemoveGlobalDoHTemplate(d.Log); return nil })
	r.do("remove kill-switch WFP filters", func() error {
		return killswitch.Reconcile(filepath.Join(d.PrivateDataDir, "killswitch.marker"), d.Log)
	})
	r.do("remove AmneziaWG tunnel service", func() error { return privatemode.RecoverLeftoverTunnel(d.AmneziaWGPath, d.Log) })
	r.do("remove WinDivert driver service", func() error { return fastmode.RemoveWinDivertService(d.Log) })
	r.do("shred leftover plaintext config", func() error {
		privatemode.ShredLeftoverPlaintextConfig(d.PrivateDataDir, d.Log)
		return nil
	})

	return r.results
}

// PurgeTraces runs RestoreNetworkState and then removes every filesystem and
// registry trace: the HKCU autostart Run key, Start-Menu/Desktop shortcuts,
// any Add/Remove-Programs uninstall entry, the %LocalAppData%\Slipstream tree,
// and (if it looks like a real install location) the install directory. This
// is the full "zero traces" teardown, run from the self-deleting uninstaller.
func PurgeTraces(d Deps) []StepResult {
	results := RestoreNetworkState(d)
	r := &stepRunner{log: d.Log, results: results}

	r.do("remove autostart Run key", func() error { return autostart.Disable(d.AppName) })
	r.do("remove shortcuts", func() error { return removeShortcuts(d.AppName) })
	r.do("remove uninstall registry entries", func() error { return removeUninstallEntries(d.AppName) })
	r.do("delete app data directory", func() error { return removeDir(d.RootDir) })
	r.do("delete install directory", func() error { return removeInstallDir(d.InstallExePath) })

	return r.results
}

// HasFailures reports whether any step returned an error.
func HasFailures(results []StepResult) bool {
	for _, s := range results {
		if s.Err != nil {
			return true
		}
	}
	return false
}

// removeShortcuts deletes "<AppName>.lnk" from the per-user and all-users Start
// Menu and Desktop (the locations the NSIS installer writes to). Missing files
// are not errors.
func removeShortcuts(appName string) error {
	var errs []string
	for _, dir := range shortcutDirs() {
		p := filepath.Join(dir, appName+".lnk")
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func shortcutDirs() []string {
	var dirs []string
	if v := os.Getenv("APPDATA"); v != "" {
		dirs = append(dirs, filepath.Join(v, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if v := os.Getenv("ProgramData"); v != "" {
		dirs = append(dirs, filepath.Join(v, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if v := os.Getenv("USERPROFILE"); v != "" {
		dirs = append(dirs, filepath.Join(v, "Desktop"))
	}
	if v := os.Getenv("PUBLIC"); v != "" {
		dirs = append(dirs, filepath.Join(v, "Desktop"))
	}
	return dirs
}

// removeUninstallEntries deletes any Add/Remove-Programs registry entry whose
// DisplayName matches appName, under both HKCU and HKLM. Matching by
// DisplayName (rather than a fragile installer-generated key name) makes this
// robust regardless of how the app was installed.
func removeUninstallEntries(appName string) error {
	const uninstallPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	var errs []string
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, uninstallPath, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue // hive/path absent — nothing to remove here
		}
		subs, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, sub := range subs {
			subPath := uninstallPath + `\` + sub
			sk, err := registry.OpenKey(root, subPath, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			name, _, _ := sk.GetStringValue("DisplayName")
			sk.Close()
			if !strings.EqualFold(strings.TrimSpace(name), appName) {
				continue
			}
			if err := registry.DeleteKey(root, subPath); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", subPath, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// removeInstallDir deletes the directory containing installExePath, but only
// when it looks like a genuine install location (under Program Files or
// %LocalAppData%\Programs). This guards against nuking a development/build
// directory when the app is run in place. A blank path is a no-op.
func removeInstallDir(installExePath string) error {
	if installExePath == "" {
		return nil
	}
	dir := filepath.Dir(installExePath)
	if !looksLikeInstallDir(dir) {
		return nil
	}
	return removeDir(dir)
}

func looksLikeInstallDir(dir string) bool {
	lower := strings.ToLower(dir)
	markers := []string{
		strings.ToLower(os.Getenv("ProgramFiles")),
		strings.ToLower(os.Getenv("ProgramFiles(x86)")),
		strings.ToLower(filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs")),
	}
	for _, m := range markers {
		if m != "" && strings.HasPrefix(lower, m) {
			return true
		}
	}
	return false
}

func removeDir(path string) error {
	if path == "" {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
