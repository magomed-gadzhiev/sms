package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/shared"
)

type mockRouteRepo struct {
	mu     sync.Mutex
	routes []*shared.Route
	calls  int
}

func (m *mockRouteRepo) GetAllActive(ctx context.Context) ([]*shared.Route, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.routes, nil
}

func (m *mockRouteRepo) GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error) {
	return m.routes, nil
}

func (m *mockRouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
	for _, r := range m.routes {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

type mockProviderRepo struct {
	mu        sync.Mutex
	providers []*shared.Provider
	calls     int
}

func (m *mockProviderRepo) GetAllActive(ctx context.Context) ([]*shared.Provider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.providers, nil
}

func (m *mockProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func TestRouteCache_InitialLoad(t *testing.T) {
	providerID := uuid.New()
	routeID := uuid.New()

	routeRepo := &mockRouteRepo{
		routes: []*shared.Route{
			{ID: routeID, Name: "test-route", ProviderID: providerID, Active: true, Priority: 10},
		},
	}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{
			{ID: providerID, Name: "test-provider", Active: true, Priority: 10},
		},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	require.NoError(t, err)

	routes := cache.GetAllActiveRoutes()
	assert.Len(t, routes, 1)
	assert.Equal(t, routeID, routes[0].ID)

	provider, ok := cache.GetProvider(providerID)
	assert.True(t, ok)
	assert.Equal(t, "test-provider", provider.Name)
}

func TestRouteCache_RefreshUpdatesData(t *testing.T) {
	providerID := uuid.New()
	routeRepo := &mockRouteRepo{
		routes: []*shared.Route{},
	}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{
			{ID: providerID, Name: "old-name", Active: true},
		},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	require.NoError(t, err)

	providerRepo.mu.Lock()
	providerRepo.providers = []*shared.Provider{
		{ID: providerID, Name: "new-name", Active: true},
	}
	providerRepo.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	provider, ok := cache.GetProvider(providerID)
	assert.True(t, ok)
	assert.Equal(t, "new-name", provider.Name)
}

func TestRouteCache_ConcurrentAccess(t *testing.T) {
	providerID := uuid.New()
	routeRepo := &mockRouteRepo{routes: []*shared.Route{}}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{
			{ID: providerID, Name: "provider", Active: true},
		},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.GetAllActiveRoutes()
			cache.GetProvider(providerID)
		}()
	}
	wg.Wait()
}
