package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const defaultTTL = 60 * time.Second

type entry struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  time.Time       `json:"cached_at"`
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(dir, "nxt")
}

func cachePath(key string) string {
	return filepath.Join(cacheDir(), key+".json")
}

// Get retrieves a cached value if it exists and is fresh. Returns nil if stale or missing.
func Get(key string, dest interface{}) bool {
	data, err := os.ReadFile(cachePath(key))
	if err != nil {
		return false
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return false
	}

	if time.Since(e.CachedAt) > defaultTTL {
		return false
	}

	return json.Unmarshal(e.Data, dest) == nil
}

// Set stores a value in the cache.
func Set(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	e := entry{
		Data:     json.RawMessage(data),
		CachedAt: time.Now(),
	}

	buf, err := json.Marshal(e)
	if err != nil {
		return err
	}

	dir := cacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(cachePath(key), buf, 0644)
}
