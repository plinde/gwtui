// Package cache provides a small TTL'd JSON cache on disk, used to avoid
// re-hitting the GitHub GraphQL API (via the gh CLI) on every load and every
// auto-refresh tick. Its second job is resilience: when a live gh call fails
// (most importantly a rate-limit / 429), callers can serve the last-known
// cached value via GetStale instead of surfacing an error.
//
// Copied from plinde/ghwatch (internal/cache), where it cut that tool's
// GraphQL spend to ~48 calls/hour. Deliberately duplicated rather than shared:
// the two tools cache different things and a shared module would couple their
// release cycles for ~120 lines. Keep fixes in sync by hand.
//
// Cache files live under ~/.config/gwtui/cache/<key>.json and wrap the
// payload in an envelope carrying the write time, so freshness is judged
// against the file contents, not the filesystem mtime.
package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTTL is the freshness window used when no --cache-ttl is given.
const DefaultTTL = 15 * time.Minute

// Options controls cache behavior for a single load.
type Options struct {
	Dir      string        // cache directory; empty => ~/.config/gwtui/cache
	TTL      time.Duration // freshness window; entries older than this are not fresh
	Force    bool          // bypass fresh-read (force a live call) but still write-through
	Disabled bool          // --no-cache: no read, no write, no stale fallback
}

// entry is the on-disk envelope wrapping a cached payload.
type entry struct {
	WrittenAt time.Time       `json:"written_at"`
	Payload   json.RawMessage `json:"payload"`
}

// Dir returns the resolved cache directory.
func (o Options) dir() string {
	if o.Dir != "" {
		return o.Dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gwtui-cache"
	}
	return filepath.Join(home, ".config", "gwtui", "cache")
}

// sanitizeKey makes a key safe as a filename.
func sanitizeKey(key string) string {
	repl := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return repl.Replace(key)
}

func (o Options) path(key string) string {
	return filepath.Join(o.dir(), sanitizeKey(key)+".json")
}

// read loads and unmarshals the envelope for key. Returns the decoded age of
// the entry and whether a usable entry was found.
func (o Options) read(key string, v any) (age time.Duration, ok bool) {
	data, err := os.ReadFile(o.path(key))
	if err != nil {
		return 0, false
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return 0, false
	}
	if err := json.Unmarshal(e.Payload, v); err != nil {
		return 0, false
	}
	return time.Since(e.WrittenAt), true
}

// Get reads a cached value into v when the entry exists and is fresh
// (age < TTL). Returns hit=true only on a fresh read. When Disabled or Force
// is set, Get always misses so the caller performs a live call.
func Get(o Options, key string, v any) (hit bool, age time.Duration) {
	if o.Disabled || o.Force {
		return false, 0
	}
	age, ok := o.read(key, v)
	if !ok {
		return false, 0
	}
	// TTL <= 0 means "never treat cache as fresh" — force a live call while
	// still write-through (and GetStale remains available for error fallback).
	if o.TTL <= 0 || age >= o.TTL {
		return false, age
	}
	return true, age
}

// GetStale reads a cached value into v regardless of age. It is the
// serve-on-error path: after a live call fails (e.g. rate-limited), callers
// fall back to the last-known value. Returns false when Disabled.
func GetStale(o Options, key string, v any) (hit bool, age time.Duration) {
	if o.Disabled {
		return false, 0
	}
	age, ok := o.read(key, v)
	return ok, age
}

// Set writes v to the cache under key. No-op when Disabled. Writes are
// best-effort: a marshalling or IO failure is returned but callers typically
// ignore it (a failed cache write must not break a successful load).
func Set(o Options, key string, v any) error {
	if o.Disabled {
		return nil
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	e := entry{WrittenAt: nowFunc(), Payload: payload}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	dir := o.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Write atomically via a temp file + rename so a concurrent reader never
	// sees a half-written file.
	tmp, err := os.CreateTemp(dir, sanitizeKey(key)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, o.path(key))
}

// nowFunc is overridable in tests. Package callers must not rely on wall-clock
// determinism.
var nowFunc = time.Now
