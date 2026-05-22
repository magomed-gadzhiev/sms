package router_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/router"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// countingRouteRepo wraps mockRouteRepo and counts ListByClientAndOperator calls
// to verify whether the inner router was invoked.
type countingRouteRepo struct {
	inner    mockRouteRepo
	hitCount atomic.Int64
}

func (c *countingRouteRepo) ListByClientAndOperator(ctx context.Context, cid, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	c.hitCount.Add(1)
	return c.inner.ListByClientAndOperator(ctx, cid, oid)
}

func (c *countingRouteRepo) ListSharedByClientAndOperator(ctx context.Context, cid, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return c.inner.ListSharedByClientAndOperator(ctx, cid, oid)
}

func (c *countingRouteRepo) ListDefaultByOperator(ctx context.Context, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return c.inner.ListDefaultByOperator(ctx, oid)
}

func newCachedRouterWithCounting(routes map[uuid.UUID][]*shared.ClientRoute) (*router.CachedUnifiedRouter, *countingRouteRepo) {
	repo := &countingRouteRepo{
		inner: mockRouteRepo{byClient: routes},
	}
	inner := router.NewUnifiedRouter(repo, mockClientRepo{})
	cached := router.NewCachedUnifiedRouter(inner)
	return cached, repo
}

func TestCachedUnifiedRouter_CacheHit(t *testing.T) {
	rt := makeRoute(&testClientID, testProviderID, 100, false)
	cached, repo := newCachedRouterWithCounting(map[uuid.UUID][]*shared.ClientRoute{
		testClientID: {rt},
	})
	ctx := context.Background()

	// First call — cache miss, inner router called.
	dec1, err := cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, testProviderID, dec1.ProviderID)
	assert.Equal(t, int64(1), repo.hitCount.Load())

	// Second call — cache hit, inner router NOT called again.
	dec2, err := cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, testProviderID, dec2.ProviderID)
	assert.Equal(t, int64(1), repo.hitCount.Load(), "inner router should not be called on cache hit")

	// The returned decisions should be the same pointer (from cache).
	assert.Same(t, dec1, dec2)
}

func TestCachedUnifiedRouter_CacheMiss(t *testing.T) {
	rt := makeRoute(&testClientID, testProviderID, 100, false)
	cached, repo := newCachedRouterWithCounting(map[uuid.UUID][]*shared.ClientRoute{
		testClientID: {rt},
	})
	ctx := context.Background()

	dec, err := cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, testProviderID, dec.ProviderID)
	assert.Equal(t, int64(1), repo.hitCount.Load(), "inner router must be called on cache miss")
}

func TestCachedUnifiedRouter_Invalidate(t *testing.T) {
	rt := makeRoute(&testClientID, testProviderID, 100, false)
	cached, repo := newCachedRouterWithCounting(map[uuid.UUID][]*shared.ClientRoute{
		testClientID: {rt},
	})
	ctx := context.Background()

	// Populate cache.
	_, err := cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), repo.hitCount.Load())

	// Invalidate the entry.
	cached.Invalidate(testClientID, testOperatorID)

	// Next call should miss cache and call inner router again.
	_, err = cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), repo.hitCount.Load(), "inner router must be called after invalidation")
}

func TestCachedUnifiedRouter_InvalidateThenRoute_SimulatesExpiry(t *testing.T) {
	// We cannot wait 30 seconds for real TTL expiry, so Invalidate + Route
	// exercises the same code path (cache miss after removal).
	rt := makeRoute(&testClientID, testProviderID, 100, false)
	cached, repo := newCachedRouterWithCounting(map[uuid.UUID][]*shared.ClientRoute{
		testClientID: {rt},
	})
	ctx := context.Background()

	// Populate cache.
	dec1, err := cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	require.Equal(t, int64(1), repo.hitCount.Load())

	// Invalidate to simulate expiry.
	cached.Invalidate(testClientID, testOperatorID)

	// Re-fetch — should call inner router.
	dec2, err := cached.Route(ctx, testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), repo.hitCount.Load(), "inner router must be called after simulated expiry")

	// Both decisions should have the same provider (data unchanged).
	assert.Equal(t, dec1.ProviderID, dec2.ProviderID)
}

func TestCachedUnifiedRouter_DifferentKeysCachedIndependently(t *testing.T) {
	clientA := uuid.MustParse("aaaaaaaa-1111-1111-1111-111111111111")
	clientB := uuid.MustParse("bbbbbbbb-2222-2222-2222-222222222222")
	providerA := uuid.MustParse("dddddddd-1111-1111-1111-111111111111")
	providerB := uuid.MustParse("dddddddd-2222-2222-2222-222222222222")

	rtA := makeRoute(&clientA, providerA, 100, false)
	rtB := makeRoute(&clientB, providerB, 100, false)

	cached, repo := newCachedRouterWithCounting(map[uuid.UUID][]*shared.ClientRoute{
		clientA: {rtA},
		clientB: {rtB},
	})
	ctx := context.Background()

	// Cache both entries.
	decA, err := cached.Route(ctx, clientA, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, providerA, decA.ProviderID)

	decB, err := cached.Route(ctx, clientB, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, providerB, decB.ProviderID)

	assert.Equal(t, int64(2), repo.hitCount.Load(), "two different keys should each call inner router once")

	// Re-fetch both — should be cache hits.
	decA2, err := cached.Route(ctx, clientA, testOperatorID)
	require.NoError(t, err)
	decB2, err := cached.Route(ctx, clientB, testOperatorID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), repo.hitCount.Load(), "re-fetch should not call inner router")
	assert.Equal(t, providerA, decA2.ProviderID)
	assert.Equal(t, providerB, decB2.ProviderID)

	// Invalidate only clientA — clientB should remain cached.
	cached.Invalidate(clientA, testOperatorID)

	_, err = cached.Route(ctx, clientA, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), repo.hitCount.Load(), "invalidated key should trigger inner router call")

	_, err = cached.Route(ctx, clientB, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), repo.hitCount.Load(), "non-invalidated key should still be cached")
}

func TestCachedUnifiedRouter_InnerRouterError(t *testing.T) {
	// Empty repos = no routes found = ErrNoRouteFound from inner router.
	inner := router.NewUnifiedRouter(mockRouteRepo{}, mockClientRepo{})
	cached := router.NewCachedUnifiedRouter(inner)

	_, err := cached.Route(context.Background(), testClientID, testOperatorID)
	assert.ErrorIs(t, err, router.ErrNoRouteFound, "error from inner router must propagate")

	// Calling again should still return error (errors are not cached).
	_, err = cached.Route(context.Background(), testClientID, testOperatorID)
	assert.ErrorIs(t, err, router.ErrNoRouteFound)
}
