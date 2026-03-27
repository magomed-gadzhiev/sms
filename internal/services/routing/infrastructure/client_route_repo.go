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

type ClientRouteRepo struct {
	pool *pgxpool.Pool
}

func NewClientRouteRepo(pool *pgxpool.Pool) *ClientRouteRepo {
	return &ClientRouteRepo{pool: pool}
}

func (r *ClientRouteRepo) Create(ctx context.Context, route *domain.ClientRoute) error {
	query := `INSERT INTO client_routes (id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.pool.Exec(ctx, query,
		route.ID, route.ClientID, route.OperatorID, route.ProviderID,
		route.Priority, route.Weight, route.Active, route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create client_route: %w", err)
	}
	return nil
}

func (r *ClientRouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE id = $1`
	route := &domain.ClientRoute{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
		&route.Priority, &route.Weight, &route.Active, &route.CreatedAt, &route.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrClientRouteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get client_route: %w", err)
	}
	return route, nil
}

func (r *ClientRouteRepo) ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID, activeOnly bool) ([]*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE client_id = $1 AND operator_id = $2`
	if activeOnly {
		query += " AND active = true"
	}
	query += " ORDER BY priority DESC, weight DESC"
	return r.scanMany(ctx, query, clientID, operatorID)
}

func (r *ClientRouteRepo) ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE client_id = $1 ORDER BY operator_id, priority DESC`
	return r.scanMany(ctx, query, clientID)
}

func (r *ClientRouteRepo) Update(ctx context.Context, route *domain.ClientRoute) error {
	query := `UPDATE client_routes SET priority = $1, weight = $2, active = $3 WHERE id = $4`
	tag, err := r.pool.Exec(ctx, query, route.Priority, route.Weight, route.Active, route.ID)
	if err != nil {
		return fmt.Errorf("update client_route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}
	return nil
}

func (r *ClientRouteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM client_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete client_route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}
	return nil
}

func (r *ClientRouteRepo) scanMany(ctx context.Context, query string, args ...interface{}) ([]*domain.ClientRoute, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query client_routes: %w", err)
	}
	defer rows.Close()

	var result []*domain.ClientRoute
	for rows.Next() {
		route := &domain.ClientRoute{}
		if err := rows.Scan(
			&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
			&route.Priority, &route.Weight, &route.Active, &route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan client_route row: %w", err)
		}
		result = append(result, route)
	}
	return result, nil
}
