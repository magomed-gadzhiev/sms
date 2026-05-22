package session

import (
	"sync"
	"time"
)

// RateLimiter реализует простой rate limiter на основе токенов
type RateLimiter struct {
	tokens     int
	maxTokens  int
	rate       time.Duration // интервал между токенами
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter создает новый rate limiter
func NewRateLimiter(perSecond int) *RateLimiter {
	if perSecond <= 0 {
		perSecond = 100 // По умолчанию
	}
	
	rate := time.Second / time.Duration(perSecond)
	if rate == 0 {
		rate = time.Millisecond
	}
	
	return &RateLimiter{
		tokens:     perSecond,
		maxTokens:  perSecond,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Allow проверяет, разрешена ли операция
func (rl *RateLimiter) Allow() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	
	// Пополняем токены
	tokensToAdd := int(elapsed / rl.rate)
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}
	
	// Проверяем наличие токенов
	if rl.tokens <= 0 {
		return ErrRateLimitExceeded
	}
	
	rl.tokens--
	return nil
}

// SetRate устанавливает новый rate limit
func (rl *RateLimiter) SetRate(perSecond int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if perSecond <= 0 {
		perSecond = 100
	}
	
	rate := time.Second / time.Duration(perSecond)
	if rate == 0 {
		rate = time.Millisecond
	}
	
	rl.maxTokens = perSecond
	rl.rate = rate
	rl.tokens = perSecond
	rl.lastRefill = time.Now()
}
