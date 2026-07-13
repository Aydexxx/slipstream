package statemachine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// settingsFileName is the persisted-settings file within a Manager's
// StateDataDir.
const settingsFileName = "settings.json"

func settingsPath(stateDataDir string) string {
	return filepath.Join(stateDataDir, settingsFileName)
}

// loadSettings reads persisted settings, returning the zero value (not an
// error) when none exist yet — the common case on first launch.
func loadSettings(stateDataDir string) (Settings, error) {
	data, err := os.ReadFile(settingsPath(stateDataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("read state settings: %w", err)
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("decode state settings: %w", err)
	}
	return s, nil
}

// saveSettings writes settings atomically (tmp file + rename), matching the
// idiom already used for the DNS backup file.
func saveSettings(stateDataDir string, s Settings) error {
	if err := os.MkdirAll(stateDataDir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state settings: %w", err)
	}
	path := settingsPath(stateDataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write state settings: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("commit state settings: %w", err)
	}
	return nil
}
