package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
	args := []interface{}{filters.From, filters.To}
	argIndex := 3

	// Общие условия WHERE
	whereExtra := ""
	if filters.ClientID != nil {
		whereExtra += fmt.Sprintf(" AND client_id = $%d", argIndex)
		args = append(args, *filters.ClientID)
		argIndex++
	}
	if len(filters.ProviderIDs) > 0 {
		whereExtra += fmt.Sprintf(" AND provider_id = ANY($%d)", argIndex)
		args = append(args, filters.ProviderIDs)
		argIndex++
	}

	// 1. Запрос totals по статусам (всегда нужен)
	totalsQuery := `
		SELECT
			status,
			COUNT(*) as count,
			AVG(EXTRACT(EPOCH FROM (COALESCE(delivered_at, updated_at) - created_at)) * 1000) as avg_delivery_time_ms
		FROM messages
		WHERE created_at >= $1 AND created_at <= $2` + whereExtra + ` GROUP BY status`

	type statRow struct {
		Status            string  `db:"status"`
		Count             int64   `db:"count"`
		AvgDeliveryTimeMs float64 `db:"avg_delivery_time_ms"`
	}

	var statusRows []statRow
	err := r.db.SelectContext(ctx, &statusRows, totalsQuery, args...)
	if err != nil {
		return nil, err
	}

	totals := &domain.TotalStats{}
	for _, row := range statusRows {
		switch row.Status {
		case "sent":
			totals.TotalSent += row.Count
		case "delivered":
			totals.TotalDelivered = row.Count
			totals.TotalSent += row.Count
		case "failed", "expired", "rejected":
			totals.TotalFailed += row.Count
			totals.TotalSent += row.Count
		case "pending":
			totals.TotalPending = row.Count
		case "queued":
			totals.TotalQueued = row.Count
		}
	}

	// Вычисляем success rate (доставлено / всего отправлено)
	if totals.TotalSent > 0 {
		totals.SuccessRate = int32((float64(totals.TotalDelivered) / float64(totals.TotalSent)) * 100)
	}

	// Вычисляем среднее время доставки
	var totalAvgDeliveryTime float64
	var countWithDeliveryTime int
	for _, row := range statusRows {
		if row.AvgDeliveryTimeMs > 0 {
			totalAvgDeliveryTime += row.AvgDeliveryTimeMs
			countWithDeliveryTime++
		}
	}
	if countWithDeliveryTime > 0 {
		totals.AvgDeliveryTimeMs = int64(totalAvgDeliveryTime / float64(countWithDeliveryTime))
	}

	// 2. Запрос groups с группировкой по нужному полю
	var groupByExpr, groupKeyExpr string
	switch filters.GroupBy {
	case "week":
		groupByExpr = "DATE_TRUNC('week', created_at)"
		groupKeyExpr = "TO_CHAR(DATE_TRUNC('week', created_at), 'YYYY-MM-DD')"
	case "country":
		groupByExpr = "COALESCE(country, 'unknown')"
		groupKeyExpr = "COALESCE(country, 'unknown')"
	default: // "day" or empty
		groupByExpr = "DATE_TRUNC('day', created_at)"
		groupKeyExpr = "TO_CHAR(DATE_TRUNC('day', created_at), 'YYYY-MM-DD')"
	}

	groupQuery := fmt.Sprintf(`
		SELECT
			%s as group_key,
			COUNT(*) FILTER (WHERE status IN ('sent', 'delivered', 'failed', 'expired', 'rejected')) as total_sent,
			COUNT(*) FILTER (WHERE status = 'delivered') as total_delivered,
			COUNT(*) FILTER (WHERE status IN ('failed', 'expired', 'rejected')) as total_failed,
			AVG(EXTRACT(EPOCH FROM (COALESCE(delivered_at, updated_at) - created_at)) * 1000) as avg_delivery_time_ms
		FROM messages
		WHERE created_at >= $1 AND created_at <= $2%s
		GROUP BY %s
		ORDER BY %s`, groupKeyExpr, whereExtra, groupByExpr, groupByExpr)

	type groupRow struct {
		GroupKey           string  `db:"group_key"`
		TotalSent          int64   `db:"total_sent"`
		TotalDelivered     int64   `db:"total_delivered"`
		TotalFailed        int64   `db:"total_failed"`
		AvgDeliveryTimeMs  float64 `db:"avg_delivery_time_ms"`
	}

	var groupRows []groupRow
	err = r.db.SelectContext(ctx, &groupRows, groupQuery, args...)
	if err != nil {
		return nil, err
	}

	groups := make([]*domain.StatisticGroup, 0, len(groupRows))
	for _, row := range groupRows {
		successRate := int32(0)
		if row.TotalSent > 0 {
			successRate = int32((float64(row.TotalDelivered) / float64(row.TotalSent)) * 100)
		}
		groups = append(groups, &domain.StatisticGroup{
			Key: row.GroupKey,
			Stats: &domain.TotalStats{
				TotalSent:         row.TotalSent,
				TotalDelivered:    row.TotalDelivered,
				TotalFailed:       row.TotalFailed,
				SuccessRate:       successRate,
				AvgDeliveryTimeMs: int64(row.AvgDeliveryTimeMs),
			},
		})
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
			totalSent += row.Count
		case "delivered":
			totalDelivered += row.Count
			totalSent += row.Count
		case "failed", "expired", "rejected":
			totalFailed += row.Count
			totalSent += row.Count
		}

		perf.AvgDeliveryTimeMs = int64(row.AvgDeliveryTimeMs)
	}

	perf.TotalSent = totalSent
	perf.TotalDelivered = totalDelivered
	perf.TotalFailed = totalFailed

	// Вычисляем success rate (доставлено / всего отправлено)
	if totalSent > 0 {
		perf.SuccessRate = int32((float64(totalDelivered) / float64(totalSent)) * 100)
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

