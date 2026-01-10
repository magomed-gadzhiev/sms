package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// ReportRepository реализует domain.ReportRepository
type ReportRepository struct {
	db *sqlx.DB
}

// NewReportRepository создает новый репозиторий отчетов
func NewReportRepository(db *sqlx.DB) *ReportRepository {
	return &ReportRepository{
		db: db,
	}
}

// Create сохраняет отчет
func (r *ReportRepository) Create(ctx context.Context, report *domain.Report) error {
	query := `
		INSERT INTO reports (
			id, report_type, format, client_id, provider_id,
			period_start, period_end, data, generated_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		report.ID,
		string(report.Type),
		string(report.Format),
		report.ClientID,
		report.ProviderID,
		report.PeriodStart,
		report.PeriodEnd,
		report.Data,
		report.GeneratedAt,
		report.CreatedAt,
	)

	return err
}

// GetByID получает отчет по ID
func (r *ReportRepository) GetByID(ctx context.Context, reportID uuid.UUID) (*domain.Report, error) {
	query := `
		SELECT id, report_type, format, client_id, provider_id,
		       period_start, period_end, data, generated_at, created_at
		FROM reports
		WHERE id = $1
	`

	var report domain.Report
	err := r.db.GetContext(ctx, &report, query, reportID)
	if err != nil {
		return nil, err
	}

	return &report, nil
}

// List получает список отчетов
func (r *ReportRepository) List(ctx context.Context, filters *domain.ReportFilters) ([]*domain.Report, error) {
	query := `
		SELECT id, report_type, format, client_id, provider_id,
		       period_start, period_end, data, generated_at, created_at
		FROM reports
		WHERE 1=1
	`

	args := []interface{}{}
	argIndex := 1

	if filters.ReportType != nil {
		query += fmt.Sprintf(" AND report_type = $%d", argIndex)
		args = append(args, string(*filters.ReportType))
		argIndex++
	}

	if filters.ClientID != nil {
		query += fmt.Sprintf(" AND client_id = $%d", argIndex)
		args = append(args, *filters.ClientID)
		argIndex++
	}

	if filters.ProviderID != nil {
		query += fmt.Sprintf(" AND provider_id = $%d", argIndex)
		args = append(args, *filters.ProviderID)
		argIndex++
	}

	if filters.From != nil {
		query += fmt.Sprintf(" AND period_start >= $%d", argIndex)
		args = append(args, *filters.From)
		argIndex++
	}

	if filters.To != nil {
		query += fmt.Sprintf(" AND period_end <= $%d", argIndex)
		args = append(args, *filters.To)
		argIndex++
	}

	limit := filters.Limit
	if limit == 0 {
		limit = 100
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", limit)

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	var reports []*domain.Report
	err := r.db.SelectContext(ctx, &reports, query, args...)
	if err != nil {
		return nil, err
	}

	return reports, nil
}
