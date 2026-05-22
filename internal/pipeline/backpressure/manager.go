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

// Manager управляет двухуровневым backpressure:
//   - global:    per-provider token bucket (общий потолок)
//   - client:    per-(client,provider) token bucket (верхняя граница клиента)
type Manager struct {
	global map[uuid.UUID]*State
	client map[clientProviderKey]*State
	mu     sync.RWMutex
	logger zerolog.Logger
}

type clientProviderKey struct {
	clientID   uuid.UUID
	providerID uuid.UUID
}

// State — token bucket для одного ограничения.
type State struct {
	ProviderID      uuid.UUID
	TokensPerSecond int
	AvailableTokens int64 // atomic
	BurstSize       int
	IsThrottled     int32 // atomic bool
	LastRefill      int64 // atomic unix nano
	PendingCount    int64 // atomic
	MaxWindow       int
}

func NewManager() *Manager {
	return &Manager{
		global: make(map[uuid.UUID]*State),
		client: make(map[clientProviderKey]*State),
		logger: log.With().Str("component", "backpressure").Logger(),
	}
}

// Register регистрирует глобальный (per-provider) лимит.
func (m *Manager) Register(providerID uuid.UUID, tokensPerSecond int, maxWindow int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.global[providerID] = newState(providerID, tokensPerSecond, maxWindow)
}

// RegisterClient регистрирует per-client лимит для провайдера.
func (m *Manager) RegisterClient(clientID, providerID uuid.UUID, tokensPerSecond int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := clientProviderKey{clientID, providerID}
	m.client[key] = newState(providerID, tokensPerSecond, 50)
}

// UpdateClient обновляет per-client TPS (вызывается при инвалидации лимитов).
func (m *Manager) UpdateClient(clientID, providerID uuid.UUID, tokensPerSecond int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := clientProviderKey{clientID, providerID}
	m.client[key] = newState(providerID, tokensPerSecond, 50)
}

// TryAcquire пытается получить токен для пары (client, provider).
// Сначала проверяет per-client лимит, затем глобальный.
// Если клиент не зарегистрирован — проверяет только глобальный.
func (m *Manager) TryAcquire(clientID, providerID uuid.UUID) bool {
	m.mu.RLock()
	globalState := m.global[providerID]
	clientState := m.client[clientProviderKey{clientID, providerID}]
	m.mu.RUnlock()

	// Per-client check (если зарегистрирован)
	if clientState != nil {
		if !tryConsume(clientState) {
			monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(1)
			return false
		}
	}

	// Global check
	if globalState != nil {
		if !tryConsume(globalState) {
			// Вернуть токен клиента, если взяли
			if clientState != nil {
				atomic.AddInt64(&clientState.AvailableTokens, 1)
			}
			monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(1)
			return false
		}
		monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(0)
	}

	return true
}

// DrainClientTokens исчерпывает per-client токены (для тестов).
func (m *Manager) DrainClientTokens(clientID, providerID uuid.UUID) {
	m.mu.RLock()
	state := m.client[clientProviderKey{clientID, providerID}]
	m.mu.RUnlock()
	if state != nil {
		atomic.StoreInt64(&state.AvailableTokens, 0)
	}
}

// DrainGlobalTokens исчерпывает глобальные токены (для тестов).
func (m *Manager) DrainGlobalTokens(providerID uuid.UUID) {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state != nil {
		atomic.StoreInt64(&state.AvailableTokens, 0)
	}
}

// IncrementPending / DecrementPending / IsThrottled остаются на глобальном уровне.
func (m *Manager) IncrementPending(providerID uuid.UUID) {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state != nil {
		atomic.AddInt64(&state.PendingCount, 1)
	}
}

func (m *Manager) DecrementPending(providerID uuid.UUID) {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state != nil {
		atomic.AddInt64(&state.PendingCount, -1)
	}
}

func (m *Manager) IsThrottled(providerID uuid.UUID) bool {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state == nil {
		return false
	}
	return atomic.LoadInt32(&state.IsThrottled) == 1
}

// --- helpers ---

func newState(providerID uuid.UUID, tps int, maxWindow int) *State {
	if tps <= 0 {
		tps = 1
	}
	burst := tps * 2
	return &State{
		ProviderID:      providerID,
		TokensPerSecond: tps,
		AvailableTokens: int64(burst),
		BurstSize:       burst,
		LastRefill:      time.Now().UnixNano(),
		MaxWindow:       maxWindow,
	}
}

func tryConsume(state *State) bool {
	// Пополнение токенов
	now := time.Now().UnixNano()
	lastRefill := atomic.LoadInt64(&state.LastRefill)
	elapsed := now - lastRefill
	tokensToAdd := int64(state.TokensPerSecond) * elapsed / int64(time.Second)
	if tokensToAdd > 0 {
		if atomic.CompareAndSwapInt64(&state.LastRefill, lastRefill, now) {
			newTokens := atomic.AddInt64(&state.AvailableTokens, tokensToAdd)
			if newTokens > int64(state.BurstSize) {
				atomic.StoreInt64(&state.AvailableTokens, int64(state.BurstSize))
			}
		}
	}
	// Попытка взять токен
	for {
		current := atomic.LoadInt64(&state.AvailableTokens)
		if current <= 0 {
			atomic.StoreInt32(&state.IsThrottled, 1)
			return false
		}
		if atomic.CompareAndSwapInt64(&state.AvailableTokens, current, current-1) {
			atomic.StoreInt32(&state.IsThrottled, 0)
			return true
		}
	}
}
