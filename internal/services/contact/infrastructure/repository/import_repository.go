package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

// ImportRepository handles CRUD for contact_imports.
type ImportRepository struct {
	db *sqlx.DB
}

// NewImportRepository creates a new ImportRepository.
func NewImportRepository(db *sqlx.DB) *ImportRepository {
	return &ImportRepository{db: db}
}

type importRow struct {
	ID            uuid.UUID      `db:"id"`
	ContactListID uuid.UUID      `db:"contact_list_id"`
	ClientID      uuid.UUID      `db:"client_id"`
	FileName      string         `db:"file_name"`
	FileSize      int64          `db:"file_size"`
	Status        string         `db:"status"`
	TotalRows     int32          `db:"total_rows"`
	ImportedCount int32          `db:"imported_count"`
	UpdatedCount  int32          `db:"updated_count"`
	ErrorCount    int32          `db:"error_count"`
	Errors        []byte         `db:"errors"`
	ColumnMapping []byte         `db:"column_mapping"`
	CreatedAt     sql.NullTime   `db:"created_at"`
	CompletedAt   sql.NullTime   `db:"completed_at"`
}

func (r *importRow) toDomain() (*domain.ImportJob, error) {
	job := &domain.ImportJob{
		ID:            r.ID,
		ContactListID: r.ContactListID,
		ClientID:      r.ClientID,
		FileName:      r.FileName,
		FileSize:      r.FileSize,
		Status:        r.Status,
		TotalRows:     r.TotalRows,
		ImportedCount: r.ImportedCount,
		UpdatedCount:  r.UpdatedCount,
		ErrorCount:    r.ErrorCount,
	}

	if r.Errors != nil && len(r.Errors) > 0 {
		var errs []domain.ImportError
		if err := json.Unmarshal(r.Errors, &errs); err != nil {
			// Not critical, just leave empty
			job.Errors = nil
		} else {
			job.Errors = errs
		}
	}

	if r.ColumnMapping != nil && len(r.ColumnMapping) > 0 {
		var mapping []domain.ColumnMapping
		if err := json.Unmarshal(r.ColumnMapping, &mapping); err != nil {
			job.ColumnMapping = nil
		} else {
			job.ColumnMapping = mapping
		}
	}

	if r.CreatedAt.Valid {
		job.CreatedAt = r.CreatedAt.Time
	}
	if r.CompletedAt.Valid {
		t := r.CompletedAt.Time
		job.CompletedAt = &t
	}
	return job, nil
}

// Create inserts a new import job.
func (r *ImportRepository) Create(ctx context.Context, job *domain.ImportJob) (*domain.ImportJob, error) {
	mappingJSON, err := json.Marshal(job.ColumnMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal column mapping: %w", err)
	}

	query := `INSERT INTO contact_imports (id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping, created_at, completed_at`

	var errorsJSON []byte
	if job.Errors != nil {
		errorsJSON, _ = json.Marshal(job.Errors)
	} else {
		errorsJSON = []byte("[]")
	}

	var row importRow
	err = r.db.QueryRowxContext(ctx, query,
		job.ID, job.ContactListID, job.ClientID, job.FileName, job.FileSize,
		job.Status, job.TotalRows, job.ImportedCount, job.UpdatedCount, job.ErrorCount,
		string(errorsJSON), string(mappingJSON),
	).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to create import job: %w", err)
	}
	return row.toDomain()
}

// GetByID retrieves an import job by ID.
func (r *ImportRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	query := `SELECT id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping, created_at, completed_at
		FROM contact_imports WHERE id = $1`

	var row importRow
	err := r.db.QueryRowxContext(ctx, query, id).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrImportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get import job: %w", err)
	}
	return row.toDomain()
}

// List retrieves import jobs for a contact list with pagination.
func (r *ImportRepository) List(ctx context.Context, contactListID, clientID uuid.UUID, limit, offset int) ([]*domain.ImportJob, int, error) {
	countQuery := `SELECT COUNT(*) FROM contact_imports WHERE contact_list_id = $1 AND client_id = $2`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, contactListID, clientID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count import jobs: %w", err)
	}

	listQuery := `SELECT id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping, created_at, completed_at
		FROM contact_imports WHERE contact_list_id = $1 AND client_id = $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`

	rows, err := r.db.QueryxContext(ctx, listQuery, contactListID, clientID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list import jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.ImportJob
	for rows.Next() {
		var row importRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan import job: %w", err)
		}
		job, err := row.toDomain()
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, nil
}

// UpdateStatus updates the status and counters of an import job.
func (r *ImportRepository) UpdateStatus(ctx context.Context, job *domain.ImportJob) error {
	errorsJSON, _ := json.Marshal(job.Errors)

	query := `UPDATE contact_imports SET
		status = $1,
		total_rows = $2,
		imported_count = $3,
		updated_count = $4,
		error_count = $5,
		errors = $6,
		completed_at = $7
		WHERE id = $8`

	var completedAt sql.NullTime
	if job.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *job.CompletedAt, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		job.Status, job.TotalRows, job.ImportedCount, job.UpdatedCount,
		job.ErrorCount, string(errorsJSON), completedAt, job.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update import status: %w", err)
	}
	return nil
}
