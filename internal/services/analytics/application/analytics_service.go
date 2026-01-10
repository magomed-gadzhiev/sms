package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// AnalyticsService предоставляет бизнес-логику для аналитики
type AnalyticsService struct {
	metricRepo domain.MetricRepository
}

// NewAnalyticsService создает новый сервис аналитики
func NewAnalyticsService(metricRepo domain.MetricRepository) *AnalyticsService {
	return &AnalyticsService{
		metricRepo: metricRepo,
	}
}

// RecordMessageCreated записывает метрику создания сообщения
func (s *AnalyticsService) RecordMessageCreated(ctx context.Context, messageID string, clientID *string, providerID *string, timestamp int64) error {
	metric := s.createMetric(domain.MetricTypeMessageCreated, "created", clientID, providerID, &messageID, timestamp)
	return s.metricRepo.Create(ctx, metric)
}

// RecordMessageSent записывает метрику отправки сообщения
func (s *AnalyticsService) RecordMessageSent(ctx context.Context, messageID string, clientID *string, providerID *string, timestamp int64) error {
	metric := s.createMetric(domain.MetricTypeMessageSent, "sent", clientID, providerID, &messageID, timestamp)
	return s.metricRepo.Create(ctx, metric)
}

// RecordMessageDelivered записывает метрику доставки сообщения
func (s *AnalyticsService) RecordMessageDelivered(ctx context.Context, messageID string, clientID *string, providerID *string, timestamp int64) error {
	metric := s.createMetric(domain.MetricTypeMessageDelivered, "delivered", clientID, providerID, &messageID, timestamp)
	return s.metricRepo.Create(ctx, metric)
}

// RecordMessageFailed записывает метрику неудачной отправки сообщения
func (s *AnalyticsService) RecordMessageFailed(ctx context.Context, messageID string, clientID *string, providerID *string, timestamp int64, reason string) error {
	metric := s.createMetric(domain.MetricTypeMessageFailed, "failed", clientID, providerID, &messageID, timestamp)
	if reason != "" {
		metric.Metadata = map[string]interface{}{
			"reason": reason,
		}
	}
	return s.metricRepo.Create(ctx, metric)
}

// GetStatistics получает статистику по фильтрам
func (s *AnalyticsService) GetStatistics(ctx context.Context, filters *domain.StatisticsFilters) (*domain.Statistics, error) {
	return s.metricRepo.GetStatistics(ctx, filters)
}

// GetProviderPerformance получает производительность провайдера
func (s *AnalyticsService) GetProviderPerformance(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*domain.ProviderPerformance, error) {
	return s.metricRepo.GetProviderPerformance(ctx, providerID, from, to)
}

// createMetric создает метрику из параметров
func (s *AnalyticsService) createMetric(metricType domain.MetricType, status string, clientID *string, providerID *string, messageID *string, timestamp int64) *domain.Metric {
	var clientUUID *uuid.UUID
	if clientID != nil {
		if id, err := uuid.Parse(*clientID); err == nil {
			clientUUID = &id
		}
	}

	var providerUUID *uuid.UUID
	if providerID != nil {
		if id, err := uuid.Parse(*providerID); err == nil {
			providerUUID = &id
		}
	}

	var messageUUID *uuid.UUID
	if messageID != nil {
		if id, err := uuid.Parse(*messageID); err == nil {
			messageUUID = &id
		}
	}

	metricTime := time.Unix(timestamp, 0)
	return domain.NewMetric(metricType, status, 1, clientUUID, providerUUID, messageUUID)
}
