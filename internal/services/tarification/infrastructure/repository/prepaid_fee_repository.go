package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// PrepaidFeeRepository реализует domain.PrepaidFeeRepository
type PrepaidFeeRepository struct {
	db *sqlx.DB
}

// NewPrepaidFeeRepository создает новый репозиторий предоплат
func NewPrepaidFeeRepository(db *sqlx.DB) *PrepaidFeeRepository {
	return &PrepaidFeeRepository{db: db}
}

// Create создает запись предоплаты
func (r *PrepaidFeeRepository) Create(ctx context.Context, fee *domain.PrepaidFee) error {
	query := `
		INSERT INTO prepaid_fees (id, tariff_plan_id, tariff_period_id, amount, currency, charged, charged_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		fee.ID, fee.TariffPlanID, fee.TariffPeriodID,
		fee.Amount, fee.Currency, fee.Charged, fee.ChargedAt, fee.CreatedAt,
	)
	return err
}

// GetByID получает предоплату по ID
func (r *PrepaidFeeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PrepaidFee, error) {
	var fee domain.PrepaidFee
	query := `
		SELECT id, tariff_plan_id, tariff_period_id, amount, currency, charged, charged_at, created_at
		FROM prepaid_fees WHERE id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&fee.ID, &fee.TariffPlanID, &fee.TariffPeriodID,
		&fee.Amount, &fee.Currency, &fee.Charged, &fee.ChargedAt, &fee.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrPrepaidFeeNotFound
		}
		return nil, err
	}
	return &fee, nil
}

// GetByPeriodID получает предоплату по ID тарифного периода
func (r *PrepaidFeeRepository) GetByPeriodID(ctx context.Context, tariffPeriodID uuid.UUID) (*domain.PrepaidFee, error) {
	var fee domain.PrepaidFee
	query := `
		SELECT id, tariff_plan_id, tariff_period_id, amount, currency, charged, charged_at, created_at
		FROM prepaid_fees WHERE tariff_period_id = $1
	`
	err := r.db.QueryRowContext(ctx, query, tariffPeriodID).Scan(
		&fee.ID, &fee.TariffPlanID, &fee.TariffPeriodID,
		&fee.Amount, &fee.Currency, &fee.Charged, &fee.ChargedAt, &fee.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &fee, nil
}

// GetUncharged получает все неоплаченные предоплаты, периоды которых уже начались
func (r *PrepaidFeeRepository) GetUncharged(ctx context.Context, now time.Time) ([]*domain.PrepaidFee, error) {
	query := `
		SELECT pf.id, pf.tariff_plan_id, pf.tariff_period_id, pf.amount, pf.currency, pf.charged, pf.charged_at, pf.created_at
		FROM prepaid_fees pf
		JOIN tariff_periods tp ON tp.id = pf.tariff_period_id
		WHERE pf.charged = false AND tp.start_date <= $1
	`
	rows, err := r.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fees []*domain.PrepaidFee
	for rows.Next() {
		var f domain.PrepaidFee
		if err := rows.Scan(&f.ID, &f.TariffPlanID, &f.TariffPeriodID, &f.Amount, &f.Currency, &f.Charged, &f.ChargedAt, &f.CreatedAt); err != nil {
			return nil, err
		}
		fees = append(fees, &f)
	}
	return fees, rows.Err()
}

// MarkCharged помечает предоплату как списанную
func (r *PrepaidFeeRepository) MarkCharged(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE prepaid_fees SET charged = true, charged_at = NOW() WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrPrepaidFeeNotFound
	}
	return nil
}
