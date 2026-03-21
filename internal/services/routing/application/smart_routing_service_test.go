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

func TestSmartRoutingService(t *testing.T) {

	t.Run("SelectOptimalProvider", func(t *testing.T) {

		t.Run("selects_best_provider", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			cheapProviderID := uuid.New()
			qualityProviderID := uuid.New()

			candidates := []*domain.ProviderCandidate{
				{
					ProviderID:   cheapProviderID,
					Cost:         0.01,
					DeliveryRate: 85.0,
					Active:       true,
					Healthy:      true,
				},
				{
					ProviderID:   qualityProviderID,
					Cost:         0.05,
					DeliveryRate: 99.0,
					Active:       true,
					Healthy:      true,
				},
			}

			// Cost-oriented weights: 0.7 cost, 0.3 quality
			weights := &domain.SmartRouteWeight{
				ID:            uuid.New(),
				OperatorCode:  "mts-ru",
				CountryCode:   "RU",
				CostWeight:    0.7,
				QualityWeight: 0.3,
				Active:        true,
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "mts-ru", "RU").
				Return(weights, nil)

			selectedID, score, err := svc.SelectOptimalProvider(
				context.Background(), "mts-ru", "RU", candidates,
			)

			require.NoError(t, err)
			assert.Equal(t, cheapProviderID, selectedID)
			assert.Greater(t, score, 0.0)

			weightRepo.AssertExpectations(t)
		})

		t.Run("quality_oriented_weights", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			cheapProviderID := uuid.New()
			qualityProviderID := uuid.New()

			candidates := []*domain.ProviderCandidate{
				{
					ProviderID:   cheapProviderID,
					Cost:         0.01,
					DeliveryRate: 70.0,
					Active:       true,
					Healthy:      true,
				},
				{
					ProviderID:   qualityProviderID,
					Cost:         0.10,
					DeliveryRate: 99.0,
					Active:       true,
					Healthy:      true,
				},
			}

			// Quality-oriented weights: 0.2 cost, 0.8 quality
			weights := &domain.SmartRouteWeight{
				ID:            uuid.New(),
				OperatorCode:  "mts-ru",
				CountryCode:   "RU",
				CostWeight:    0.2,
				QualityWeight: 0.8,
				Active:        true,
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "mts-ru", "RU").
				Return(weights, nil)

			selectedID, score, err := svc.SelectOptimalProvider(
				context.Background(), "mts-ru", "RU", candidates,
			)

			require.NoError(t, err)
			assert.Equal(t, qualityProviderID, selectedID)
			assert.Greater(t, score, 0.0)
		})

		t.Run("no_candidates", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			selectedID, score, err := svc.SelectOptimalProvider(
				context.Background(), "mts-ru", "RU", []*domain.ProviderCandidate{},
			)

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
			assert.Equal(t, uuid.Nil, selectedID)
			assert.Equal(t, 0.0, score)
		})

		t.Run("uses_default_weights_on_repo_error", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			providerID := uuid.New()
			candidates := []*domain.ProviderCandidate{
				{
					ProviderID:   providerID,
					Cost:         0.05,
					DeliveryRate: 95.0,
					Active:       true,
					Healthy:      true,
				},
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "unknown", "XX").
				Return(nil, errors.New("not found"))

			selectedID, score, err := svc.SelectOptimalProvider(
				context.Background(), "unknown", "XX", candidates,
			)

			require.NoError(t, err)
			assert.Equal(t, providerID, selectedID)
			assert.Greater(t, score, 0.0)
		})

		t.Run("excludes_inactive_providers", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			inactiveID := uuid.New()
			activeID := uuid.New()

			candidates := []*domain.ProviderCandidate{
				{
					ProviderID:   inactiveID,
					Cost:         0.001,
					DeliveryRate: 99.9,
					Active:       false,
					Healthy:      true,
				},
				{
					ProviderID:   activeID,
					Cost:         0.10,
					DeliveryRate: 80.0,
					Active:       true,
					Healthy:      true,
				},
			}

			weights := &domain.SmartRouteWeight{
				CostWeight:    0.6,
				QualityWeight: 0.4,
				Active:        true,
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "mts-ru", "RU").
				Return(weights, nil)

			selectedID, _, err := svc.SelectOptimalProvider(
				context.Background(), "mts-ru", "RU", candidates,
			)

			require.NoError(t, err)
			assert.Equal(t, activeID, selectedID)
		})

		t.Run("excludes_unhealthy_providers", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			unhealthyID := uuid.New()
			healthyID := uuid.New()

			candidates := []*domain.ProviderCandidate{
				{
					ProviderID:   unhealthyID,
					Cost:         0.001,
					DeliveryRate: 99.9,
					Active:       true,
					Healthy:      false,
				},
				{
					ProviderID:   healthyID,
					Cost:         0.10,
					DeliveryRate: 80.0,
					Active:       true,
					Healthy:      true,
				},
			}

			weights := &domain.SmartRouteWeight{
				CostWeight:    0.6,
				QualityWeight: 0.4,
				Active:        true,
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "mts-ru", "RU").
				Return(weights, nil)

			selectedID, _, err := svc.SelectOptimalProvider(
				context.Background(), "mts-ru", "RU", candidates,
			)

			require.NoError(t, err)
			assert.Equal(t, healthyID, selectedID)
		})

		t.Run("all_candidates_inactive", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			candidates := []*domain.ProviderCandidate{
				{
					ProviderID:   uuid.New(),
					Cost:         0.01,
					DeliveryRate: 95.0,
					Active:       false,
					Healthy:      true,
				},
			}

			weights := &domain.SmartRouteWeight{
				CostWeight:    0.6,
				QualityWeight: 0.4,
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "mts-ru", "RU").
				Return(weights, nil)

			_, _, err := svc.SelectOptimalProvider(
				context.Background(), "mts-ru", "RU", candidates,
			)

			require.Error(t, err)
		})
	})

	t.Run("SetWeights", func(t *testing.T) {

		t.Run("valid_weights", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			weightRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SmartRouteWeight")).
				Return(nil)

			result, err := svc.SetWeights(context.Background(), "mts-ru", "RU", 0.6, 0.4)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "mts-ru", result.OperatorCode)
			assert.Equal(t, "RU", result.CountryCode)
			assert.Equal(t, 0.6, result.CostWeight)
			assert.Equal(t, 0.4, result.QualityWeight)

			weightRepo.AssertExpectations(t)
		})

		t.Run("invalid_weights_sum_not_one", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			result, err := svc.SetWeights(context.Background(), "mts-ru", "RU", 0.5, 0.3)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrInvalidWeights)

			weightRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
		})

		t.Run("repo_error", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			weightRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SmartRouteWeight")).
				Return(errors.New("db error"))

			result, err := svc.SetWeights(context.Background(), "mts-ru", "RU", 0.6, 0.4)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "ошибка сохранения весов")
		})
	})

	t.Run("GetWeights", func(t *testing.T) {

		t.Run("found", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			expected := &domain.SmartRouteWeight{
				ID:            uuid.New(),
				OperatorCode:  "mts-ru",
				CountryCode:   "RU",
				CostWeight:    0.6,
				QualityWeight: 0.4,
			}

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "mts-ru", "RU").
				Return(expected, nil)

			result, err := svc.GetWeights(context.Background(), "mts-ru", "RU")

			require.NoError(t, err)
			assert.Equal(t, expected, result)
		})

		t.Run("not_found", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			weightRepo.On("GetByOperatorAndCountry", mock.Anything, "unknown", "XX").
				Return(nil, domain.ErrSmartRouteWeightNotFound)

			result, err := svc.GetWeights(context.Background(), "unknown", "XX")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrSmartRouteWeightNotFound)
		})
	})

	t.Run("ListWeights", func(t *testing.T) {

		t.Run("returns_list", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			expected := []*domain.SmartRouteWeight{
				{ID: uuid.New(), OperatorCode: "mts-ru", CountryCode: "RU"},
				{ID: uuid.New(), OperatorCode: "beeline-ru", CountryCode: "RU"},
			}

			weightRepo.On("List", mock.Anything, "RU").Return(expected, nil)

			result, err := svc.ListWeights(context.Background(), "RU")

			require.NoError(t, err)
			assert.Len(t, result, 2)
		})
	})

	t.Run("DeleteWeights", func(t *testing.T) {

		t.Run("success", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			id := uuid.New()
			weightRepo.On("Delete", mock.Anything, id).Return(nil)

			err := svc.DeleteWeights(context.Background(), id)

			require.NoError(t, err)
			weightRepo.AssertExpectations(t)
		})

		t.Run("not_found", func(t *testing.T) {
			weightRepo := new(mocks.MockSmartRouteWeightRepository)
			svc := NewSmartRoutingService(weightRepo)

			id := uuid.New()
			weightRepo.On("Delete", mock.Anything, id).
				Return(domain.ErrSmartRouteWeightNotFound)

			err := svc.DeleteWeights(context.Background(), id)

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrSmartRouteWeightNotFound)
		})
	})
}
