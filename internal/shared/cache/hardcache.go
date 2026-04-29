package cache

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// HardCacheEnabled returns true if HARD_CACHE_ENABLED env is truthy.
// Hard cache — process-local in-memory lookup cache for hot-path reads
// (api-key→user, client→approved-senders, routes, tariffs). Sacrifices
// freshness (TTL-bounded) for zero-latency lookups under load.
func HardCacheEnabled() bool {
	v := os.Getenv("HARD_CACHE_ENABLED")
	if v == "" {
		return false
	}
	b, _ := strconv.ParseBool(v)
	return b
}

// HardCacheTTL returns TTL from HARD_CACHE_TTL_SECONDS env or default.
func HardCacheTTL(defaultTTL time.Duration) time.Duration {
	v := os.Getenv("HARD_CACHE_TTL_SECONDS")
	if v == "" {
		return defaultTTL
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultTTL
	}
	return time.Duration(n) * time.Second
}

type hardEntry struct {
	value  any
	expire int64 // unix-nano
}

// HardCache is a lock-free (sync.Map) TTL cache with optional janitor.
// Zero allocations on hit fast-path (value is stored as any).
type HardCache struct {
	m         sync.Map
	hits      atomic.Uint64
	misses    atomic.Uint64
	ttl       time.Duration
	enabled   bool
	stopClean chan struct{}
}

// NewHardCache creates a cache. If env-flag is off, Get always misses
// and Set is a no-op — callers can unconditionally use the cache.
func NewHardCache(defaultTTL time.Duration) *HardCache {
	c := &HardCache{
		ttl:       HardCacheTTL(defaultTTL),
		enabled:   HardCacheEnabled(),
		stopClean: make(chan struct{}),
	}
	if c.enabled {
		go c.janitor()
	}
	return c
}

// Enabled reports whether the cache is active.
func (c *HardCache) Enabled() bool { return c.enabled }

// Get returns cached value if present and not expired.
func (c *HardCache) Get(key string) (any, bool) {
	if !c.enabled {
		c.misses.Add(1)
		return nil, false
	}
	v, ok := c.m.Load(key)
	if !ok {
		c.misses.Add(1)
		return nil, false
	}
	e := v.(*hardEntry)
	if time.Now().UnixNano() > e.expire {
		c.m.Delete(key)
		c.misses.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	return e.value, true
}

// Set stores a value with the cache's configured TTL.
func (c *HardCache) Set(key string, value any) {
	if !c.enabled {
		return
	}
	c.m.Store(key, &hardEntry{
		value:  value,
		expire: time.Now().Add(c.ttl).UnixNano(),
	})
}

// SetWithTTL stores a value with an explicit TTL.
func (c *HardCache) SetWithTTL(key string, value any, ttl time.Duration) {
	if !c.enabled {
		return
	}
	c.m.Store(key, &hardEntry{
		value:  value,
		expire: time.Now().Add(ttl).UnixNano(),
	})
}

// Invalidate removes a specific key.
func (c *HardCache) Invalidate(key string) {
	c.m.Delete(key)
}

// Clear drops all entries.
func (c *HardCache) Clear() {
	c.m.Range(func(k, _ any) bool {
		c.m.Delete(k)
		return true
	})
}

// Stats returns hit/miss counters.
func (c *HardCache) Stats() (hits, misses uint64) {
	return c.hits.Load(), c.misses.Load()
}

// Close stops the janitor goroutine.
func (c *HardCache) Close() {
	select {
	case <-c.stopClean:
	default:
		close(c.stopClean)
	}
}

func (c *HardCache) janitor() {
	t := time.NewTicker(c.ttl)
	defer t.Stop()
	for {
		select {
		case <-c.stopClean:
			return
		case <-t.C:
			now := time.Now().UnixNano()
			c.m.Range(func(k, v any) bool {
				if e, ok := v.(*hardEntry); ok && now > e.expire {
					c.m.Delete(k)
				}
				return true
			})
		}
	}
}
