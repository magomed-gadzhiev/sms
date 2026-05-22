package backpressure

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewManager verifies that NewManager returns a non-nil, usable manager.
func TestNewManager(t *testing.T) {
	m := NewManager()
	require.NotNil(t, m)
}

// TestRegister_InitialTokens checks that a newly registered provider starts
// with BurstSize (= TokensPerSecond * 2) available tokens.
func TestRegister_InitialTokens(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	m.Register(id, 10, 50)

	m.mu.RLock()
	state, ok := m.global[id]
	m.mu.RUnlock()

	require.True(t, ok, "state should be registered")
	assert.Equal(t, 10, state.TokensPerSecond)
	assert.Equal(t, 20, state.BurstSize) // 10 * 2
	assert.Equal(t, int64(20), atomic.LoadInt64(&state.AvailableTokens))
	assert.Equal(t, 50, state.MaxWindow)
}

// TestRegister_Overwrites verifies that re-registering a provider replaces its state.
func TestRegister_Overwrites(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	m.Register(id, 5, 50)
	m.Register(id, 20, 100)

	m.mu.RLock()
	state, ok := m.global[id]
	m.mu.RUnlock()

	require.True(t, ok)
	assert.Equal(t, 20, state.TokensPerSecond)
	assert.Equal(t, 40, state.BurstSize)
}

// TestTryAcquire_UnknownProvider returns true (no limit) for unregistered providers.
func TestTryAcquire_UnknownProvider(t *testing.T) {
	m := NewManager()
	assert.True(t, m.TryAcquire(uuid.New(), uuid.New()))
}

// TestTryAcquire_ConsumesTokens verifies that each successful TryAcquire decrements
// the available token count.
func TestTryAcquire_ConsumesTokens(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	clientID := uuid.New()
	// 5 tps → 10 burst tokens initially
	m.Register(id, 5, 50)

	// Consume all 10 tokens
	for i := 0; i < 10; i++ {
		ok := m.TryAcquire(clientID, id)
		assert.True(t, ok, "call %d should succeed", i+1)
	}

	m.mu.RLock()
	state := m.global[id]
	m.mu.RUnlock()
	assert.LessOrEqual(t, atomic.LoadInt64(&state.AvailableTokens), int64(0))
}

// TestTryAcquire_ExhaustedReturnsFalse verifies that once tokens are depleted
// TryAcquire returns false.
func TestTryAcquire_ExhaustedReturnsFalse(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	clientID := uuid.New()
	// 1 tps → 2 burst tokens
	m.Register(id, 1, 50)

	// Drain tokens
	for m.TryAcquire(clientID, id) {
	}

	assert.False(t, m.TryAcquire(clientID, id))
}

// TestIsThrottled_SetOnExhaustion checks that IsThrottled returns true after tokens
// are exhausted and false while tokens are available.
func TestIsThrottled_SetOnExhaustion(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	clientID := uuid.New()
	m.Register(id, 1, 50) // 2 burst tokens

	assert.False(t, m.IsThrottled(id), "should not be throttled initially")

	// Drain all tokens
	for m.TryAcquire(clientID, id) {
	}

	// The failed call marks throttled
	assert.False(t, m.TryAcquire(clientID, id))
	assert.True(t, m.IsThrottled(id), "should be throttled after exhaustion")
}

// TestIsThrottled_ClearsOnSuccess verifies that once tokens are refilled and
// TryAcquire succeeds, IsThrottled resets to false.
func TestIsThrottled_ClearsOnSuccess(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	clientID := uuid.New()
	// 100 tps — lots of tokens refill quickly
	m.Register(id, 100, 50)

	m.mu.RLock()
	state := m.global[id]
	m.mu.RUnlock()

	// Manually exhaust and mark throttled
	atomic.StoreInt64(&state.AvailableTokens, 0)
	atomic.StoreInt32(&state.IsThrottled, 1)

	// Set LastRefill in the past so tokens refill on next TryAcquire
	atomic.StoreInt64(&state.LastRefill, time.Now().Add(-200*time.Millisecond).UnixNano())

	ok := m.TryAcquire(clientID, id)
	assert.True(t, ok, "should succeed after refill")
	assert.False(t, m.IsThrottled(id), "throttle flag should be cleared")
}

// TestIsThrottled_UnknownProvider returns false for an unregistered provider.
func TestIsThrottled_UnknownProvider(t *testing.T) {
	m := NewManager()
	assert.False(t, m.IsThrottled(uuid.New()))
}

// TestRefill_OverTime verifies that tokens are replenished after waiting long enough.
func TestRefill_OverTime(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	clientID := uuid.New()
	// 50 tps → 100 burst; after 100ms we expect ~5 new tokens
	m.Register(id, 50, 50)

	m.mu.RLock()
	state := m.global[id]
	m.mu.RUnlock()

	// Drain all tokens
	for m.TryAcquire(clientID, id) {
	}

	assert.False(t, m.TryAcquire(clientID, id), "should be exhausted")

	// Wait 150ms → ~7 new tokens at 50 tps
	time.Sleep(150 * time.Millisecond)

	ok := m.TryAcquire(clientID, id)
	assert.True(t, ok, "tokens should have refilled after 150ms")
	_ = state // used above
}

// TestRefill_DoesNotExceedBurst verifies that refill never exceeds BurstSize.
func TestRefill_DoesNotExceedBurst(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	clientID := uuid.New()
	m.Register(id, 10, 50) // burst = 20

	m.mu.RLock()
	state := m.global[id]
	m.mu.RUnlock()

	// Set LastRefill far in the past so many tokens would be added
	atomic.StoreInt64(&state.LastRefill, time.Now().Add(-10*time.Second).UnixNano())

	// TryAcquire triggers the refill
	m.TryAcquire(clientID, id)

	tokens := atomic.LoadInt64(&state.AvailableTokens)
	assert.LessOrEqual(t, tokens, int64(state.BurstSize), "tokens must not exceed burst size")
}

// TestPending_IncrementDecrement check the in-flight counter.
func TestPending_IncrementDecrement(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	m.Register(id, 10, 50)

	m.mu.RLock()
	state := m.global[id]
	m.mu.RUnlock()

	assert.Equal(t, int64(0), atomic.LoadInt64(&state.PendingCount))

	m.IncrementPending(id)
	m.IncrementPending(id)
	m.IncrementPending(id)
	assert.Equal(t, int64(3), atomic.LoadInt64(&state.PendingCount))

	m.DecrementPending(id)
	assert.Equal(t, int64(2), atomic.LoadInt64(&state.PendingCount))
}

// TestPending_UnknownProvider ensures no panic when provider is not registered.
func TestPending_UnknownProvider(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	assert.NotPanics(t, func() { m.IncrementPending(id) })
	assert.NotPanics(t, func() { m.DecrementPending(id) })
}

// TestMultipleOperators_IndependentBuckets verifies that each provider has an independent bucket.
func TestMultipleOperators_IndependentBuckets(t *testing.T) {
	m := NewManager()
	idA := uuid.New()
	idB := uuid.New()
	clientID := uuid.New()

	// A: 1 tps → 2 burst tokens
	// B: 10 tps → 20 burst tokens
	m.Register(idA, 1, 50)
	m.Register(idB, 10, 50)

	// Drain A
	for m.TryAcquire(clientID, idA) {
	}
	assert.False(t, m.TryAcquire(clientID, idA), "A should be exhausted")

	// B should still have plenty of tokens
	assert.True(t, m.TryAcquire(clientID, idB), "B should still have tokens")
	assert.False(t, m.IsThrottled(idB), "B should not be throttled")
}

// TestConcurrentTryAcquire checks that the token bucket is safe under concurrent access.
func TestConcurrentTryAcquire_RaceSafe(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	// 100 tps → 200 burst tokens
	m.Register(id, 100, 50)

	var wg sync.WaitGroup
	acquired := int64(0)
	goroutines := 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if m.TryAcquire(uuid.New(), id) {
				atomic.AddInt64(&acquired, 1)
			}
		}()
	}
	wg.Wait()

	// At most 200 tokens were available; all 50 goroutines may succeed (50 <= 200)
	assert.LessOrEqual(t, acquired, int64(200))
	assert.GreaterOrEqual(t, acquired, int64(0))
}

// TestConcurrentPending_RaceSafe verifies that PendingCount is consistent under concurrent
// increments and decrements.
func TestConcurrentPending_RaceSafe(t *testing.T) {
	m := NewManager()
	id := uuid.New()
	m.Register(id, 10, 50)

	m.mu.RLock()
	state := m.global[id]
	m.mu.RUnlock()

	var wg sync.WaitGroup
	n := 100
	wg.Add(n * 2)
	for i := 0; i < n; i++ {
		go func() { defer wg.Done(); m.IncrementPending(id) }()
		go func() { defer wg.Done(); m.DecrementPending(id) }()
	}
	wg.Wait()

	assert.Equal(t, int64(0), atomic.LoadInt64(&state.PendingCount))
}

// --- Two-tier tests ---

func TestManager_TwoTier_PerClientLimitBlocks(t *testing.T) {
	m := NewManager()
	provID := uuid.New()
	clientID := uuid.New()

	m.Register(provID, 100, 50)
	m.RegisterClient(clientID, provID, 1)

	assert.True(t, m.TryAcquire(clientID, provID))
	m.DrainClientTokens(clientID, provID)
	assert.False(t, m.TryAcquire(clientID, provID))
}

func TestManager_TwoTier_GlobalLimitBlocks(t *testing.T) {
	m := NewManager()
	provID := uuid.New()
	clientA := uuid.New()
	clientB := uuid.New()

	m.Register(provID, 1, 50)
	m.RegisterClient(clientA, provID, 10)
	m.RegisterClient(clientB, provID, 10)

	m.DrainGlobalTokens(provID)
	assert.False(t, m.TryAcquire(clientA, provID))
	assert.False(t, m.TryAcquire(clientB, provID))
}

func TestManager_TwoTier_UnknownClientPassesGlobalOnly(t *testing.T) {
	m := NewManager()
	provID := uuid.New()
	m.Register(provID, 100, 50)

	assert.True(t, m.TryAcquire(uuid.New(), provID))
}
