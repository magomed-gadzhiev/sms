package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// LookupLogRepository implements domain.LookupLogRepository
type LookupLogRepository struct {
	db *sqlx.DB
}

// NewLookupLogRepository creates a new lookup log repository
func NewLookupLogRepository(db *sqlx.DB) *LookupLogRepository {
	return &LookupLogRepository{db: db}
}

func (r *LookupLogRepository) Insert(ctx context.Context, entry *domain.LookupLogEntry) error {
	query := `
		INSERT INTO lookup_log (id, msisdn, operator_mccmnc, operator_name, number_status,
			country_code, number_type, is_ported, hlr_provider_id, source, client_id,
			cached, latency_ms, request_id, message_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err := r.db.ExecContext(ctx, query,
		entry.ID, entry.MSISDN, entry.OperatorMCCMNC, entry.OperatorName,
		entry.NumberStatus, entry.CountryCode, entry.NumberType, entry.IsPorted,
		entry.HLRProviderID, string(entry.Source), entry.ClientID,
		entry.Cached, entry.LatencyMs, entry.RequestID, entry.MessageID, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка записи в lookup_log: %w", err)
	}
	return nil
}

func (r *LookupLogRepository) ListByClient(ctx context.Context, clientID uuid.UUID, from, to *time.Time, msisdnFilter, sourceFilter string, page, pageSize int) ([]*domain.LookupLogEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("client_id = $%d", argIdx))
	args = append(args, clientID)
	argIdx++

	if from != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *to)
		argIdx++
	}
	if msisdnFilter != "" {
		conditions = append(conditions, fmt.Sprintf("msisdn = $%d", argIdx))
		args = append(args, msisdnFilter)
		argIdx++
	}
	if sourceFilter != "" {
		conditions = append(conditions, fmt.Sprintf("source = $%d", argIdx))
		args = append(args, sourceFilter)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM lookup_log WHERE %s", where)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчёта записей: %w", err)
	}

	// Data query
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(`
		SELECT id, msisdn, operator_mccmnc, operator_name, number_status,
			country_code, number_type, is_ported, hlr_provider_id, source,
			client_id, cached, latency_ms, request_id, message_id, created_at
		FROM lookup_log
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryxContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения записей lookup_log: %w", err)
	}
	defer rows.Close()

	var entries []*domain.LookupLogEntry
	for rows.Next() {
		var e domain.LookupLogEntry
		if err := rows.StructScan(&e); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования записи: %w", err)
		}
		entries = append(entries, &e)
	}

	return entries, total, nil
}

func (r *LookupLogRepository) CountByClient(ctx context.Context, clientID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM lookup_log WHERE client_id = $1", clientID)
	if err != nil {
		return 0, fmt.Errorf("ошибка подсчёта записей: %w", err)
	}
	return count, nil
}
