package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderUsageCounterRepo struct {
	pool *pgxpool.Pool
}

func NewProviderUsageCounterRepo(pool *pgxpool.Pool) *ProviderUsageCounterRepo {
	return &ProviderUsageCounterRepo{pool: pool}
}

func (r *ProviderUsageCounterRepo) GetOrCreate(ctx context.Context, planID, periodID uuid.UUID) (*domain.ProviderUsageCounter, error) {
	query := `SELECT id, provider_tariff_plan_id, provider_tariff_period_id, segment_count, updated_at
		FROM provider_usage_counters WHERE provider_tariff_plan_id = $1 AND provider_tariff_period_id = $2`
	c := &domain.ProviderUsageCounter{}
	err := r.pool.QueryRow(ctx, query, planID, periodID).Scan(&c.ID, &c.ProviderTariffPlanID, &c.ProviderTariffPeriodID, &c.SegmentCount, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		c = domain.NewProviderUsageCounter(planID, periodID)
		insertQ := `INSERT INTO provider_usage_counters (id, provider_tariff_plan_id, provider_tariff_period_id, segment_count, updated_at)
			VALUES ($1, $2, $3, $4, $5)`
		_, err = r.pool.Exec(ctx, insertQ, c.ID, c.ProviderTariffPlanID, c.ProviderTariffPeriodID, c.SegmentCount, c.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("create provider_usage_counter: %w", err)
		}
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_usage_counter: %w", err)
	}
	return c, nil
}

func (r *ProviderUsageCounterRepo) IncrementAndGet(ctx context.Context, planID, periodID uuid.UUID, segments int) (*domain.ProviderUsageCounter, error) {
	query := `UPDATE provider_usage_counters SET segment_count = segment_count + $1
		WHERE provider_tariff_plan_id = $2 AND provider_tariff_period_id = $3
		RETURNING id, provider_tariff_plan_id, provider_tariff_period_id, segment_count, updated_at`
	c := &domain.ProviderUsageCounter{}
	err := r.pool.QueryRow(ctx, query, segments, planID, periodID).Scan(
		&c.ID, &c.ProviderTariffPlanID, &c.ProviderTariffPeriodID, &c.SegmentCount, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("increment provider_usage_counter: %w", err)
	}
	return c, nil
}
