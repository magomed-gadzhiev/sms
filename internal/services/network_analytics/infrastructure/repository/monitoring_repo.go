package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// MonitoringRepo implements domain.MonitoringRepository using PostgreSQL and Redis.
type MonitoringRepo struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// NewMonitoringRepo creates a new MonitoringRepo.
func NewMonitoringRepo(db *pgxpool.Pool, rdb *redis.Client) *MonitoringRepo {
	return &MonitoringRepo{db: db, redis: rdb}
}

// GetCurrentMetrics reads live provider metrics from Redis.
// Keys follow the pattern network:{partnerID}:provider:{providerID}
func (r *MonitoringRepo) GetCurrentMetrics(ctx context.Context, partnerID int64) ([]domain.ProviderLiveMetrics, error) {
	pattern := fmt.Sprintf("network:%d:provider:*", partnerID)

	var cursor uint64
	var keys []string
	for {
		var batch []string
		var err error
		batch, cursor, err = r.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return nil, nil
	}

	var metrics []domain.ProviderLiveMetrics
	for _, key := range keys {
		// Extract provider ID from key: network:{partnerID}:provider:{providerID}
		parts := strings.Split(key, ":")
		if len(parts) < 4 {
			continue
		}
		providerID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			continue
		}

		fields, err := r.redis.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		if len(fields) == 0 {
			continue
		}

		m := domain.ProviderLiveMetrics{
			ProviderID: providerID,
		}
		m.ProviderName = fields["provider_name"]
		m.HealthStatus = fields["health_status"]
		m.Throughput = parseFloat(fields["throughput"])
		m.QueueDepth = parseInt(fields["queue_depth"])
		m.Pending = parseInt(fields["pending"])
		m.ErrorCount = parseInt(fields["error_count"])
		m.TimeoutCount = parseInt(fields["timeout_count"])
		m.LatencyP50 = parseInt(fields["latency_p50"])
		m.LatencyP95 = parseInt(fields["latency_p95"])
		m.ActiveConns = parseInt(fields["active_conns"])

		if m.HealthStatus == "" {
			m.HealthStatus = domain.HealthOK
		}

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// GetMonitoringMetrics returns current monitoring table rows, optionally hiding healthy ones.
func (r *MonitoringRepo) GetMonitoringMetrics(ctx context.Context, filter *domain.SharedFilter, hideHealthy bool) ([]domain.MonitorRow, error) {
	filter.Normalize()

	// Get live data from Redis
	liveMetrics, err := r.GetCurrentMetrics(ctx, filter.PartnerID)
	if err != nil {
		return nil, err
	}

	// Get top error from DB for each provider
	topErrors, err := r.fetchTopErrors(ctx, filter)
	if err != nil {
		// Non-fatal
		topErrors = map[int64]string{}
	}

	var result []domain.MonitorRow
	for _, m := range liveMetrics {
		if hideHealthy && m.HealthStatus == domain.HealthOK {
			continue
		}

		dlrRate := 0.0
		// We don't have sent/delivered breakdown in live metrics; health is already computed
		row := domain.MonitorRow{
			Slice:         m.ProviderName,
			Throughput:    m.Throughput,
			Pending:       int64(m.Pending),
			Timeout:       int64(m.TimeoutCount),
			Error:         int64(m.ErrorCount),
			DLRLatencyP50: float64(m.LatencyP50),
			DLRLatencyP95: float64(m.LatencyP95),
			DLRRate:       dlrRate,
			TopError:      topErrors[m.ProviderID],
			Health:        m.HealthStatus,
		}
		result = append(result, row)
	}

	return result, nil
}

// fetchTopErrors gets the most frequent error per provider in the filter period.
func (r *MonitoringRepo) fetchTopErrors(ctx context.Context, filter *domain.SharedFilter) (map[int64]string, error) {
	query := `
		SELECT DISTINCT ON (provider_id)
			provider_id,
			top_error
		FROM network_monitoring_snapshot
		WHERE partner_id = $1
			AND ts >= $2 AND ts <= $3
			AND top_error IS NOT NULL AND top_error <> ''
		ORDER BY provider_id, ts DESC`

	from := filter.DateFrom
	to := filter.DateTo
	if from.IsZero() {
		from = to.Add(-6 * 3600 * 1e9) // 6 hours default
	}

	rows, err := r.db.Query(ctx, query, filter.PartnerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch top errors: %w", err)
	}
	defer rows.Close()

	result := map[int64]string{}
	for rows.Next() {
		var providerID int64
		var topError string
		if err := rows.Scan(&providerID, &topError); err != nil {
			continue
		}
		result[providerID] = topError
	}
	return result, rows.Err()
}

// GetMonitoringChart returns time-series data for monitoring charts from network_monitoring_snapshot.
func (r *MonitoringRepo) GetMonitoringChart(ctx context.Context, filter *domain.SharedFilter, metric string) ([]domain.MetricPoint, error) {
	filter.Normalize()

	// Map metric name to DB column
	metricCol := "throughput"
	switch metric {
	case "throughput":
		metricCol = "AVG(throughput)"
	case "queue_depth":
		metricCol = "AVG(queue_depth)"
	case "error_count":
		metricCol = "SUM(error_count)"
	case "timeout_count":
		metricCol = "SUM(timeout_count)"
	case "pending":
		metricCol = "AVG(pending_count)"
	case "latency_p95":
		metricCol = "AVG(dlr_latency_p95)"
	case "latency_p50":
		metricCol = "AVG(dlr_latency_p50)"
	default:
		metricCol = "AVG(throughput)"
	}

	query := fmt.Sprintf(`
		SELECT
			date_trunc('minute', ts) AS bucket,
			%s AS value
		FROM network_monitoring_snapshot
		WHERE partner_id = $1 AND ts >= $2 AND ts <= $3
		GROUP BY bucket
		ORDER BY bucket ASC`, metricCol)

	from := filter.DateFrom
	to := filter.DateTo
	if from.IsZero() {
		from = to.Add(-6 * 3600 * 1e9)
	}

	rows, err := r.db.Query(ctx, query, filter.PartnerID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query monitoring chart: %w", err)
	}
	defer rows.Close()

	var points []domain.MetricPoint
	for rows.Next() {
		var ts interface{}
		var value float64
		if err := rows.Scan(&ts, &value); err != nil {
			return nil, fmt.Errorf("scan chart point: %w", err)
		}
		// ts is a time.Time from pgx
		var ms int64
		switch t := ts.(type) {
		case interface{ UnixMilli() int64 }:
			ms = t.UnixMilli()
		}
		points = append(points, domain.MetricPoint{Timestamp: ms, Value: value})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return points, nil
}

// SaveSnapshot persists Redis snapshots to PostgreSQL.
func (r *MonitoringRepo) SaveSnapshot(ctx context.Context, snapshots []domain.MonitoringSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	query := `
		INSERT INTO network_monitoring_snapshot (
			partner_id, provider_id, ts,
			throughput, queue_depth, active_conns,
			error_count, timeout_count, pending_count,
			dlr_latency_p50, dlr_latency_p95, health_status
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			$10, $11, $12
		)`

	for _, s := range snapshots {
		_, err := r.db.Exec(ctx, query,
			s.PartnerID, s.ProviderID, s.Timestamp,
			s.Throughput, s.QueueDepth, s.ActiveConns,
			s.ErrorCount, s.TimeoutCount, s.PendingCount,
			s.DLRLatencyP50, s.DLRLatencyP95, s.HealthStatus,
		)
		if err != nil {
			return fmt.Errorf("save snapshot (partner=%d, provider=%d): %w",
				s.PartnerID, s.ProviderID, err)
		}
	}
	return nil
}

// parseFloat safely parses a string to float64, returning 0 on error.
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseInt safely parses a string to int, returning 0 on error.
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}
