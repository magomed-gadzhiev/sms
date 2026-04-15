package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type AggregatorQuotaRepository struct {
	db *sqlx.DB
}

func NewAggregatorQuotaRepository(db *sqlx.DB) *AggregatorQuotaRepository {
	return &AggregatorQuotaRepository{db: db}
}

func (r *AggregatorQuotaRepository) Create(ctx context.Context, quota *domain.AggregatorQuota) error {
	query := `
		INSERT INTO aggregator_quotas (id, aggregator_id, period_start, period_end,
			segment_limit, segments_used, overage_rate, currency, auto_renew,
			notified_80pct, notified_100pct, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		quota.ID, quota.AggregatorID, quota.PeriodStart, quota.PeriodEnd,
		quota.SegmentLimit, quota.SegmentsUsed, quota.OverageRate, quota.Currency,
		quota.AutoRenew, quota.Notified80Pct, quota.Notified100Pct,
		quota.CreatedAt, quota.UpdatedAt,
	)
	return err
}

func (r *AggregatorQuotaRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorQuota, error) {
	var q domain.AggregatorQuota
	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas WHERE id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
		&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &q, err
}

func (r *AggregatorQuotaRepository) GetActive(ctx context.Context, aggregatorID uuid.UUID, now time.Time) (*domain.AggregatorQuota, error) {
	var q domain.AggregatorQuota
	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas
		WHERE aggregator_id = $1 AND period_start <= $2 AND period_end > $2
		ORDER BY period_start DESC
		LIMIT 1
	`
	err := r.db.QueryRowContext(ctx, query, aggregatorID, now).Scan(
		&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
		&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &q, err
}

func (r *AggregatorQuotaRepository) Update(ctx context.Context, quota *domain.AggregatorQuota) error {
	query := `
		UPDATE aggregator_quotas
		SET segment_limit = $2, overage_rate = $3, auto_renew = $4, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, quota.ID, quota.SegmentLimit, quota.OverageRate, quota.AutoRenew)
	return err
}

func (r *AggregatorQuotaRepository) IncrementUsage(ctx context.Context, quotaID uuid.UUID, segments int) (*domain.IncrementResult, error) {
	var result domain.IncrementResult
	query := `
		UPDATE aggregator_quotas
		SET segments_used = segments_used + $2, updated_at = now()
		WHERE id = $1
		RETURNING segments_used, segment_limit, overage_rate
	`
	var overageRate string
	err := r.db.QueryRowContext(ctx, query, quotaID, segments).Scan(
		&result.SegmentsUsed, &result.SegmentLimit, &overageRate,
	)
	if err != nil {
		return nil, err
	}
	result.OverageRate = overageRate

	prevUsed := result.SegmentsUsed - int64(segments)
	result.WasWithinQuota = prevUsed < result.SegmentLimit

	if result.SegmentsUsed > result.SegmentLimit {
		if prevUsed >= result.SegmentLimit {
			result.OverageCount = segments
		} else {
			result.OverageCount = int(result.SegmentsUsed - result.SegmentLimit)
		}
	}

	return &result, nil
}

func (r *AggregatorQuotaRepository) ListByAggregator(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*domain.AggregatorQuota, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM aggregator_quotas WHERE aggregator_id = $1`, aggregatorID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas WHERE aggregator_id = $1
		ORDER BY period_start DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, aggregatorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var quotas []*domain.AggregatorQuota
	for rows.Next() {
		var q domain.AggregatorQuota
		if err := rows.Scan(
			&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
			&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		quotas = append(quotas, &q)
	}
	return quotas, total, rows.Err()
}

func (r *AggregatorQuotaRepository) SetNotified80(ctx context.Context, quotaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE aggregator_quotas SET notified_80pct = true WHERE id = $1`, quotaID)
	return err
}

func (r *AggregatorQuotaRepository) SetNotified100(ctx context.Context, quotaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE aggregator_quotas SET notified_100pct = true WHERE id = $1`, quotaID)
	return err
}

func (r *AggregatorQuotaRepository) ListAutoRenewable(ctx context.Context, beforeDate time.Time) ([]*domain.AggregatorQuota, error) {
	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas
		WHERE auto_renew = true AND period_end <= $1
		AND NOT EXISTS (
			SELECT 1 FROM aggregator_quotas aq2
			WHERE aq2.aggregator_id = aggregator_quotas.aggregator_id
			AND aq2.period_start = aggregator_quotas.period_end
		)
	`
	rows, err := r.db.QueryContext(ctx, query, beforeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quotas []*domain.AggregatorQuota
	for rows.Next() {
		var q domain.AggregatorQuota
		if err := rows.Scan(
			&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
			&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, err
		}
		quotas = append(quotas, &q)
	}
	return quotas, rows.Err()
}
