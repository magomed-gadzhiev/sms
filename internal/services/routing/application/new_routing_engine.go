package application

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
	"github.com/smpp-server/smpp-server/internal/shared"
)

var (
	ErrNoRouteFound        = errors.New("no route found for operator")
	ErrNoAvailableProvider = errors.New("no available provider after filtering")
)

// ProviderInfo holds provider data needed for routing decisions.
type ProviderInfo struct {
	ID           uuid.UUID
	Active       bool
	TPSLimit     int
	DailyQuota   int
	MonthlyQuota int
}

// ProviderLookup resolves provider info by ID.
type ProviderLookup interface {
	GetProviderInfo(ctx context.Context, id uuid.UUID) (*ProviderInfo, error)
}

// NewRoutingEngine implements the new client-based routing pipeline.
type NewRoutingEngine struct {
	strategyRepo domain.ClientRoutingStrategyRepository
	routeRepo    domain.ClientRouteRepository
	providerRepo domain.ClientProviderRepository
	capacity     *infrastructure.CapacityTracker
	providers    ProviderLookup
	logger       zerolog.Logger
}

func NewNewRoutingEngine(
	strategyRepo domain.ClientRoutingStrategyRepository,
	routeRepo domain.ClientRouteRepository,
	providerRepo domain.ClientProviderRepository,
	capacity *infrastructure.CapacityTracker,
	providers ProviderLookup,
) *NewRoutingEngine {
	return &NewRoutingEngine{
		strategyRepo: strategyRepo,
		routeRepo:    routeRepo,
		providerRepo: providerRepo,
		capacity:     capacity,
		providers:    providers,
		logger:       log.With().Str("component", "new-routing-engine").Logger(),
	}
}

// SelectProvider selects a provider for the given client and operator using the new routing system.
func (e *NewRoutingEngine) SelectProvider(ctx context.Context, clientID, operatorID uuid.UUID) (*uuid.UUID, error) {
	// 1. Resolve routing strategy: (client, operator) -> (client, nil) -> default priority
	strategy := e.resolveStrategy(ctx, clientID, operatorID)

	// 2. Get active routes for client + operator
	routes, err := e.routeRepo.ListByClientAndOperator(ctx, clientID, operatorID, true)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	if len(routes) == 0 {
		return nil, ErrNoRouteFound
	}

	// 3. Filter and select by strategy
	switch strategy {
	case domain.StrategyWeighted:
		return e.selectWeighted(ctx, routes)
	default: // priority is the default
		return e.selectPriority(ctx, routes)
	}
}

func (e *NewRoutingEngine) resolveStrategy(ctx context.Context, clientID, operatorID uuid.UUID) domain.RoutingStrategy {
	// Try (client, operator)
	s, err := e.strategyRepo.Get(ctx, clientID, &operatorID)
	if err == nil {
		return s.Strategy
	}

	// Fallback: (client, nil) — account default
	s, err = e.strategyRepo.Get(ctx, clientID, nil)
	if err == nil {
		return s.Strategy
	}

	// Global default
	return domain.StrategyPriority
}

func (e *NewRoutingEngine) selectPriority(ctx context.Context, routes []*domain.ClientRoute) (*uuid.UUID, error) {
	// Routes already sorted by priority DESC from repo
	for _, route := range routes {
		available, err := e.isProviderAvailable(ctx, route.ProviderID)
		if err != nil {
			e.logger.Warn().Err(err).Str("provider_id", route.ProviderID.String()).Msg("provider availability check failed, skipping")
			continue
		}
		if available {
			id := route.ProviderID
			return &id, nil
		}
		e.logger.Debug().Str("provider_id", route.ProviderID.String()).Int("priority", route.Priority).Msg("provider unavailable, trying next")
	}
	return nil, ErrNoAvailableProvider
}

func (e *NewRoutingEngine) selectWeighted(ctx context.Context, routes []*domain.ClientRoute) (*uuid.UUID, error) {
	// Filter to available providers first
	var available []*domain.ClientRoute
	for _, route := range routes {
		ok, err := e.isProviderAvailable(ctx, route.ProviderID)
		if err != nil {
			e.logger.Warn().Err(err).Str("provider_id", route.ProviderID.String()).Msg("provider availability check failed, skipping")
			continue
		}
		if ok {
			available = append(available, route)
		}
	}
	if len(available) == 0 {
		return nil, ErrNoAvailableProvider
	}

	// Weighted random selection
	totalWeight := 0
	for _, r := range available {
		totalWeight += r.Weight
	}

	pick := rand.Intn(totalWeight)
	cumulative := 0
	for _, r := range available {
		cumulative += r.Weight
		if pick < cumulative {
			id := r.ProviderID
			return &id, nil
		}
	}

	// Fallback (shouldn't reach here)
	id := available[0].ProviderID
	return &id, nil
}

func (e *NewRoutingEngine) isProviderAvailable(ctx context.Context, providerID uuid.UUID) (bool, error) {
	info, err := e.providers.GetProviderInfo(ctx, providerID)
	if err != nil {
		return false, err
	}
	if !info.Active {
		return false, nil
	}

	// Check capacity
	ok, err := e.capacity.CheckAndIncrement(ctx, providerID, info.TPSLimit, info.DailyQuota, info.MonthlyQuota)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// RouteMessage routes a message using client's routing_mode to decide legacy vs new pipeline.
// Returns providerID. If routing_mode is "legacy", returns nil to signal legacy fallback.
func RouteMessage(ctx context.Context, client *shared.Client, operatorID uuid.UUID, engine *NewRoutingEngine) (*uuid.UUID, error) {
	switch client.RoutingMode {
	case "legacy":
		return nil, nil // signal to use legacy router
	case "new":
		return engine.SelectProvider(ctx, client.ID, operatorID)
	case "hybrid":
		providerID, err := engine.SelectProvider(ctx, client.ID, operatorID)
		if errors.Is(err, ErrNoRouteFound) {
			return nil, nil // fallback to legacy
		}
		return providerID, err
	default:
		return nil, nil // unknown mode, use legacy
	}
}
