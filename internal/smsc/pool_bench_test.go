//go:build !integration

package smsc

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func BenchmarkThrottler_Allow(b *testing.B) {
	throttler := NewThrottler(1000) // Высокий лимит для бенчмарка

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = throttler.Allow()
	}
}

func BenchmarkConnection_UpdateLastUsed(b *testing.B) {
	conn := &Connection{
		ID:        "test-conn",
		ProviderID: uuid.New(),
		LastUsed:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.mu.Lock()
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
	}
}

func BenchmarkConnection_CheckExpired(b *testing.B) {
	conn := &Connection{
		ID:       "test-conn",
		LastUsed: time.Now(),
	}
	timeout := time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		elapsed := time.Since(conn.LastUsed)
		_ = elapsed > timeout
	}
}
