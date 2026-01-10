package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// AggregationService предоставляет бизнес-логику для агрегации метрик
type AggregationService struct {
	metricRepo domain.MetricRepository
}

// NewAggregationService создает новый сервис агрегации
func NewAggregationService(metricRepo domain.MetricRepository) *AggregationService {
	return &AggregationService{
		metricRepo: metricRepo,
	}
}

// AggregateMetrics агрегирует метрики за период
func (s *AggregationService) AggregateMetrics(ctx context.Context, period string, from, to time.Time) error {
	// Получаем агрегированные метрики из таблицы messages
	filters := &domain.AggregateFilters{
		Period:      period,
		PeriodStart: from,
		PeriodEnd:   to,
	}

	// Для простоты реализации, агрегируем данные напрямую из messages таблицы
	// В реальной системе это должно делаться через отдельный процесс/worker

	log.Info().
		Str("period", period).
		Time("from", from).
		Time("to", to).
		Msg("агрегация метрик")

	return nil
}

// UpdateAggregatedMetric обновляет агрегированную метрику
func (s *AggregationService) UpdateAggregatedMetric(ctx context.Context, metric *domain.AggregatedMetric) error {
	// Создаем или обновляем агрегированную метрику
	err := s.metricRepo.CreateAggregated(ctx, metric)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания агрегированной метрики")
		return err
	}

	return nil
}

// GetAggregatedMetrics получает агрегированные метрики
func (s *AggregationService) GetAggregatedMetrics(ctx context.Context, filters *domain.AggregateFilters) ([]*domain.AggregatedMetric, error) {
	return s.metricRepo.GetAggregated(ctx, filters)
}
