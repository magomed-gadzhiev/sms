package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

// StatsRepository handles stats snapshots and analytics queries.
type StatsRepository struct {
	db *sqlx.DB
}

// NewStatsRepository creates a new StatsRepository.
func NewStatsRepository(db *sqlx.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// InsertSnapshot inserts a stats snapshot.
func (r *StatsRepository) InsertSnapshot(ctx context.Context, snap *domain.StatsSnapshot) error {
	query := `INSERT INTO campaign_stats_snapshots
		(campaign_id, variant_id, snapshot_at, sent, delivered, failed, pending, avg_delivery_time_ms, cost)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		snap.CampaignID, snap.VariantID, snap.SnapshotAt,
		snap.Sent, snap.Delivered, snap.Failed, snap.Pending,
		snap.AvgDeliveryTimeMs, snap.Cost,
	)
	if err != nil {
		return fmt.Errorf("failed to insert stats snapshot: %w", err)
	}
	return nil
}

// GetLatestSnapshot retrieves the latest stats snapshot for a campaign (campaign-level).
func (r *StatsRepository) GetLatestSnapshot(ctx context.Context, campaignID uuid.UUID) (*domain.StatsSnapshot, error) {
	// Campaign-level variant_id is the nil UUID
	nilUUID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	query := `SELECT campaign_id, variant_id, snapshot_at, sent, delivered, failed, pending, avg_delivery_time_ms, cost
		FROM campaign_stats_snapshots
		WHERE campaign_id = $1 AND variant_id = $2
		ORDER BY snapshot_at DESC LIMIT 1`

	var snap domain.StatsSnapshot
	err := r.db.QueryRowxContext(ctx, query, campaignID, nilUUID).Scan(
		&snap.CampaignID, &snap.VariantID, &snap.SnapshotAt,
		&snap.Sent, &snap.Delivered, &snap.Failed, &snap.Pending,
		&snap.AvgDeliveryTimeMs, &snap.Cost,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}
	return &snap, nil
}

// GetTimeline retrieves timeline data for a campaign, grouping by time intervals.
func (r *StatsRepository) GetTimeline(ctx context.Context, campaignID uuid.UUID, interval, metric string) ([]domain.TimelinePoint, error) {
	// Map interval to PostgreSQL interval
	pgInterval := "1 hour"
	switch interval {
	case "5m":
		pgInterval = "5 minutes"
	case "15m":
		pgInterval = "15 minutes"
	case "1h":
		pgInterval = "1 hour"
	case "1d":
		pgInterval = "1 day"
	}

	// Map metric to column; default to "delivered"
	statusFilter := "delivered"
	switch metric {
	case "sent":
		statusFilter = "sent"
	case "failed":
		statusFilter = "failed"
	case "delivered":
		statusFilter = "delivered"
	}

	query := fmt.Sprintf(`SELECT
		date_trunc('minute', updated_at) - (EXTRACT(MINUTE FROM updated_at)::int %% EXTRACT(MINUTE FROM INTERVAL '%s')::int) * INTERVAL '1 minute' AS bucket,
		COUNT(*) AS value
		FROM campaign_recipients
		WHERE campaign_id = $1 AND status = $2
		GROUP BY bucket
		ORDER BY bucket ASC`, pgInterval)

	// For intervals >= 1 hour, use simpler truncation
	if interval == "1h" || interval == "" {
		query = `SELECT
			date_trunc('hour', updated_at) AS bucket,
			COUNT(*) AS value
			FROM campaign_recipients
			WHERE campaign_id = $1 AND status = $2
			GROUP BY bucket
			ORDER BY bucket ASC`
	} else if interval == "1d" {
		query = `SELECT
			date_trunc('day', updated_at) AS bucket,
			COUNT(*) AS value
			FROM campaign_recipients
			WHERE campaign_id = $1 AND status = $2
			GROUP BY bucket
			ORDER BY bucket ASC`
	}

	rows, err := r.db.QueryxContext(ctx, query, campaignID, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}
	defer rows.Close()

	var points []domain.TimelinePoint
	for rows.Next() {
		var ts time.Time
		var value int32
		if err := rows.Scan(&ts, &value); err != nil {
			return nil, fmt.Errorf("failed to scan timeline point: %w", err)
		}
		points = append(points, domain.TimelinePoint{Timestamp: ts, Value: value})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return points, nil
}

// GetHeatmap returns delivery data as a day_of_week x hour matrix.
func (r *StatsRepository) GetHeatmap(ctx context.Context, campaignID uuid.UUID) ([]domain.HeatmapCell, error) {
	query := `SELECT
		EXTRACT(DOW FROM updated_at)::int AS day_of_week,
		EXTRACT(HOUR FROM updated_at)::int AS hour,
		COUNT(*) FILTER (WHERE status = 'delivered') AS delivered_count,
		CASE
			WHEN COUNT(*) FILTER (WHERE status IN ('delivered', 'failed')) > 0
			THEN COUNT(*) FILTER (WHERE status = 'delivered')::float / COUNT(*) FILTER (WHERE status IN ('delivered', 'failed'))::float
			ELSE 0
		END AS delivery_rate
		FROM campaign_recipients
		WHERE campaign_id = $1 AND status IN ('delivered', 'failed')
		GROUP BY day_of_week, hour
		ORDER BY day_of_week, hour`

	rows, err := r.db.QueryxContext(ctx, query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get heatmap: %w", err)
	}
	defer rows.Close()

	var cells []domain.HeatmapCell
	for rows.Next() {
		var cell domain.HeatmapCell
		if err := rows.Scan(&cell.DayOfWeek, &cell.Hour, &cell.DeliveredCount, &cell.DeliveryRate); err != nil {
			return nil, fmt.Errorf("failed to scan heatmap cell: %w", err)
		}
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return cells, nil
}

// GetVariantComparison returns per-variant comparison data for a campaign.
func (r *StatsRepository) GetVariantComparison(ctx context.Context, campaignID uuid.UUID) ([]domain.VariantComparison, error) {
	query := `SELECT
		v.id AS variant_id,
		v.name AS variant_name,
		COALESCE(r.audience, 0) AS audience_size,
		v.sent_count AS sent,
		v.delivered_count AS delivered,
		v.failed_count AS failed,
		CASE WHEN v.sent_count > 0
			THEN v.delivered_count::float / v.sent_count::float
			ELSE 0
		END AS delivery_rate,
		0 AS avg_delivery_time_ms,
		0.0 AS cost
		FROM campaign_variants v
		LEFT JOIN (
			SELECT variant_id, COUNT(*) AS audience
			FROM campaign_recipients WHERE campaign_id = $1
			GROUP BY variant_id
		) r ON r.variant_id = v.id::text
		WHERE v.campaign_id = $1
		ORDER BY v.percentage DESC`

	rows, err := r.db.QueryxContext(ctx, query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get variant comparison: %w", err)
	}
	defer rows.Close()

	var comparisons []domain.VariantComparison
	for rows.Next() {
		var vc domain.VariantComparison
		if err := rows.Scan(
			&vc.VariantID, &vc.VariantName, &vc.AudienceSize,
			&vc.Sent, &vc.Delivered, &vc.Failed,
			&vc.DeliveryRate, &vc.AvgDeliveryTimeMs, &vc.Cost,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variant comparison: %w", err)
		}
		comparisons = append(comparisons, vc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return comparisons, nil
}

// GetOptimalSendTimes returns top-performing time slots based on historical data for a client.
func (r *StatsRepository) GetOptimalSendTimes(ctx context.Context, clientID uuid.UUID) ([]domain.TimeSlot, error) {
	query := `SELECT
		EXTRACT(DOW FROM cr.updated_at)::int AS day_of_week,
		EXTRACT(HOUR FROM cr.updated_at)::int AS hour,
		CASE
			WHEN COUNT(*) FILTER (WHERE cr.status IN ('delivered', 'failed')) > 0
			THEN COUNT(*) FILTER (WHERE cr.status = 'delivered')::float / COUNT(*) FILTER (WHERE cr.status IN ('delivered', 'failed'))::float
			ELSE 0
		END AS delivery_rate,
		0 AS avg_delivery_time_ms,
		COUNT(*) FILTER (WHERE cr.status = 'delivered')::float AS score
		FROM campaign_recipients cr
		JOIN campaigns c ON c.id = cr.campaign_id
		WHERE c.client_id = $1 AND cr.status IN ('delivered', 'failed')
		GROUP BY day_of_week, hour
		ORDER BY score DESC
		LIMIT 10`

	rows, err := r.db.QueryxContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get optimal send times: %w", err)
	}
	defer rows.Close()

	var slots []domain.TimeSlot
	for rows.Next() {
		var slot domain.TimeSlot
		if err := rows.Scan(&slot.DayOfWeek, &slot.Hour, &slot.DeliveryRate, &slot.AvgDeliveryTimeMs, &slot.Score); err != nil {
			return nil, fmt.Errorf("failed to scan time slot: %w", err)
		}
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return slots, nil
}
