package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// CommitIdempotencyGuardRepository — Postgres impl для commit_idempotency_guard.
type CommitIdempotencyGuardRepository struct {
	db *sqlx.DB
}

func NewCommitIdempotencyGuardRepository(db *sqlx.DB) *CommitIdempotencyGuardRepository {
	return &CommitIdempotencyGuardRepository{db: db}
}

// ClaimTx — INSERT ON CONFLICT (message_id) DO NOTHING. При conflict'е row не
// создаётся; inserted возвращает false.
func (r *CommitIdempotencyGuardRepository) ClaimTx(ctx context.Context, tx *sqlx.Tx, messageID uuid.UUID) (bool, error) {
	const query = `
		INSERT INTO commit_idempotency_guard (message_id)
		VALUES ($1)
		ON CONFLICT (message_id) DO NOTHING
	`
	res, err := tx.ExecContext(ctx, query, messageID)
	if err != nil {
		return false, fmt.Errorf("commit_idempotency_guard claim: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n == 1, nil
}
