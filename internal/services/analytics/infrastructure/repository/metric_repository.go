package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// MetricRepository реализует domain.MetricRepository
type MetricRepository struct {
	db *sqlx.DB
}

// NewMetricRepository создает новый репозиторий метрик
func NewMetricRepository(db *sqlx.DB) *MetricRepository {
	return &MetricRepository{
		db: db,
	}
}

// Create сохраняет метрику
func (r *MetricRepository) Create(ctx context.Context, metric *domain.Metric) error {
	query := `
		INSERT INTO message_stats (
			id, metric_type, client_id, provider_id, message_id, 
			status, value, timestamp, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	var metadataJSON []byte
	if metric.Metadata != nil && len(metric.Metadata) > 0 {
		var err error
		metadataJSON, err = json.Marshal(metric.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		metric.ID,
		string(metric.Type),
		metric.ClientID,
		metric.ProviderID,
		metric.MessageID,
		metric.Status,
		metric.Value,
		metric.Timestamp,
		metadataJSON,
		metric.CreatedAt,
	)

	return err
}

// CreateAggregated сохраняет агрегированную метрику
func (r *MetricRepository) CreateAggregated(ctx context.Context, metric *domain.AggregatedMetric) error {
	query := `
		INSERT INTO aggregated_metrics (
			id, period, period_start, period_end, client_id, 
			provider_id, status, count, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		ON CONFLICT (period, period_start, client_id, provider_id, status) 
		DO UPDATE SET
			count = aggregated_metrics.count + $8,
			updated_at = $10
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		metric.ID,
		metric.Period,
		metric.PeriodStart,
		metric.PeriodEnd,
		metric.ClientID,
		metric.ProviderID,
		metric.Status,
		metric.Count,
		metric.CreatedAt,
		metric.UpdatedAt,
	)

	return err
}

// UpdateAggregated обновляет агрегированную метрику
func (r *MetricRepository) UpdateAggregated(ctx context.Context, metric *domain.AggregatedMetric) error {
	query := `
		UPDATE aggregated_metrics
		SET count = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		metric.Count,
		metric.UpdatedAt,
		metric.ID,
	)

	return err
}

// GetAggregated получает агрегированные метрики
func (r *MetricRepository) GetAggregated(ctx context.Context, filters *domain.AggregateFilters) ([]*domain.AggregatedMetric, error) {
	query := `
		SELECT id, period, period_start, period_end, client_id, 
		       provider_id, status, count, created_at, updated_at
		FROM aggregated_metrics
		WHERE 1=1
	`

	args := []interface{}{}
	argIndex := 1

	if filters.Period != "" {
		query += fmt.Sprintf(" AND period = $%d", argIndex)
		args = append(args, filters.Period)
		argIndex++
	}

	if !filters.PeriodStart.IsZero() {
		query += fmt.Sprintf(" AND period_start >= $%d", argIndex)
		args = append(args, filters.PeriodStart)
		argIndex++
	}

	if !filters.PeriodEnd.IsZero() {
		query += fmt.Sprintf(" AND period_end <= $%d", argIndex)
		args = append(args, filters.PeriodEnd)
		argIndex++
	}

	if filters.ClientID != nil {
		query += fmt.Sprintf(" AND client_id = $%d", argIndex)
		args = append(args, *filters.ClientID)
		argIndex++
	}

	if filters.ProviderID != nil {
		query += fmt.Sprintf(" AND provider_id = $%d", argIndex)
		args = append(args, *filters.ProviderID)
		argIndex++
	}

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *filters.Status)
		argIndex++
	}

	query += " ORDER BY period_start DESC"

	var metrics []*domain.AggregatedMetric
	err := r.db.SelectContext(ctx, &metrics, query, args...)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetStatistics получает статистику
func (r *MetricRepository) GetStatistics(ctx context.Context, filters *domain.StatisticsFilters) (*domain.Statistics, error) {
	// Основной запрос для получения статистики из таблицы messages
	query := `
		SELECT 
			status,
			COUNT(*) as count,
			AVG(EXTRACT(EPOCH FROM (COALESCE(delivered_at, updated_at) - created_at)) * 1000) as avg_delivery_time_ms
		FROM messages
		WHERE created_at >= $1 AND created_at <= $2
	`

	args := []interface{}{filters.From, filters.To}
	argIndex := 3

	if filters.ClientID != nil {
		query += fmt.Sprintf(" AND client_id = $%d", argIndex)
		args = append(args, *filters.ClientID)
		argIndex++
	}

	if len(filters.ProviderIDs) > 0 {
		query += fmt.Sprintf(" AND provider_id = ANY($%d)", argIndex)
		args = append(args, filters.ProviderIDs)
		argIndex++
	}

	query += " GROUP BY status"

	type statRow struct {
		Status            string  `db:"status"`
		Count             int64   `db:"count"`
		AvgDeliveryTimeMs float64 `db:"avg_delivery_time_ms"`
	}

	var rows []statRow
	err := r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, err
	}

	// Собираем общую статистику
	totals := &domain.TotalStats{}
	groups := make([]*domain.StatisticGroup, 0)

	for _, row := range rows {
		switch row.Status {
		case "sent":
			totals.TotalSent = row.Count
		case "delivered":
			totals.TotalDelivered = row.Count
		case "failed":
			totals.TotalFailed = row.Count
		case "pending":
			totals.TotalPending = row.Count
		case "queued":
			totals.TotalQueued = row.Count
		}

		groups = append(groups, &domain.StatisticGroup{
			Key: row.Status,
			Stats: &domain.TotalStats{
				TotalSent:         row.Count,
				AvgDeliveryTimeMs: int64(row.AvgDeliveryTimeMs),
			},
		})
	}

	// Вычисляем success rate
	totalProcessed := totals.TotalDelivered + totals.TotalFailed
	if totalProcessed > 0 {
		totals.SuccessRate = int32((float64(totals.TotalDelivered) / float64(totalProcessed)) * 100)
	}

	// Вычисляем среднее время доставки из всех строк
	var totalAvgDeliveryTime float64
	var countWithDeliveryTime int
	for _, row := range rows {
		if row.AvgDeliveryTimeMs > 0 {
			totalAvgDeliveryTime += row.AvgDeliveryTimeMs
			countWithDeliveryTime++
		}
	}
	if countWithDeliveryTime > 0 {
		totals.AvgDeliveryTimeMs = int64(totalAvgDeliveryTime / float64(countWithDeliveryTime))
	}

	return &domain.Statistics{
		Groups: groups,
		Totals: totals,
	}, nil
}

// GetProviderPerformance получает производительность провайдера
func (r *MetricRepository) GetProviderPerformance(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*domain.ProviderPerformance, error) {
	query := `
		SELECT 
			status,
			COUNT(*) as count,
			AVG(EXTRACT(EPOCH FROM (COALESCE(delivered_at, updated_at) - created_at)) * 1000) as avg_delivery_time_ms
		FROM messages
		WHERE provider_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY status
	`

	type perfRow struct {
		Status            string  `db:"status"`
		Count             int64   `db:"count"`
		AvgDeliveryTimeMs float64 `db:"avg_delivery_time_ms"`
	}

	var rows []perfRow
	err := r.db.SelectContext(ctx, &rows, query, providerID, from, to)
	if err != nil {
		return nil, err
	}

	perf := &domain.ProviderPerformance{
		ProviderID:      providerID,
		StatusBreakdown: make(map[string]int64),
		Points:          make([]*domain.PerformancePoint, 0),
	}

	var totalSent, totalDelivered, totalFailed int64
	for _, row := range rows {
		perf.StatusBreakdown[row.Status] = row.Count

		switch row.Status {
		case "sent":
			totalSent = row.Count
		case "delivered":
			totalDelivered = row.Count
		case "failed":
			totalFailed = row.Count
		}

		perf.AvgDeliveryTimeMs = int64(row.AvgDeliveryTimeMs)
	}

	perf.TotalSent = totalSent
	perf.TotalDelivered = totalDelivered
	perf.TotalFailed = totalFailed

	// Вычисляем success rate
	totalProcessed := totalDelivered + totalFailed
	if totalProcessed > 0 {
		perf.SuccessRate = int32((float64(totalDelivered) / float64(totalProcessed)) * 100)
	}

	return perf, nil
}

// BatchCreate сохраняет несколько метрик одним запросом
func (r *MetricRepository) BatchCreate(ctx context.Context, metrics []*domain.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	query := `INSERT INTO message_stats (
		id, metric_type, client_id, provider_id, message_id,
		status, value, timestamp, metadata, created_at
	) VALUES `

	args := make([]interface{}, 0, len(metrics)*10)
	for i, m := range metrics {
		if i > 0 {
			query += ","
		}
		base := i * 10
		query += fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5,
			base+6, base+7, base+8, base+9, base+10,
		)

		var metadataJSON []byte
		if m.Metadata != nil && len(m.Metadata) > 0 {
			metadataJSON, _ = json.Marshal(m.Metadata)
		}

		args = append(args,
			m.ID, string(m.Type), m.ClientID, m.ProviderID, m.MessageID,
			m.Status, m.Value, m.Timestamp, metadataJSON, m.CreatedAt,
		)
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// BufferedMetricWriter буферизует метрики в памяти и сбрасывает их пакетами
type BufferedMetricWriter struct {
	repo          *MetricRepository
	mu            sync.Mutex
	buffer        []*domain.Metric
	flushSize     int
	flushInterval time.Duration
	logger        zerolog.Logger
}

// NewBufferedMetricWriter создаёт новый буферизованный писатель метрик
func NewBufferedMetricWriter(repo *MetricRepository, flushSize int, flushInterval time.Duration) *BufferedMetricWriter {
	return &BufferedMetricWriter{
		repo:          repo,
		buffer:        make([]*domain.Metric, 0, flushSize),
		flushSize:     flushSize,
		flushInterval: flushInterval,
		logger:        log.With().Str("component", "metric-buffer").Logger(),
	}
}

// Add добавляет метрику в буфер и сбрасывает его при достижении flushSize
func (w *BufferedMetricWriter) Add(metric *domain.Metric) {
	w.mu.Lock()
	w.buffer = append(w.buffer, metric)
	shouldFlush := len(w.buffer) >= w.flushSize
	w.mu.Unlock()

	if shouldFlush {
		w.Flush(context.Background())
	}
}

// Flush сбрасывает текущий буфер в базу данных одним батч-запросом
func (w *BufferedMetricWriter) Flush(ctx context.Context) {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.buffer
	w.buffer = make([]*domain.Metric, 0, w.flushSize)
	w.mu.Unlock()

	if err := w.repo.BatchCreate(ctx, batch); err != nil {
		w.logger.Error().Err(err).Int("count", len(batch)).Msg("batch metric flush failed")
		// Возвращаем метрики в буфер для повторной попытки
		w.mu.Lock()
		w.buffer = append(batch, w.buffer...)
		w.mu.Unlock()
	}
}

// Start запускает фоновый тикер, периодически сбрасывающий буфер
func (w *BufferedMetricWriter) Start(ctx context.Context) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.Flush(context.Background())
			return
		case <-ticker.C:
			w.Flush(ctx)
		}
	}
}
