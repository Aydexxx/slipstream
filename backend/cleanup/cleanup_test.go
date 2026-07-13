package cleanup

import (
	"os"
	"path/filepath"
	"testing"
)

// NOTE: these tests only exercise the pure/local helpers. RestoreNetworkState
// and PurgeTraces drive real netsh/sc/service teardown and must never run
// in a unit test — they are covered by the manual verification procedure in
// docs/UNINSTALL-VERIFICATION.md.

func TestHasFlag(t *testing.T) {
	args := []string{"slipstream.exe", "--autostart", "--uninstall"}
	if !HasFlag(args, FlagUninstall) {
		t.Error("expected --uninstall to be found")
	}
	if HasFlag(args, FlagUninstallFinalize) {
		t.Error("did not expect --uninstall-finalize")
	}
}

func TestFinalizeArg(t *testing.T) {
	args := []string{"tmp.exe", FlagUninstallFinalize, `C:\Program Files\Company\Slipstream\slipstream.exe`}
	if got := finalizeArg(args); got != `C:\Program Files\Company\Slipstream\slipstream.exe` {
		t.Errorf("finalizeArg = %q", got)
	}
	if got := finalizeArg([]string{"tmp.exe", FlagUninstallFinalize}); got != "" {
		t.Errorf("finalizeArg with no value = %q, want empty", got)
	}
	if got := finalizeArg([]string{"tmp.exe"}); got != "" {
		t.Errorf("finalizeArg with no flag = %q, want empty", got)
	}
}

func TestDefaultDepsLayout(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\test\AppData\Local`)
	d := DefaultDeps("Slipstream", `C:\install\slipstream.exe`, nil)
	if d.RootDir != `C:\Users\test\AppData\Local\Slipstream` {
		t.Errorf("RootDir = %q", d.RootDir)
	}
	if d.FastDataDir != filepath.Join(d.RootDir, "fastmode") {
		t.Errorf("FastDataDir = %q", d.FastDataDir)
	}
	if d.InstallExePath != `C:\install\slipstream.exe` {
		t.Errorf("InstallExePath = %q", d.InstallExePath)
	}
}

func TestLooksLikeInstallDir(t *testing.T) {
	t.Setenv("ProgramFiles", `C:\Program Files`)
	t.Setenv("LOCALAPPDATA", `C:\Users\test\AppData\Local`)

	cases := map[string]bool{
		`C:\Program Files\Company\Slipstream`:             true,
		`C:\Users\test\AppData\Local\Programs\Slipstream`: true,
		`C:\dev\slipstream\build\bin`:                     false,
		`D:\somewhere\else`:                               false,
	}
	for dir, want := range cases {
		if got := looksLikeInstallDir(dir); got != want {
			t.Errorf("looksLikeInstallDir(%q) = %v, want %v", dir, got, want)
		}
	}
}

// removeInstallDir must refuse to delete a directory that isn't a recognized
// install location — the guard against nuking a dev/build tree.
func TestRemoveInstallDirRefusesNonInstallLocation(t *testing.T) {
	t.Setenv("ProgramFiles", `C:\Program Files`)
	t.Setenv("LOCALAPPDATA", `C:\Users\test\AppData\Local`)

	dir := t.TempDir()
	exe := filepath.Join(dir, "slipstream.exe")
	if err := os.WriteFile(exe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeInstallDir(exe); err != nil {
		t.Fatalf("removeInstallDir returned error: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("removeInstallDir deleted a non-install directory (must not): %v", err)
	}
}

func TestRemoveInstallDirBlankIsNoOp(t *testing.T) {
	if err := removeInstallDir(""); err != nil {
		t.Errorf("removeInstallDir(\"\") = %v, want nil", err)
	}
}

// removeShortcuts should delete "<AppName>.lnk" under a Start-Menu-style path
// and treat missing files as success.
func TestRemoveShortcuts(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	// Point the other shortcut roots at empty temp dirs so the test is hermetic.
	t.Setenv("ProgramData", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("PUBLIC", t.TempDir())

	startMenu := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
	if err := os.MkdirAll(startMenu, 0o755); err != nil {
		t.Fatal(err)
	}
	lnk := filepath.Join(startMenu, "Slipstream.lnk")
	if err := os.WriteFile(lnk, []byte("shortcut"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeShortcuts("Slipstream"); err != nil {
		t.Fatalf("removeShortcuts error = %v", err)
	}
	if _, err := os.Stat(lnk); !os.IsNotExist(err) {
		t.Errorf("shortcut was not removed: %v", err)
	}
	// A second call (nothing present) must be a clean no-op.
	if err := removeShortcuts("Slipstream"); err != nil {
		t.Errorf("removeShortcuts second call = %v, want nil", err)
	}
}

func TestRemoveDirMissingIsError(t *testing.T) {
	// RemoveAll of a non-existent path is nil in Go; confirm our wrapper agrees.
	if err := removeDir(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("removeDir(missing) = %v, want nil", err)
	}
	if err := removeDir(""); err != nil {
		t.Errorf("removeDir(\"\") = %v, want nil", err)
	}
}

func TestHasFailures(t *testing.T) {
	if HasFailures([]StepResult{{Name: "a"}, {Name: "b"}}) {
		t.Error("no errors should report no failures")
	}
	if !HasFailures([]StepResult{{Name: "a"}, {Name: "b", Err: os.ErrPermission}}) {
		t.Error("an error should report a failure")
	}
}
