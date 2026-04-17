package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// ExportRepo implements domain.ExportRepository using PostgreSQL.
type ExportRepo struct {
	db *pgxpool.Pool
}

// NewExportRepo creates a new ExportRepo.
func NewExportRepo(db *pgxpool.Pool) *ExportRepo {
	return &ExportRepo{db: db}
}

// CreateJob inserts a new export job record.
func (r *ExportRepo) CreateJob(ctx context.Context, job *domain.ExportJob) error {
	job.CreatedAt = time.Now()
	if job.Status == "" {
		job.Status = "pending"
	}

	query := `
		INSERT INTO export_jobs (
			partner_id, user_id, mode, filters, format,
			status, file_path, row_count, error, created_at, completed_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11
		) RETURNING id`

	err := r.db.QueryRow(ctx, query,
		job.PartnerID, job.UserID, job.Mode, job.Filters, job.Format,
		job.Status, job.FilePath, job.RowCount, job.Error, job.CreatedAt, job.CompletedAt,
	).Scan(&job.ID)
	if err != nil {
		return fmt.Errorf("create export job: %w", err)
	}
	return nil
}

// GetJob retrieves an export job by ID.
func (r *ExportRepo) GetJob(ctx context.Context, jobID string) (*domain.ExportJob, error) {
	query := `
		SELECT id, partner_id, user_id, mode, filters, format,
			status, file_path, row_count, error, created_at, completed_at
		FROM export_jobs
		WHERE id = $1`

	var job domain.ExportJob
	err := r.db.QueryRow(ctx, query, jobID).Scan(
		&job.ID, &job.PartnerID, &job.UserID, &job.Mode, &job.Filters, &job.Format,
		&job.Status, &job.FilePath, &job.RowCount, &job.Error, &job.CreatedAt, &job.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get export job %s: %w", jobID, err)
	}
	return &job, nil
}

// UpdateJob updates the mutable fields of an export job.
func (r *ExportRepo) UpdateJob(ctx context.Context, job *domain.ExportJob) error {
	query := `
		UPDATE export_jobs SET
			status       = $1,
			file_path    = $2,
			row_count    = $3,
			error        = $4,
			completed_at = $5
		WHERE id = $6`

	tag, err := r.db.Exec(ctx, query,
		job.Status, job.FilePath, job.RowCount, job.Error, job.CompletedAt,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("update export job %s: %w", job.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("export job %s not found", job.ID)
	}
	return nil
}
