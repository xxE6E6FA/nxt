package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetUnmarshalableValue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Channels can't be marshaled to JSON
	ch := make(chan int)
	err := Set("unmarshalable", ch)
	if err == nil {
		t.Error("Set with unmarshalable value should return error")
	}
}

func TestGetCorruptJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write corrupt data directly to cache file
	dir := filepath.Join(tmp, ".cache", "nxt")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not valid json!}"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out string
	if GetWithTTL("corrupt", &out, time.Hour) {
		t.Error("GetWithTTL should return false for corrupt JSON")
	}
}

func TestGetExpiredEntry(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write an entry with old CachedAt directly
	dir := filepath.Join(tmp, ".cache", "nxt")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal("hello")
	e := entry{
		Data:     json.RawMessage(data),
		CachedAt: time.Now().Add(-2 * time.Hour),
	}
	buf, _ := json.Marshal(e)
	if err := os.WriteFile(filepath.Join(dir, "expired.json"), buf, 0o600); err != nil {
		t.Fatal(err)
	}

	var out string
	if GetWithTTL("expired", &out, time.Minute) {
		t.Error("GetWithTTL should return false for expired entry")
	}
}

func TestGetTypeMismatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Cache a string
	if err := Set("typed", "hello"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Try to read it as a struct
	var out struct{ Field int }
	if GetWithTTL("typed", &out, time.Hour) {
		t.Error("GetWithTTL should return false when cached string can't unmarshal into struct")
	}
}

func TestGetMissingKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out string
	if Get("nonexistent", &out) {
		t.Error("Get should return false for missing key")
	}
}

func TestGetValidDataInEntry(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write valid entry but with data that doesn't match dest type
	dir := filepath.Join(tmp, ".cache", "nxt")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Valid entry wrapper, but Data is a JSON number — can't unmarshal into []string
	e := entry{
		Data:     json.RawMessage(`42`),
		CachedAt: time.Now(),
	}
	buf, _ := json.Marshal(e)
	if err := os.WriteFile(filepath.Join(dir, "badtype.json"), buf, 0o600); err != nil {
		t.Fatal(err)
	}

	var out []string
	if GetWithTTL("badtype", &out, time.Hour) {
		t.Error("GetWithTTL should return false when data can't unmarshal into dest type")
	}
}

func TestGetStaleCorruptJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".cache", "nxt")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stale-corrupt.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out string
	hit, stale := GetStale("stale-corrupt", &out, time.Minute, time.Hour)
	if hit || stale {
		t.Errorf("GetStale should miss for corrupt JSON, got hit=%v stale=%v", hit, stale)
	}
}

func TestGetStaleTypeMismatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Cache a number
	if err := Set("stale-typed", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Try to read as []string — Data unmarshal will fail
	var out []string
	hit, stale := GetStale("stale-typed", &out, time.Hour, 2*time.Hour)
	if hit || stale {
		t.Errorf("GetStale should miss for type mismatch, got hit=%v stale=%v", hit, stale)
	}
}

func TestSetRoundTripComplex(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	type nested struct {
		Name  string   `json:"name"`
		Tags  []string `json:"tags"`
		Count int      `json:"count"`
	}

	original := nested{Name: "test", Tags: []string{"a", "b"}, Count: 42}
	if err := Set("complex", original); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var loaded nested
	if !Get("complex", &loaded) {
		t.Fatal("Get returned false for fresh complex entry")
	}
	if loaded.Name != original.Name || loaded.Count != original.Count {
		t.Errorf("got %+v, want %+v", loaded, original)
	}
	if len(loaded.Tags) != 2 || loaded.Tags[0] != "a" || loaded.Tags[1] != "b" {
		t.Errorf("Tags = %v, want [a b]", loaded.Tags)
	}
}
