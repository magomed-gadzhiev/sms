package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

const (
	hlrCacheKeyPrefix = "hlr:"
	DefaultCacheTTL   = 24 * time.Hour
)

// HLRCacheImpl implements domain.HLRCache using Redis
type HLRCacheImpl struct {
	client *redis.Client
	ttl    time.Duration
}

// NewHLRCache creates a new Redis-based HLR cache
func NewHLRCache(client *redis.Client, ttl time.Duration) *HLRCacheImpl {
	if ttl == 0 {
		ttl = DefaultCacheTTL
	}
	return &HLRCacheImpl{
		client: client,
		ttl:    ttl,
	}
}

func hlrCacheKey(msisdn string) string {
	return hlrCacheKeyPrefix + msisdn
}

// Get retrieves a cached HLR lookup result
func (c *HLRCacheImpl) Get(ctx context.Context, msisdn string) (*domain.LookupResult, error) {
	key := hlrCacheKey(msisdn)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // cache miss
		}
		return nil, fmt.Errorf("ошибка получения из кеша: %w", err)
	}

	var result domain.LookupResult
	if err := json.Unmarshal(data, &result); err != nil {
		// Corrupted cache entry — delete and return miss
		log.Warn().Err(err).Str("msisdn", msisdn).Msg("повреждённая запись в кеше, удаляем")
		_ = c.Delete(ctx, msisdn)
		return nil, nil
	}

	result.Cached = true
	return &result, nil
}

// Set stores an HLR lookup result in cache
func (c *HLRCacheImpl) Set(ctx context.Context, msisdn string, result *domain.LookupResult) error {
	key := hlrCacheKey(msisdn)

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("ошибка сериализации результата HLR: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("ошибка записи в кеш: %w", err)
	}

	return nil
}

// Delete removes an HLR lookup result from cache (for invalidation)
func (c *HLRCacheImpl) Delete(ctx context.Context, msisdn string) error {
	key := hlrCacheKey(msisdn)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("ошибка удаления из кеша: %w", err)
	}

	return nil
}
