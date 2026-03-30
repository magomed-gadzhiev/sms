package smsc

import (
	"sync"
	"time"
)

// Throttler управляет ограничением скорости отправки
type Throttler struct {
	tokensPerSecond int
	tokens          int64
	lastUpdate      int64
	mu              sync.Mutex
}

// NewThrottler создает новый throttler
func NewThrottler(tokensPerSecond int) *Throttler {
	return &Throttler{
		tokensPerSecond: tokensPerSecond,
		tokens:          int64(tokensPerSecond),
		lastUpdate:      time.Now().UnixNano(),
	}
}

// Allow проверяет, можно ли отправить сообщение
func (t *Throttler) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UnixNano()
	elapsed := now - t.lastUpdate
	tokensToAdd := int64(t.tokensPerSecond) * elapsed / int64(time.Second)

	if tokensToAdd > 0 {
		t.tokens = min(int64(t.tokensPerSecond), t.tokens+tokensToAdd)
		t.lastUpdate = now
	}

	if t.tokens > 0 {
		t.tokens--
		return true
	}
	return false
}

// min возвращает минимальное значение
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
