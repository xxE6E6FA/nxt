package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Default TTLs per source. Callers can override via GetWithTTL.
const (
	DefaultTTL   = 5 * time.Minute
	GitHubTTL    = 5 * time.Minute
	LinearTTL    = 2 * time.Minute
	WorktreesTTL = 30 * time.Second
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

// StaleTTL is the maximum age for serving stale data while revalidating.
const StaleTTL = 10 * time.Minute

// GetStale retrieves a cached value with two-tier freshness.
// Returns (hit, stale):
//   - (true, false) if data is within freshTTL — fresh hit
//   - (true, true)  if data is between freshTTL and staleTTL — stale hit, caller should revalidate
//   - (false, false) if data is beyond staleTTL or missing — full miss
func GetStale(key string, dest interface{}, freshTTL, staleTTL time.Duration) (hit, stale bool) {
	data, err := os.ReadFile(cachePath(key))
	if err != nil {
		return false, false
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return false, false
	}

	age := time.Since(e.CachedAt)

	if age > staleTTL {
		return false, false
	}

	if err := json.Unmarshal(e.Data, dest); err != nil {
		return false, false
	}

	if age > freshTTL {
		return true, true
	}

	return true, false
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
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	// Atomic write: temp file + rename to prevent corruption on crash
	tmpFile, err := os.CreateTemp(dir, ".nxt-cache-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(buf); err != nil {
		tmpFile.Close()    //nolint:gosec // best-effort cleanup
		os.Remove(tmpPath) //nolint:gosec // best-effort cleanup
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath) //nolint:gosec // best-effort cleanup
		return err
	}
	return os.Rename(tmpPath, cachePath(key))
}
