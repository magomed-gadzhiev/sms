package router

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func TestRouter(t *testing.T) {
	t.Run("RouteMessage", func(t *testing.T) {
		t.Run("with provider ID", func(t *testing.T) {
			t.Run("routes to active provider", func(t *testing.T) {
				providerID := uuid.New()
				provider := testutil.NewTestProvider()
				provider.ID = providerID

				routeRepo := &testutil.MockRouteRepository{}
				providerRepo := &testutil.MockProviderRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
						if id == providerID {
							return provider, nil
						}
						return nil, storage.ErrNotFound
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.ProviderID = &providerID

				result, err := router.RouteMessage(context.Background(), msg)

				require.NoError(t, err)
				assert.Equal(t, providerID, result.ID)
			})

			t.Run("returns error for inactive provider", func(t *testing.T) {
				providerID := uuid.New()
				provider := testutil.NewTestProvider()
				provider.ID = providerID
				provider.Active = false

				routeRepo := &testutil.MockRouteRepository{}
				providerRepo := &testutil.MockProviderRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
						if id == providerID {
							return provider, nil
						}
						return nil, storage.ErrNotFound
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.ProviderID = &providerID

				_, err := router.RouteMessage(context.Background(), msg)

				assert.Error(t, err)
				assert.Contains(t, err.Error(), "неактивен")
			})
		})

		t.Run("with route ID", func(t *testing.T) {
			t.Run("routes to provider from route", func(t *testing.T) {
				providerID := uuid.New()
				routeID := uuid.New()
				provider := testutil.NewTestProvider()
				provider.ID = providerID

				route := testutil.NewTestRoute(providerID)
				route.ID = routeID

				routeRepo := &testutil.MockRouteRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
						if id == routeID {
							return route, nil
						}
						return nil, storage.ErrNotFound
					},
				}
				providerRepo := &testutil.MockProviderRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
						if id == providerID {
							return provider, nil
						}
						return nil, storage.ErrNotFound
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.RouteID = &routeID

				result, err := router.RouteMessage(context.Background(), msg)

				require.NoError(t, err)
				assert.Equal(t, providerID, result.ID)
			})

			t.Run("uses failover when primary provider inactive", func(t *testing.T) {
				providerID := uuid.New()
				failoverProviderID := uuid.New()
				routeID := uuid.New()

				provider := testutil.NewTestProvider()
				provider.ID = providerID
				provider.Active = false

				failoverProvider := testutil.NewTestProvider()
				failoverProvider.ID = failoverProviderID

				route := testutil.NewTestRoute(providerID)
				route.ID = routeID
				route.FailoverProviderID = &failoverProviderID

				routeRepo := &testutil.MockRouteRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
						if id == routeID {
							return route, nil
						}
						return nil, storage.ErrNotFound
					},
				}
				providerRepo := &testutil.MockProviderRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
						if id == providerID {
							return provider, nil
						}
						if id == failoverProviderID {
							return failoverProvider, nil
						}
						return nil, storage.ErrNotFound
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.RouteID = &routeID

				result, err := router.RouteMessage(context.Background(), msg)

				require.NoError(t, err)
				assert.Equal(t, failoverProviderID, result.ID)
			})
		})

		t.Run("by destination", func(t *testing.T) {
			t.Run("matches route by prefix", func(t *testing.T) {
				providerID := uuid.New()
				provider := testutil.NewTestProvider()
				provider.ID = providerID

				route := testutil.NewTestRoute(providerID)
				route.Pattern = "7900"
				route.PatternType = "prefix"

				routeRepo := &testutil.MockRouteRepository{
					GetActiveByDestinationFunc: func(ctx context.Context, destination string) ([]*shared.Route, error) {
						return []*shared.Route{route}, nil
					},
				}
				providerRepo := &testutil.MockProviderRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
						if id == providerID {
							return provider, nil
						}
						return nil, storage.ErrNotFound
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.Destination = "79001234567"

				result, err := router.RouteMessage(context.Background(), msg)

				require.NoError(t, err)
				assert.Equal(t, providerID, result.ID)
			})

			t.Run("falls back to default provider when no routes", func(t *testing.T) {
				providerID := uuid.New()
				provider := testutil.NewTestProvider()
				provider.ID = providerID

				routeRepo := &testutil.MockRouteRepository{
					GetActiveByDestinationFunc: func(ctx context.Context, destination string) ([]*shared.Route, error) {
						return []*shared.Route{}, nil
					},
				}
				providerRepo := &testutil.MockProviderRepository{
					GetAllActiveFunc: func(ctx context.Context) ([]*shared.Provider, error) {
						return []*shared.Provider{provider}, nil
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.Destination = "79001234567"

				result, err := router.RouteMessage(context.Background(), msg)

				require.NoError(t, err)
				assert.Equal(t, providerID, result.ID)
			})

			t.Run("returns error when no routes and no providers", func(t *testing.T) {
				routeRepo := &testutil.MockRouteRepository{
					GetActiveByDestinationFunc: func(ctx context.Context, destination string) ([]*shared.Route, error) {
						return []*shared.Route{}, nil
					},
				}
				providerRepo := &testutil.MockProviderRepository{
					GetAllActiveFunc: func(ctx context.Context) ([]*shared.Provider, error) {
						return []*shared.Provider{}, nil
					},
				}

				router := NewRouter(routeRepo, providerRepo)

				msg := testutil.NewTestMessage()
				msg.Destination = "79001234567"

				_, err := router.RouteMessage(context.Background(), msg)

				assert.Error(t, err)
				assert.Contains(t, err.Error(), "нет активных провайдеров")
			})
		})
	})

	t.Run("GetFailoverProvider", func(t *testing.T) {
		t.Run("returns failover provider when configured", func(t *testing.T) {
			routeID := uuid.New()
			providerID := uuid.New()
			failoverProviderID := uuid.New()

			failoverProvider := testutil.NewTestProvider()
			failoverProvider.ID = failoverProviderID

			route := testutil.NewTestRoute(providerID)
			route.ID = routeID
			route.FailoverProviderID = &failoverProviderID

			routeRepo := &testutil.MockRouteRepository{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
					if id == routeID {
						return route, nil
					}
					return nil, storage.ErrNotFound
				},
			}
			providerRepo := &testutil.MockProviderRepository{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
					if id == failoverProviderID {
						return failoverProvider, nil
					}
					return nil, storage.ErrNotFound
				},
			}

			router := NewRouter(routeRepo, providerRepo)

			result, err := router.GetFailoverProvider(context.Background(), routeID)

			require.NoError(t, err)
			assert.Equal(t, failoverProviderID, result.ID)
		})

		t.Run("returns error when no failover configured", func(t *testing.T) {
			routeID := uuid.New()
			providerID := uuid.New()

			route := testutil.NewTestRoute(providerID)
			route.ID = routeID
			route.FailoverProviderID = nil

			routeRepo := &testutil.MockRouteRepository{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
					if id == routeID {
						return route, nil
					}
					return nil, storage.ErrNotFound
				},
			}
			providerRepo := &testutil.MockProviderRepository{}

			router := NewRouter(routeRepo, providerRepo)

			_, err := router.GetFailoverProvider(context.Background(), routeID)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "нет failover провайдера")
		})
	})
}
