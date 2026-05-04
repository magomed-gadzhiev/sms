package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubAccountRoutingAssignment — строка subaccount_routing_assignment для одного суб-аккаунта.
type SubAccountRoutingAssignment struct {
	ClientID      uuid.UUID
	ProviderSetID *uuid.UUID
	RouteSetID    *uuid.UUID
	AssignedAt    time.Time
}

// SubAccountRoutingAssignmentRepository управляет назначениями routing для суб-аккаунтов.
type SubAccountRoutingAssignmentRepository struct {
	pool *pgxpool.Pool
}

// NewSubAccountRoutingAssignmentRepository конструирует репозиторий.
func NewSubAccountRoutingAssignmentRepository(pool *pgxpool.Pool) *SubAccountRoutingAssignmentRepository {
	return &SubAccountRoutingAssignmentRepository{pool: pool}
}

// GetByClient возвращает назначение для суб-аккаунта.
// Возвращает ErrNotFound, если запись отсутствует.
func (r *SubAccountRoutingAssignmentRepository) GetByClient(ctx context.Context, clientID uuid.UUID) (*SubAccountRoutingAssignment, error) {
	var a SubAccountRoutingAssignment
	err := r.pool.QueryRow(ctx,
		`SELECT client_id, provider_set_id, route_set_id, assigned_at
		 FROM subaccount_routing_assignment WHERE client_id = $1`, clientID,
	).Scan(&a.ClientID, &a.ProviderSetID, &a.RouteSetID, &a.AssignedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

// Upsert выполняет INSERT ... ON CONFLICT DO UPDATE для назначения суб-аккаунта.
// Передача nil для providerSetID или routeSetID запишет NULL в БД.
func (r *SubAccountRoutingAssignmentRepository) Upsert(ctx context.Context, clientID uuid.UUID, providerSetID, routeSetID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (client_id) DO UPDATE SET
		   provider_set_id = EXCLUDED.provider_set_id,
		   route_set_id    = EXCLUDED.route_set_id,
		   assigned_at     = now()`,
		clientID, providerSetID, routeSetID)
	return err
}

// ListClientsByProviderSet возвращает все client_id, назначенные на указанный provider-set.
func (r *SubAccountRoutingAssignmentRepository) ListClientsByProviderSet(ctx context.Context, providerSetID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT client_id FROM subaccount_routing_assignment WHERE provider_set_id = $1`, providerSetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListByReseller возвращает все назначения суб-аккаунтов заданного агрегатора.
// Фильтрует через JOIN с clients по parent_client_id.
func (r *SubAccountRoutingAssignmentRepository) ListByReseller(ctx context.Context, resellerID uuid.UUID) ([]SubAccountRoutingAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT sra.client_id, sra.provider_set_id, sra.route_set_id, sra.assigned_at
		 FROM subaccount_routing_assignment sra
		 JOIN clients c ON c.id = sra.client_id
		 WHERE c.parent_client_id = $1`, resellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubAccountRoutingAssignment
	for rows.Next() {
		var a SubAccountRoutingAssignment
		if err := rows.Scan(&a.ClientID, &a.ProviderSetID, &a.RouteSetID, &a.AssignedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
