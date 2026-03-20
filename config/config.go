package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Linear  LinearConfig  `toml:"linear"`
	Local   LocalConfig   `toml:"local"`
	Display DisplayConfig `toml:"display"`
}

type LinearConfig struct {
	APIKey string `toml:"api_key,omitempty"`
}

type LocalConfig struct {
	BaseDirs []string `toml:"base_dirs"`
}

type DisplayConfig struct {
	MaxItems int    `toml:"max_items"`
	Editor   string `toml:"editor,omitempty"` // command to open a folder, e.g. "code", "cursor", "zed"
}

// EditorCommand returns the configured editor, falling back to
// $VISUAL → $EDITOR → "open" (macOS default).
func (c *Config) EditorCommand() string {
	if c.Display.Editor != "" {
		return c.Display.Editor
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "open"
}

// Path returns the config file path (~/.config/nxt/config.toml).
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "nxt", "config.toml"), nil
}

func Load() (*Config, error) {
	cfg := &Config{
		Display: DisplayConfig{MaxItems: 20},
	}

	configPath, err := Path()
	if err != nil {
		//nolint:nilerr // If we can't determine the config path, return default config gracefully.
		return cfg, nil
	}

	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, err
		}
	}

	// Env var overrides
	if key := os.Getenv("LINEAR_API_KEY"); key != "" {
		cfg.Linear.APIKey = key
	}

	return cfg, nil
}

// Write persists the config to disk (excluding secrets).
func Write(cfg *Config) error {
	configPath, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return err
	}

	// Write config without the API key (stored in keychain)
	writeCfg := *cfg
	writeCfg.Linear.APIKey = ""

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(writeCfg); err != nil {
		return err
	}

	// Atomic write: temp file + rename to prevent corruption on crash
	tmpFile, err := os.CreateTemp(filepath.Dir(configPath), ".nxt-config-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()    //nolint:gosec // best-effort cleanup
		os.Remove(tmpPath) //nolint:gosec // best-effort cleanup
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath) //nolint:gosec // best-effort cleanup
		return err
	}
	return os.Rename(tmpPath, configPath)
}
