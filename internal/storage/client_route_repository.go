package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/shared"
)

type ClientRouteRepository struct {
	db *sqlx.DB
}

func NewClientRouteRepository(db *DB) *ClientRouteRepository {
	return &ClientRouteRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *ClientRouteRepository) ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error) {
	var routes []*shared.ClientRoute
	err := r.db.SelectContext(ctx, &routes, `
		SELECT id, client_id, operator_id, provider_id, priority, weight, active, shared, created_at, updated_at
		FROM client_routes
		WHERE client_id = $1 AND operator_id = $2 AND active = true
		ORDER BY priority DESC`, clientID, operatorID)
	return routes, err
}

func (r *ClientRouteRepository) ListSharedByClientAndOperator(ctx context.Context, parentClientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error) {
	var routes []*shared.ClientRoute
	err := r.db.SelectContext(ctx, &routes, `
		SELECT id, client_id, operator_id, provider_id, priority, weight, active, shared, created_at, updated_at
		FROM client_routes
		WHERE client_id = $1 AND operator_id = $2 AND active = true AND shared = true
		ORDER BY priority DESC`, parentClientID, operatorID)
	return routes, err
}

func (r *ClientRouteRepository) ListDefaultByOperator(ctx context.Context, operatorID uuid.UUID) ([]*shared.ClientRoute, error) {
	var routes []*shared.ClientRoute
	err := r.db.SelectContext(ctx, &routes, `
		SELECT id, client_id, operator_id, provider_id, priority, weight, active, shared, created_at, updated_at
		FROM client_routes
		WHERE client_id IS NULL AND operator_id = $1 AND active = true
		ORDER BY priority DESC`, operatorID)
	return routes, err
}

func (r *ClientRouteRepository) GetParentClientID(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, error) {
	var parentID uuid.NullUUID
	err := r.db.QueryRowContext(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1`, clientID,
	).Scan(&parentID)
	if err != nil {
		return nil, err
	}
	if !parentID.Valid {
		return nil, nil
	}
	v := parentID.UUID
	return &v, nil
}

func (r *ClientRouteRepository) GetRoutingMode(ctx context.Context, clientID uuid.UUID) (string, error) {
	var mode string
	err := r.db.QueryRowContext(ctx,
		`SELECT routing_mode FROM clients WHERE id = $1`, clientID,
	).Scan(&mode)
	if err != nil {
		return "hybrid", err
	}
	return mode, nil
}
