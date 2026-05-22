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

type ProviderTariffTierRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTariffTierRepo(pool *pgxpool.Pool) *ProviderTariffTierRepo {
	return &ProviderTariffTierRepo{pool: pool}
}

func (r *ProviderTariffTierRepo) Create(ctx context.Context, tier *domain.ProviderTariffTier) error {
	query := `INSERT INTO provider_tariff_tiers (id, provider_tariff_period_id, from_count, price_per_segment)
		VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, query, tier.ID, tier.ProviderTariffPeriodID, tier.FromCount, tier.PricePerSegment)
	if err != nil {
		return fmt.Errorf("create provider_tariff_tier: %w", err)
	}
	return nil
}

func (r *ProviderTariffTierRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffTier, error) {
	query := `SELECT id, provider_tariff_period_id, from_count, price_per_segment FROM provider_tariff_tiers WHERE id = $1`
	t := &domain.ProviderTariffTier{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&t.ID, &t.ProviderTariffPeriodID, &t.FromCount, &t.PricePerSegment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("provider tariff tier not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tariff_tier: %w", err)
	}
	return t, nil
}

func (r *ProviderTariffTierRepo) Update(ctx context.Context, tier *domain.ProviderTariffTier) error {
	query := `UPDATE provider_tariff_tiers SET from_count = $1, price_per_segment = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, tier.FromCount, tier.PricePerSegment, tier.ID)
	if err != nil {
		return fmt.Errorf("update provider_tariff_tier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("provider tariff tier not found")
	}
	return nil
}

func (r *ProviderTariffTierRepo) ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*domain.ProviderTariffTier, error) {
	query := `SELECT id, provider_tariff_period_id, from_count, price_per_segment
		FROM provider_tariff_tiers WHERE provider_tariff_period_id = $1 ORDER BY from_count ASC`
	rows, err := r.pool.Query(ctx, query, periodID)
	if err != nil {
		return nil, fmt.Errorf("list provider_tariff_tiers: %w", err)
	}
	defer rows.Close()

	var result []*domain.ProviderTariffTier
	for rows.Next() {
		t := &domain.ProviderTariffTier{}
		if err := rows.Scan(&t.ID, &t.ProviderTariffPeriodID, &t.FromCount, &t.PricePerSegment); err != nil {
			return nil, fmt.Errorf("scan provider_tariff_tier: %w", err)
		}
		result = append(result, t)
	}
	return result, nil
}
