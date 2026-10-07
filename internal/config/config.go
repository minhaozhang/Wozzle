// Package config loads Wozzle's optional JSON settings file.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config mirrors wozzle.json next to the executable.
type Config struct {
	// Addr is the listen address, e.g. "127.0.0.1:8080".
	Addr string `json:"addr"`
	// OpenBrowser opens the default browser after startup (headless mode).
	OpenBrowser bool `json:"openBrowser"`
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{Addr: "127.0.0.1:8080"}
}

// DefaultPath returns the config path next to the executable.
func DefaultPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "wozzle.json"), nil
}

// Load reads the config file at path, falling back to defaults when it is
// missing or partially invalid.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	if cfg.Addr == "" {
		cfg.Addr = Default().Addr
	}
	return cfg, nil
}

// Save writes the config back to path (best effort, pretty printed).
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
