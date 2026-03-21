package application

import (
	"context"
	"errors"
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

			// Provider 1 has high load (80%)
			providerRepo.On("GetHealth", mock.Anything, providerID1).Return(&domain.ProviderHealth{
				ActiveConnections: 80,
				TotalConnections:  100,
			}, nil)
			// Provider 2 has low load (20%)
			providerRepo.On("GetHealth", mock.Anything, providerID2).Return(&domain.ProviderHealth{
				ActiveConnections: 20,
				TotalConnections:  100,
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

			providerRepo.On("GetHealth", mock.Anything, providerID1).
				Return(nil, errors.New("no health data"))
			providerRepo.On("GetHealth", mock.Anything, providerID2).
				Return(&domain.ProviderHealth{
					ActiveConnections: 10,
					TotalConnections:  100,
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

			providerRepo.On("GetHealth", mock.Anything, providerID).
				Return(nil, errors.New("no health data"))

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

			// Provider 1 has zero total connections (load = 0)
			providerRepo.On("GetHealth", mock.Anything, providerID1).Return(&domain.ProviderHealth{
				ActiveConnections: 0,
				TotalConnections:  0,
			}, nil)
			// Provider 2 has high load
			providerRepo.On("GetHealth", mock.Anything, providerID2).Return(&domain.ProviderHealth{
				ActiveConnections: 90,
				TotalConnections:  100,
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
