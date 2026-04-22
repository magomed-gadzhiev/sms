package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type operatorBindingRow struct {
	ID              uuid.UUID      `db:"id"`
	TemplateID      uuid.UUID      `db:"template_id"`
	SenderNameID    uuid.UUID      `db:"sender_name_id"`
	OperatorID      uuid.UUID      `db:"operator_id"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	ReviewedBy      *uuid.UUID     `db:"reviewed_by"`
	ReviewedAt      sql.NullTime   `db:"reviewed_at"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

func (r *operatorBindingRow) toDomain() *domain.OperatorTemplateBinding {
	b := &domain.OperatorTemplateBinding{
		ID:           r.ID,
		TemplateID:   r.TemplateID,
		SenderNameID: r.SenderNameID,
		OperatorID:   r.OperatorID,
		Status:       r.Status,
		ReviewedBy:   r.ReviewedBy,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if r.RejectionReason.Valid {
		b.RejectionReason = r.RejectionReason.String
	}
	if r.ReviewedAt.Valid {
		t := r.ReviewedAt.Time
		b.ReviewedAt = &t
	}
	return b
}

// OperatorBindingRepo persists operator_template_bindings using sqlx (same driver as SenderNameRepository).
type OperatorBindingRepo struct {
	db *sqlx.DB
}

func NewOperatorBindingRepo(db *sqlx.DB) *OperatorBindingRepo {
	return &OperatorBindingRepo{db: db}
}

// Create inserts a new binding. Returns domain.ErrDuplicateOperatorBinding on 23505.
func (r *OperatorBindingRepo) Create(ctx context.Context, b *domain.OperatorTemplateBinding) error {
	const q = `
		INSERT INTO operator_template_bindings
		  (id, template_id, sender_name_id, operator_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, q,
		b.ID, b.TemplateID, b.SenderNameID, b.OperatorID, b.Status, b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateOperatorBinding
		}
		return fmt.Errorf("create operator binding: %w", err)
	}
	return nil
}

// GetByID fetches a binding by primary key. Returns domain.ErrOperatorBindingNotFound when absent.
func (r *OperatorBindingRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.OperatorTemplateBinding, error) {
	const q = `
		SELECT id, template_id, sender_name_id, operator_id, status,
		       rejection_reason, reviewed_by, reviewed_at, created_at, updated_at
		FROM operator_template_bindings
		WHERE id = $1`

	var row operatorBindingRow
	if err := r.db.QueryRowxContext(ctx, q, id).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOperatorBindingNotFound
		}
		return nil, fmt.Errorf("get operator binding by id: %w", err)
	}
	return row.toDomain(), nil
}

// ListPendingByOperator returns pending bindings for an operator ordered by created_at ASC.
func (r *OperatorBindingRepo) ListPendingByOperator(ctx context.Context, opID uuid.UUID, limit, offset int) ([]*domain.OperatorTemplateBinding, error) {
	const q = `
		SELECT id, template_id, sender_name_id, operator_id, status,
		       rejection_reason, reviewed_by, reviewed_at, created_at, updated_at
		FROM operator_template_bindings
		WHERE operator_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, q, opID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending operator bindings: %w", err)
	}
	defer rows.Close()

	var out []*domain.OperatorTemplateBinding
	for rows.Next() {
		var row operatorBindingRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("scan operator binding: %w", err)
		}
		out = append(out, row.toDomain())
	}
	return out, nil
}

// GetByTemplateOperator fetches a binding by (template_id, operator_id). Returns domain.ErrOperatorBindingNotFound when absent.
func (r *OperatorBindingRepo) GetByTemplateOperator(ctx context.Context, templateID, operatorID uuid.UUID) (*domain.OperatorTemplateBinding, error) {
	const q = `
		SELECT id, template_id, sender_name_id, operator_id, status,
		       rejection_reason, reviewed_by, reviewed_at, created_at, updated_at
		FROM operator_template_bindings
		WHERE template_id = $1 AND operator_id = $2`

	var row operatorBindingRow
	if err := r.db.QueryRowxContext(ctx, q, templateID, operatorID).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOperatorBindingNotFound
		}
		return nil, fmt.Errorf("get operator binding by template+operator: %w", err)
	}
	return row.toDomain(), nil
}

// UpdateStatus transitions status, stores optional reason and reviewer.
// Returns domain.ErrOperatorBindingNotFound when no row is matched.
func (r *OperatorBindingRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status, reason string, reviewer uuid.UUID) error {
	const q = `
		UPDATE operator_template_bindings
		SET status           = $2,
		    rejection_reason = NULLIF($3, ''),
		    reviewed_by      = $4,
		    reviewed_at      = NOW(),
		    updated_at       = NOW()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id, status, reason, reviewer)
	if err != nil {
		return fmt.Errorf("update operator binding status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrOperatorBindingNotFound
	}
	return nil
}
