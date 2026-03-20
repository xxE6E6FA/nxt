package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStripsAPIKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{
		Linear:  LinearConfig{APIKey: "secret-key-123"},
		Display: DisplayConfig{MaxItems: 20, Editor: "code"},
	}

	if err := Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Linear.APIKey != "" {
		t.Errorf("APIKey = %q, want empty (Write should strip it)", loaded.Linear.APIKey)
	}

	// Verify the file itself doesn't contain the key
	configPath := filepath.Join(tmp, ".config", "nxt", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if contains := string(data); containsString(contains, "secret-key-123") {
		t.Error("config file on disk should not contain the API key")
	}
}

func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (haystack == needle || findSubstring(haystack, needle))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestWriteReadOnlyDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create .config as read-only so MkdirAll for .config/nxt fails
	readonlyDir := filepath.Join(tmp, ".config")
	if err := os.MkdirAll(readonlyDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readonlyDir, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(readonlyDir, 0o750) // restore so cleanup can remove
	})

	cfg := &Config{Display: DisplayConfig{MaxItems: 10}}
	err := Write(cfg)
	if err == nil {
		t.Error("Write to read-only directory should return error")
	}
}

func TestWriteCreatesConfigFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{
		Local:   LocalConfig{BaseDirs: []string{"/code"}},
		Display: DisplayConfig{MaxItems: 25, Editor: "zed"},
	}

	if err := Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	configPath := filepath.Join(tmp, ".config", "nxt", "config.toml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("config file is empty")
	}
}

func TestWriteDoesNotMutateOriginal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{
		Linear:  LinearConfig{APIKey: "keep-this"},
		Display: DisplayConfig{MaxItems: 20},
	}

	if err := Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The original cfg should still have its API key
	if cfg.Linear.APIKey != "keep-this" {
		t.Errorf("Write mutated original config APIKey: got %q, want %q", cfg.Linear.APIKey, "keep-this")
	}
}
