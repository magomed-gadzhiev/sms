package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type AuditRepository struct {
	db *sqlx.DB
}

func NewAuditRepository(db *sqlx.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

type auditRow struct {
	ID         uuid.UUID      `db:"id"`
	TemplateID *uuid.UUID     `db:"template_id"`
	Action     string         `db:"action"`
	OldBody    sql.NullString `db:"old_body"`
	NewBody    sql.NullString `db:"new_body"`
	ActorID    *uuid.UUID     `db:"actor_id"`
	ActorType  sql.NullString `db:"actor_type"`
	Reason     sql.NullString `db:"reason"`
	CreatedAt  sql.NullTime   `db:"created_at"`
}

func (r *auditRow) toDomain() *domain.AuditEntry {
	e := &domain.AuditEntry{
		ID:         r.ID,
		TemplateID: r.TemplateID,
		Action:     r.Action,
	}
	if r.OldBody.Valid {
		e.OldBody = r.OldBody.String
	}
	if r.NewBody.Valid {
		e.NewBody = r.NewBody.String
	}
	if r.ActorID != nil {
		e.ActorID = r.ActorID
	}
	if r.ActorType.Valid {
		e.ActorType = r.ActorType.String
	}
	if r.Reason.Valid {
		e.Reason = r.Reason.String
	}
	if r.CreatedAt.Valid {
		e.CreatedAt = r.CreatedAt.Time
	}
	return e
}

func (r *AuditRepository) Create(ctx context.Context, entry *domain.AuditEntry) error {
	query := `INSERT INTO template_audit_log (id, template_id, action, old_body, new_body, actor_id, actor_type, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	var oldBody, newBody, actorType, reason sql.NullString
	if entry.OldBody != "" {
		oldBody = sql.NullString{String: entry.OldBody, Valid: true}
	}
	if entry.NewBody != "" {
		newBody = sql.NullString{String: entry.NewBody, Valid: true}
	}
	if entry.ActorType != "" {
		actorType = sql.NullString{String: entry.ActorType, Valid: true}
	}
	if entry.Reason != "" {
		reason = sql.NullString{String: entry.Reason, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		uuid.New(), entry.TemplateID, entry.Action, oldBody, newBody, entry.ActorID, actorType, reason,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit entry: %w", err)
	}
	return nil
}

func (r *AuditRepository) ListByTemplateID(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM template_audit_log WHERE template_id = $1`, templateID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count audit entries: %w", err)
	}

	query := `SELECT id, template_id, action, old_body, new_body, actor_id, actor_type, reason, created_at
		FROM template_audit_log WHERE template_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, templateID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditEntry
	for rows.Next() {
		var row auditRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit entry: %w", err)
		}
		entries = append(entries, row.toDomain())
	}
	return entries, total, nil
}
