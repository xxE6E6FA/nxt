package cache

import (
	"os"
	"testing"
	"time"
)

func TestGetStale(t *testing.T) {
	// Use a temp dir for cache during tests
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	key := "test-stale"
	freshTTL := 100 * time.Millisecond
	staleTTL := 500 * time.Millisecond

	// Miss: nothing cached
	var out string
	hit, stale := GetStale(key, &out, freshTTL, staleTTL)
	if hit || stale {
		t.Fatalf("expected miss, got hit=%v stale=%v", hit, stale)
	}

	// Set a value
	if err := Set(key, "hello"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Fresh hit: within freshTTL
	hit, stale = GetStale(key, &out, freshTTL, staleTTL)
	if !hit || stale {
		t.Fatalf("expected fresh hit, got hit=%v stale=%v", hit, stale)
	}
	if out != "hello" {
		t.Fatalf("expected 'hello', got %q", out)
	}

	// Wait past freshTTL but within staleTTL
	time.Sleep(150 * time.Millisecond)
	hit, stale = GetStale(key, &out, freshTTL, staleTTL)
	if !hit || !stale {
		t.Fatalf("expected stale hit, got hit=%v stale=%v", hit, stale)
	}

	// Wait past staleTTL
	time.Sleep(400 * time.Millisecond)
	hit, stale = GetStale(key, &out, freshTTL, staleTTL)
	if hit || stale {
		t.Fatalf("expected miss after staleTTL, got hit=%v stale=%v", hit, stale)
	}
}

func TestGetStale_BackwardCompatWithGet(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	key := "compat-test"
	if err := Set(key, 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get should still work
	var val int
	if !Get(key, &val) {
		t.Fatal("Get returned false for fresh entry")
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}
