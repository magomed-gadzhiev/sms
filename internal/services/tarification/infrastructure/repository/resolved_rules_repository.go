package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ResolvedRulesRepository реализует domain.ResolvedRulesRepository —
// денормализованный горячий кеш правил, читается по PK.
type ResolvedRulesRepository struct {
	db *sqlx.DB
}

func NewResolvedRulesRepository(db *sqlx.DB) *ResolvedRulesRepository {
	return &ResolvedRulesRepository{db: db}
}

const getResolvedSQL = `
SELECT subaccount_id, country, operator, sender_category, traffic_type, effective_date,
       price_model, price_value::text, tiers_json,
       source_rule_id, source_level, aggregator_id, resolved_at, rules_version
FROM resolved_rules
WHERE subaccount_id=$1 AND country=$2 AND operator=$3 AND sender_category=$4
  AND traffic_type=$5 AND effective_date=$6;
`

func (r *ResolvedRulesRepository) Get(ctx context.Context, subID uuid.UUID, country, operator, cat, traffic string, date time.Time) (*domain.ResolvedRule, error) {
	var (
		rr            domain.ResolvedRule
		priceValueStr sql.NullString
		tiersJSON     []byte
	)
	err := r.db.QueryRowxContext(ctx, getResolvedSQL,
		subID, country, operator, cat, traffic, date,
	).Scan(
		&rr.SubaccountID, &rr.Country, &rr.Operator, &rr.SenderCategory, &rr.TrafficType, &rr.EffectiveDate,
		&rr.PriceModel, &priceValueStr, &tiersJSON,
		&rr.SourceRuleID, &rr.SourceLevel, &rr.AggregatorID, &rr.ResolvedAt, &rr.RulesVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if priceValueStr.Valid {
		v := priceValueStr.String
		rr.PriceValue = &v
	}
	if len(tiersJSON) > 0 {
		rr.TiersJSON = tiersJSON
	}
	return &rr, nil
}

const upsertResolvedSQL = `
INSERT INTO resolved_rules (
  subaccount_id, country, operator, sender_category, traffic_type, effective_date,
  price_model, price_value, tiers_json,
  source_rule_id, source_level, aggregator_id, rules_version
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8::numeric, $9,
  $10, $11, $12, $13
)
ON CONFLICT (subaccount_id, country, operator, sender_category, traffic_type, effective_date)
DO UPDATE SET
  price_model   = EXCLUDED.price_model,
  price_value   = EXCLUDED.price_value,
  tiers_json    = EXCLUDED.tiers_json,
  source_rule_id= EXCLUDED.source_rule_id,
  source_level  = EXCLUDED.source_level,
  aggregator_id = EXCLUDED.aggregator_id,
  resolved_at   = now(),
  rules_version = EXCLUDED.rules_version;
`

func (r *ResolvedRulesRepository) Upsert(ctx context.Context, rr *domain.ResolvedRule) error {
	_, err := r.db.ExecContext(ctx, upsertResolvedSQL,
		rr.SubaccountID, rr.Country, rr.Operator, rr.SenderCategory, rr.TrafficType, rr.EffectiveDate,
		rr.PriceModel, rr.PriceValue, rr.TiersJSON,
		rr.SourceRuleID, rr.SourceLevel, rr.AggregatorID, rr.RulesVersion,
	)
	return err
}

// DeleteAffected удаляет строки resolved_rules, подходящие под критерий инвалидации.
// Стратегия по owner_type:
//   - subaccount → WHERE subaccount_id = owner_id
//   - aggregator → WHERE aggregator_id = owner_id (денорм в resolved_rules)
//   - platform   → без owner-фильтра (потенциально вся таблица)
//
// Dims-фильтры опциональны: nil означает "любое значение" (не добавляется в WHERE).
func (r *ResolvedRulesRepository) DeleteAffected(ctx context.Context, ownerType domain.PriceOwnerType, ownerID *uuid.UUID, country, operator, cat, traffic *string) (int64, error) {
	var (
		where []string
		args  []any
	)
	nextPlaceholder := func() string {
		return fmt.Sprintf("$%d", len(args)+1)
	}
	switch ownerType {
	case domain.OwnerSubaccount:
		if ownerID == nil {
			return 0, errors.New("subaccount invalidation requires owner_id")
		}
		args = append(args, *ownerID)
		where = append(where, "subaccount_id = "+nextPlaceholder())
	case domain.OwnerAggregator:
		if ownerID == nil {
			return 0, errors.New("aggregator invalidation requires owner_id")
		}
		args = append(args, *ownerID)
		where = append(where, "aggregator_id = "+nextPlaceholder())
	case domain.OwnerPlatform:
		// no owner filter
	default:
		return 0, fmt.Errorf("unknown owner_type: %s", ownerType)
	}
	if country != nil {
		args = append(args, *country)
		where = append(where, "country = "+nextPlaceholder())
	}
	if operator != nil {
		args = append(args, *operator)
		where = append(where, "operator = "+nextPlaceholder())
	}
	if cat != nil {
		args = append(args, *cat)
		where = append(where, "sender_category = "+nextPlaceholder())
	}
	if traffic != nil {
		args = append(args, *traffic)
		where = append(where, "traffic_type = "+nextPlaceholder())
	}

	q := "DELETE FROM resolved_rules"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
