package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type TemplateRepository struct {
	db *sqlx.DB
}

func normalizeVariables(vars []string) pq.StringArray {
	if vars == nil {
		return pq.StringArray{}
	}
	return pq.StringArray(vars)
}

func NewTemplateRepository(db *sqlx.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

type templateRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	Name            string         `db:"name"`
	Body            string         `db:"body"`
	Variables       pq.StringArray `db:"variables"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	ReviewerID      *uuid.UUID     `db:"reviewer_id"`
	ReviewComment   sql.NullString `db:"review_comment"`
	ReviewedAt      sql.NullTime   `db:"reviewed_at"`
	CreatedAt       sql.NullTime   `db:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at"`
	SenderNameID    *uuid.UUID     `db:"sender_name_id"`
	SenderName      sql.NullString `db:"sender_name"`
	TrafficType     string         `db:"traffic_type"`
}

func (r *templateRow) toDomain() *domain.Template {
	t := &domain.Template{
		ID:           r.ID,
		ClientID:     r.ClientID,
		Name:         r.Name,
		Body:         r.Body,
		Variables:    []string(r.Variables),
		Status:       r.Status,
		ReviewerID:   r.ReviewerID,
		SenderNameID: r.SenderNameID,
	}
	if r.RejectionReason.Valid {
		t.RejectionReason = r.RejectionReason.String
	}
	if r.ReviewComment.Valid {
		t.ReviewComment = r.ReviewComment.String
	}
	if r.ReviewedAt.Valid {
		reviewedAt := r.ReviewedAt.Time
		t.ReviewedAt = &reviewedAt
	}
	if r.CreatedAt.Valid {
		t.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		t.UpdatedAt = r.UpdatedAt.Time
	}
	if r.SenderName.Valid {
		t.SenderName = r.SenderName.String
	}
	t.TrafficType = r.TrafficType
	return t
}

func (r *TemplateRepository) Create(ctx context.Context, t *domain.Template) (*domain.Template, error) {
	query := `INSERT INTO templates (id, client_id, name, body, variables, status, sender_name_id, traffic_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, client_id, name, body, variables, status, rejection_reason, reviewer_id, review_comment, reviewed_at, created_at, updated_at, sender_name_id, NULL AS sender_name, traffic_type`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query,
		t.ID, t.ClientID, t.Name, t.Body, normalizeVariables(t.Variables), t.Status, t.SenderNameID, t.TrafficType,
	).StructScan(&row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicateTemplateName
		}
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	return row.toDomain(), nil
}

func (r *TemplateRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error) {
	query := `SELECT t.id, t.client_id, t.name, t.body, t.variables, t.status, t.rejection_reason, t.reviewer_id, t.review_comment, t.reviewed_at, t.created_at, t.updated_at, t.sender_name_id, sn.name AS sender_name, t.traffic_type
		FROM templates t
		LEFT JOIN sender_names sn ON t.sender_name_id = sn.id
		WHERE t.id = $1 AND t.client_id = $2`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return row.toDomain(), nil
}

// GetByIDAdmin retrieves a template without client_id check (for admin operations)
func (r *TemplateRepository) GetByIDAdmin(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	query := `SELECT t.id, t.client_id, t.name, t.body, t.variables, t.status, t.rejection_reason, t.reviewer_id, t.review_comment, t.reviewed_at, t.created_at, t.updated_at, t.sender_name_id, sn.name AS sender_name, t.traffic_type
		FROM templates t
		LEFT JOIN sender_names sn ON t.sender_name_id = sn.id
		WHERE t.id = $1`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query, id).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return row.toDomain(), nil
}

func (r *TemplateRepository) ListByClientID(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error) {
	countQuery := `SELECT COUNT(*) FROM templates t`
	listQuery := `SELECT t.id, t.client_id, t.name, t.body, t.variables, t.status, t.rejection_reason, t.reviewer_id, t.review_comment, t.reviewed_at, t.created_at, t.updated_at, t.sender_name_id, sn.name AS sender_name, t.traffic_type
		FROM templates t
		LEFT JOIN sender_names sn ON t.sender_name_id = sn.id`
	args := []interface{}{}
	paramIdx := 1
	conditions := []string{}

	if clientID != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("t.client_id = $%d", paramIdx))
		args = append(args, clientID)
		paramIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("t.status = $%d", paramIdx))
		args = append(args, status)
		paramIdx++
	}
	if len(conditions) > 0 {
		where := " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			where += " AND " + c
		}
		countQuery += where
		listQuery += where
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count templates: %w", err)
	}

	listQuery += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryxContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		var row templateRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan template: %w", err)
		}
		templates = append(templates, row.toDomain())
	}
	return templates, total, nil
}

func (r *TemplateRepository) Update(ctx context.Context, t *domain.Template) (*domain.Template, error) {
	query := `UPDATE templates SET name = $1, body = $2, variables = $3, status = $4, rejection_reason = $5, sender_name_id = $6, traffic_type = $7
		WHERE id = $8 AND client_id = $9
		RETURNING id, client_id, name, body, variables, status, rejection_reason, reviewer_id, review_comment, reviewed_at, created_at, updated_at, sender_name_id, NULL AS sender_name, traffic_type`

	var rejReason sql.NullString
	if t.RejectionReason != "" {
		rejReason = sql.NullString{String: t.RejectionReason, Valid: true}
	}

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query,
		t.Name, t.Body, normalizeVariables(t.Variables), t.Status, rejReason, t.SenderNameID, t.TrafficType, t.ID, t.ClientID,
	).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicateTemplateName
		}
		return nil, fmt.Errorf("failed to update template: %w", err)
	}
	return row.toDomain(), nil
}

// UpdateStatus updates only the status and rejection_reason fields (for admin moderation)
func (r *TemplateRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, rejectionReason string) (*domain.Template, error) {
	var rejReason sql.NullString
	if rejectionReason != "" {
		rejReason = sql.NullString{String: rejectionReason, Valid: true}
	}

	query := `UPDATE templates SET status = $1, rejection_reason = $2
		WHERE id = $3
		RETURNING id, client_id, name, body, variables, status, rejection_reason, reviewer_id, review_comment, reviewed_at, created_at, updated_at, sender_name_id, NULL AS sender_name, traffic_type`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query, status, rejReason, id).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update template status: %w", err)
	}
	return row.toDomain(), nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	query := `DELETE FROM templates WHERE id = $1 AND client_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// AssignReviewer sets the reviewer and transitions the template to 'review' status.
func (r *TemplateRepository) AssignReviewer(ctx context.Context, templateID, reviewerID uuid.UUID) error {
	query := `UPDATE templates SET reviewer_id = $1, status = 'review', reviewed_at = NOW() WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, reviewerID, templateID)
	if err != nil {
		return fmt.Errorf("failed to assign reviewer: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// RequestRevision sets the template status to 'revision_requested' with a review comment.
func (r *TemplateRepository) RequestRevision(ctx context.Context, templateID, reviewerID uuid.UUID, comment string) error {
	query := `UPDATE templates SET status = 'revision_requested', review_comment = $1, reviewer_id = $2, reviewed_at = NOW() WHERE id = $3`
	result, err := r.db.ExecContext(ctx, query, comment, reviewerID, templateID)
	if err != nil {
		return fmt.Errorf("failed to request revision: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}
