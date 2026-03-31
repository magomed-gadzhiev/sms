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

// ---------------------------------------------------------------------------
// matchesPattern — unit tests (Bug #1: numbers stored with '+' prefix)
// ---------------------------------------------------------------------------

func TestMatchesPattern_PrefixWithoutPlus(t *testing.T) {
	t.Parallel()
	route := &shared.Route{Pattern: "7910", PatternType: "prefix", Active: true}
	assert.True(t, matchesPattern(route, "79100300104"), "plain number should match prefix")
}

func TestMatchesPattern_PrefixWithPlusPrefix(t *testing.T) {
	// Regression: before the fix destination[:4] == "+791" != "7910" → route_id = NULL
	t.Parallel()
	route := &shared.Route{Pattern: "7910", PatternType: "prefix", Active: true}
	assert.True(t, matchesPattern(route, "+79100300104"), "number with '+' must match prefix after stripping")
}

func TestMatchesPattern_ExactWithPlusPrefix(t *testing.T) {
	t.Parallel()
	route := &shared.Route{Pattern: "79001234567", PatternType: "exact", Active: true}
	assert.True(t, matchesPattern(route, "+79001234567"), "exact match must work after stripping '+'")
}

func TestMatchesPattern_ExactWithoutPlus(t *testing.T) {
	t.Parallel()
	route := &shared.Route{Pattern: "79001234567", PatternType: "exact", Active: true}
	assert.True(t, matchesPattern(route, "79001234567"))
}

func TestMatchesPattern_PrefixNoMatch(t *testing.T) {
	t.Parallel()
	route := &shared.Route{Pattern: "7900", PatternType: "prefix", Active: true}
	assert.False(t, matchesPattern(route, "+79100300104"), "different prefix should not match")
}

func TestMatchesPattern_PrefixShorterThanPattern(t *testing.T) {
	t.Parallel()
	route := &shared.Route{Pattern: "79100", PatternType: "prefix", Active: true}
	assert.False(t, matchesPattern(route, "+791"), "destination shorter than pattern should not match")
}

func TestMatchesPattern_UnknownPatternType(t *testing.T) {
	t.Parallel()
	route := &shared.Route{Pattern: "7910", PatternType: "regex", Active: true}
	assert.False(t, matchesPattern(route, "+79100300104"))
}

func TestRouteCache_MatchRoutes_WithPlusPrefix(t *testing.T) {
	// Integration-level: verify MatchRoutes returns routes when destination has '+'.
	t.Parallel()

	routeID := uuid.New()
	providerID := uuid.New()
	routeRepo := &mockRouteRepo{
		routes: []*shared.Route{
			{ID: routeID, Name: "mts-ru", ProviderID: providerID, Active: true, Priority: 10, Pattern: "7910", PatternType: "prefix"},
		},
	}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{{ID: providerID, Name: "MTS-RU", Active: true}},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, cache.Start(ctx))

	matched := cache.MatchRoutes("+79100300104")
	require.Len(t, matched, 1, "route must be found for number with '+' prefix")
	assert.Equal(t, routeID, matched[0].ID)

	// Without '+' must also work.
	matched2 := cache.MatchRoutes("79100300104")
	require.Len(t, matched2, 1)
	assert.Equal(t, routeID, matched2[0].ID)
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
