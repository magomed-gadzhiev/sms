package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type ClientProviderRepo struct {
	pool *pgxpool.Pool
}

func NewClientProviderRepo(pool *pgxpool.Pool) *ClientProviderRepo {
	return &ClientProviderRepo{pool: pool}
}

func (r *ClientProviderRepo) Create(ctx context.Context, cp *domain.ClientProvider) error {
	query := `INSERT INTO client_providers (id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.pool.Exec(ctx, query,
		cp.ID, cp.ClientID, cp.ProviderID, cp.Ownership, cp.SourceClientID,
		cp.SharedPriority, cp.ExposeCost, cp.ExposeProviderName, cp.Active,
		cp.CreatedAt, cp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create client_provider: %w", err)
	}
	return nil
}

func (r *ClientProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE id = $1`
	return r.scanOne(ctx, query, id)
}

func (r *ClientProviderRepo) GetByClientAndProvider(ctx context.Context, clientID, providerID uuid.UUID) (*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE client_id = $1 AND provider_id = $2`
	return r.scanOne(ctx, query, clientID, providerID)
}

func (r *ClientProviderRepo) ListByClient(ctx context.Context, clientID uuid.UUID, activeOnly bool) ([]*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE client_id = $1`
	if activeOnly {
		query += " AND active = true"
	}
	query += " ORDER BY shared_priority DESC"
	return r.scanMany(ctx, query, clientID)
}

func (r *ClientProviderRepo) ListBySourceClient(ctx context.Context, sourceClientID uuid.UUID) ([]*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE source_client_id = $1 ORDER BY created_at`
	return r.scanMany(ctx, query, sourceClientID)
}

func (r *ClientProviderRepo) Update(ctx context.Context, cp *domain.ClientProvider) error {
	query := `UPDATE client_providers SET shared_priority = $1, expose_cost = $2, expose_provider_name = $3, active = $4
		WHERE id = $5`
	tag, err := r.pool.Exec(ctx, query, cp.SharedPriority, cp.ExposeCost, cp.ExposeProviderName, cp.Active, cp.ID)
	if err != nil {
		return fmt.Errorf("update client_provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientProviderNotFound
	}
	return nil
}

func (r *ClientProviderRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM client_providers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete client_provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientProviderNotFound
	}
	return nil
}

func (r *ClientProviderRepo) scanOne(ctx context.Context, query string, args ...interface{}) (*domain.ClientProvider, error) {
	cp := &domain.ClientProvider{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&cp.ID, &cp.ClientID, &cp.ProviderID, &cp.Ownership, &cp.SourceClientID,
		&cp.SharedPriority, &cp.ExposeCost, &cp.ExposeProviderName, &cp.Active,
		&cp.CreatedAt, &cp.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrClientProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan client_provider: %w", err)
	}
	return cp, nil
}

func (r *ClientProviderRepo) scanMany(ctx context.Context, query string, args ...interface{}) ([]*domain.ClientProvider, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query client_providers: %w", err)
	}
	defer rows.Close()

	var result []*domain.ClientProvider
	for rows.Next() {
		cp := &domain.ClientProvider{}
		if err := rows.Scan(
			&cp.ID, &cp.ClientID, &cp.ProviderID, &cp.Ownership, &cp.SourceClientID,
			&cp.SharedPriority, &cp.ExposeCost, &cp.ExposeProviderName, &cp.Active,
			&cp.CreatedAt, &cp.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan client_provider row: %w", err)
		}
		result = append(result, cp)
	}
	return result, nil
}
