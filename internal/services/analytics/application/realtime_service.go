package application

import (
	"context"
	"sync"
	"time"
)

// RealtimeMetrics представляет метрики в реальном времени
type RealtimeMetrics struct {
	MessagesPerSecond       int64
	TotalMessagesQueued     int64
	TotalMessagesProcessing int64
	ActiveProviders         int64
	ActiveConnections       int64
	ProviderMetrics         map[string]int64
	Timestamp               time.Time
}

// RealtimeService предоставляет бизнес-логику для real-time метрик
type RealtimeService struct {
	mu      sync.RWMutex
	metrics *RealtimeMetrics
}

// NewRealtimeService создает новый сервис real-time метрик
func NewRealtimeService() *RealtimeService {
	return &RealtimeService{
		metrics: &RealtimeMetrics{
			ProviderMetrics: make(map[string]int64),
		},
	}
}

// GetRealtimeMetrics получает метрики в реальном времени
func (s *RealtimeService) GetRealtimeMetrics(ctx context.Context) (*RealtimeMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Копируем текущие метрики
	metrics := &RealtimeMetrics{
		MessagesPerSecond:       s.metrics.MessagesPerSecond,
		TotalMessagesQueued:     s.metrics.TotalMessagesQueued,
		TotalMessagesProcessing: s.metrics.TotalMessagesProcessing,
		ActiveProviders:         s.metrics.ActiveProviders,
		ActiveConnections:       s.metrics.ActiveConnections,
		ProviderMetrics:         make(map[string]int64),
		Timestamp:               time.Now(),
	}

	// Копируем provider metrics
	for k, v := range s.metrics.ProviderMetrics {
		metrics.ProviderMetrics[k] = v
	}

	return metrics, nil
}

