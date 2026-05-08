package application

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// AggregationWorker runs background jobs that materialise raw message events
// into the pre-aggregated tables consumed by the analytics service.
type AggregationWorker struct {
	db             *pgxpool.Pool
	statsRepo      domain.StatsRepository
	monitoringRepo domain.MonitoringRepository
	logger         zerolog.Logger
}

// NewAggregationWorker creates a new AggregationWorker.
func NewAggregationWorker(
	db *pgxpool.Pool,
	statsRepo domain.StatsRepository,
	monitoringRepo domain.MonitoringRepository,
	logger zerolog.Logger,
) *AggregationWorker {
	return &AggregationWorker{
		db:             db,
		statsRepo:      statsRepo,
		monitoringRepo: monitoringRepo,
		logger:         logger,
	}
}

// rawAggQuery selects one hour's worth of message data grouped by all analytics
// dimensions. Column names mirror the messages table schema; COALESCE guards
// against NULL values in optional columns.
// rawAggQuery resolves an "effective" partner_id: for sub-accounts
// (clients.parent_client_id IS NOT NULL) we collapse to the parent's
// partner_id so reseller analytics aggregate the entire account tree under
// one bucket. Direct clients use their own partner_id. See B.1 fix.
const rawAggQuery = `
SELECT
    COALESCE(p.partner_id, c.partner_id, 0)                    AS partner_id,
    date_trunc('hour', m.created_at)                           AS hour,
    0::bigint                                                  AS provider_id,
    COALESCE(op.name, '')                                      AS operator,
    COALESCE(co.iso_code, '')                                  AS country,
    COALESCE(NULLIF(m.channel, ''), 'sms')                     AS channel,
    COALESCE(c.name, c.email, '')                              AS login,
    COALESCE(m.source, '')                                     AS sender_name,
    COALESCE(m.service_type, '')                               AS traffic_type,
    COALESCE(m.send_method, '')                                AS method,
    COUNT(*)                                                   AS total,
    COUNT(*) FILTER (WHERE m.status = 'sent')                  AS sent,
    COUNT(*) FILTER (WHERE m.status = 'delivered')             AS delivered,
    COUNT(*) FILTER (WHERE m.status = 'failed')                AS failed,
    COUNT(*) FILTER (WHERE m.status = 'pending')               AS pending,
    COUNT(*) FILTER (WHERE m.status = 'expired')               AS timeout,
    COUNT(*) FILTER (WHERE m.status IN ('failed','rejected'))  AS error,
    COALESCE(SUM(t.total_amount), 0)                           AS revenue,
    0                                                          AS cost
FROM messages m
LEFT JOIN clients          c  ON c.id  = m.client_id
LEFT JOIN clients          p  ON p.id  = c.parent_client_id
LEFT JOIN operators        op ON op.id = m.operator_id
LEFT JOIN countries        co ON co.id = m.country_id
LEFT JOIN tarification_log t  ON t.message_id = m.id
WHERE m.created_at >= $1 AND m.created_at < $2
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
`

// RunHourlyAggregation ticks every hour, queries the raw messages table for the
// previous hour window, and upserts the result into network_stats_hourly via
// statsRepo. Errors are logged but never crash the loop. The goroutine exits
// when ctx is cancelled.
func (w *AggregationWorker) RunHourlyAggregation(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	// Run once immediately on startup so a fresh deployment does not wait an
	// entire hour for the first data point.
	w.runHourlyAggregationOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("hourly aggregation worker: shutting down")
			return
		case <-ticker.C:
			w.runHourlyAggregationOnce(ctx)
		}
	}
}

// BackfillWindow re-aggregates messages from `from` (inclusive) to `to`
// (exclusive) hour by hour. Callers are expected to TRUNCATE
// network_stats_hourly first if they want a clean recompute, because
// UpsertHourlyStats adds to existing counters. Safe to call multiple
// times only if the table is empty for the window.
func (w *AggregationWorker) BackfillWindow(ctx context.Context, from, to time.Time) error {
	from = from.UTC().Truncate(time.Hour)
	to = to.UTC().Truncate(time.Hour)
	w.logger.Info().Time("from", from).Time("to", to).Msg("hourly aggregation: backfill start")
	for cursor := from; cursor.Before(to); cursor = cursor.Add(time.Hour) {
		if err := w.aggregateHour(ctx, cursor, cursor.Add(time.Hour)); err != nil {
			return err
		}
	}
	w.logger.Info().Time("from", from).Time("to", to).Msg("hourly aggregation: backfill done")
	return nil
}

// aggregateHour runs the raw aggregation query for [start, end) and upserts.
func (w *AggregationWorker) aggregateHour(ctx context.Context, start, end time.Time) error {
	rows, err := w.db.Query(ctx, rawAggQuery, start, end)
	if err != nil {
		return err
	}
	defer rows.Close()

	var batch []domain.HourlyStatsRow
	for rows.Next() {
		var r domain.HourlyStatsRow
		if err := rows.Scan(
			&r.PartnerID, &r.Hour, &r.ProviderID, &r.Operator, &r.Country,
			&r.Channel, &r.Login, &r.SenderName, &r.TrafficType, &r.Method,
			&r.Total, &r.Sent, &r.Delivered, &r.Failed, &r.Pending,
			&r.Timeout, &r.Error, &r.Revenue, &r.Cost,
		); err != nil {
			return err
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(batch) == 0 {
		return nil
	}
	return w.statsRepo.UpsertHourlyStats(ctx, batch)
}

func (w *AggregationWorker) runHourlyAggregationOnce(ctx context.Context) {
	now := time.Now().UTC().Truncate(time.Hour)
	windowEnd := now
	windowStart := now.Add(-time.Hour)

	w.logger.Debug().
		Time("window_start", windowStart).
		Time("window_end", windowEnd).
		Msg("hourly aggregation: querying messages")

	rows, err := w.db.Query(ctx, rawAggQuery, windowStart, windowEnd)
	if err != nil {
		w.logger.Error().Err(err).Msg("hourly aggregation: query failed")
		return
	}
	defer rows.Close()

	var batch []domain.HourlyStatsRow
	for rows.Next() {
		var r domain.HourlyStatsRow
		if err := rows.Scan(
			&r.PartnerID,
			&r.Hour,
			&r.ProviderID,
			&r.Operator,
			&r.Country,
			&r.Channel,
			&r.Login,
			&r.SenderName,
			&r.TrafficType,
			&r.Method,
			&r.Total,
			&r.Sent,
			&r.Delivered,
			&r.Failed,
			&r.Pending,
			&r.Timeout,
			&r.Error,
			&r.Revenue,
			&r.Cost,
		); err != nil {
			w.logger.Error().Err(err).Msg("hourly aggregation: scan failed")
			return
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		w.logger.Error().Err(err).Msg("hourly aggregation: rows iteration error")
		return
	}

	if len(batch) == 0 {
		w.logger.Debug().
			Time("window_start", windowStart).
			Time("window_end", windowEnd).
			Msg("hourly aggregation: no rows to upsert")
		return
	}

	if err := w.statsRepo.UpsertHourlyStats(ctx, batch); err != nil {
		w.logger.Error().Err(err).Int("rows", len(batch)).Msg("hourly aggregation: upsert failed")
		return
	}

	w.logger.Info().
		Time("window_start", windowStart).
		Time("window_end", windowEnd).
		Int("rows", len(batch)).
		Msg("hourly aggregation: upsert complete")
}

// RunSnapshotCollection ticks every 5 minutes. Each tick it reads live provider
// metrics from Redis via monitoringRepo.GetCurrentMetrics (partner 0 = all
// partners in a single call), persists them as MonitoringSnapshots, and
// purges snapshots older than 30 days. Errors are logged but do not stop the
// loop.
func (w *AggregationWorker) RunSnapshotCollection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Collect once immediately on startup.
	w.runSnapshotOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("snapshot collection worker: shutting down")
			return
		case <-ticker.C:
			w.runSnapshotOnce(ctx)
		}
	}
}

func (w *AggregationWorker) runSnapshotOnce(ctx context.Context) {
	now := time.Now().UTC()

	// partnerID = 0 is the convention used by GetCurrentMetrics to return
	// metrics for all partners/providers.
	liveMetrics, err := w.monitoringRepo.GetCurrentMetrics(ctx, 0)
	if err != nil {
		w.logger.Error().Err(err).Msg("snapshot collection: GetCurrentMetrics failed")
		return
	}

	if len(liveMetrics) == 0 {
		w.logger.Debug().Msg("snapshot collection: no live metrics returned")
		return
	}

	snapshots := make([]domain.MonitoringSnapshot, 0, len(liveMetrics))
	for _, m := range liveMetrics {
		snapshots = append(snapshots, domain.MonitoringSnapshot{
			PartnerID:     0, // global / cross-partner snapshot
			ProviderID:    m.ProviderID,
			Timestamp:     now,
			Throughput:    m.Throughput,
			QueueDepth:    m.QueueDepth,
			ActiveConns:   m.ActiveConns,
			ErrorCount:    m.ErrorCount,
			TimeoutCount:  m.TimeoutCount,
			PendingCount:  m.Pending,
			DLRLatencyP50: m.LatencyP50,
			DLRLatencyP95: m.LatencyP95,
			HealthStatus:  m.HealthStatus,
		})
	}

	if err := w.monitoringRepo.SaveSnapshot(ctx, snapshots); err != nil {
		w.logger.Error().Err(err).Int("snapshots", len(snapshots)).Msg("snapshot collection: SaveSnapshot failed")
		return
	}

	w.logger.Debug().Int("snapshots", len(snapshots)).Msg("snapshot collection: saved")

	// Purge snapshots older than 30 days.
	cutoff := now.AddDate(0, 0, -30)
	if err := w.purgeOldSnapshots(ctx, cutoff); err != nil {
		w.logger.Warn().Err(err).Time("cutoff", cutoff).Msg("snapshot collection: purge failed")
	}
}

// purgeOldSnapshots deletes monitoring_snapshots rows older than cutoff.
func (w *AggregationWorker) purgeOldSnapshots(ctx context.Context, cutoff time.Time) error {
	const deleteSQL = `DELETE FROM monitoring_snapshots WHERE timestamp < $1`
	tag, err := w.db.Exec(ctx, deleteSQL, cutoff)
	if err != nil {
		return err
	}
	deleted := tag.RowsAffected()
	if deleted > 0 {
		w.logger.Info().
			Int64("deleted", deleted).
			Time("cutoff", cutoff).
			Msg("snapshot collection: purged old snapshots")
	}
	return nil
}
