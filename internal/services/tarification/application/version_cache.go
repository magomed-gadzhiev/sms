// version_cache.go
package application

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// VersionCache — TTL-обёртка над PriceRulesVersionRepository.
// GetVersion в горячем пути вызывается на каждый resolve; raw DB round-trip
// деградирует throughput. TTL=1s достаточно: инвалидация видна в течение
// секунды, при этом 99% запросов берут значение из памяти.
// singleflight защищает от thundering herd на истечении TTL или на cold start.
// GetVersion decouples the singleflight call from any one caller's ctx to prevent a single cancellation from failing coalesced readers.
type VersionCache struct {
	inner domain.PriceRulesVersionRepository
	ttl   time.Duration

	mu        sync.RWMutex
	value     int64
	expiresAt time.Time

	sf singleflight.Group
}

func NewVersionCache(inner domain.PriceRulesVersionRepository, ttl time.Duration) *VersionCache {
	return &VersionCache{inner: inner, ttl: ttl}
}

func (c *VersionCache) GetVersion(ctx context.Context) (int64, error) {
	c.mu.RLock()
	if time.Now().Before(c.expiresAt) {
		v := c.value
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	v, err, _ := c.sf.Do("version", func() (any, error) {
		// Double-check после захвата — другой caller мог уже обновить.
		c.mu.RLock()
		if time.Now().Before(c.expiresAt) {
			v := c.value
			c.mu.RUnlock()
			return v, nil
		}
		c.mu.RUnlock()

		// Decouple the shared singleflight call from any one caller's
		// cancellation — the first caller's ctx.Cancel() must not
		// fail every coalesced reader. Bound at 5*ttl to avoid a
		// truly stuck inner call holding the group forever.
		sfCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*c.ttl)
		defer cancel()
		fresh, err := c.inner.GetVersion(sfCtx)
		if err != nil {
			return int64(0), err
		}
		c.mu.Lock()
		c.value = fresh
		c.expiresAt = time.Now().Add(c.ttl)
		c.mu.Unlock()
		return fresh, nil
	})
	if err != nil {
		return 0, err
	}
	return v.(int64), nil
}
