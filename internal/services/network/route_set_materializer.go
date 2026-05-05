package network

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// RouteSetMaterializer материализует route-set агрегатора в client_routes суб-аккаунта.
// При применении: записи source='template' удаляются (CASCADE на условия/расписания) и пересоздаются.
// Записи source='override' не затрагиваются.
type RouteSetMaterializer struct {
	pool      *pgxpool.Pool
	itemsRepo *storage.ResellerRouteSetItemsRepository
}

// NewRouteSetMaterializer конструирует сервис.
func NewRouteSetMaterializer(pool *pgxpool.Pool, itemsRepo *storage.ResellerRouteSetItemsRepository) *RouteSetMaterializer {
	return &RouteSetMaterializer{pool: pool, itemsRepo: itemsRepo}
}

// ApplyToClient — материализует route-set в client_routes под транзакцией.
// routeSetID == nil → стереть только source='template' (без вставки новых).
// Записи source='override' не затрагиваются в обоих случаях.
func (m *RouteSetMaterializer) ApplyToClient(ctx context.Context, clientID uuid.UUID, routeSetID *uuid.UUID) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Шаг 1: удалить template-записи. CASCADE на route_condition_groups + route_conditions + route_schedules.
	if _, err := tx.Exec(ctx,
		`DELETE FROM client_routes WHERE client_id = $1 AND source = 'template'`,
		clientID,
	); err != nil {
		return err
	}

	// Шаг 2: если route-set указан — загрузить items + копировать в client_routes.
	if routeSetID != nil {
		items, err := m.loadItemsTx(ctx, tx, *routeSetID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := insertRouteFromItem(ctx, tx, clientID, item); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// loadItemsTx — загружает items + condition groups + schedules внутри одной транзакции.
// Дублирует логику ListBySet, но работает через tx (важно для read-after-write consistency).
func (m *RouteSetMaterializer) loadItemsTx(ctx context.Context, tx pgx.Tx, setID uuid.UUID) ([]storage.RouteSetItemFull, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, set_id, COALESCE(name,''), COALESCE(comment,''), provider_id,
		        priority, share, route_type, status
		 FROM reseller_route_set_items WHERE set_id = $1 ORDER BY priority DESC`, setID)
	if err != nil {
		return nil, err
	}
	var items []storage.RouteSetItemFull
	for rows.Next() {
		var i storage.RouteSetItemFull
		if err := rows.Scan(&i.ID, &i.SetID, &i.Name, &i.Comment, &i.ProviderID,
			&i.Priority, &i.Share, &i.RouteType, &i.Status); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for idx := range items {
		groups, err := loadGroupsTx(ctx, tx, items[idx].ID)
		if err != nil {
			return nil, err
		}
		items[idx].ConditionGroups = groups
		schedules, err := loadSchedulesTx(ctx, tx, items[idx].ID)
		if err != nil {
			return nil, err
		}
		items[idx].Schedules = schedules
	}
	return items, nil
}

func loadGroupsTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) ([]storage.RouteSetConditionGroup, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, group_index, logic_op FROM route_set_condition_groups
		 WHERE item_id = $1 ORDER BY group_index`, itemID)
	if err != nil {
		return nil, err
	}
	type row struct {
		ID         int64
		GroupIndex int16
		LogicOp    string
	}
	var rl []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.ID, &rr.GroupIndex, &rr.LogicOp); err != nil {
			rows.Close()
			return nil, err
		}
		rl = append(rl, rr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]storage.RouteSetConditionGroup, 0, len(rl))
	for _, g := range rl {
		crows, err := tx.Query(ctx,
			`SELECT condition_type, condition_value FROM route_set_conditions
			 WHERE group_id = $1 ORDER BY id`, g.ID)
		if err != nil {
			return nil, err
		}
		var conds []storage.RouteSetCondition
		for crows.Next() {
			var c storage.RouteSetCondition
			if err := crows.Scan(&c.Type, &c.Value); err != nil {
				crows.Close()
				return nil, err
			}
			conds = append(conds, c)
		}
		crows.Close()
		if err := crows.Err(); err != nil {
			return nil, err
		}
		out = append(out, storage.RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp, Conditions: conds})
	}
	return out, nil
}

func loadSchedulesTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) ([]storage.RouteSetSchedule, error) {
	rows, err := tx.Query(ctx,
		`SELECT date_from, date_to, time_from::text, time_to::text, weekdays, timezone
		 FROM route_set_schedules WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.RouteSetSchedule
	for rows.Next() {
		var s storage.RouteSetSchedule
		if err := rows.Scan(&s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// insertRouteFromItem — копирует один item в client_routes + condition_groups + conditions + schedules.
// client_routes.operator_id оставлен NULL (миграция 080 разрешает NULL); матчинг идёт через condition_groups.
func insertRouteFromItem(ctx context.Context, tx pgx.Tx, clientID uuid.UUID, item storage.RouteSetItemFull) error {
	var routeID uuid.UUID
	err := tx.QueryRow(ctx,
		`INSERT INTO client_routes
			(client_id, operator_id, provider_id, priority, weight, active,
			 name, comment, status, share, route_type, source)
		 VALUES ($1, NULL, $2, $3, 1, true, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, 'template')
		 RETURNING id`,
		clientID, item.ProviderID, item.Priority, item.Name, item.Comment,
		item.Status, item.Share, item.RouteType,
	).Scan(&routeID)
	if err != nil {
		return err
	}

	for idx, g := range item.ConditionGroups {
		var gid int64
		err := tx.QueryRow(ctx,
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			routeID, int16(idx), g.LogicOp,
		).Scan(&gid)
		if err != nil {
			return err
		}
		for _, c := range g.Conditions {
			if _, err := tx.Exec(ctx,
				`INSERT INTO route_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, gid, c.Type, c.Value); err != nil {
				return err
			}
		}
	}
	for _, s := range item.Schedules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			return err
		}
	}
	return nil
}

// ApplyToAllSubscribers — пересчитать материализацию для всех подписанных на route-set.
// Per-client транзакции (как в ProviderSetMaterializer); partial failure возможен.
func (m *RouteSetMaterializer) ApplyToAllSubscribers(ctx context.Context, routeSetID uuid.UUID) error {
	rows, err := m.pool.Query(ctx,
		`SELECT client_id FROM subaccount_routing_assignment WHERE route_set_id = $1`,
		routeSetID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	var clientIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		clientIDs = append(clientIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, cid := range clientIDs {
		if err := m.ApplyToClient(ctx, cid, &routeSetID); err != nil {
			return err
		}
	}
	return nil
}
