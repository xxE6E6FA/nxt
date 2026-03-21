package cmd

import (
	"os"
	"runtime"
	"testing"
)

func TestGetVersion(t *testing.T) {
	// With the default "dev" value, GetVersion should return something non-empty
	v := GetVersion()
	if v == "" {
		t.Error("GetVersion() returned empty string")
	}
}

func TestGetVersionNonDev(t *testing.T) {
	old := version
	version = "v1.2.3"
	defer func() { version = old }()

	got := GetVersion()
	if got != "v1.2.3" {
		t.Errorf("GetVersion() = %q, want v1.2.3", got)
	}
}

func TestAssetName(t *testing.T) {
	name := assetName()
	expected := "nxt_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if name != expected {
		t.Errorf("assetName() = %q, want %q", name, expected)
	}
}

func TestCopyFile(t *testing.T) {
	tmp := t.TempDir()
	src := tmp + "/src"
	dst := tmp + "/dst"

	if err := os.WriteFile(src, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("dst content = %q, want %q", string(got), "hello world")
	}
}

func TestCopyFileSourceMissing(t *testing.T) {
	tmp := t.TempDir()
	err := copyFile(tmp+"/nonexistent", tmp+"/dst")
	if err == nil {
		t.Error("expected error for missing source")
	}
}
