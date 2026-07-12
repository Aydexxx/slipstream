package privatemode

import (
	"os"
	"strings"
	"testing"
)

func TestStoreImportLoadRoundTrip(t *testing.T) {
	s := NewConfigStore(t.TempDir())
	if s.Exists() {
		t.Fatal("fresh store should not report Exists")
	}

	cfg, err := s.Import(goodConfig)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if cfg.Endpoint != "203.0.113.9:51820" {
		t.Errorf("returned summary endpoint = %q", cfg.Endpoint)
	}
	if !s.Exists() {
		t.Fatal("store should report Exists after import")
	}

	// On-disk bytes must be encrypted (no plaintext key material).
	raw, _ := os.ReadFile(s.path)
	if strings.Contains(string(raw), "PrivateKey") {
		t.Fatal("stored config is not encrypted at rest")
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Raw != goodConfig {
		t.Error("decrypted config text does not match imported text")
	}
}

func TestStoreRejectsInvalidImport(t *testing.T) {
	s := NewConfigStore(t.TempDir())
	if _, err := s.Import("[Interface]\nnope\n"); err == nil {
		t.Fatal("expected invalid config import to fail")
	}
	if s.Exists() {
		t.Error("store must not persist an invalid config")
	}
}

func TestStoreDeleteIsIdempotent(t *testing.T) {
	s := NewConfigStore(t.TempDir())
	if err := s.Delete(); err != nil {
		t.Errorf("Delete on empty store: %v", err)
	}
	if _, err := s.Import(goodConfig); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if err := s.Delete(); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if s.Exists() {
		t.Error("store should be empty after Delete")
	}
}
