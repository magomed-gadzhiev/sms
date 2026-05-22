package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// PriceRulesVersionRepository читает глобальную версию price_rules.
// Версия инкрементится триггером price_rules_after_change на любой INSERT/UPDATE/DELETE.
type PriceRulesVersionRepository struct {
	db *sqlx.DB
}

func NewPriceRulesVersionRepository(db *sqlx.DB) *PriceRulesVersionRepository {
	return &PriceRulesVersionRepository{db: db}
}

func (r *PriceRulesVersionRepository) GetVersion(ctx context.Context) (int64, error) {
	var v int64
	err := r.db.GetContext(ctx, &v, `SELECT version FROM price_rules_version WHERE id=1`)
	return v, err
}
