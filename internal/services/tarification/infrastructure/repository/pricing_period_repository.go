package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// PricingPeriodRepository реализует domain.PricingPeriodRepository
type PricingPeriodRepository struct {
	db *sqlx.DB
}

// NewPricingPeriodRepository создает новый репозиторий периодов тарификации
func NewPricingPeriodRepository(db *sqlx.DB) *PricingPeriodRepository {
	return &PricingPeriodRepository{db: db}
}

// Create создает новый период тарификации
func (r *PricingPeriodRepository) Create(ctx context.Context, period *domain.PricingPeriod) error {
	query := `
		INSERT INTO pricing_periods (
			id, tariff_period_id, start_date, end_date, created_at
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		period.ID,
		period.TariffPeriodID,
		period.StartDate,
		period.EndDate,
		period.CreatedAt,
	)

	return err
}

// GetByID получает период тарификации по ID
func (r *PricingPeriodRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PricingPeriod, error) {
	var period domain.PricingPeriod
	query := `
		SELECT id, tariff_period_id, start_date, end_date, created_at
		FROM pricing_periods
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&period.ID,
		&period.TariffPeriodID,
		&period.StartDate,
		&period.EndDate,
		&period.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPricingPeriodNotFound
		}
		return nil, err
	}

	return &period, nil
}

// GetActiveByTariffPeriodID получает активный период тарификации для тарифного периода на указанную дату
func (r *PricingPeriodRepository) GetActiveByTariffPeriodID(ctx context.Context, tariffPeriodID uuid.UUID, now time.Time) (*domain.PricingPeriod, error) {
	var period domain.PricingPeriod
	query := `
		SELECT id, tariff_period_id, start_date, end_date, created_at
		FROM pricing_periods
		WHERE tariff_period_id = $1 AND start_date <= $2 AND end_date >= $2
	`

	err := r.db.QueryRowContext(ctx, query, tariffPeriodID, now).Scan(
		&period.ID,
		&period.TariffPeriodID,
		&period.StartDate,
		&period.EndDate,
		&period.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPricingPeriodNotFound
		}
		return nil, err
	}

	return &period, nil
}

// ListByTariffPeriodID получает все периоды тарификации для тарифного периода
func (r *PricingPeriodRepository) ListByTariffPeriodID(ctx context.Context, tariffPeriodID uuid.UUID) ([]*domain.PricingPeriod, error) {
	query := `
		SELECT id, tariff_period_id, start_date, end_date, created_at
		FROM pricing_periods
		WHERE tariff_period_id = $1
		ORDER BY start_date ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tariffPeriodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []*domain.PricingPeriod
	for rows.Next() {
		var period domain.PricingPeriod
		if err := rows.Scan(
			&period.ID,
			&period.TariffPeriodID,
			&period.StartDate,
			&period.EndDate,
			&period.CreatedAt,
		); err != nil {
			return nil, err
		}
		periods = append(periods, &period)
	}

	return periods, rows.Err()
}
