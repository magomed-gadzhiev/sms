package application

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
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
	mu sync.RWMutex
	metrics *RealtimeMetrics
	messageCounts map[string]int64 // ключ: provider_id, значение: количество сообщений
	lastUpdate    time.Time
}

// NewRealtimeService создает новый сервис real-time метрик
func NewRealtimeService() *RealtimeService {
	return &RealtimeService{
		metrics: &RealtimeMetrics{
			ProviderMetrics: make(map[string]int64),
		},
		messageCounts: make(map[string]int64),
		lastUpdate:    time.Now(),
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

// IncrementMessageCount инкрементирует счетчик сообщений для провайдера
func (s *RealtimeService) IncrementMessageCount(providerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messageCounts[providerID]++
	
	// Обновляем метрики каждую секунду
	now := time.Now()
	if now.Sub(s.lastUpdate) >= time.Second {
		s.updateMetrics()
		s.lastUpdate = now
	}
}

// updateMetrics обновляет вычисляемые метрики
func (s *RealtimeService) updateMetrics() {
	// Вычисляем messages per second
	var totalMessages int64
	for _, count := range s.messageCounts {
		totalMessages += count
	}

	s.metrics.MessagesPerSecond = totalMessages
	
	// Копируем provider metrics
	s.metrics.ProviderMetrics = make(map[string]int64)
	for k, v := range s.messageCounts {
		s.metrics.ProviderMetrics[k] = v
	}

	// Сбрасываем счетчики
	s.messageCounts = make(map[string]int64)
}

// UpdateMetrics обновляет метрики из внешних источников
func (s *RealtimeService) UpdateMetrics(queued int64, processing int64, activeProviders int64, activeConnections int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metrics.TotalMessagesQueued = queued
	s.metrics.TotalMessagesProcessing = processing
	s.metrics.ActiveProviders = activeProviders
	s.metrics.ActiveConnections = activeConnections
	s.metrics.Timestamp = time.Now()
}
