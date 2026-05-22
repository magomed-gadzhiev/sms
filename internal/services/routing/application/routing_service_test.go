package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRoutingService(t *testing.T) {

	t.Run("RouteMessage", func(t *testing.T) {

		t.Run("matching_route", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			route := &domain.Route{
				ID:                  routeID,
				Name:                "Russia Mobile",
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Priority:            1,
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			}

			providerInfo := &domain.ProviderInfo{
				ID:     providerID,
				Name:   "TestProvider",
				Active: true,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{route}, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).
				Return(providerInfo, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).
				Return(nil)

			gotRouteID, gotProviderID, err := svc.RouteMessage(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.NoError(t, err)
			assert.Equal(t, routeID, gotRouteID)
			assert.Equal(t, providerID, gotProviderID)

			routeRepo.AssertExpectations(t)
			providerRepo.AssertExpectations(t)
			eventPub.AssertExpectations(t)
		})

		t.Run("no_route", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			destination := "99999999999"

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{}, nil)

			gotRouteID, gotProviderID, err := svc.RouteMessage(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.Error(t, err)
			assert.Equal(t, uuid.Nil, gotRouteID)
			assert.Equal(t, uuid.Nil, gotProviderID)
			assert.Contains(t, err.Error(), "маршрут")

			routeRepo.AssertExpectations(t)
		})

		t.Run("existing_route_id", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			route := &domain.Route{
				ID:                  routeID,
				Name:                "Explicit Route",
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Priority:            1,
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			providerInfo := &domain.ProviderInfo{
				ID:     providerID,
				Name:   "TestProvider",
				Active: true,
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(route, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).Return(providerInfo, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).Return(nil)

			gotRouteID, gotProviderID, err := svc.RouteMessage(
				context.Background(), messageID, destination, &clientID, &routeID, nil,
			)

			require.NoError(t, err)
			assert.Equal(t, routeID, gotRouteID)
			assert.Equal(t, providerID, gotProviderID)

			// GetActiveByDestination should NOT be called when route ID is specified
			routeRepo.AssertNotCalled(t, "GetActiveByDestination", mock.Anything, mock.Anything)
		})

		t.Run("existing_route_inactive", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			route := &domain.Route{
				ID:     routeID,
				Name:   "Inactive Route",
				Active: false,
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(route, nil)

			_, _, err := svc.RouteMessage(
				context.Background(), messageID, destination, &clientID, &routeID, nil,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "неактивен")
		})

		t.Run("existing_provider_id_in_route", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			route := &domain.Route{
				ID:                  routeID,
				Name:                "Route with provider",
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Priority:            1,
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			providerInfo := &domain.ProviderInfo{
				ID:     providerID,
				Name:   "TestProvider",
				Active: true,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{route}, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).Return(providerInfo, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).Return(nil)

			gotRouteID, gotProviderID, err := svc.RouteMessage(
				context.Background(), messageID, destination, &clientID, nil, &providerID,
			)

			require.NoError(t, err)
			assert.Equal(t, routeID, gotRouteID)
			assert.Equal(t, providerID, gotProviderID)
		})

		t.Run("no_route_but_existing_provider", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			destination := "99999999999"

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{}, nil)

			gotRouteID, gotProviderID, err := svc.RouteMessage(
				context.Background(), messageID, destination, &clientID, nil, &providerID,
			)

			require.NoError(t, err)
			assert.Equal(t, uuid.Nil, gotRouteID)
			assert.Equal(t, providerID, gotProviderID)
		})

		t.Run("route_repo_error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			destination := "79001234567"

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return(nil, errors.New("db connection error"))

			_, _, err := svc.RouteMessage(
				context.Background(), messageID, destination, nil, nil, nil,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "маршрут")
		})
	})

	t.Run("GetRoute", func(t *testing.T) {

		t.Run("returns_highest_priority", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			highPriRoute := &domain.Route{
				ID:       uuid.New(),
				Name:     "High Priority",
				Priority: 1,
				Active:   true,
			}
			lowPriRoute := &domain.Route{
				ID:       uuid.New(),
				Name:     "Low Priority",
				Priority: 10,
				Active:   true,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, "79001234567").
				Return([]*domain.Route{highPriRoute, lowPriRoute}, nil)

			result, err := svc.GetRoute(context.Background(), "79001234567", nil)

			require.NoError(t, err)
			assert.Equal(t, highPriRoute.ID, result.ID)
		})

		t.Run("no_routes", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeRepo.On("GetActiveByDestination", mock.Anything, "99999").
				Return([]*domain.Route{}, nil)

			result, err := svc.GetRoute(context.Background(), "99999", nil)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrNoMatchingRoute)
		})
	})

	t.Run("SelectProvider", func(t *testing.T) {

		t.Run("failover_on_inactive_primary", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			primaryID := uuid.New()
			failoverID := uuid.New()

			route := &domain.Route{
				ID:                  uuid.New(),
				ProviderIDs:         []uuid.UUID{primaryID, failoverID},
				FailoverEnabled:     true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			primaryProvider := &domain.ProviderInfo{
				ID:     primaryID,
				Name:   "Primary",
				Active: false, // inactive
			}
			failoverProvider := &domain.ProviderInfo{
				ID:     failoverID,
				Name:   "Failover",
				Active: true,
			}

			providerRepo.On("GetByID", mock.Anything, primaryID).Return(primaryProvider, nil)
			providerRepo.On("GetByID", mock.Anything, failoverID).Return(failoverProvider, nil)

			selectedID, err := svc.SelectProvider(context.Background(), route, nil)

			require.NoError(t, err)
			assert.Equal(t, failoverID, selectedID)
		})

		t.Run("no_providers", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			route := &domain.Route{
				ID:          uuid.New(),
				ProviderIDs: []uuid.UUID{},
			}

			_, err := svc.SelectProvider(context.Background(), route, nil)

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})

		t.Run("all_providers_unavailable", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			provID := uuid.New()
			route := &domain.Route{
				ID:                  uuid.New(),
				ProviderIDs:         []uuid.UUID{provID},
				FailoverEnabled:     false,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			providerRepo.On("GetByID", mock.Anything, provID).Return(&domain.ProviderInfo{
				ID:     provID,
				Name:   "Dead",
				Active: false,
			}, nil)

			_, err := svc.SelectProvider(context.Background(), route, nil)

			require.Error(t, err)
			// RoundRobinSelector filters out inactive providers and returns ErrNoProviders,
			// which gets wrapped by SelectProvider as "ошибка выбора провайдера"
			assert.Contains(t, err.Error(), "ошибка выбора провайдера")
		})

		t.Run("unknown_strategy_falls_back_to_round_robin", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			provID := uuid.New()
			route := &domain.Route{
				ID:                  uuid.New(),
				ProviderIDs:         []uuid.UUID{provID},
				LoadBalanceStrategy: domain.LoadBalanceStrategy("unknown_strategy"),
			}

			providerRepo.On("GetByID", mock.Anything, provID).Return(&domain.ProviderInfo{
				ID:     provID,
				Name:   "Provider1",
				Active: true,
			}, nil)

			selectedID, err := svc.SelectProvider(context.Background(), route, nil)

			require.NoError(t, err)
			assert.Equal(t, provID, selectedID)
		})

		t.Run("provider_repo_error_skips_provider", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			badProvID := uuid.New()
			goodProvID := uuid.New()
			route := &domain.Route{
				ID:                  uuid.New(),
				ProviderIDs:         []uuid.UUID{badProvID, goodProvID},
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			providerRepo.On("GetByID", mock.Anything, badProvID).Return(nil, errors.New("db error"))
			providerRepo.On("GetByID", mock.Anything, goodProvID).Return(&domain.ProviderInfo{
				ID:     goodProvID,
				Name:   "Good",
				Active: true,
			}, nil)

			selectedID, err := svc.SelectProvider(context.Background(), route, nil)

			require.NoError(t, err)
			assert.Equal(t, goodProvID, selectedID)
		})

		t.Run("all_provider_repo_errors_returns_no_providers", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			provID := uuid.New()
			route := &domain.Route{
				ID:                  uuid.New(),
				ProviderIDs:         []uuid.UUID{provID},
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			providerRepo.On("GetByID", mock.Anything, provID).Return(nil, errors.New("db error"))

			_, err := svc.SelectProvider(context.Background(), route, nil)

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrNoProviders)
		})
	})

	t.Run("SetHLRService", func(t *testing.T) {
		t.Run("sets_hlr_service", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			hlrCache := new(mocks.MockHLRCache)
			hlrProviderRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			hlrSvc := NewHLRService(hlrCache, hlrProviderRepo, logRepo, factory, 0)
			svc.SetHLRService(hlrSvc)

			assert.Equal(t, hlrSvc, svc.hlrService)
		})
	})

	t.Run("CreateRoute", func(t *testing.T) {

		t.Run("success", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			providerID := uuid.New()
			routeRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Route")).Return(nil)

			route, err := svc.CreateRoute(
				context.Background(),
				"Russia MTS",
				"+7900",
				domain.PatternTypePrefix,
				[]uuid.UUID{providerID},
				10,
				domain.LoadBalanceRoundRobin,
				true,
				map[string]string{"region": "ru"},
			)

			require.NoError(t, err)
			require.NotNil(t, route)
			assert.Equal(t, "Russia MTS", route.Name)
			assert.Equal(t, "+7900", route.Pattern)
			assert.Equal(t, domain.PatternTypePrefix, route.PatternType)
			assert.True(t, route.FailoverEnabled)
			assert.Equal(t, "ru", route.Metadata["region"])
			routeRepo.AssertExpectations(t)
		})

		t.Run("success_without_failover_and_metadata", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Route")).Return(nil)

			route, err := svc.CreateRoute(
				context.Background(),
				"Test Route",
				"+44",
				domain.PatternTypePrefix,
				[]uuid.UUID{uuid.New()},
				5,
				domain.LoadBalanceRoundRobin,
				false,
				nil,
			)

			require.NoError(t, err)
			require.NotNil(t, route)
			assert.False(t, route.FailoverEnabled)
		})

		t.Run("invalid_pattern_returns_error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			route, err := svc.CreateRoute(
				context.Background(),
				"Bad Route",
				"[invalid-regex",
				domain.PatternTypeRegex,
				[]uuid.UUID{uuid.New()},
				1,
				domain.LoadBalanceRoundRobin,
				false,
				nil,
			)

			require.Error(t, err)
			assert.Nil(t, route)
			assert.Contains(t, err.Error(), "валидация правила")
		})

		t.Run("repo_error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Route")).
				Return(errors.New("db error"))

			route, err := svc.CreateRoute(
				context.Background(),
				"Test",
				"+7",
				domain.PatternTypePrefix,
				[]uuid.UUID{uuid.New()},
				1,
				domain.LoadBalanceRoundRobin,
				false,
				nil,
			)

			require.Error(t, err)
			assert.Nil(t, route)
			assert.Contains(t, err.Error(), "ошибка создания маршрута")
		})
	})

	t.Run("UpdateRoute", func(t *testing.T) {

		t.Run("update_all_fields", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			existingRoute := &domain.Route{
				ID:                  routeID,
				Name:                "Old Name",
				Pattern:             "+7",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{uuid.New()},
				Priority:            1,
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
				FailoverEnabled:     false,
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(existingRoute, nil)
			routeRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Route")).Return(nil)

			newName := "New Name"
			newPattern := "+44"
			newPatternType := domain.PatternTypePrefix
			newPriority := 5
			newStrategy := domain.LoadBalanceCheapest
			newFailover := true
			newActive := false
			newProviders := []uuid.UUID{uuid.New(), uuid.New()}

			err := svc.UpdateRoute(context.Background(), routeID, &RouteUpdate{
				Name:            &newName,
				Pattern:         &newPattern,
				PatternType:     &newPatternType,
				Priority:        &newPriority,
				ProviderIDs:     newProviders,
				Strategy:        &newStrategy,
				FailoverEnabled: &newFailover,
				Active:          &newActive,
				Metadata:        map[string]string{"key": "val"},
			})

			require.NoError(t, err)
			routeRepo.AssertExpectations(t)
		})

		t.Run("update_activate_route", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			existingRoute := &domain.Route{
				ID:     routeID,
				Name:   "Route",
				Active: false,
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(existingRoute, nil)
			routeRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Route")).Return(nil)

			active := true
			err := svc.UpdateRoute(context.Background(), routeID, &RouteUpdate{
				Active: &active,
			})

			require.NoError(t, err)
		})

		t.Run("disable_failover", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			existingRoute := &domain.Route{
				ID:              routeID,
				Name:            "Route",
				Active:          true,
				FailoverEnabled: true,
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(existingRoute, nil)
			routeRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Route")).Return(nil)

			failover := false
			err := svc.UpdateRoute(context.Background(), routeID, &RouteUpdate{
				FailoverEnabled: &failover,
			})

			require.NoError(t, err)
		})

		t.Run("route_not_found", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			routeRepo.On("GetByID", mock.Anything, routeID).Return(nil, errors.New("not found"))

			err := svc.UpdateRoute(context.Background(), routeID, &RouteUpdate{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "маршрут не найден")
		})

		t.Run("invalid_regex_pattern", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			existingRoute := &domain.Route{
				ID:   routeID,
				Name: "Route",
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(existingRoute, nil)

			newPattern := "[invalid"
			newPatternType := domain.PatternTypeRegex
			err := svc.UpdateRoute(context.Background(), routeID, &RouteUpdate{
				Pattern:     &newPattern,
				PatternType: &newPatternType,
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "валидация правила")
		})

		t.Run("repo_update_error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			existingRoute := &domain.Route{
				ID:   routeID,
				Name: "Route",
			}

			routeRepo.On("GetByID", mock.Anything, routeID).Return(existingRoute, nil)
			routeRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Route")).
				Return(errors.New("db error"))

			err := svc.UpdateRoute(context.Background(), routeID, &RouteUpdate{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "ошибка обновления маршрута")
		})
	})

	t.Run("DeleteRoute", func(t *testing.T) {

		t.Run("success", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			routeRepo.On("Delete", mock.Anything, routeID).Return(nil)

			err := svc.DeleteRoute(context.Background(), routeID)

			require.NoError(t, err)
			routeRepo.AssertExpectations(t)
		})

		t.Run("error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeID := uuid.New()
			routeRepo.On("Delete", mock.Anything, routeID).Return(errors.New("db error"))

			err := svc.DeleteRoute(context.Background(), routeID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "ошибка удаления маршрута")
		})
	})

	t.Run("ListRoutes", func(t *testing.T) {

		t.Run("returns_routes", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routes := []*domain.Route{
				{ID: uuid.New(), Name: "Route1"},
				{ID: uuid.New(), Name: "Route2"},
			}

			routeRepo.On("List", mock.Anything, true, 10, 0).Return(routes, 2, nil)

			result, total, err := svc.ListRoutes(context.Background(), true, 10, 0)

			require.NoError(t, err)
			assert.Len(t, result, 2)
			assert.Equal(t, 2, total)
			routeRepo.AssertExpectations(t)
		})

		t.Run("error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			routeRepo.On("List", mock.Anything, false, 20, 5).Return(nil, 0, errors.New("db error"))

			result, total, err := svc.ListRoutes(context.Background(), false, 20, 5)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Equal(t, 0, total)
		})
	})

	t.Run("RouteMessageWithHLR", func(t *testing.T) {

		t.Run("without_hlr_service", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			route := &domain.Route{
				ID:                  routeID,
				Name:                "Test",
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Priority:            1,
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{route}, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).
				Return(&domain.ProviderInfo{ID: providerID, Name: "P", Active: true}, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).
				Return(nil)

			result, err := svc.RouteMessageWithHLR(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, routeID, result.RouteID)
			assert.Equal(t, providerID, result.ProviderID)
			assert.False(t, result.HLRUsed)
			assert.Nil(t, result.HLRResult)
		})

		t.Run("without_client_id_skips_hlr", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			hlrCache := new(mocks.MockHLRCache)
			hlrProviderRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			hlrSvc := NewHLRService(hlrCache, hlrProviderRepo, logRepo, factory, 0)
			svc.SetHLRService(hlrSvc)

			messageID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			route := &domain.Route{
				ID:                  routeID,
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{route}, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).
				Return(&domain.ProviderInfo{ID: providerID, Name: "P", Active: true}, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).
				Return(nil)

			result, err := svc.RouteMessageWithHLR(
				context.Background(), messageID, destination, nil, nil, nil,
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.HLRUsed)

			// HLR cache should NOT be called
			hlrCache.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
		})

		t.Run("hlr_blocks_invalid_number", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			hlrCache := new(mocks.MockHLRCache)
			hlrProviderRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			hlrSvc := NewHLRService(hlrCache, hlrProviderRepo, logRepo, factory, 0)
			svc.SetHLRService(hlrSvc)

			messageID := uuid.New()
			clientID := uuid.New()
			destination := "79001234567"

			invalidResult := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusInvalid,
				Cached:       true,
			}

			hlrCache.On("Get", mock.Anything, "79001234567").Return(invalidResult, nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.RouteMessageWithHLR(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrNumberInvalid)

			time.Sleep(50 * time.Millisecond)
		})

		t.Run("hlr_error_falls_back_to_routing", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			hlrCache := new(mocks.MockHLRCache)
			hlrProviderRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			hlrSvc := NewHLRService(hlrCache, hlrProviderRepo, logRepo, factory, 0)
			svc.SetHLRService(hlrSvc)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			// HLR cache miss, providers fail
			hlrCache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			hlrProviderRepo.On("GetByPriority", mock.Anything, "RU").Return([]*domain.HLRProvider{}, nil)
			hlrProviderRepo.On("ListActive", mock.Anything).Return([]*domain.HLRProvider{}, nil)

			// But normal routing works
			route := &domain.Route{
				ID:                  routeID,
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{route}, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).
				Return(&domain.ProviderInfo{ID: providerID, Name: "P", Active: true}, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).
				Return(nil)

			result, err := svc.RouteMessageWithHLR(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, routeID, result.RouteID)
			assert.Equal(t, providerID, result.ProviderID)
			assert.False(t, result.HLRUsed)
		})

		t.Run("hlr_success_with_valid_number", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			hlrCache := new(mocks.MockHLRCache)
			hlrProviderRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			hlrSvc := NewHLRService(hlrCache, hlrProviderRepo, logRepo, factory, 0)
			svc.SetHLRService(hlrSvc)

			messageID := uuid.New()
			clientID := uuid.New()
			providerID := uuid.New()
			routeID := uuid.New()
			destination := "79001234567"

			activeResult := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusActive,
				Cached:       true,
			}

			hlrCache.On("Get", mock.Anything, "79001234567").Return(activeResult, nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			route := &domain.Route{
				ID:                  routeID,
				Pattern:             "7900",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			}

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{route}, nil)
			providerRepo.On("GetByID", mock.Anything, providerID).
				Return(&domain.ProviderInfo{ID: providerID, Name: "P", Active: true}, nil)
			eventPub.On("PublishMessageRouted", mock.Anything, messageID, routeID, providerID).
				Return(nil)

			result, err := svc.RouteMessageWithHLR(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.HLRUsed)
			assert.NotNil(t, result.HLRResult)
			assert.Equal(t, routeID, result.RouteID)

			time.Sleep(50 * time.Millisecond)
		})

		t.Run("routing_error_returns_error", func(t *testing.T) {
			routeRepo := new(mocks.MockRouteRepository)
			providerRepo := new(mocks.MockProviderRepository)
			eventPub := new(mocks.MockEventPublisher)

			svc := NewRoutingService(routeRepo, providerRepo, eventPub)

			messageID := uuid.New()
			clientID := uuid.New()
			destination := "99999999999"

			routeRepo.On("GetActiveByDestination", mock.Anything, destination).
				Return([]*domain.Route{}, nil)

			result, err := svc.RouteMessageWithHLR(
				context.Background(), messageID, destination, &clientID, nil, nil,
			)

			require.Error(t, err)
			assert.Nil(t, result)
		})
	})
}
