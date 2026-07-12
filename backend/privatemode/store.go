package privatemode

import (
	"fmt"
	"os"
	"path/filepath"

	"slipstream/backend/dpapi"
)

// ConfigStore persists exactly one imported AmneziaWG config, DPAPI-encrypted
// at rest under %LocalAppData%\Slipstream\private\. The config is the Phase-4
// artifact the user imported; it is never hardcoded and its private key never
// touches disk in plaintext.
type ConfigStore struct {
	path string
}

// NewConfigStore roots the store at dir (created on first import).
func NewConfigStore(dir string) *ConfigStore {
	return &ConfigStore{path: filepath.Join(dir, "tunnel.conf.dpapi")}
}

// Exists reports whether a config has been imported.
func (s *ConfigStore) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Import validates raw as an AmneziaWG config and, if valid, stores it
// DPAPI-encrypted (atomically). It returns the parsed config so the caller can
// show a summary. An invalid config is rejected without touching the store.
func (s *ConfigStore) Import(raw string) (*Config, error) {
	cfg, err := ParseConfig(raw)
	if err != nil {
		return nil, err
	}
	blob, err := dpapi.Protect([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("commit config: %w", err)
	}
	return cfg, nil
}

// Load decrypts and parses the stored config.
func (s *ConfigStore) Load() (*Config, error) {
	raw, err := s.LoadRaw()
	if err != nil {
		return nil, err
	}
	return ParseConfig(raw)
}

// LoadRaw decrypts the stored config to its original text.
func (s *ConfigStore) LoadRaw() (string, error) {
	blob, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no Private Mode config imported yet")
		}
		return "", fmt.Errorf("read config: %w", err)
	}
	plain, err := dpapi.Unprotect(blob)
	if err != nil {
		return "", fmt.Errorf("decrypt config (was it imported by this Windows user?): %w", err)
	}
	return string(plain), nil
}

// Delete removes the stored config. A missing file is not an error.
func (s *ConfigStore) Delete() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete config: %w", err)
	}
	return nil
}
