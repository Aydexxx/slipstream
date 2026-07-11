// Package engine manages the vendored third-party binaries Slipstream
// drives (zapret for Fast Mode, AmneziaWG for Private Mode). Binaries are
// embedded at build time (see assets/assets.go) and never downloaded at
// runtime; this package only extracts them to a per-user directory and
// verifies their integrity.
package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"slipstream/assets"
)

// Manager extracts and verifies the vendored engine binaries under
// %LocalAppData%\Slipstream\engine\.
type Manager struct {
	baseDir string
	log     *slog.Logger
}

// New creates a Manager rooted at %LocalAppData%\Slipstream\engine\.
func New(log *slog.Logger) (*Manager, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return nil, fmt.Errorf("LOCALAPPDATA environment variable is not set")
	}
	return &Manager{
		baseDir: filepath.Join(localAppData, "Slipstream", "engine"),
		log:     log,
	}, nil
}

// Dir returns the per-user extraction directory for the given mode.
func (m *Manager) Dir(mode Mode) string {
	return filepath.Join(m.baseDir, string(mode))
}

// Path returns the extracted path of a named file within a mode's directory.
func (m *Manager) Path(mode Mode, filename string) string {
	return filepath.Join(m.Dir(mode), filename)
}

// Fast Mode (zapret) file paths.
func (m *Manager) WinwsPath() string        { return m.Path(ModeFast, "winws.exe") }
func (m *Manager) WinDivertSysPath() string { return m.Path(ModeFast, "WinDivert64.sys") }
func (m *Manager) WinDivertDLLPath() string { return m.Path(ModeFast, "WinDivert.dll") }
func (m *Manager) CygwinDLLPath() string    { return m.Path(ModeFast, "cygwin1.dll") }

// Private Mode (AmneziaWG) file paths.
func (m *Manager) AmneziaWGPath() string { return m.Path(ModePrivate, "amneziawg.exe") }
func (m *Manager) WintunDLLPath() string { return m.Path(ModePrivate, "wintun.dll") }

// EnsureExtracted extracts every vendored engine file to its per-user
// destination, but only if that destination is missing or its on-disk
// SHA-256 no longer matches the hardcoded manifest (tampering, a partial
// write, or a stale previous version). Files that already match are left
// untouched. Intended to be called once on startup.
func (m *Manager) EnsureExtracted() error {
	for _, mode := range allModes {
		files, err := filesForMode(mode)
		if err != nil {
			return err
		}
		dir := m.Dir(mode)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s directory: %w", mode, err)
		}
		for _, f := range files {
			dest := filepath.Join(dir, path.Base(f.embedPath))
			if hashMatches(dest, f.sha256) {
				continue
			}
			if err := m.extract(f, dest); err != nil {
				return fmt.Errorf("extract %s: %w", f.embedPath, err)
			}
			if m.log != nil {
				m.log.Info("engine file extracted", "mode", string(mode), "file", path.Base(f.embedPath))
			}
		}
	}
	return nil
}

// extract reads f from the embedded asset filesystem, re-verifies it
// against the hardcoded hash (catching a corrupt build before it ever
// touches disk), and atomically writes it to dest.
func (m *Manager) extract(f fileSpec, dest string) error {
	data, err := assets.Bin.ReadFile(f.embedPath)
	if err != nil {
		return fmt.Errorf("read embedded asset: %w", err)
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != f.sha256 {
		return fmt.Errorf("embedded asset %s does not match hardcoded hash (got %s, want %s) - build is corrupt", f.embedPath, got, f.sha256)
	}

	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}

// Verify recomputes SHA-256 for every extracted file belonging to mode and
// compares it against the hardcoded manifest. Callers must treat a
// non-nil error as "refuse to run this mode" - the error names the first
// missing or mismatching file.
func (m *Manager) Verify(mode Mode) error {
	files, err := filesForMode(mode)
	if err != nil {
		return err
	}
	dir := m.Dir(mode)
	for _, f := range files {
		dest := filepath.Join(dir, path.Base(f.embedPath))
		if !hashMatches(dest, f.sha256) {
			return fmt.Errorf("engine file %s failed verification (missing or hash mismatch): refusing to run %s mode", dest, mode)
		}
	}
	return nil
}

func hashMatches(path string, want string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == want
}
