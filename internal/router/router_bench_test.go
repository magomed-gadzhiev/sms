//go:build !integration

package router

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func BenchmarkRouteMessage(b *testing.B) {
	providerID := uuid.New()
	provider := testutil.NewTestProvider()
	provider.ID = providerID

	routeRepo := &testutil.MockRouteRepository{}
	providerRepo := &testutil.MockProviderRepository{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
			return provider, nil
		},
	}

	router := NewRouter(routeRepo, providerRepo)
	msg := testutil.NewTestMessage()
	msg.ProviderID = &providerID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = router.RouteMessage(context.Background(), msg)
	}
}

func BenchmarkRouteMessage_WithFailover(b *testing.B) {
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
			return route, nil
		},
	}
	providerRepo := &testutil.MockProviderRepository{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
			if id == providerID {
				return provider, nil
			}
			return failoverProvider, nil
		},
	}

	router := NewRouter(routeRepo, providerRepo)
	msg := testutil.NewTestMessage()
	msg.RouteID = &routeID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = router.RouteMessage(context.Background(), msg)
	}
}

func BenchmarkRouteMessage_ByDestination(b *testing.B) {
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
			return provider, nil
		},
	}

	router := NewRouter(routeRepo, providerRepo)
	msg := testutil.NewTestMessage()
	msg.Destination = "79001234567"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = router.RouteMessage(context.Background(), msg)
	}
}
