package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

type OperatorSupportRepo struct {
	pool *pgxpool.Pool
}

func NewOperatorSupportRepo(pool *pgxpool.Pool) *OperatorSupportRepo {
	return &OperatorSupportRepo{pool: pool}
}

func NewOperatorSupportRepository(pool *pgxpool.Pool) *OperatorSupportRepo {
	return NewOperatorSupportRepo(pool)
}

func (r *OperatorSupportRepo) List(ctx context.Context, operatorID *uuid.UUID) ([]*domain.OperatorChannelSupport, error) {
	query := `SELECT ocs.id, ocs.operator_id, ocs.channel_type, ocs.supported, ocs.notes, ocs.updated_at, ocs.updated_by
		FROM operator_channel_support ocs`
	args := []interface{}{}

	if operatorID != nil {
		query += " WHERE ocs.operator_id = $1"
		args = append(args, *operatorID)
	}
	query += " ORDER BY ocs.operator_id, ocs.channel_type"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list operator channel support: %w", err)
	}
	defer rows.Close()

	var result []*domain.OperatorChannelSupport
	for rows.Next() {
		ocs, err := r.scanOCS(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ocs row: %w", err)
		}
		result = append(result, ocs)
	}
	return result, nil
}

func (r *OperatorSupportRepo) GetSupport(ctx context.Context, operatorID uuid.UUID, channelType domain.ChannelType) (*domain.OperatorChannelSupport, error) {
	query := `SELECT id, operator_id, channel_type, supported, notes, updated_at, updated_by
		FROM operator_channel_support WHERE operator_id = $1 AND channel_type = $2`

	ocs := &domain.OperatorChannelSupport{}
	var id uuid.UUID
	var notes *string
	var updatedBy *uuid.UUID
	err := r.pool.QueryRow(ctx, query, operatorID, string(channelType)).Scan(
		&id, &ocs.OperatorID, &ocs.ChannelType, &ocs.Supported, &notes, &ocs.UpdatedAt, &updatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not found — assume supported by default (SMS is always supported)
		return &domain.OperatorChannelSupport{
			OperatorID:  operatorID,
			ChannelType: channelType,
			Supported:   channelType == domain.ChannelSMS, // SMS supported by default
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get operator channel support: %w", err)
	}
	if notes != nil {
		ocs.Notes = *notes
	}
	ocs.UpdatedBy = updatedBy
	return ocs, nil
}

func (r *OperatorSupportRepo) Upsert(ctx context.Context, ocs *domain.OperatorChannelSupport) error {
	ocs.UpdatedAt = time.Now()

	query := `INSERT INTO operator_channel_support
		(id, operator_id, channel_type, supported, notes, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (operator_id, channel_type) DO UPDATE SET
		supported = EXCLUDED.supported,
		notes = EXCLUDED.notes,
		updated_at = EXCLUDED.updated_at,
		updated_by = EXCLUDED.updated_by`

	id := uuid.New()
	_, err := r.pool.Exec(ctx, query,
		id, ocs.OperatorID, string(ocs.ChannelType), ocs.Supported,
		nullString(ocs.Notes), ocs.UpdatedAt, ocs.UpdatedBy,
	)
	if err != nil {
		return fmt.Errorf("upsert operator channel support: %w", err)
	}
	return nil
}

func (r *OperatorSupportRepo) GetOperatorIDByPrefix(ctx context.Context, msisdn string) (uuid.UUID, error) {
	// Normalize MSISDN: strip leading +
	normalized := msisdn
	if len(normalized) > 0 && normalized[0] == '+' {
		normalized = normalized[1:]
	}

	// Match longest prefix in operator_prefixes
	query := `SELECT operator_id FROM operator_prefixes
		WHERE active = true AND $1 LIKE prefix || '%'
		ORDER BY length(prefix) DESC
		LIMIT 1`

	var operatorID uuid.UUID
	err := r.pool.QueryRow(ctx, query, normalized).Scan(&operatorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get operator by prefix: %w", err)
	}
	return operatorID, nil
}

func (r *OperatorSupportRepo) scanOCS(rows pgx.Rows) (*domain.OperatorChannelSupport, error) {
	ocs := &domain.OperatorChannelSupport{}
	var id uuid.UUID
	var notes *string
	var updatedBy *uuid.UUID
	if err := rows.Scan(&id, &ocs.OperatorID, &ocs.ChannelType, &ocs.Supported, &notes, &ocs.UpdatedAt, &updatedBy); err != nil {
		return nil, err
	}
	if notes != nil {
		ocs.Notes = *notes
	}
	ocs.UpdatedBy = updatedBy
	return ocs, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
