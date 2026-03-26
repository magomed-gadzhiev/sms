package cache

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared"
)

type RouteRepository interface {
	GetAllActive(ctx context.Context) ([]*shared.Route, error)
	GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error)
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error)
}

type ProviderRepository interface {
	GetAllActive(ctx context.Context) ([]*shared.Provider, error)
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error)
}

type RouteCache struct {
	routeRepo    RouteRepository
	providerRepo ProviderRepository
	refreshTTL   time.Duration
	logger       zerolog.Logger

	mu        sync.RWMutex
	routes    []*shared.Route
	providers map[uuid.UUID]*shared.Provider
}

func NewRouteCache(routeRepo RouteRepository, providerRepo ProviderRepository, refreshTTL time.Duration) *RouteCache {
	return &RouteCache{
		routeRepo:    routeRepo,
		providerRepo: providerRepo,
		refreshTTL:   refreshTTL,
		logger:       log.With().Str("component", "route-cache").Logger(),
		providers:    make(map[uuid.UUID]*shared.Provider),
	}
}

func (c *RouteCache) Start(ctx context.Context) error {
	if err := c.refresh(ctx); err != nil {
		return err
	}
	go c.refreshLoop(ctx)
	return nil
}

func (c *RouteCache) GetAllActiveRoutes() []*shared.Route {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.routes
}

func (c *RouteCache) GetProvider(id uuid.UUID) (*shared.Provider, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.providers[id]
	return p, ok
}

func (c *RouteCache) GetRouteByID(id uuid.UUID) (*shared.Route, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, r := range c.routes {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

func (c *RouteCache) MatchRoutes(destination string) []*shared.Route {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var matched []*shared.Route
	for _, r := range c.routes {
		if r.Active && matchesPattern(r, destination) {
			matched = append(matched, r)
		}
	}
	return matched
}

func matchesPattern(route *shared.Route, destination string) bool {
	switch route.PatternType {
	case "prefix":
		return len(destination) >= len(route.Pattern) && destination[:len(route.Pattern)] == route.Pattern
	case "exact":
		return destination == route.Pattern
	default:
		return false
	}
}

func (c *RouteCache) refresh(ctx context.Context) error {
	routes, err := c.routeRepo.GetAllActive(ctx)
	if err != nil {
		return err
	}
	providers, err := c.providerRepo.GetAllActive(ctx)
	if err != nil {
		return err
	}

	providerMap := make(map[uuid.UUID]*shared.Provider, len(providers))
	for _, p := range providers {
		providerMap[p.ID] = p
	}

	c.mu.Lock()
	c.routes = routes
	c.providers = providerMap
	c.mu.Unlock()

	c.logger.Debug().Int("routes", len(routes)).Int("providers", len(providers)).Msg("cache refreshed")
	return nil
}

func (c *RouteCache) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(c.refreshTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.refresh(ctx); err != nil {
				c.logger.Error().Err(err).Msg("cache refresh failed, using stale data")
			}
		}
	}
}
