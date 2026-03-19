package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Linear  LinearConfig  `toml:"linear"`
	GitHub  GitHubConfig  `toml:"github"`
	Local   LocalConfig   `toml:"local"`
	Display DisplayConfig `toml:"display"`
}

type LinearConfig struct {
	APIKey string `toml:"api_key"`
}

type GitHubConfig struct {
	Repos []string `toml:"repos"`
}

type LocalConfig struct {
	BaseDirs []string `toml:"base_dirs"`
}

type DisplayConfig struct {
	MaxItems int `toml:"max_items"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Display: DisplayConfig{MaxItems: 20},
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	configPath := filepath.Join(configDir, "nxt", "config.toml")
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
