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
		OperatorID uuid.UUID `db:"operator_id"`
		Prefix     string    `db:"prefix"`
		Priority   int       `db:"priority"`
	}
	var rows []row
	err := r.db.SelectContext(ctx, &rows,
		`SELECT operator_id, prefix, priority FROM operator_prefixes WHERE active = true`)
	if err != nil {
		return nil, err
	}
	result := make([]shared.OperatorPrefix, len(rows))
	for i, r := range rows {
		result[i] = shared.OperatorPrefix{OperatorID: r.OperatorID, Prefix: r.Prefix, Priority: r.Priority}
	}
	return result, nil
}
