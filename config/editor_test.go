package config

import "testing"

func TestEditorCommandFromConfig(t *testing.T) {
	cfg := &Config{Display: DisplayConfig{Editor: "cursor"}}
	t.Setenv("VISUAL", "should-not-use")
	t.Setenv("EDITOR", "should-not-use")

	got := cfg.EditorCommand()
	if got != "cursor" {
		t.Errorf("EditorCommand() = %q, want %q (config value takes priority)", got, "cursor")
	}
}

func TestEditorCommandFallbackVISUAL(t *testing.T) {
	cfg := &Config{}
	t.Setenv("VISUAL", "code")
	t.Setenv("EDITOR", "should-not-use")

	got := cfg.EditorCommand()
	if got != "code" {
		t.Errorf("EditorCommand() = %q, want %q (VISUAL fallback)", got, "code")
	}
}

func TestEditorCommandFallbackEDITOR(t *testing.T) {
	cfg := &Config{}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim")

	got := cfg.EditorCommand()
	if got != "vim" {
		t.Errorf("EditorCommand() = %q, want %q (EDITOR fallback)", got, "vim")
	}
}

func TestEditorCommandDefault(t *testing.T) {
	cfg := &Config{}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	got := cfg.EditorCommand()
	if got != "open" {
		t.Errorf("EditorCommand() = %q, want %q (default fallback)", got, "open")
	}
}
