package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type SenderNameRepository struct {
	db *sqlx.DB
}

func NewSenderNameRepository(db *sqlx.DB) *SenderNameRepository {
	return &SenderNameRepository{db: db}
}

type senderNameRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	Name            string         `db:"name"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	ReviewerID      *uuid.UUID     `db:"reviewer_id"`
	ReviewedAt      sql.NullTime   `db:"reviewed_at"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

func (r *senderNameRow) toDomain() *domain.SenderName {
	sn := &domain.SenderName{
		ID:        r.ID,
		ClientID:  r.ClientID,
		Name:      r.Name,
		Status:    r.Status,
		ReviewerID: r.ReviewerID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.RejectionReason.Valid {
		sn.RejectionReason = r.RejectionReason.String
	}
	if r.ReviewedAt.Valid {
		t := r.ReviewedAt.Time
		sn.ReviewedAt = &t
	}
	return sn
}

func (r *SenderNameRepository) Create(ctx context.Context, sn *domain.SenderName) (*domain.SenderName, error) {
	const q = `
		INSERT INTO sender_names (id, client_id, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`

	var row senderNameRow
	err := r.db.QueryRowxContext(ctx, q, sn.ID, sn.ClientID, sn.Name, sn.Status).StructScan(&row)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrDuplicateSenderName
		}
		return nil, fmt.Errorf("create sender name: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SenderNameRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SenderName, error) {
	const q = `
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names WHERE id = $1`

	var row senderNameRow
	if err := r.db.QueryRowxContext(ctx, q, id).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSenderNameNotFound
		}
		return nil, fmt.Errorf("get sender name by id: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SenderNameRepository) GetByClientAndName(ctx context.Context, clientID uuid.UUID, name string) (*domain.SenderName, error) {
	const q = `
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names WHERE client_id = $1 AND name = $2`

	var row senderNameRow
	if err := r.db.QueryRowxContext(ctx, q, clientID, name).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSenderNameNotFound
		}
		return nil, fmt.Errorf("get sender name by client and name: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SenderNameRepository) ListByClient(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.SenderName, int, error) {
	args := []interface{}{clientID}
	where := "WHERE client_id = $1"
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int
	if err := r.db.QueryRowxContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM sender_names %s", where), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sender names: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))

	rows, err := r.db.QueryxContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sender names: %w", err)
	}
	defer rows.Close()

	var result []*domain.SenderName
	for rows.Next() {
		var row senderNameRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("scan sender name: %w", err)
		}
		result = append(result, row.toDomain())
	}
	return result, total, nil
}

func (r *SenderNameRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, rejectionReason string, reviewerID *uuid.UUID) (*domain.SenderName, error) {
	const q = `
		UPDATE sender_names
		SET status = $2, rejection_reason = $3, reviewer_id = $4, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`

	var reason sql.NullString
	if rejectionReason != "" {
		reason = sql.NullString{String: rejectionReason, Valid: true}
	}

	var row senderNameRow
	if err := r.db.QueryRowxContext(ctx, q, id, status, reason, reviewerID).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSenderNameNotFound
		}
		return nil, fmt.Errorf("update sender name status: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SenderNameRepository) Update(ctx context.Context, sn *domain.SenderName) (*domain.SenderName, error) {
	const q = `
		UPDATE sender_names
		SET name = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`

	var row senderNameRow
	if err := r.db.QueryRowxContext(ctx, q, sn.ID, sn.Name).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSenderNameNotFound
		}
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrDuplicateSenderName
		}
		return nil, fmt.Errorf("update sender name: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SenderNameRepository) ListAll(ctx context.Context, clientID *uuid.UUID, status, nameQuery string, limit, offset int) ([]*domain.SenderName, int, error) {
	var conditions []string
	var args []interface{}

	if clientID != nil {
		args = append(args, *clientID)
		conditions = append(conditions, fmt.Sprintf("client_id = $%d", len(args)))
	}
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if nameQuery != "" {
		args = append(args, "%"+nameQuery+"%")
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", len(args)))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRowxContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM sender_names %s", where), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count all sender names: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))

	rows, err := r.db.QueryxContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list all sender names: %w", err)
	}
	defer rows.Close()

	var result []*domain.SenderName
	for rows.Next() {
		var row senderNameRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("scan sender name: %w", err)
		}
		result = append(result, row.toDomain())
	}
	return result, total, nil
}

type senderNameHistoryRow struct {
	ID           uuid.UUID      `db:"id"`
	SenderNameID uuid.UUID      `db:"sender_name_id"`
	OldStatus    sql.NullString `db:"old_status"`
	NewStatus    string         `db:"new_status"`
	ActorID      *uuid.UUID     `db:"actor_id"`
	ActorType    string         `db:"actor_type"`
	Comment      sql.NullString `db:"comment"`
	CreatedAt    time.Time      `db:"created_at"`
}

func (r *senderNameHistoryRow) toDomain() *domain.SenderNameStatusHistory {
	h := &domain.SenderNameStatusHistory{
		ID:           r.ID,
		SenderNameID: r.SenderNameID,
		NewStatus:    r.NewStatus,
		ActorID:      r.ActorID,
		ActorType:    r.ActorType,
		CreatedAt:    r.CreatedAt,
	}
	if r.OldStatus.Valid {
		s := r.OldStatus.String
		h.OldStatus = &s
	}
	if r.Comment.Valid {
		h.Comment = r.Comment.String
	}
	return h
}

func (r *SenderNameRepository) AddHistoryEntry(ctx context.Context, entry *domain.SenderNameStatusHistory) error {
	const q = `
		INSERT INTO sender_name_status_history
		    (id, sender_name_id, old_status, new_status, actor_id, actor_type, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`

	var oldStatus sql.NullString
	if entry.OldStatus != nil {
		oldStatus = sql.NullString{String: *entry.OldStatus, Valid: true}
	}
	var comment sql.NullString
	if entry.Comment != "" {
		comment = sql.NullString{String: entry.Comment, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, q,
		entry.ID, entry.SenderNameID, oldStatus, entry.NewStatus,
		entry.ActorID, entry.ActorType, comment)
	if err != nil {
		return fmt.Errorf("add history entry: %w", err)
	}
	return nil
}

func (r *SenderNameRepository) GetHistory(ctx context.Context, senderNameID uuid.UUID, limit, offset int) ([]*domain.SenderNameStatusHistory, int, error) {
	var total int
	if err := r.db.QueryRowxContext(ctx,
		"SELECT COUNT(*) FROM sender_name_status_history WHERE sender_name_id = $1", senderNameID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count history: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}
	const q = `
		SELECT id, sender_name_id, old_status, new_status, actor_id, actor_type, comment, created_at
		FROM sender_name_status_history
		WHERE sender_name_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, q, senderNameID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var result []*domain.SenderNameStatusHistory
	for rows.Next() {
		var row senderNameHistoryRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("scan history: %w", err)
		}
		result = append(result, row.toDomain())
	}
	return result, total, nil
}
