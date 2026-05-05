package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RouteSetCondition struct {
	Type  string
	Value string
}

type RouteSetConditionGroup struct {
	GroupIndex int16
	LogicOp    string
	Conditions []RouteSetCondition
}

type RouteSetSchedule struct {
	DateFrom *time.Time
	DateTo   *time.Time
	TimeFrom *string
	TimeTo   *string
	Weekdays int16
	Timezone string
}

type RouteSetItemFull struct {
	ID              uuid.UUID
	SetID           uuid.UUID
	Name            string
	Comment         string
	ProviderID      uuid.UUID
	Priority        int
	Share           int
	RouteType       string
	Status          string
	ConditionGroups []RouteSetConditionGroup
	Schedules       []RouteSetSchedule
}

type RouteSetReorderEntry struct {
	ItemID   uuid.UUID
	Priority int
}

type ResellerRouteSetItemsRepository struct {
	pool *pgxpool.Pool
}

func NewResellerRouteSetItemsRepository(pool *pgxpool.Pool) *ResellerRouteSetItemsRepository {
	return &ResellerRouteSetItemsRepository{pool: pool}
}

func (r *ResellerRouteSetItemsRepository) ListBySet(ctx context.Context, setID uuid.UUID) ([]RouteSetItemFull, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, set_id, COALESCE(name,''), COALESCE(comment,''), provider_id,
		        priority, share, route_type, status
		 FROM reseller_route_set_items WHERE set_id = $1
		 ORDER BY priority DESC, name`, setID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []RouteSetItemFull
	for rows.Next() {
		var i RouteSetItemFull
		if err := rows.Scan(&i.ID, &i.SetID, &i.Name, &i.Comment, &i.ProviderID,
			&i.Priority, &i.Share, &i.RouteType, &i.Status); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil { return nil, err }
	for idx := range out {
		groups, err := r.loadGroups(ctx, out[idx].ID)
		if err != nil { return nil, err }
		out[idx].ConditionGroups = groups
		schedules, err := r.loadSchedules(ctx, out[idx].ID)
		if err != nil { return nil, err }
		out[idx].Schedules = schedules
	}
	return out, nil
}

func (r *ResellerRouteSetItemsRepository) LoadFullByItem(ctx context.Context, itemID uuid.UUID) (*RouteSetItemFull, error) {
	var i RouteSetItemFull
	err := r.pool.QueryRow(ctx,
		`SELECT id, set_id, COALESCE(name,''), COALESCE(comment,''), provider_id,
		        priority, share, route_type, status
		 FROM reseller_route_set_items WHERE id = $1`, itemID,
	).Scan(&i.ID, &i.SetID, &i.Name, &i.Comment, &i.ProviderID,
		&i.Priority, &i.Share, &i.RouteType, &i.Status)
	if errors.Is(err, pgx.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	groups, err := r.loadGroups(ctx, itemID)
	if err != nil { return nil, err }
	i.ConditionGroups = groups
	schedules, err := r.loadSchedules(ctx, itemID)
	if err != nil { return nil, err }
	i.Schedules = schedules
	return &i, nil
}

func (r *ResellerRouteSetItemsRepository) loadGroups(ctx context.Context, itemID uuid.UUID) ([]RouteSetConditionGroup, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, group_index, logic_op FROM route_set_condition_groups
		 WHERE item_id = $1 ORDER BY group_index`, itemID)
	if err != nil { return nil, err }
	defer rows.Close()
	type row struct { ID int64; GroupIndex int16; LogicOp string }
	var rowsList []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.ID, &rr.GroupIndex, &rr.LogicOp); err != nil { return nil, err }
		rowsList = append(rowsList, rr)
	}
	if err := rows.Err(); err != nil { return nil, err }
	out := make([]RouteSetConditionGroup, 0, len(rowsList))
	for _, g := range rowsList {
		conds, err := r.loadConditions(ctx, g.ID)
		if err != nil { return nil, err }
		out = append(out, RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp, Conditions: conds})
	}
	return out, nil
}

func (r *ResellerRouteSetItemsRepository) loadConditions(ctx context.Context, groupID int64) ([]RouteSetCondition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT condition_type, condition_value FROM route_set_conditions
		 WHERE group_id = $1 ORDER BY id`, groupID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []RouteSetCondition
	for rows.Next() {
		var c RouteSetCondition
		if err := rows.Scan(&c.Type, &c.Value); err != nil { return nil, err }
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ResellerRouteSetItemsRepository) loadSchedules(ctx context.Context, itemID uuid.UUID) ([]RouteSetSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT date_from, date_to, time_from::text, time_to::text, weekdays, timezone
		 FROM route_set_schedules WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []RouteSetSchedule
	for rows.Next() {
		var s RouteSetSchedule
		if err := rows.Scan(&s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ResellerRouteSetItemsRepository) CreateFull(ctx context.Context, setID uuid.UUID, in RouteSetItemFull) (*RouteSetItemFull, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx) //nolint:errcheck

	var id uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO reseller_route_set_items (set_id, name, comment, provider_id, priority, share, route_type, status)
		 VALUES ($1, NULLIF($2,''), NULLIF($3,''), $4, $5, $6, $7, $8) RETURNING id`,
		setID, in.Name, in.Comment, in.ProviderID, in.Priority, in.Share, in.RouteType, in.Status,
	).Scan(&id)
	if err != nil { return nil, err }
	if err := writeGroupsAndSchedules(ctx, tx, id, in.ConditionGroups, in.Schedules); err != nil { return nil, err }
	if err := tx.Commit(ctx); err != nil { return nil, err }
	in.ID = id
	in.SetID = setID
	return &in, nil
}

func (r *ResellerRouteSetItemsRepository) UpdateFull(ctx context.Context, itemID uuid.UUID, in RouteSetItemFull) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx,
		`UPDATE reseller_route_set_items
		 SET name=NULLIF($1,''), comment=NULLIF($2,''), provider_id=$3,
		     priority=$4, share=$5, route_type=$6, status=$7, updated_at=now()
		 WHERE id=$8`,
		in.Name, in.Comment, in.ProviderID, in.Priority, in.Share, in.RouteType, in.Status, itemID)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return ErrNotFound }
	if _, err := tx.Exec(ctx, `DELETE FROM route_set_condition_groups WHERE item_id = $1`, itemID); err != nil { return err }
	if _, err := tx.Exec(ctx, `DELETE FROM route_set_schedules WHERE item_id = $1`, itemID); err != nil { return err }
	if err := writeGroupsAndSchedules(ctx, tx, itemID, in.ConditionGroups, in.Schedules); err != nil { return err }
	return tx.Commit(ctx)
}

func writeGroupsAndSchedules(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, groups []RouteSetConditionGroup, schedules []RouteSetSchedule) error {
	for idx, g := range groups {
		var gid int64
		err := tx.QueryRow(ctx,
			`INSERT INTO route_set_condition_groups (item_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			itemID, int16(idx), g.LogicOp,
		).Scan(&gid)
		if err != nil { return err }
		for _, c := range g.Conditions {
			if _, err := tx.Exec(ctx,
				`INSERT INTO route_set_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`,
				gid, c.Type, c.Value); err != nil {
				return err
			}
		}
	}
	for _, s := range schedules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO route_set_schedules (item_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			itemID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResellerRouteSetItemsRepository) Delete(ctx context.Context, itemID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reseller_route_set_items WHERE id = $1`, itemID)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return ErrNotFound }
	return nil
}

func (r *ResellerRouteSetItemsRepository) Reorder(ctx context.Context, setID uuid.UUID, entries []RouteSetReorderEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx) //nolint:errcheck
	for _, e := range entries {
		if _, err := tx.Exec(ctx,
			`UPDATE reseller_route_set_items SET priority=$1, updated_at=now()
			 WHERE id=$2 AND set_id=$3`,
			e.Priority, e.ItemID, setID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *ResellerRouteSetItemsRepository) ProvidersInSet(ctx context.Context, setID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT provider_id FROM reseller_route_set_items WHERE set_id = $1`, setID)
	if err != nil { return nil, err }
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil { return nil, err }
		ids = append(ids, id)
	}
	return ids, rows.Err()
}