package engine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	m, err := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return m
}

func TestEnsureExtractedThenVerifyPasses(t *testing.T) {
	m := newTestManager(t)

	if err := m.EnsureExtracted(); err != nil {
		t.Fatalf("EnsureExtracted() error = %v", err)
	}

	if err := m.Verify(ModeFast); err != nil {
		t.Errorf("Verify(ModeFast) error = %v, want nil", err)
	}
	if err := m.Verify(ModePrivate); err != nil {
		t.Errorf("Verify(ModePrivate) error = %v, want nil", err)
	}

	for _, p := range []string{m.WinwsPath(), m.WinDivertSysPath(), m.WinDivertDLLPath(), m.CygwinDLLPath(), m.AmneziaWGPath(), m.WintunDLLPath()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected extracted file at %s: %v", p, err)
		}
	}
}

func TestVerifyFailsOnMissingFile(t *testing.T) {
	m := newTestManager(t)

	if err := m.Verify(ModeFast); err == nil {
		t.Fatal("Verify(ModeFast) on a fresh, never-extracted dir: got nil error, want non-nil")
	}
}

func TestTamperingIsDetectedAndBlocksMode(t *testing.T) {
	m := newTestManager(t)

	if err := m.EnsureExtracted(); err != nil {
		t.Fatalf("EnsureExtracted() error = %v", err)
	}

	tampered := m.WinwsPath()
	data, err := os.ReadFile(tampered)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", tampered, err)
	}
	data = append(data, 0x00)
	if err := os.WriteFile(tampered, data, 0o755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", tampered, err)
	}

	if err := m.Verify(ModeFast); err == nil {
		t.Fatal("Verify(ModeFast) after tampering winws.exe: got nil error, want non-nil (mode should be blocked)")
	}

	// The other mode's files are untouched and must still verify fine.
	if err := m.Verify(ModePrivate); err != nil {
		t.Errorf("Verify(ModePrivate) error = %v, want nil (unaffected by ModeFast tampering)", err)
	}

	// A fresh EnsureExtracted should detect the mismatch and heal it.
	if err := m.EnsureExtracted(); err != nil {
		t.Fatalf("EnsureExtracted() (heal) error = %v", err)
	}
	if err := m.Verify(ModeFast); err != nil {
		t.Errorf("Verify(ModeFast) after healing error = %v, want nil", err)
	}
}

func TestVerifyFailsOnDeletedFile(t *testing.T) {
	m := newTestManager(t)

	if err := m.EnsureExtracted(); err != nil {
		t.Fatalf("EnsureExtracted() error = %v", err)
	}
	if err := os.Remove(m.WintunDLLPath()); err != nil {
		t.Fatalf("Remove(%s) error = %v", m.WintunDLLPath(), err)
	}

	if err := m.Verify(ModePrivate); err == nil {
		t.Fatal("Verify(ModePrivate) after deleting wintun.dll: got nil error, want non-nil")
	}

	if err := m.EnsureExtracted(); err != nil {
		t.Fatalf("EnsureExtracted() (heal) error = %v", err)
	}
	if err := m.Verify(ModePrivate); err != nil {
		t.Errorf("Verify(ModePrivate) after healing error = %v, want nil", err)
	}
}

func TestUnknownModeRejected(t *testing.T) {
	m := newTestManager(t)
	if err := m.Verify(Mode("bogus")); err == nil {
		t.Fatal("Verify(bogus mode): got nil error, want non-nil")
	}
}

func TestExtractedFilesLiveUnderModeDir(t *testing.T) {
	m := newTestManager(t)
	if err := m.EnsureExtracted(); err != nil {
		t.Fatalf("EnsureExtracted() error = %v", err)
	}
	wantDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Slipstream", "engine", "fastmode")
	if got := m.Dir(ModeFast); got != wantDir {
		t.Errorf("Dir(ModeFast) = %s, want %s", got, wantDir)
	}
}
