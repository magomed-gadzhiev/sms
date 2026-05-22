package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// SubaccountUsageCounterRepository реализует domain.SubaccountUsageCounterRepository.
// Один счётчик per (subaccount_id, period_key). source_rule_id не в PK —
// счётчик продолжается при смене правила в середине периода.
type SubaccountUsageCounterRepository struct {
	db *sqlx.DB
}

func NewSubaccountUsageCounterRepository(db *sqlx.DB) *SubaccountUsageCounterRepository {
	return &SubaccountUsageCounterRepository{db: db}
}

const incrementUsageSQL = `
INSERT INTO subaccount_usage_counters (subaccount_id, period_key, segments_used, amount_charged, last_updated)
VALUES ($1, $2, $3, $4::numeric, now())
ON CONFLICT (subaccount_id, period_key)
DO UPDATE SET
  segments_used  = subaccount_usage_counters.segments_used  + EXCLUDED.segments_used,
  amount_charged = subaccount_usage_counters.amount_charged + EXCLUDED.amount_charged,
  last_updated   = now()
RETURNING segments_used;
`

// Increment атомарно инкрементит счётчик. Возвращает segments_used ПОСЛЕ инкремента —
// это нужно для корректного определения тира в прогрессивной tiered-модели, когда
// текущее сообщение пересекает границу тира.
func (r *SubaccountUsageCounterRepository) Increment(ctx context.Context, subID uuid.UUID, periodKey string, segments int64, amount string) (int64, error) {
	var newTotal int64
	err := r.db.QueryRowxContext(ctx, incrementUsageSQL, subID, periodKey, segments, amount).Scan(&newTotal)
	return newTotal, err
}

const getUsageSQL = `
SELECT subaccount_id, period_key, segments_used, amount_charged::text, last_updated
FROM subaccount_usage_counters
WHERE subaccount_id=$1 AND period_key=$2;
`

func (r *SubaccountUsageCounterRepository) Get(ctx context.Context, subID uuid.UUID, periodKey string) (*domain.SubaccountUsageCounter, error) {
	var c domain.SubaccountUsageCounter
	err := r.db.QueryRowxContext(ctx, getUsageSQL, subID, periodKey).Scan(
		&c.SubaccountID, &c.PeriodKey, &c.SegmentsUsed, &c.AmountCharged, &c.LastUpdated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, err
}
