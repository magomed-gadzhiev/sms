package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// InvalidationOutboxRepository читает события инвалидации, публикуемые триггером
// price_rules_after_change. ResolvedRulesJanitor потребляет эти события и удаляет
// затронутые строки resolved_rules.
type InvalidationOutboxRepository struct {
	db *sqlx.DB
}

func NewInvalidationOutboxRepository(db *sqlx.DB) *InvalidationOutboxRepository {
	return &InvalidationOutboxRepository{db: db}
}

const listUnprocessedSQL = `
SELECT id, rule_id, owner_type, owner_id,
       affected_dims->>'country'         AS country,
       affected_dims->>'operator'        AS operator,
       affected_dims->>'sender_category' AS sender_category,
       affected_dims->>'traffic_type'    AS traffic_type,
       created_at
FROM resolved_rules_invalidation_outbox
WHERE processed_at IS NULL
ORDER BY id
LIMIT $1;
`

func (r *InvalidationOutboxRepository) ListUnprocessed(ctx context.Context, limit int) ([]domain.InvalidationEvent, error) {
	rows, err := r.db.QueryxContext(ctx, listUnprocessedSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.InvalidationEvent
	for rows.Next() {
		var (
			ev        domain.InvalidationEvent
			ownerID   sql.NullString
			country   sql.NullString
			operator  sql.NullString
			senderCat sql.NullString
			traffic   sql.NullString
		)
		if err := rows.Scan(&ev.ID, &ev.RuleID, &ev.OwnerType, &ownerID,
			&country, &operator, &senderCat, &traffic, &ev.CreatedAt); err != nil {
			return nil, err
		}
		if ownerID.Valid {
			id, perr := uuid.Parse(ownerID.String)
			if perr != nil {
				return nil, perr
			}
			ev.OwnerID = &id
		}
		if country.Valid {
			s := country.String
			ev.Country = &s
		}
		if operator.Valid {
			s := operator.String
			ev.Operator = &s
		}
		if senderCat.Valid {
			s := senderCat.String
			ev.SenderCat = &s
		}
		if traffic.Valid {
			s := traffic.String
			ev.Traffic = &s
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *InvalidationOutboxRepository) MarkProcessed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE resolved_rules_invalidation_outbox
SET processed_at = now()
WHERE id = ANY($1)
`, ids)
	return err
}
