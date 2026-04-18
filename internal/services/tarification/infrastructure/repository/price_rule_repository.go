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

// PriceRuleRepository реализует domain.PriceRuleRepository поверх единой
// таблицы price_rules (см. migration 000104).
type PriceRuleRepository struct {
	db *sqlx.DB
}

func NewPriceRuleRepository(db *sqlx.DB) *PriceRuleRepository {
	return &PriceRuleRepository{db: db}
}

const findApplicableSQL = `
WITH ranked AS (
  SELECT
    id, owner_type, owner_id, country, operator, sender_category, traffic_type,
    valid_from, valid_to, price_model, price_value::text AS price_value_str, tiers_json,
    created_at, updated_at, created_by,
    CASE owner_type
      WHEN 'subaccount' THEN 3
      WHEN 'aggregator' THEN 2
      WHEN 'platform'   THEN 1
    END AS owner_rank,
    (CASE WHEN traffic_type    IS NOT NULL THEN 8 ELSE 0 END) +
    (CASE WHEN sender_category IS NOT NULL THEN 4 ELSE 0 END) +
    (CASE WHEN operator        IS NOT NULL THEN 2 ELSE 0 END) +
    (CASE WHEN country         IS NOT NULL THEN 1 ELSE 0 END) AS specificity_rank
  FROM price_rules
  WHERE (
         (owner_type='subaccount' AND owner_id = $1)
      OR (owner_type='aggregator' AND owner_id = $2)
      OR (owner_type='platform')
  )
    AND (country         IS NULL OR country         = $3)
    AND (operator        IS NULL OR operator        = $4)
    AND (sender_category IS NULL OR sender_category = $5)
    AND (traffic_type    IS NULL OR traffic_type    = $6)
    AND valid_from <= $7
    AND (valid_to IS NULL OR valid_to > $7)
)
SELECT id, owner_type, owner_id, country, operator, sender_category, traffic_type,
       valid_from, valid_to, price_model, price_value_str, tiers_json,
       created_at, updated_at, created_by
FROM ranked
ORDER BY owner_rank DESC, specificity_rank DESC, valid_from DESC
LIMIT 1;
`

func (r *PriceRuleRepository) FindApplicable(ctx context.Context, in domain.ResolveInput) (*domain.PriceRule, error) {
	row := r.db.QueryRowxContext(ctx, findApplicableSQL,
		in.SubaccountID, in.AggregatorID,
		nullableString(in.Country), nullableString(in.Operator),
		nullableString(in.SenderCategory), nullableString(in.TrafficType),
		in.Now,
	)
	rule, err := scanPriceRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNoApplicableRule
	}
	if err != nil {
		return nil, fmt.Errorf("find applicable: %w", err)
	}
	return rule, nil
}

const createPriceRuleSQL = `
INSERT INTO price_rules (
  id, owner_type, owner_id, country, operator, sender_category, traffic_type,
  valid_from, valid_to, price_model, price_value, tiers_json, created_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11::numeric, $12, $13
);
`

func (r *PriceRuleRepository) Create(ctx context.Context, p *domain.PriceRule) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, createPriceRuleSQL,
		p.ID, p.OwnerType, p.OwnerID,
		p.Country, p.Operator, p.SenderCategory, p.TrafficType,
		p.ValidFrom, p.ValidTo, p.PriceModel, p.PriceValue, p.TiersJSON, p.CreatedBy,
	)
	return err
}

const updatePriceRuleSQL = `
UPDATE price_rules SET
  country         = $2,
  operator        = $3,
  sender_category = $4,
  traffic_type    = $5,
  valid_from      = $6,
  valid_to        = $7,
  price_model     = $8,
  price_value     = $9::numeric,
  tiers_json      = $10,
  updated_at      = now()
WHERE id = $1;
`

func (r *PriceRuleRepository) Update(ctx context.Context, p *domain.PriceRule) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, updatePriceRuleSQL,
		p.ID,
		p.Country, p.Operator, p.SenderCategory, p.TrafficType,
		p.ValidFrom, p.ValidTo, p.PriceModel, p.PriceValue, p.TiersJSON,
	)
	return err
}

func (r *PriceRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM price_rules WHERE id = $1`, id)
	return err
}

const getByIDPriceRuleSQL = `
SELECT id, owner_type, owner_id, country, operator, sender_category, traffic_type,
       valid_from, valid_to, price_model, price_value::text, tiers_json,
       created_at, updated_at, created_by
FROM price_rules WHERE id = $1;
`

func (r *PriceRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PriceRule, error) {
	row := r.db.QueryRowxContext(ctx, getByIDPriceRuleSQL, id)
	p, err := scanPriceRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNoApplicableRule
	}
	return p, err
}

const hasCatchAllSQL = `
SELECT COUNT(*) FROM price_rules
WHERE owner_type='platform'
  AND country IS NULL AND operator IS NULL
  AND sender_category IS NULL AND traffic_type IS NULL
  AND valid_to IS NULL
  AND valid_from <= now();
`

func (r *PriceRuleRepository) HasPlatformCatchAll(ctx context.Context) (bool, error) {
	var n int
	if err := r.db.GetContext(ctx, &n, hasCatchAllSQL); err != nil {
		return false, err
	}
	return n > 0, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPriceRule(row rowScanner) (*domain.PriceRule, error) {
	var (
		p             domain.PriceRule
		country       sql.NullString
		operator      sql.NullString
		senderCat     sql.NullString
		trafficType   sql.NullString
		validTo       sql.NullTime
		priceValueStr sql.NullString
		createdBy     uuid.NullUUID
		ownerID       uuid.NullUUID
		tiersJSON     []byte
	)
	err := row.Scan(
		&p.ID, &p.OwnerType, &ownerID,
		&country, &operator, &senderCat, &trafficType,
		&p.ValidFrom, &validTo, &p.PriceModel, &priceValueStr, &tiersJSON,
		&p.CreatedAt, &p.UpdatedAt, &createdBy,
	)
	if err != nil {
		return nil, err
	}
	if ownerID.Valid {
		p.OwnerID = &ownerID.UUID
	}
	if country.Valid {
		p.Country = &country.String
	}
	if operator.Valid {
		p.Operator = &operator.String
	}
	if senderCat.Valid {
		p.SenderCategory = &senderCat.String
	}
	if trafficType.Valid {
		p.TrafficType = &trafficType.String
	}
	if validTo.Valid {
		p.ValidTo = &validTo.Time
	}
	if priceValueStr.Valid {
		v := priceValueStr.String
		p.PriceValue = &v
	}
	if createdBy.Valid {
		p.CreatedBy = &createdBy.UUID
	}
	if len(tiersJSON) > 0 {
		p.TiersJSON = tiersJSON
	}
	return &p, nil
}

// nullableString — конвертация пустой строки в NULL для SQL-запросов с
// NULL = wildcard семантикой в price_rules.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

