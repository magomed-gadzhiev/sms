package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// AggregatorTariffRepository репозиторий тарифов агрегатора
type AggregatorTariffRepository struct {
	db *sqlx.DB
}

// NewAggregatorTariffRepository создает новый репозиторий тарифов агрегатора
func NewAggregatorTariffRepository(db *sqlx.DB) *AggregatorTariffRepository {
	return &AggregatorTariffRepository{db: db}
}

// GetForSubAccount ищет тариф с каскадом:
// 1. Специфичный для субаккаунта (aggregator_id + sub_account_id + operator_id + category)
// 2. Дефолтный агрегатора (aggregator_id + sub_account_id IS NULL + operator_id + category)
func (r *AggregatorTariffRepository) GetForSubAccount(
	ctx context.Context,
	aggregatorID, subAccountID, operatorID uuid.UUID,
	category domain.SenderCategory,
) (*domain.AggregatorTariff, error) {
	query := `
		SELECT id, aggregator_id, sub_account_id, operator_id, sender_category,
		       price_per_segment, active, created_at, updated_at
		FROM aggregator_tariffs
		WHERE aggregator_id = $1
		  AND operator_id = $2
		  AND sender_category = $3
		  AND active = true
		  AND (sub_account_id = $4 OR sub_account_id IS NULL)
		ORDER BY sub_account_id NULLS LAST
		LIMIT 1
	`

	var t domain.AggregatorTariff
	var subAccID *uuid.UUID

	err := r.db.QueryRowContext(ctx, query,
		aggregatorID, operatorID, string(category), subAccountID,
	).Scan(
		&t.ID, &t.AggregatorID, &subAccID, &t.OperatorID, &t.SenderCategory,
		&t.PricePerSegment, &t.Active, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("aggregator tariff lookup: %w", err)
	}
	t.SubAccountID = subAccID

	return &t, nil
}
