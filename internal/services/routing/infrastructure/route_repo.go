// internal/services/routing/infrastructure/route_repo.go
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type RouteRepo struct {
	pool *pgxpool.Pool
}

func NewRouteRepo(pool *pgxpool.Pool) *RouteRepo {
	return &RouteRepo{pool: pool}
}

// Create inserts a route with its condition groups, conditions, and schedules in a single transaction.
func (r *RouteRepo) Create(ctx context.Context, route *domain.ClientRoute) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO client_routes (id, client_id, operator_id, provider_id, priority, weight, active, name, comment, status, share, route_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		route.ID, route.ClientID, route.OperatorID, route.ProviderID,
		route.Priority, route.Weight, route.Active,
		route.Name, route.Comment, route.Status, route.Share, route.RouteType,
		route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert route: %w", err)
	}

	if err := r.insertGroupsAndConditions(ctx, tx, route.ID, route.Groups); err != nil {
		return err
	}
	if err := r.insertSchedules(ctx, tx, route.ID, route.Schedules); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Update replaces the route and its children (full replace strategy).
func (r *RouteRepo) Update(ctx context.Context, route *domain.ClientRoute) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE client_routes SET client_id=$1, operator_id=$2, provider_id=$3, priority=$4, weight=$5, active=$6,
		name=$7, comment=$8, status=$9, share=$10, route_type=$11, updated_at=now()
		WHERE id=$12`,
		route.ClientID, route.OperatorID, route.ProviderID,
		route.Priority, route.Weight, route.Active,
		route.Name, route.Comment, route.Status, route.Share, route.RouteType,
		route.ID,
	)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}

	// Delete old children (CASCADE handles conditions via group FK)
	_, _ = tx.Exec(ctx, `DELETE FROM route_condition_groups WHERE route_id = $1`, route.ID)
	_, _ = tx.Exec(ctx, `DELETE FROM route_schedules WHERE route_id = $1`, route.ID)

	if err := r.insertGroupsAndConditions(ctx, tx, route.ID, route.Groups); err != nil {
		return err
	}
	if err := r.insertSchedules(ctx, tx, route.ID, route.Schedules); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID loads a route with all condition groups, conditions, and schedules.
func (r *RouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientRoute, error) {
	route := &domain.ClientRoute{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, operator_id, provider_id, priority, weight, active,
		 COALESCE(name, ''), COALESCE(comment, ''), status, share, route_type, created_at, updated_at
		 FROM client_routes WHERE id = $1`, id,
	).Scan(
		&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
		&route.Priority, &route.Weight, &route.Active,
		&route.Name, &route.Comment, &route.Status, &route.Share, &route.RouteType,
		&route.CreatedAt, &route.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrClientRouteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}

	groups, err := r.loadGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Groups = groups

	schedules, err := r.loadSchedules(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Schedules = schedules

	return route, nil
}

// List returns routes with optional filters. Does NOT load children (use GetByID for full detail).
func (r *RouteRepo) List(ctx context.Context, filters RouteFilters) ([]*domain.ClientRoute, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filters.ClientID != nil {
		where += fmt.Sprintf(" AND client_id = $%d", argIdx)
		args = append(args, *filters.ClientID)
		argIdx++
	} else if filters.DefaultOnly {
		where += " AND client_id IS NULL"
	}
	if filters.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.RouteType != "" {
		where += fmt.Sprintf(" AND route_type = $%d", argIdx)
		args = append(args, filters.RouteType)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM client_routes " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count routes: %w", err)
	}

	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active,
		COALESCE(name, ''), COALESCE(comment, ''), status, share, route_type, created_at, updated_at
		FROM client_routes ` + where + " ORDER BY priority ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.ClientRoute
	for rows.Next() {
		route := &domain.ClientRoute{}
		if err := rows.Scan(
			&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
			&route.Priority, &route.Weight, &route.Active,
			&route.Name, &route.Comment, &route.Status, &route.Share, &route.RouteType,
			&route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, route)
	}

	return routes, total, nil
}

// Delete removes a route (CASCADE deletes children).
func (r *RouteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM client_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}
	return nil
}

// LoadAllActive loads all active routes with full children for the RouteMatcher.
func (r *RouteRepo) LoadAllActive(ctx context.Context) ([]*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active,
		COALESCE(name, ''), COALESCE(comment, ''), status, share, route_type, created_at, updated_at
		FROM client_routes WHERE status = 'active' ORDER BY priority ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load active routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.ClientRoute
	for rows.Next() {
		route := &domain.ClientRoute{}
		if err := rows.Scan(
			&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
			&route.Priority, &route.Weight, &route.Active,
			&route.Name, &route.Comment, &route.Status, &route.Share, &route.RouteType,
			&route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, route)
	}

	// Load children for each route
	for _, route := range routes {
		groups, err := r.loadGroups(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		route.Groups = groups

		schedules, err := r.loadSchedules(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		route.Schedules = schedules
	}

	return routes, nil
}

// --- private helpers ---

func (r *RouteRepo) insertGroupsAndConditions(ctx context.Context, tx pgx.Tx, routeID uuid.UUID, groups []domain.ConditionGroup) error {
	for _, g := range groups {
		var groupID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op) VALUES ($1, $2, $3) RETURNING id`,
			routeID, g.GroupIndex, g.LogicOp,
		).Scan(&groupID)
		if err != nil {
			return fmt.Errorf("insert condition group: %w", err)
		}
		for _, c := range g.Conditions {
			_, err := tx.Exec(ctx,
				`INSERT INTO route_conditions (group_id, condition_type, condition_value) VALUES ($1, $2, $3)`,
				groupID, c.Type, c.Value,
			)
			if err != nil {
				return fmt.Errorf("insert condition: %w", err)
			}
		}
	}
	return nil
}

func (r *RouteRepo) insertSchedules(ctx context.Context, tx pgx.Tx, routeID uuid.UUID, schedules []domain.Schedule) error {
	for _, s := range schedules {
		_, err := tx.Exec(ctx,
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone,
		)
		if err != nil {
			return fmt.Errorf("insert schedule: %w", err)
		}
	}
	return nil
}

func (r *RouteRepo) loadGroups(ctx context.Context, routeID uuid.UUID) ([]domain.ConditionGroup, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, group_index, logic_op FROM route_condition_groups WHERE route_id = $1 ORDER BY group_index`, routeID)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	defer rows.Close()

	var groups []domain.ConditionGroup
	for rows.Next() {
		var g domain.ConditionGroup
		if err := rows.Scan(&g.ID, &g.GroupIndex, &g.LogicOp); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}

	for i := range groups {
		conds, err := r.loadConditions(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		groups[i].Conditions = conds
	}

	return groups, nil
}

func (r *RouteRepo) loadConditions(ctx context.Context, groupID int64) ([]domain.Condition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, condition_type, condition_value FROM route_conditions WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load conditions: %w", err)
	}
	defer rows.Close()

	var conds []domain.Condition
	for rows.Next() {
		var c domain.Condition
		if err := rows.Scan(&c.ID, &c.Type, &c.Value); err != nil {
			return nil, fmt.Errorf("scan condition: %w", err)
		}
		conds = append(conds, c)
	}
	return conds, nil
}

func (r *RouteRepo) loadSchedules(ctx context.Context, routeID uuid.UUID) ([]domain.Schedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, date_from, date_to, time_from::text, time_to::text, weekdays, timezone
		FROM route_schedules WHERE route_id = $1`, routeID)
	if err != nil {
		return nil, fmt.Errorf("load schedules: %w", err)
	}
	defer rows.Close()

	var scheds []domain.Schedule
	for rows.Next() {
		var s domain.Schedule
		if err := rows.Scan(&s.ID, &s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		scheds = append(scheds, s)
	}
	return scheds, nil
}

type RouteFilters struct {
	ClientID    *uuid.UUID
	DefaultOnly bool
	Status      string
	RouteType   string
}
