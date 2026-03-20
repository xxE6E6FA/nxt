package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Default TTLs per source. Callers can override via GetWithTTL.
const (
	DefaultTTL    = 5 * time.Minute
	GitHubTTL     = 5 * time.Minute
	LinearTTL     = 2 * time.Minute
	WorktreesTTL  = 30 * time.Second
)

type entry struct {
	Data     json.RawMessage `json:"data"`
	CachedAt time.Time       `json:"cached_at"`
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

// Get retrieves a cached value using the DefaultTTL. Returns false if stale or missing.
func Get(key string, dest interface{}) bool {
	return GetWithTTL(key, dest, DefaultTTL)
}

// GetWithTTL retrieves a cached value if it exists and is fresher than the given TTL.
// Returns false if stale or missing.
func GetWithTTL(key string, dest interface{}, ttl time.Duration) bool {
	data, err := os.ReadFile(cachePath(key))
	if err != nil {
		return false
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return false
	}

	if time.Since(e.CachedAt) > ttl {
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
