package limits_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/pipeline/limits"
)

// callCountingQuerier wraps mockLimitQuerier and counts calls to GetClientProviderTPS.
type callCountingQuerier struct {
	mockLimitQuerier
	clientProviderTPSCalls int
}

func (c *callCountingQuerier) GetClientProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (*int, error) {
	c.clientProviderTPSCalls++
	return c.mockLimitQuerier.GetClientProviderTPS(ctx, clientID, providerID)
}

// errorQuerier returns errors from various methods to test fallback behaviour.
type errorQuerier struct {
	clientProviderTPSErr    error
	tariffPlanDefaultTPSErr error
	systemDefaultTPSErr     error
	systemDefaultTPS        int
	parentBudgetErr         error
	allocatedTPSErr         error

	clientProviderTPS    *int
	tariffPlanDefaultTPS *int
	parentClientID       *uuid.UUID
	allocatedTPSBudget   *int
	allocatedTPSUsed     int
}

func (e errorQuerier) GetClientProviderTPS(_ context.Context, _, _ uuid.UUID) (*int, error) {
	return e.clientProviderTPS, e.clientProviderTPSErr
}
func (e errorQuerier) GetTariffPlanDefaultTPS(_ context.Context, _ uuid.UUID) (*int, error) {
	return e.tariffPlanDefaultTPS, e.tariffPlanDefaultTPSErr
}
func (e errorQuerier) GetSystemDefaultTPS(_ context.Context) (int, error) {
	return e.systemDefaultTPS, e.systemDefaultTPSErr
}
func (e errorQuerier) GetClientParentAndBudget(_ context.Context, _ uuid.UUID) (*uuid.UUID, *int, error) {
	return e.parentClientID, e.allocatedTPSBudget, e.parentBudgetErr
}
func (e errorQuerier) GetSubAccountAllocatedTPS(_ context.Context, _, _ uuid.UUID) (int, error) {
	return e.allocatedTPSUsed, e.allocatedTPSErr
}

func newMiniredisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return rdb, mr
}

// ---------- CachedLimitResolver tests ----------

func TestCachedLimitResolver_CachesResult(t *testing.T) {
	rdb, _ := newMiniredisClient(t)
	querier := &callCountingQuerier{
		mockLimitQuerier: mockLimitQuerier{clientProviderTPS: intPtr(42)},
	}
	inner := limits.NewDBLimitResolver(querier)
	cached := limits.NewCachedLimitResolver(inner, rdb)

	clientID := uuid.New()
	providerID := uuid.New()
	ctx := context.Background()

	tps1, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 42, tps1)
	assert.Equal(t, 1, querier.clientProviderTPSCalls)

	// Second call should come from cache — no extra DB hit.
	tps2, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 42, tps2)
	assert.Equal(t, 1, querier.clientProviderTPSCalls, "expected cache hit, DB querier should not be called again")
}

func TestCachedLimitResolver_DifferentKeysMissCache(t *testing.T) {
	rdb, _ := newMiniredisClient(t)
	querier := &callCountingQuerier{
		mockLimitQuerier: mockLimitQuerier{clientProviderTPS: intPtr(10)},
	}
	inner := limits.NewDBLimitResolver(querier)
	cached := limits.NewCachedLimitResolver(inner, rdb)
	ctx := context.Background()

	_, err := cached.ResolveProviderTPS(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	_, err = cached.ResolveProviderTPS(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	assert.Equal(t, 2, querier.clientProviderTPSCalls, "different keys should both miss cache")
}

func TestCachedLimitResolver_InvalidateProviderTPS(t *testing.T) {
	rdb, _ := newMiniredisClient(t)
	querier := &callCountingQuerier{
		mockLimitQuerier: mockLimitQuerier{clientProviderTPS: intPtr(50)},
	}
	inner := limits.NewDBLimitResolver(querier)
	cached := limits.NewCachedLimitResolver(inner, rdb)

	clientID := uuid.New()
	providerID := uuid.New()
	ctx := context.Background()

	// Warm cache.
	_, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 1, querier.clientProviderTPSCalls)

	// Invalidate.
	cached.InvalidateProviderTPS(ctx, clientID, providerID)

	// Next call must go to DB again.
	_, err = cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 2, querier.clientProviderTPSCalls, "after invalidation DB should be queried again")
}

func TestCachedLimitResolver_TTLExpiry(t *testing.T) {
	rdb, mr := newMiniredisClient(t)
	querier := &callCountingQuerier{
		mockLimitQuerier: mockLimitQuerier{clientProviderTPS: intPtr(15)},
	}
	inner := limits.NewDBLimitResolver(querier)
	cached := limits.NewCachedLimitResolver(inner, rdb)

	clientID := uuid.New()
	providerID := uuid.New()
	ctx := context.Background()

	_, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 1, querier.clientProviderTPSCalls)

	// Fast-forward past the 30-second TTL.
	mr.FastForward(31 * 1e9) // 31 seconds in nanoseconds

	_, err = cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 2, querier.clientProviderTPSCalls, "after TTL expiry, DB should be queried again")
}

func TestCachedLimitResolver_CacheKeyFormat(t *testing.T) {
	rdb, mr := newMiniredisClient(t)
	querier := &callCountingQuerier{
		mockLimitQuerier: mockLimitQuerier{clientProviderTPS: intPtr(99)},
	}
	inner := limits.NewDBLimitResolver(querier)
	cached := limits.NewCachedLimitResolver(inner, rdb)

	clientID := uuid.New()
	providerID := uuid.New()
	ctx := context.Background()

	_, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)

	expectedKey := fmt.Sprintf("limits:%s:%s:tps", clientID, providerID)
	assert.True(t, mr.Exists(expectedKey), "expected Redis key %q to exist", expectedKey)
}

func TestCachedLimitResolver_InnerErrorPassedThrough(t *testing.T) {
	rdb, _ := newMiniredisClient(t)
	querier := errorQuerier{
		clientProviderTPSErr:    errors.New("db down"),
		tariffPlanDefaultTPSErr: errors.New("db down"),
		systemDefaultTPSErr:     errors.New("system fail"),
	}
	inner := limits.NewDBLimitResolver(querier)
	cached := limits.NewCachedLimitResolver(inner, rdb)

	_, err := cached.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetSystemDefaultTPS")
}

func TestCachedLimitResolver_InnerErrorNotCached(t *testing.T) {
	rdb, _ := newMiniredisClient(t)
	callCount := 0
	// Use a querier that fails once then succeeds.
	failOnce := &toggleQuerier{
		callCount: &callCount,
		failUntil: 1,
		successTPS: 25,
	}
	inner := limits.NewDBLimitResolver(failOnce)
	cached := limits.NewCachedLimitResolver(inner, rdb)
	ctx := context.Background()
	clientID := uuid.New()
	providerID := uuid.New()

	// First call: error from inner, should NOT be cached.
	_, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.Error(t, err)

	// Second call: inner succeeds now.
	tps, err := cached.ResolveProviderTPS(ctx, clientID, providerID)
	require.NoError(t, err)
	assert.Equal(t, 25, tps)
}

// ---------- DBLimitResolver fallback chain edge cases ----------

func TestDBLimitResolver_FallbackChain_Level1Error_FallsToLevel2(t *testing.T) {
	resolver := limits.NewDBLimitResolver(errorQuerier{
		clientProviderTPSErr: errors.New("db error"),
		tariffPlanDefaultTPS: intPtr(18),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 18, tps)
}

func TestDBLimitResolver_FallbackChain_Level1And2Error_FallsToLevel3(t *testing.T) {
	resolver := limits.NewDBLimitResolver(errorQuerier{
		clientProviderTPSErr:    errors.New("db error"),
		tariffPlanDefaultTPSErr: errors.New("db error"),
		systemDefaultTPS:        7,
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 7, tps)
}

func TestDBLimitResolver_FallbackChain_AllLevelsError(t *testing.T) {
	resolver := limits.NewDBLimitResolver(errorQuerier{
		clientProviderTPSErr:    errors.New("db error"),
		tariffPlanDefaultTPSErr: errors.New("db error"),
		systemDefaultTPSErr:     errors.New("fatal"),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	// Falls back to hardcoded 5 on system default error.
	assert.Equal(t, 5, tps)
}

func TestDBLimitResolver_SubAccountBudgetZero(t *testing.T) {
	parentID := uuid.New()
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:  intPtr(100),
		parentClientID:     &parentID,
		allocatedTPSBudget: intPtr(0),
		allocatedTPSUsed:   0,
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, tps, "zero budget should cap TPS to 0")
}

func TestDBLimitResolver_SubAccountNegativeRemaining(t *testing.T) {
	parentID := uuid.New()
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:  intPtr(50),
		parentClientID:     &parentID,
		allocatedTPSBudget: intPtr(30),
		allocatedTPSUsed:   40, // Used more than budget.
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, tps, "negative remaining budget should clamp to 0")
}

func TestDBLimitResolver_SubAccountBudgetErrorIgnored(t *testing.T) {
	parentID := uuid.New()
	resolver := limits.NewDBLimitResolver(errorQuerier{
		clientProviderTPS:  intPtr(25),
		parentClientID:     &parentID,
		allocatedTPSBudget: intPtr(100),
		allocatedTPSErr:    errors.New("redis down"),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 25, tps, "error in GetSubAccountAllocatedTPS should be ignored, resolved TPS returned as-is")
}

func TestDBLimitResolver_SubAccountParentBudgetError(t *testing.T) {
	resolver := limits.NewDBLimitResolver(errorQuerier{
		clientProviderTPS: intPtr(25),
		parentBudgetErr:   errors.New("db error"),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 25, tps, "error in GetClientParentAndBudget should be ignored")
}

// --- helpers ---

// toggleQuerier fails for the first N calls to GetSystemDefaultTPS, then succeeds.
type toggleQuerier struct {
	callCount  *int
	failUntil  int
	successTPS int
}

func (t *toggleQuerier) GetClientProviderTPS(_ context.Context, _, _ uuid.UUID) (*int, error) {
	return nil, errors.New("skip")
}
func (t *toggleQuerier) GetTariffPlanDefaultTPS(_ context.Context, _ uuid.UUID) (*int, error) {
	return nil, errors.New("skip")
}
func (t *toggleQuerier) GetSystemDefaultTPS(_ context.Context) (int, error) {
	*t.callCount++
	if *t.callCount <= t.failUntil {
		return 0, errors.New("temporary failure")
	}
	return t.successTPS, nil
}
func (t *toggleQuerier) GetClientParentAndBudget(_ context.Context, _ uuid.UUID) (*uuid.UUID, *int, error) {
	return nil, nil, nil
}
func (t *toggleQuerier) GetSubAccountAllocatedTPS(_ context.Context, _, _ uuid.UUID) (int, error) {
	return 0, nil
}
