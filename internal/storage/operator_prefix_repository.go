package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/shared"
)

type OperatorPrefixRepository struct {
	db *sqlx.DB
}

func NewOperatorPrefixRepository(db *DB) *OperatorPrefixRepository {
	return &OperatorPrefixRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *OperatorPrefixRepository) GetAllActive(ctx context.Context) ([]shared.OperatorPrefix, error) {
	type row struct {
		OperatorID uuid.UUID  `db:"operator_id"`
		CountryID  *uuid.UUID `db:"country_id"`
		Prefix     string     `db:"prefix"`
		Priority   int        `db:"priority"`
	}
	var rows []row
	// JOIN'имся на operators, чтобы получить country_id для enrichment
	// в pipeline (bug #15: operator_id/country_id должны писаться при INSERT).
	err := r.db.SelectContext(ctx, &rows,
		`SELECT op.operator_id, o.country_id, op.prefix, op.priority
		 FROM operator_prefixes op
		 LEFT JOIN operators o ON o.id = op.operator_id
		 WHERE op.active = true`)
	if err != nil {
		return nil, err
	}
	result := make([]shared.OperatorPrefix, len(rows))
	for i, r := range rows {
		result[i] = shared.OperatorPrefix{
			OperatorID: r.OperatorID,
			CountryID:  r.CountryID,
			Prefix:     r.Prefix,
			Priority:   r.Priority,
		}
	}
	return result, nil
}
