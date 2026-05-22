package router

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// ErrNoRouteFound возвращается, когда не найден ни один маршрут.
var ErrNoRouteFound = errors.New("no route found")

// ClientRouteRepository — запросы к таблице client_routes.
type ClientRouteRepository interface {
	ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error)
	ListSharedByClientAndOperator(ctx context.Context, parentClientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error)
	ListDefaultByOperator(ctx context.Context, operatorID uuid.UUID) ([]*shared.ClientRoute, error)
}

// ClientParentRepository — получение родителя клиента и режима маршрутизации.
type ClientParentRepository interface {
	GetParentClientID(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, error)
	GetRoutingMode(ctx context.Context, clientID uuid.UUID) (string, error)
}

// UnifiedRouter маршрутизирует сообщение по трём уровням:
//  1. Собственные маршруты клиента
//  2. Shared маршруты реселлера (parent)
//  3. Платформенные дефолты (client_id IS NULL)
type UnifiedRouter struct {
	routeRepo  ClientRouteRepository
	clientRepo ClientParentRepository
	logger     zerolog.Logger
}

func NewUnifiedRouter(routeRepo ClientRouteRepository, clientRepo ClientParentRepository) *UnifiedRouter {
	return &UnifiedRouter{
		routeRepo:  routeRepo,
		clientRepo: clientRepo,
		logger:     log.With().Str("component", "unified_router").Logger(),
	}
}

// Route возвращает RoutingDecision для пары (clientID, operatorID).
// Поведение зависит от routing_mode клиента:
//   - "legacy"  — только платформенные дефолты (уровень 3)
//   - "new"     — только собственные маршруты клиента (уровень 1); ошибка если нет
//   - "hybrid"  — 3-уровневый fallback: client → reseller → platform (поведение по умолчанию)
func (r *UnifiedRouter) Route(ctx context.Context, clientID, operatorID uuid.UUID) (*shared.RoutingDecision, error) {
	mode, err := r.clientRepo.GetRoutingMode(ctx, clientID)
	if err != nil {
		r.logger.Warn().Err(err).Str("client_id", clientID.String()).Msg("GetRoutingMode failed, falling back to hybrid")
		mode = "hybrid"
	}

	var routes []*shared.ClientRoute

	switch mode {
	case "legacy":
		// Только платформенные дефолты
		routes, _ = r.routeRepo.ListDefaultByOperator(ctx, operatorID)

	case "new":
		// Только собственные маршруты — без fallback на глобальные
		routes, err = r.routeRepo.ListByClientAndOperator(ctx, clientID, operatorID)
		if err != nil {
			r.logger.Error().Err(err).Msg("ListByClientAndOperator failed")
		}

	default: // "hybrid" и всё остальное
		// Уровень 1: собственные маршруты
		routes, err = r.routeRepo.ListByClientAndOperator(ctx, clientID, operatorID)
		if err != nil {
			r.logger.Error().Err(err).Msg("ListByClientAndOperator failed")
		}

		// Уровень 2: shared маршруты реселлера
		if len(routes) == 0 {
			parentID, _ := r.clientRepo.GetParentClientID(ctx, clientID)
			if parentID != nil {
				routes, _ = r.routeRepo.ListSharedByClientAndOperator(ctx, *parentID, operatorID)
			}
		}

		// Уровень 3: платформенные дефолты
		if len(routes) == 0 {
			routes, _ = r.routeRepo.ListDefaultByOperator(ctx, operatorID)
		}
	}

	if len(routes) == 0 {
		return nil, ErrNoRouteFound
	}

	// Weighted selection within the top-priority bucket. This repository
	// sorts ClientRoute by `priority DESC` (see client_route_repository.go),
	// so the first element is the highest priority.
	// Legacy callers (all-weight-zero rows) keep deterministic behaviour:
	// they fall through to `routes[0]`. See pickWeightedSharedRoute for
	// tie-breaker semantics. Bug #11 (QA 2026-04-22).
	selected := pickWeightedSharedRoute(routes, nil)
	return &shared.RoutingDecision{
		ProviderID: selected.ProviderID,
		RouteID:    selected.ID,
	}, nil
}

// --- Weighted pick (shared.ClientRoute variant) ---

var (
	unifiedPickRNGMu sync.Mutex
	unifiedPickRNG   = rand.New(rand.NewSource(rand.Int63()))
)

// pickWeightedSharedRoute selects one route from a priority-ordered list using
// `Weight` as the split factor within the top-priority bucket.
//
// shared.ClientRoute (used by UnifiedRouter) has no `Share` column — only
// `Weight`. The storage layer orders rows by `priority DESC`, so the top
// priority bucket is the prefix of identical-priority rows at the start.
//
// Semantics mirror routing/application.PickWeightedRoute:
//   - Single-element bucket → return it.
//   - All weights zero in the bucket → return bucket[0] (legacy-compatible).
//   - Otherwise cumulative-distribution pick weighted by Weight; rows with
//     Weight <= 0 are excluded from the draw.
//
// Pass a non-nil *rand.Rand for deterministic behaviour in tests; pass nil
// to use the package-level RNG (process-random seed).
func pickWeightedSharedRoute(routes []*shared.ClientRoute, r *rand.Rand) *shared.ClientRoute {
	if len(routes) == 0 {
		return nil
	}

	topPriority := routes[0].Priority
	bucketEnd := 1
	for bucketEnd < len(routes) && routes[bucketEnd].Priority == topPriority {
		bucketEnd++
	}
	bucket := routes[:bucketEnd]

	if len(bucket) == 1 {
		return bucket[0]
	}

	total := 0
	for _, rt := range bucket {
		if rt.Weight > 0 {
			total += rt.Weight
		}
	}
	if total == 0 {
		return bucket[0]
	}

	var roll int
	if r == nil {
		unifiedPickRNGMu.Lock()
		roll = unifiedPickRNG.Intn(total)
		unifiedPickRNGMu.Unlock()
	} else {
		roll = r.Intn(total)
	}

	cumulative := 0
	for _, rt := range bucket {
		if rt.Weight <= 0 {
			continue
		}
		cumulative += rt.Weight
		if roll < cumulative {
			return rt
		}
	}
	return bucket[0]
}

// --- In-memory кеш ---

type cacheKey struct {
	clientID   uuid.UUID
	operatorID uuid.UUID
}

type cacheEntry struct {
	decision  *shared.RoutingDecision
	expiresAt time.Time
}

// CachedUnifiedRouter оборачивает UnifiedRouter in-memory кешем с TTL 30 сек.
type CachedUnifiedRouter struct {
	inner *UnifiedRouter
	cache map[cacheKey]*cacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

func NewCachedUnifiedRouter(inner *UnifiedRouter) *CachedUnifiedRouter {
	return &CachedUnifiedRouter{
		inner: inner,
		cache: make(map[cacheKey]*cacheEntry),
		ttl:   30 * time.Second,
	}
}

func (c *CachedUnifiedRouter) Route(ctx context.Context, clientID, operatorID uuid.UUID) (*shared.RoutingDecision, error) {
	key := cacheKey{clientID, operatorID}

	c.mu.RLock()
	if entry, ok := c.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.decision, nil
	}
	c.mu.RUnlock()

	decision, err := c.inner.Route(ctx, clientID, operatorID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[key] = &cacheEntry{decision: decision, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return decision, nil
}

func (c *CachedUnifiedRouter) Invalidate(clientID, operatorID uuid.UUID) {
	c.mu.Lock()
	delete(c.cache, cacheKey{clientID, operatorID})
	c.mu.Unlock()
}
