package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TariffTierRepository реализует domain.TariffTierRepository
type TariffTierRepository struct {
	db *sqlx.DB
}

// NewTariffTierRepository создает новый репозиторий тарифных уровней
func NewTariffTierRepository(db *sqlx.DB) *TariffTierRepository {
	return &TariffTierRepository{db: db}
}

// Create создает новый тарифный уровень
func (r *TariffTierRepository) Create(ctx context.Context, tier *domain.TariffTier) error {
	query := `
		INSERT INTO tariff_tiers (
			id, tariff_period_id, from_count, price_per_segment
		) VALUES (
			$1, $2, $3, $4
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		tier.ID,
		tier.TariffPeriodID,
		tier.FromCount,
		tier.PricePerSegment,
	)

	return err
}

// GetByID получает тарифный уровень по ID
func (r *TariffTierRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TariffTier, error) {
	var tier domain.TariffTier
	query := `
		SELECT id, tariff_period_id, from_count, price_per_segment
		FROM tariff_tiers
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tier.ID,
		&tier.TariffPeriodID,
		&tier.FromCount,
		&tier.PricePerSegment,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTariffTierNotFound
		}
		return nil, err
	}

	return &tier, nil
}

// Update обновляет тарифный уровень
func (r *TariffTierRepository) Update(ctx context.Context, tier *domain.TariffTier) error {
	query := `
		UPDATE tariff_tiers
		SET tariff_period_id = $1, from_count = $2, price_per_segment = $3
		WHERE id = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		tier.TariffPeriodID,
		tier.FromCount,
		tier.PricePerSegment,
		tier.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrTariffTierNotFound
	}

	return nil
}

// ListByPeriodID получает все тарифные уровни для периода
func (r *TariffTierRepository) ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*domain.TariffTier, error) {
	query := `
		SELECT id, tariff_period_id, from_count, price_per_segment
		FROM tariff_tiers
		WHERE tariff_period_id = $1
		ORDER BY from_count ASC
	`

	rows, err := r.db.QueryContext(ctx, query, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tiers []*domain.TariffTier
	for rows.Next() {
		var tier domain.TariffTier
		if err := rows.Scan(
			&tier.ID,
			&tier.TariffPeriodID,
			&tier.FromCount,
			&tier.PricePerSegment,
		); err != nil {
			return nil, err
		}
		tiers = append(tiers, &tier)
	}

	return tiers, rows.Err()
}
