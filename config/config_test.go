package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Display.MaxItems != 20 {
		t.Errorf("MaxItems = %d, want 20", cfg.Display.MaxItems)
	}
	if len(cfg.Local.BaseDirs) != 0 {
		t.Errorf("BaseDirs = %v, want empty", cfg.Local.BaseDirs)
	}
	if cfg.Display.Editor != "" {
		t.Errorf("Editor = %q, want empty", cfg.Display.Editor)
	}
}

func TestLoadFromFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configDir := filepath.Join(tmp, ".config", "nxt")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}

	content := `[local]
base_dirs = ["/code", "/projects"]

[display]
max_items = 30
editor = "vim"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Display.MaxItems != 30 {
		t.Errorf("MaxItems = %d, want 30", cfg.Display.MaxItems)
	}
	if cfg.Display.Editor != "vim" {
		t.Errorf("Editor = %q, want %q", cfg.Display.Editor, "vim")
	}
	want := []string{"/code", "/projects"}
	if len(cfg.Local.BaseDirs) != len(want) {
		t.Fatalf("BaseDirs length = %d, want %d", len(cfg.Local.BaseDirs), len(want))
	}
	for i, d := range cfg.Local.BaseDirs {
		if d != want[i] {
			t.Errorf("BaseDirs[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestWriteAndRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	original := &Config{
		Local: LocalConfig{
			BaseDirs: []string{"/a", "/b"},
		},
		Display: DisplayConfig{
			MaxItems: 42,
			Editor:   "code",
		},
	}

	if err := Write(original); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if loaded.Display.MaxItems != original.Display.MaxItems {
		t.Errorf("MaxItems = %d, want %d", loaded.Display.MaxItems, original.Display.MaxItems)
	}
	if loaded.Display.Editor != original.Display.Editor {
		t.Errorf("Editor = %q, want %q", loaded.Display.Editor, original.Display.Editor)
	}
	if len(loaded.Local.BaseDirs) != len(original.Local.BaseDirs) {
		t.Fatalf("BaseDirs length = %d, want %d", len(loaded.Local.BaseDirs), len(original.Local.BaseDirs))
	}
	for i, d := range loaded.Local.BaseDirs {
		if d != original.Local.BaseDirs[i] {
			t.Errorf("BaseDirs[%d] = %q, want %q", i, d, original.Local.BaseDirs[i])
		}
	}
	// Write strips the API key, so it should be empty after round-trip.
	if loaded.Linear.APIKey != "" {
		t.Errorf("APIKey = %q, want empty (Write should strip it)", loaded.Linear.APIKey)
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error for missing file: %v", err)
	}

	if cfg.Display.MaxItems != 20 {
		t.Errorf("MaxItems = %d, want default 20", cfg.Display.MaxItems)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configDir := filepath.Join(tmp, ".config", "nxt")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("{{{{not valid toml!@#$"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error for invalid TOML, got nil")
	}
}

func TestLoadPartialConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configDir := filepath.Join(tmp, ".config", "nxt")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Only set editor, leave everything else out.
	content := `[display]
editor = "nvim"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Display.Editor != "nvim" {
		t.Errorf("Editor = %q, want %q", cfg.Display.Editor, "nvim")
	}
	// MaxItems default should survive partial decode.
	if cfg.Display.MaxItems != 20 {
		t.Errorf("MaxItems = %d, want default 20", cfg.Display.MaxItems)
	}
	if len(cfg.Local.BaseDirs) != 0 {
		t.Errorf("BaseDirs = %v, want empty", cfg.Local.BaseDirs)
	}
}
