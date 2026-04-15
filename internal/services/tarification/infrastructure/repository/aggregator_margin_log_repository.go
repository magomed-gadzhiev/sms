package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// AggregatorMarginLogRepository репозиторий логов маржи агрегатора
type AggregatorMarginLogRepository struct {
	db *sqlx.DB
}

// NewAggregatorMarginLogRepository создает новый репозиторий
func NewAggregatorMarginLogRepository(db *sqlx.DB) *AggregatorMarginLogRepository {
	return &AggregatorMarginLogRepository{db: db}
}

// Create вставляет запись маржи агрегатора
func (r *AggregatorMarginLogRepository) Create(ctx context.Context, entry *domain.AggregatorMarginLog) error {
	query := `
		INSERT INTO aggregator_margin_log
			(id, aggregator_id, sub_account_id, message_id, operator_id, segment_count,
			 sub_account_price, aggregator_price, sub_account_total, aggregator_total,
			 margin, idempotency_key, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (idempotency_key, created_at) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query,
		entry.ID, entry.AggregatorID, entry.SubAccountID, entry.MessageID, entry.OperatorID,
		entry.SegmentCount,
		entry.SubAccountPrice, entry.AggregatorPrice,
		entry.SubAccountTotal, entry.AggregatorTotal,
		entry.Margin,
		entry.IdempotencyKey, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("aggregator_margin_log insert: %w", err)
	}
	return nil
}
