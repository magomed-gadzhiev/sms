package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLeastLoadedSelector(t *testing.T) {

	t.Run("SelectProvider", func(t *testing.T) {

		t.Run("selects_provider_with_lowest_load", func(t *testing.T) {
			providerRepo := new(mocks.MockProviderRepository)
			selector := NewLeastLoadedSelector(providerRepo)

			providerID1 := uuid.New()
			providerID2 := uuid.New()
			route := &domain.Route{ID: uuid.New()}

			providers := []*domain.ProviderInfo{
				{ID: providerID1, Name: "HighLoad", Active: true},
				{ID: providerID2, Name: "LowLoad", Active: true},
			}

			providerRepo.On("GetHealthBatch", mock.Anything, mock.Anything).Return(
				map[uuid.UUID]*domain.ProviderHealth{
					providerID1: {ActiveConnections: 80, TotalConnections: 100},
					providerID2: {ActiveConnections: 20, TotalConnections: 100},
				}, nil)

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.NoError(t, err)
			assert.Equal(t, providerID2, selected.ID)
		})

		t.Run("no_providers_returns_error", func(t *testing.T) {
			providerRepo := new(mocks.MockProviderRepository)
			selector := NewLeastLoadedSelector(providerRepo)

			route := &domain.Route{ID: uuid.New()}

			selected, err := selector.SelectProvider(context.Background(), route, []*domain.ProviderInfo{})

			require.Error(t, err)
			assert.Nil(t, selected)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})

		t.Run("all_inactive_returns_error", func(t *testing.T) {
			providerRepo := new(mocks.MockProviderRepository)
			selector := NewLeastLoadedSelector(providerRepo)

			route := &domain.Route{ID: uuid.New()}
			providers := []*domain.ProviderInfo{
				{ID: uuid.New(), Name: "Inactive", Active: false},
			}

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.Error(t, err)
			assert.Nil(t, selected)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})

		t.Run("health_error_skips_provider", func(t *testing.T) {
			providerRepo := new(mocks.MockProviderRepository)
			selector := NewLeastLoadedSelector(providerRepo)

			providerID1 := uuid.New()
			providerID2 := uuid.New()
			route := &domain.Route{ID: uuid.New()}

			providers := []*domain.ProviderInfo{
				{ID: providerID1, Name: "NoHealth", Active: true},
				{ID: providerID2, Name: "HasHealth", Active: true},
			}

			// providerID1 absent from map simulates health error / not found
			providerRepo.On("GetHealthBatch", mock.Anything, mock.Anything).Return(
				map[uuid.UUID]*domain.ProviderHealth{
					providerID2: {ActiveConnections: 10, TotalConnections: 100},
				}, nil)

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.NoError(t, err)
			assert.Equal(t, providerID2, selected.ID)
		})

		t.Run("all_health_errors_returns_no_providers", func(t *testing.T) {
			providerRepo := new(mocks.MockProviderRepository)
			selector := NewLeastLoadedSelector(providerRepo)

			providerID := uuid.New()
			route := &domain.Route{ID: uuid.New()}

			providers := []*domain.ProviderInfo{
				{ID: providerID, Name: "NoHealth", Active: true},
			}

			// Empty map: all providers absent — all skipped
			providerRepo.On("GetHealthBatch", mock.Anything, mock.Anything).Return(
				map[uuid.UUID]*domain.ProviderHealth{}, nil)

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.Error(t, err)
			assert.Nil(t, selected)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})

		t.Run("zero_total_connections_treated_as_zero_load", func(t *testing.T) {
			providerRepo := new(mocks.MockProviderRepository)
			selector := NewLeastLoadedSelector(providerRepo)

			providerID1 := uuid.New()
			providerID2 := uuid.New()
			route := &domain.Route{ID: uuid.New()}

			providers := []*domain.ProviderInfo{
				{ID: providerID1, Name: "ZeroConn", Active: true},
				{ID: providerID2, Name: "HighLoad", Active: true},
			}

			providerRepo.On("GetHealthBatch", mock.Anything, mock.Anything).Return(
				map[uuid.UUID]*domain.ProviderHealth{
					providerID1: {ActiveConnections: 0, TotalConnections: 0},
					providerID2: {ActiveConnections: 90, TotalConnections: 100},
				}, nil)

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.NoError(t, err)
			assert.Equal(t, providerID1, selected.ID)
		})
	})
}

func TestCheapestSelector(t *testing.T) {

	t.Run("SelectProvider", func(t *testing.T) {

		t.Run("selects_highest_priority_provider", func(t *testing.T) {
			selector := NewCheapestSelector()

			lowPriorityID := uuid.New()
			highPriorityID := uuid.New()
			route := &domain.Route{ID: uuid.New()}

			providers := []*domain.ProviderInfo{
				{ID: lowPriorityID, Name: "Expensive", Active: true, Priority: 1},
				{ID: highPriorityID, Name: "Cheap", Active: true, Priority: 10},
			}

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.NoError(t, err)
			assert.Equal(t, highPriorityID, selected.ID)
		})

		t.Run("no_providers_returns_error", func(t *testing.T) {
			selector := NewCheapestSelector()

			route := &domain.Route{ID: uuid.New()}

			selected, err := selector.SelectProvider(context.Background(), route, []*domain.ProviderInfo{})

			require.Error(t, err)
			assert.Nil(t, selected)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})

		t.Run("all_inactive_returns_error", func(t *testing.T) {
			selector := NewCheapestSelector()

			route := &domain.Route{ID: uuid.New()}
			providers := []*domain.ProviderInfo{
				{ID: uuid.New(), Name: "Inactive", Active: false, Priority: 100},
			}

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.Error(t, err)
			assert.Nil(t, selected)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})

		t.Run("filters_inactive_selects_best_active", func(t *testing.T) {
			selector := NewCheapestSelector()

			inactiveID := uuid.New()
			activeID := uuid.New()
			route := &domain.Route{ID: uuid.New()}

			providers := []*domain.ProviderInfo{
				{ID: inactiveID, Name: "CheapInactive", Active: false, Priority: 100},
				{ID: activeID, Name: "ExpensiveActive", Active: true, Priority: 5},
			}

			selected, err := selector.SelectProvider(context.Background(), route, providers)

			require.NoError(t, err)
			assert.Equal(t, activeID, selected.ID)
		})
	})
}

func TestRoundRobinSelector_SelectProvider(t *testing.T) {

	t.Run("single_provider_always_selected", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		providerID := uuid.New()
		route := &domain.Route{ID: uuid.New()}
		providers := []*domain.ProviderInfo{
			{ID: providerID, Name: "OnlyProvider", Active: true},
		}

		selected, err := selector.SelectProvider(context.Background(), route, providers)

		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, providerID, selected.ID)

		// Calling again on same single provider must still return it
		selected2, err := selector.SelectProvider(context.Background(), route, providers)
		require.NoError(t, err)
		assert.Equal(t, providerID, selected2.ID)
	})

	t.Run("cycles_through_multiple_providers", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()
		route := &domain.Route{ID: uuid.New()}
		providers := []*domain.ProviderInfo{
			{ID: id1, Name: "Provider1", Active: true},
			{ID: id2, Name: "Provider2", Active: true},
			{ID: id3, Name: "Provider3", Active: true},
		}

		// First call returns providers[0]
		s1, err := selector.SelectProvider(context.Background(), route, providers)
		require.NoError(t, err)
		assert.Equal(t, id1, s1.ID)

		// Second call returns providers[1]
		s2, err := selector.SelectProvider(context.Background(), route, providers)
		require.NoError(t, err)
		assert.Equal(t, id2, s2.ID)

		// Third call returns providers[2]
		s3, err := selector.SelectProvider(context.Background(), route, providers)
		require.NoError(t, err)
		assert.Equal(t, id3, s3.ID)

		// Fourth call wraps around to providers[0]
		s4, err := selector.SelectProvider(context.Background(), route, providers)
		require.NoError(t, err)
		assert.Equal(t, id1, s4.ID)
	})

	t.Run("filters_inactive_providers", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		activeID := uuid.New()
		route := &domain.Route{ID: uuid.New()}
		providers := []*domain.ProviderInfo{
			{ID: uuid.New(), Name: "Inactive1", Active: false},
			{ID: activeID, Name: "Active", Active: true},
			{ID: uuid.New(), Name: "Inactive2", Active: false},
		}

		selected, err := selector.SelectProvider(context.Background(), route, providers)

		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, activeID, selected.ID)
	})

	t.Run("empty_provider_list_returns_error", func(t *testing.T) {
		selector := NewRoundRobinSelector()
		route := &domain.Route{ID: uuid.New()}

		selected, err := selector.SelectProvider(context.Background(), route, []*domain.ProviderInfo{})

		require.Error(t, err)
		assert.Nil(t, selected)
		assert.ErrorIs(t, err, domain.ErrNoProviders)
	})

	t.Run("all_inactive_returns_error", func(t *testing.T) {
		selector := NewRoundRobinSelector()
		route := &domain.Route{ID: uuid.New()}
		providers := []*domain.ProviderInfo{
			{ID: uuid.New(), Name: "Inactive1", Active: false},
			{ID: uuid.New(), Name: "Inactive2", Active: false},
		}

		selected, err := selector.SelectProvider(context.Background(), route, providers)

		require.Error(t, err)
		assert.Nil(t, selected)
		assert.ErrorIs(t, err, domain.ErrNoProviders)
	})

	t.Run("independent_indices_per_route", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		id1 := uuid.New()
		id2 := uuid.New()
		routeA := &domain.Route{ID: uuid.New()}
		routeB := &domain.Route{ID: uuid.New()}
		providers := []*domain.ProviderInfo{
			{ID: id1, Name: "Provider1", Active: true},
			{ID: id2, Name: "Provider2", Active: true},
		}

		// Advance routeA twice so its index is at 0 again
		_, err := selector.SelectProvider(context.Background(), routeA, providers)
		require.NoError(t, err)
		_, err = selector.SelectProvider(context.Background(), routeA, providers)
		require.NoError(t, err)

		// routeB should still start at index 0 (provider1)
		sB, err := selector.SelectProvider(context.Background(), routeB, providers)
		require.NoError(t, err)
		assert.Equal(t, id1, sB.ID)
	})
}

func TestRoundRobinSelector_PurgeStaleRoutes(t *testing.T) {

	t.Run("removes_stale_route_indices", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		staleRouteID := uuid.New()
		keepRouteID := uuid.New()

		providers := []*domain.ProviderInfo{
			{ID: uuid.New(), Name: "P1", Active: true},
			{ID: uuid.New(), Name: "P2", Active: true},
		}

		// Seed indices for both routes by calling SelectProvider
		_, err := selector.SelectProvider(context.Background(), &domain.Route{ID: staleRouteID}, providers)
		require.NoError(t, err)
		_, err = selector.SelectProvider(context.Background(), &domain.Route{ID: keepRouteID}, providers)
		require.NoError(t, err)

		// Verify both routes have an index stored
		assert.Contains(t, selector.currentIndex, staleRouteID)
		assert.Contains(t, selector.currentIndex, keepRouteID)

		// Purge: only keepRouteID is active
		selector.PurgeStaleRoutes([]uuid.UUID{keepRouteID})

		assert.NotContains(t, selector.currentIndex, staleRouteID)
		assert.Contains(t, selector.currentIndex, keepRouteID)
	})

	t.Run("keeps_all_when_all_active", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		routeID1 := uuid.New()
		routeID2 := uuid.New()
		providers := []*domain.ProviderInfo{
			{ID: uuid.New(), Name: "P1", Active: true},
		}

		_, _ = selector.SelectProvider(context.Background(), &domain.Route{ID: routeID1}, providers)
		_, _ = selector.SelectProvider(context.Background(), &domain.Route{ID: routeID2}, providers)

		selector.PurgeStaleRoutes([]uuid.UUID{routeID1, routeID2})

		assert.Contains(t, selector.currentIndex, routeID1)
		assert.Contains(t, selector.currentIndex, routeID2)
	})

	t.Run("purge_with_empty_active_list_clears_all", func(t *testing.T) {
		selector := NewRoundRobinSelector()

		routeID := uuid.New()
		providers := []*domain.ProviderInfo{
			{ID: uuid.New(), Name: "P1", Active: true},
		}

		_, _ = selector.SelectProvider(context.Background(), &domain.Route{ID: routeID}, providers)
		assert.Contains(t, selector.currentIndex, routeID)

		selector.PurgeStaleRoutes([]uuid.UUID{})

		assert.Empty(t, selector.currentIndex)
	})
}

func TestStrategyFactory(t *testing.T) {

	t.Run("creates_round_robin", func(t *testing.T) {
		providerRepo := new(mocks.MockProviderRepository)
		selector, err := StrategyFactory(domain.LoadBalanceRoundRobin, providerRepo)

		require.NoError(t, err)
		assert.IsType(t, &RoundRobinSelector{}, selector)
	})

	t.Run("creates_least_loaded", func(t *testing.T) {
		providerRepo := new(mocks.MockProviderRepository)
		selector, err := StrategyFactory(domain.LoadBalanceLeastLoaded, providerRepo)

		require.NoError(t, err)
		assert.IsType(t, &LeastLoadedSelector{}, selector)
	})

	t.Run("creates_cheapest", func(t *testing.T) {
		providerRepo := new(mocks.MockProviderRepository)
		selector, err := StrategyFactory(domain.LoadBalanceCheapest, providerRepo)

		require.NoError(t, err)
		assert.IsType(t, &CheapestSelector{}, selector)
	})

	t.Run("unknown_strategy_returns_error", func(t *testing.T) {
		providerRepo := new(mocks.MockProviderRepository)
		selector, err := StrategyFactory(domain.LoadBalanceStrategy("unknown"), providerRepo)

		require.Error(t, err)
		assert.Nil(t, selector)
		assert.Contains(t, err.Error(), "неизвестная стратегия")
	})
}
