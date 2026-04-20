package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
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

const marginLogInsertColumns = `
	(id, aggregator_id, sub_account_id, message_id, operator_id, segment_count,
	 sub_account_price, aggregator_price, sub_account_total, aggregator_total,
	 margin, idempotency_key, created_at,
	 charge_mode, pool_segments, overage_segments)
`

const marginLogInsertValues = `
	($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
`

// Create вставляет запись маржи агрегатора. Идемпотентно по idempotency_key.
func (r *AggregatorMarginLogRepository) Create(ctx context.Context, entry *domain.AggregatorMarginLog) error {
	query := `
		INSERT INTO aggregator_margin_log ` + marginLogInsertColumns + `
		VALUES ` + marginLogInsertValues + `
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query,
		entry.ID, entry.AggregatorID, entry.SubAccountID, entry.MessageID, entry.OperatorID,
		entry.SegmentCount,
		entry.SubAccountPrice, entry.AggregatorPrice,
		entry.SubAccountTotal, entry.AggregatorTotal,
		entry.Margin,
		entry.IdempotencyKey, entry.CreatedAt,
		entry.ChargeMode, entry.PoolSegments, entry.OverageSegments,
	)
	if err != nil {
		return fmt.Errorf("aggregator_margin_log insert: %w", err)
	}
	return nil
}

// CreateTx вставляет margin_log в рамках переданной транзакции.
// Возвращает (true, nil), если запись создана; (false, nil) если уже существовала (ON CONFLICT).
// Используется billing-service'ом для атомарного dual-charge: INSERT margin_log первым,
// и если idempotency-guard сработал — транзакция откатывается без трогания балансов.
func (r *AggregatorMarginLogRepository) CreateTx(ctx context.Context, tx *sqlx.Tx, entry *domain.AggregatorMarginLog) (bool, error) {
	query := `
		INSERT INTO aggregator_margin_log ` + marginLogInsertColumns + `
		VALUES ` + marginLogInsertValues + `
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`

	var id uuid.UUID
	err := tx.QueryRowxContext(ctx, query,
		entry.ID, entry.AggregatorID, entry.SubAccountID, entry.MessageID, entry.OperatorID,
		entry.SegmentCount,
		entry.SubAccountPrice, entry.AggregatorPrice,
		entry.SubAccountTotal, entry.AggregatorTotal,
		entry.Margin,
		entry.IdempotencyKey, entry.CreatedAt,
		entry.ChargeMode, entry.PoolSegments, entry.OverageSegments,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("aggregator_margin_log insert tx: %w", err)
	}
	return true, nil
}
