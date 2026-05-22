package limits

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LimitQuerier — DB-запросы, необходимые LimitResolver.
type LimitQuerier interface {
	GetClientProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (*int, error)
	GetTariffPlanDefaultTPS(ctx context.Context, clientID uuid.UUID) (*int, error)
	GetSystemDefaultTPS(ctx context.Context) (int, error)
	GetClientParentAndBudget(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, *int, error)
	GetSubAccountAllocatedTPS(ctx context.Context, clientID, excludeProviderID uuid.UUID) (int, error)
}

// DBLimitResolver разрешает TPS-лимиты по цепочке:
// client_providers → tariff_plan → system_defaults, с учётом бюджета субаккаунта.
type DBLimitResolver struct {
	q      LimitQuerier
	logger zerolog.Logger
}

func NewDBLimitResolver(q LimitQuerier) *DBLimitResolver {
	return &DBLimitResolver{q: q, logger: log.With().Str("component", "limit_resolver").Logger()}
}

// ResolveProviderTPS возвращает TPS-лимит для пары (client, provider).
func (r *DBLimitResolver) ResolveProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (int, error) {
	// 1. Per-client TPS на провайдере
	if v, err := r.q.GetClientProviderTPS(ctx, clientID, providerID); err == nil && v != nil {
		tps := *v
		return r.applySubAccountBudgetCap(ctx, clientID, providerID, tps)
	}

	// 2. Дефолт из тарифного плана клиента
	if v, err := r.q.GetTariffPlanDefaultTPS(ctx, clientID); err == nil && v != nil {
		tps := *v
		return r.applySubAccountBudgetCap(ctx, clientID, providerID, tps)
	}

	// 3. Системный дефолт (всегда есть)
	tps, err := r.q.GetSystemDefaultTPS(ctx)
	if err != nil {
		return 5, fmt.Errorf("GetSystemDefaultTPS: %w", err)
	}
	return r.applySubAccountBudgetCap(ctx, clientID, providerID, tps)
}

func (r *DBLimitResolver) applySubAccountBudgetCap(ctx context.Context, clientID, providerID uuid.UUID, resolved int) (int, error) {
	parentID, budget, err := r.q.GetClientParentAndBudget(ctx, clientID)
	if err != nil || parentID == nil || budget == nil {
		return resolved, nil
	}

	used, err := r.q.GetSubAccountAllocatedTPS(ctx, clientID, providerID)
	if err != nil {
		return resolved, nil
	}

	maxAllowed := *budget - used
	if maxAllowed < 0 {
		maxAllowed = 0
	}
	if resolved > maxAllowed {
		return maxAllowed, nil
	}
	return resolved, nil
}

// --- Redis-cached wrapper ---

type CachedLimitResolver struct {
	inner  *DBLimitResolver
	redis  *redis.Client
	ttl    time.Duration
	logger zerolog.Logger
}

func NewCachedLimitResolver(inner *DBLimitResolver, rdb *redis.Client) *CachedLimitResolver {
	return &CachedLimitResolver{
		inner:  inner,
		redis:  rdb,
		ttl:    30 * time.Second,
		logger: log.With().Str("component", "cached_limit_resolver").Logger(),
	}
}

func (c *CachedLimitResolver) ResolveProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (int, error) {
	key := fmt.Sprintf("limits:%s:%s:tps", clientID, providerID)

	if val, err := c.redis.Get(ctx, key).Int(); err == nil {
		return val, nil
	}

	tps, err := c.inner.ResolveProviderTPS(ctx, clientID, providerID)
	if err != nil {
		return tps, err
	}

	c.redis.Set(ctx, key, tps, c.ttl)
	return tps, nil
}

// InvalidateProviderTPS сбрасывает кеш TPS для конкретной пары (client, provider).
func (c *CachedLimitResolver) InvalidateProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) {
	key := fmt.Sprintf("limits:%s:%s:tps", clientID, providerID)
	c.redis.Del(ctx, key)
}
