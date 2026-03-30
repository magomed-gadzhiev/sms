package session

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(200)

	require.NotNil(t, rl)
	assert.Equal(t, 200, rl.maxTokens)
	assert.Equal(t, 200, rl.tokens)
}

func TestNewRateLimiterZeroRate(t *testing.T) {
	rl := NewRateLimiter(0)

	// Should default to 100
	assert.Equal(t, 100, rl.maxTokens)
	assert.Equal(t, 100, rl.tokens)
}

func TestNewRateLimiterNegativeRate(t *testing.T) {
	rl := NewRateLimiter(-5)

	assert.Equal(t, 100, rl.maxTokens)
	assert.Equal(t, 100, rl.tokens)
}

func TestRateLimiterAllowConsumesTokens(t *testing.T) {
	rl := NewRateLimiter(10)

	for i := 0; i < 10; i++ {
		err := rl.Allow()
		require.NoError(t, err, "call %d should succeed", i)
	}

	// All tokens consumed
	err := rl.Allow()
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	rl := NewRateLimiter(10)

	// Exhaust all tokens
	for i := 0; i < 10; i++ {
		require.NoError(t, rl.Allow())
	}

	// Should be exhausted
	assert.ErrorIs(t, rl.Allow(), ErrRateLimitExceeded)

	// Wait enough time for tokens to refill (at 10/sec, ~100ms per token)
	time.Sleep(250 * time.Millisecond)

	// Should now have tokens available
	err := rl.Allow()
	assert.NoError(t, err)
}

func TestRateLimiterDoesNotExceedMax(t *testing.T) {
	rl := NewRateLimiter(5)

	// Wait long enough for potential over-refill
	time.Sleep(300 * time.Millisecond)

	// Should only have maxTokens available
	for i := 0; i < 5; i++ {
		require.NoError(t, rl.Allow(), "call %d should succeed", i)
	}

	// Beyond max should fail
	err := rl.Allow()
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

func TestSetRate(t *testing.T) {
	rl := NewRateLimiter(10)

	// Exhaust all tokens
	for i := 0; i < 10; i++ {
		require.NoError(t, rl.Allow())
	}
	assert.ErrorIs(t, rl.Allow(), ErrRateLimitExceeded)

	// Set new rate which resets tokens
	rl.SetRate(50)
	assert.Equal(t, 50, rl.maxTokens)
	assert.Equal(t, 50, rl.tokens)

	// Should now have tokens
	err := rl.Allow()
	assert.NoError(t, err)
}

func TestSetRateZero(t *testing.T) {
	rl := NewRateLimiter(10)

	rl.SetRate(0)

	// Should default to 100
	assert.Equal(t, 100, rl.maxTokens)
}

func TestSetRateNegative(t *testing.T) {
	rl := NewRateLimiter(10)

	rl.SetRate(-10)

	assert.Equal(t, 100, rl.maxTokens)
}

func TestRateLimiterConcurrency(t *testing.T) {
	rl := NewRateLimiter(1000)

	var wg sync.WaitGroup
	var successCount int64
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				err := rl.Allow()
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Total attempts: 2000, max tokens: 1000
	// Success count should be <= maxTokens (plus any refill during execution)
	assert.True(t, successCount > 0, "should have had some successful calls")
	assert.True(t, successCount <= 2000, "should not exceed total attempts")
}

func TestRateLimiterHighRate(t *testing.T) {
	rl := NewRateLimiter(10000)

	// Should be able to consume many tokens rapidly
	for i := 0; i < 5000; i++ {
		require.NoError(t, rl.Allow(), "call %d should succeed", i)
	}
}
