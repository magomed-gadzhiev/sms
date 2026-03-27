package backpressure

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// Manager manages per-operator backpressure using token buckets.
type Manager struct {
	states map[uuid.UUID]*State
	mu     sync.RWMutex
	logger zerolog.Logger
}

// State holds backpressure state for a single operator.
type State struct {
	ProviderID      uuid.UUID
	TokensPerSecond int
	AvailableTokens int64 // atomic
	BurstSize       int   // TokensPerSecond * 2
	IsThrottled     int32 // atomic bool (0 or 1)
	LastRefill      int64 // atomic unix nano
	PendingCount    int64 // atomic — in-flight PDUs awaiting response
	MaxWindow       int   // max sliding window size (default 50)
}

func NewManager() *Manager {
	return &Manager{
		states: make(map[uuid.UUID]*State),
		logger: log.With().Str("component", "backpressure").Logger(),
	}
}

// Register adds or updates a provider's backpressure config.
func (m *Manager) Register(providerID uuid.UUID, tokensPerSecond int, maxWindow int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	burstSize := tokensPerSecond * 2
	m.states[providerID] = &State{
		ProviderID:      providerID,
		TokensPerSecond: tokensPerSecond,
		AvailableTokens: int64(burstSize),
		BurstSize:       burstSize,
		LastRefill:      time.Now().UnixNano(),
		MaxWindow:       maxWindow,
	}
}

// TryAcquire tries to acquire a token for the given provider.
// Returns true if allowed, false if backpressure should apply.
func (m *Manager) TryAcquire(providerID uuid.UUID) bool {
	m.mu.RLock()
	state, ok := m.states[providerID]
	m.mu.RUnlock()
	if !ok {
		return true // no config = no limit
	}

	// Refill tokens based on elapsed time
	now := time.Now().UnixNano()
	lastRefill := atomic.LoadInt64(&state.LastRefill)
	elapsed := now - lastRefill
	tokensToAdd := int64(state.TokensPerSecond) * elapsed / int64(time.Second)

	if tokensToAdd > 0 {
		if atomic.CompareAndSwapInt64(&state.LastRefill, lastRefill, now) {
			newTokens := atomic.AddInt64(&state.AvailableTokens, tokensToAdd)
			maxTokens := int64(state.BurstSize)
			if newTokens > maxTokens {
				atomic.StoreInt64(&state.AvailableTokens, maxTokens)
			}
		}
	}

	// Try to consume a token
	for {
		current := atomic.LoadInt64(&state.AvailableTokens)
		if current <= 0 {
			atomic.StoreInt32(&state.IsThrottled, 1)
			monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(1)
			return false
		}
		if atomic.CompareAndSwapInt64(&state.AvailableTokens, current, current-1) {
			if atomic.LoadInt32(&state.IsThrottled) == 1 {
				atomic.StoreInt32(&state.IsThrottled, 0)
				monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(0)
			}
			return true
		}
	}
}

// IncrementPending increments the in-flight counter.
func (m *Manager) IncrementPending(providerID uuid.UUID) {
	m.mu.RLock()
	state, ok := m.states[providerID]
	m.mu.RUnlock()
	if ok {
		atomic.AddInt64(&state.PendingCount, 1)
	}
}

// DecrementPending decrements the in-flight counter.
func (m *Manager) DecrementPending(providerID uuid.UUID) {
	m.mu.RLock()
	state, ok := m.states[providerID]
	m.mu.RUnlock()
	if ok {
		atomic.AddInt64(&state.PendingCount, -1)
	}
}

// IsThrottled checks if a provider is currently throttled.
func (m *Manager) IsThrottled(providerID uuid.UUID) bool {
	m.mu.RLock()
	state, ok := m.states[providerID]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return atomic.LoadInt32(&state.IsThrottled) == 1
}
