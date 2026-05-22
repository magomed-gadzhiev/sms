package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TarificationLogRepository реализует domain.TarificationLogRepository
type TarificationLogRepository struct {
	db *sqlx.DB
}

// NewTarificationLogRepository создает новый репозиторий логов тарификации
func NewTarificationLogRepository(db *sqlx.DB) *TarificationLogRepository {
	return &TarificationLogRepository{db: db}
}

// Create создает запись в логе тарификации
func (r *TarificationLogRepository) Create(ctx context.Context, log *domain.TarificationLog) error {
	query := `
		INSERT INTO tarification_log (
			id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, source_rule_id,
			segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.ClientID, log.MessageID, log.OperatorID,
		log.SenderCategory, log.Strategy,
		log.TariffPlanID, log.TariffPeriodID, log.SourceRuleID,
		log.SegmentCount, log.PricePerSegment,
		log.TotalAmount, log.RecalcAmount,
		log.IdempotencyKey, log.CreatedAt,
	)

	return err
}

// GetByIdempotencyKey получает запись по ключу идемпотентности
func (r *TarificationLogRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.TarificationLog, error) {
	var log domain.TarificationLog
	query := `
		SELECT id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, source_rule_id,
			segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		FROM tarification_log
		WHERE idempotency_key = $1
	`

	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&log.ID, &log.ClientID, &log.MessageID, &log.OperatorID,
		&log.SenderCategory, &log.Strategy,
		&log.TariffPlanID, &log.TariffPeriodID, &log.SourceRuleID,
		&log.SegmentCount, &log.PricePerSegment,
		&log.TotalAmount, &log.RecalcAmount,
		&log.IdempotencyKey, &log.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // не ошибка — ключ просто не найден
		}
		return nil, err
	}

	return &log, nil
}

// GetByMessageID получает запись по ID сообщения
func (r *TarificationLogRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) (*domain.TarificationLog, error) {
	var log domain.TarificationLog
	query := `
		SELECT id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, source_rule_id,
			segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		FROM tarification_log
		WHERE message_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&log.ID, &log.ClientID, &log.MessageID, &log.OperatorID,
		&log.SenderCategory, &log.Strategy,
		&log.TariffPlanID, &log.TariffPeriodID, &log.SourceRuleID,
		&log.SegmentCount, &log.PricePerSegment,
		&log.TotalAmount, &log.RecalcAmount,
		&log.IdempotencyKey, &log.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &log, nil
}
