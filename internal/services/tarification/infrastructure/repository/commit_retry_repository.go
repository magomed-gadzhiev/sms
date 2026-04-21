package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// CommitRetryRepository — Postgres implementation of domain.CommitRetryRepository.
type CommitRetryRepository struct {
	db *sqlx.DB
}

func NewCommitRetryRepository(db *sqlx.DB) *CommitRetryRepository {
	return &CommitRetryRepository{db: db}
}

func (r *CommitRetryRepository) Enqueue(ctx context.Context, e *domain.CommitRetryEntry) (bool, error) {
	const query = `
		INSERT INTO commit_retry_queue (
			message_id, client_id, operator_id, sender_name, segment_count,
			idempotency_key, attempt_count, last_error, next_retry_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11)
		ON CONFLICT (message_id) DO NOTHING
	`
	res, err := r.db.ExecContext(ctx, query,
		e.MessageID, e.ClientID, e.OperatorID, e.SenderName, e.SegmentCount,
		e.IdempotencyKey, e.AttemptCount, e.LastError, e.NextRetryAt,
		e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("enqueue commit retry: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n == 1, nil
}

func (r *CommitRetryRepository) ClaimBatch(ctx context.Context, tx *sqlx.Tx, limit int, now time.Time) ([]*domain.CommitRetryEntry, error) {
	const query = `
		SELECT message_id, client_id, operator_id, sender_name, segment_count,
		       idempotency_key, attempt_count, COALESCE(last_error, ''),
		       next_retry_at, created_at, updated_at
		FROM commit_retry_queue
		WHERE next_retry_at <= $1
		ORDER BY next_retry_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.QueryxContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim batch: %w", err)
	}
	defer rows.Close()

	var out []*domain.CommitRetryEntry
	for rows.Next() {
		e := &domain.CommitRetryEntry{}
		if err := rows.Scan(
			&e.MessageID, &e.ClientID, &e.OperatorID, &e.SenderName, &e.SegmentCount,
			&e.IdempotencyKey, &e.AttemptCount, &e.LastError,
			&e.NextRetryAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iter: %w", err)
	}
	return out, nil
}

func (r *CommitRetryRepository) UpdateAttempt(ctx context.Context, tx *sqlx.Tx, messageID uuid.UUID, attemptCount int, nextRetryAt time.Time, lastError string) error {
	const query = `
		UPDATE commit_retry_queue
		SET attempt_count = $2,
		    next_retry_at = $3,
		    last_error    = NULLIF($4, ''),
		    updated_at    = NOW()
		WHERE message_id = $1
	`
	_, err := tx.ExecContext(ctx, query, messageID, attemptCount, nextRetryAt, lastError)
	if err != nil {
		return fmt.Errorf("update attempt: %w", err)
	}
	return nil
}

func (r *CommitRetryRepository) Delete(ctx context.Context, messageID uuid.UUID) error {
	const query = `DELETE FROM commit_retry_queue WHERE message_id = $1`
	if _, err := r.db.ExecContext(ctx, query, messageID); err != nil {
		return fmt.Errorf("delete commit retry: %w", err)
	}
	return nil
}

func (r *CommitRetryRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
