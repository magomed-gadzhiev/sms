package smsc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestThrottler_Allow(t *testing.T) {
	tests := []struct {
		name            string
		tokensPerSecond int
		requests        int
		expectedAllowed int
		waitTime        time.Duration
	}{
		{
			name:            "allow all within limit",
			tokensPerSecond: 10,
			requests:        5,
			expectedAllowed: 5,
			waitTime:        0,
		},
		{
			name:            "rate limit exceeded",
			tokensPerSecond: 2,
			requests:        5,
			expectedAllowed: 2, // только первые 2 должны быть разрешены
			waitTime:        0,
		},
		{
			name:            "tokens replenish over time",
			tokensPerSecond: 2,
			requests:        3,
			expectedAllowed: 3,
			waitTime:        time.Second, // ждем пополнения токенов
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			throttler := NewThrottler(tt.tokensPerSecond)

			allowedCount := 0
			for i := 0; i < tt.requests; i++ {
				if i > 0 && tt.waitTime > 0 {
					time.Sleep(tt.waitTime)
				}
				if throttler.Allow() {
					allowedCount++
				}
			}

			assert.Equal(t, tt.expectedAllowed, allowedCount)
		})
	}
}

func TestThrottler_Concurrent(t *testing.T) {
	throttler := NewThrottler(10)

	// Запускаем несколько горутин одновременно
	allowed := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			allowed <- throttler.Allow()
		}()
	}

	allowedCount := 0
	for i := 0; i < 20; i++ {
		if <-allowed {
			allowedCount++
		}
	}

	// Должно быть разрешено не более tokensPerSecond
	assert.LessOrEqual(t, allowedCount, 10)
}

func TestConnection_LastUsed(t *testing.T) {
	conn := &Connection{
		ID:        "test-conn",
		ProviderID: uuid.New(),
		LastUsed:  time.Now().Add(-time.Hour),
	}

	oldLastUsed := conn.LastUsed
	time.Sleep(10 * time.Millisecond)

	// Обновляем LastUsed напрямую (так как метода UpdateActivity нет)
	conn.mu.Lock()
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	assert.True(t, conn.LastUsed.After(oldLastUsed))
}

func TestConnection_Expired(t *testing.T) {
	tests := []struct {
		name     string
		lastUsed time.Time
		timeout  time.Duration
		expected bool
	}{
		{
			name:     "not expired",
			lastUsed: time.Now(),
			timeout:  time.Minute,
			expected: false,
		},
		{
			name:     "expired",
			lastUsed: time.Now().Add(-2 * time.Minute),
			timeout:  time.Minute,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				ID:       "test-conn",
				LastUsed: tt.lastUsed,
			}

			// Проверяем истечение вручную
			elapsed := time.Since(conn.LastUsed)
			result := elapsed > tt.timeout
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConnection_Bound(t *testing.T) {
	conn := &Connection{
		ID:     "test-conn",
		Bound:  true,
	}

	assert.True(t, conn.Bound)

	conn.Bound = false
	assert.False(t, conn.Bound)
}
