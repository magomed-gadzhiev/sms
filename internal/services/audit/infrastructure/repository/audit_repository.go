package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
)

// AuditRepository implements domain.AuditLogRepository using database/sql.
type AuditRepository struct {
	db *sql.DB
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// QueryAuditLog queries the audit_log table with the given filters and returns paginated results.
func (r *AuditRepository) QueryAuditLog(ctx context.Context, filters *domain.AuditLogFilters) ([]*domain.AuditLogEntry, int, error) {
	// Build WHERE clause
	where := "WHERE tenant_id = $1"
	args := []interface{}{filters.TenantID}
	argIdx := 2

	if filters.Action != "" {
		where += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, filters.Action)
		argIdx++
	}
	if filters.UserID != "" {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, filters.UserID)
		argIdx++
	}
	if filters.DateFrom != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}
	if filters.DateTo != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		argIdx++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_log %s", where)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		log.Error().Err(err).Msg("error counting audit log entries")
		return nil, 0, fmt.Errorf("audit: count: %w", err)
	}

	// Pagination
	page := filters.Page
	perPage := filters.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	// Query entries
	query := fmt.Sprintf(
		`SELECT id, tenant_id, user_id, action, resource_type, resource_id,
		        COALESCE(details, '{}') as details, COALESCE(ip_address, '') as ip_address, created_at
		 FROM audit_log %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, perPage, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("error querying audit log")
		return nil, 0, fmt.Errorf("audit: query: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditLogEntry
	for rows.Next() {
		e := &domain.AuditLogEntry{}
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.UserID, &e.Action,
			&e.ResourceType, &e.ResourceID, &e.Details,
			&e.IPAddress, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("audit: scan row: %w", err)
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit: rows iteration: %w", err)
	}

	return entries, total, nil
}
