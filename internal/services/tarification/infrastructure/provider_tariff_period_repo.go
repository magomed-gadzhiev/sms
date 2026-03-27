package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTariffPeriodRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTariffPeriodRepo(pool *pgxpool.Pool) *ProviderTariffPeriodRepo {
	return &ProviderTariffPeriodRepo{pool: pool}
}

func (r *ProviderTariffPeriodRepo) Create(ctx context.Context, period *domain.ProviderTariffPeriod) error {
	query := `INSERT INTO provider_tariff_periods (id, provider_tariff_plan_id, start_date, end_date, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, period.ID, period.ProviderTariffPlanID, period.StartDate, period.EndDate, period.CreatedAt)
	if err != nil {
		return fmt.Errorf("create provider_tariff_period: %w", err)
	}
	return nil
}

func (r *ProviderTariffPeriodRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffPeriod, error) {
	query := `SELECT id, provider_tariff_plan_id, start_date, end_date, created_at FROM provider_tariff_periods WHERE id = $1`
	p := &domain.ProviderTariffPeriod{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.ProviderTariffPlanID, &p.StartDate, &p.EndDate, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProviderTariffPeriodNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tariff_period: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPeriodRepo) GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*domain.ProviderTariffPeriod, error) {
	query := `SELECT id, provider_tariff_plan_id, start_date, end_date, created_at
		FROM provider_tariff_periods WHERE provider_tariff_plan_id = $1 AND start_date <= $2 AND end_date >= $2`
	p := &domain.ProviderTariffPeriod{}
	err := r.pool.QueryRow(ctx, query, planID, now).Scan(&p.ID, &p.ProviderTariffPlanID, &p.StartDate, &p.EndDate, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoActiveProviderTariffPeriod
	}
	if err != nil {
		return nil, fmt.Errorf("get active provider_tariff_period: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPeriodRepo) ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*domain.ProviderTariffPeriod, error) {
	query := `SELECT id, provider_tariff_plan_id, start_date, end_date, created_at
		FROM provider_tariff_periods WHERE provider_tariff_plan_id = $1 ORDER BY start_date`
	rows, err := r.pool.Query(ctx, query, planID)
	if err != nil {
		return nil, fmt.Errorf("list provider_tariff_periods: %w", err)
	}
	defer rows.Close()

	var result []*domain.ProviderTariffPeriod
	for rows.Next() {
		p := &domain.ProviderTariffPeriod{}
		if err := rows.Scan(&p.ID, &p.ProviderTariffPlanID, &p.StartDate, &p.EndDate, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan provider_tariff_period: %w", err)
		}
		result = append(result, p)
	}
	return result, nil
}
