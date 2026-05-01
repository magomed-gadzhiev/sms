package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// LimitQuerierDB реализует limits.LimitQuerier через PostgreSQL.
type LimitQuerierDB struct {
	db *sqlx.DB
}

func NewLimitQuerierDB(db *DB) *LimitQuerierDB {
	return &LimitQuerierDB{db: sqlx.NewDb(db.DB, "pgx")}
}

func (q *LimitQuerierDB) GetClientProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (*int, error) {
	var tps sql.NullInt32
	err := q.db.QueryRowContext(ctx,
		`SELECT tps_limit FROM client_providers WHERE client_id=$1 AND provider_id=$2 AND active=true`,
		clientID, providerID,
	).Scan(&tps)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !tps.Valid {
		return nil, nil
	}
	v := int(tps.Int32)
	return &v, nil
}

func (q *LimitQuerierDB) GetTariffPlanDefaultTPS(ctx context.Context, clientID uuid.UUID) (*int, error) {
	var tps sql.NullInt32
	err := q.db.QueryRowContext(ctx, `
		SELECT tp.default_tps_per_provider
		FROM tariff_plans tp
		JOIN tariff_periods tper ON tper.plan_id = tp.id
		WHERE tper.client_id = $1
		  AND tper.start_date <= now()
		  AND (tper.end_date IS NULL OR tper.end_date > now())
		ORDER BY tper.start_date DESC
		LIMIT 1`,
		clientID,
	).Scan(&tps)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !tps.Valid {
		return nil, nil
	}
	v := int(tps.Int32)
	return &v, nil
}

func (q *LimitQuerierDB) GetSystemDefaultTPS(ctx context.Context) (int, error) {
	var raw string
	err := q.db.QueryRowContext(ctx,
		`SELECT value #>> '{}' FROM system_defaults WHERE key='default_tps_per_provider'`,
	).Scan(&raw)
	if err != nil {
		return 5, err
	}
	var n int
	if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr != nil {
		return 5, nil
	}
	return n, nil
}

func (q *LimitQuerierDB) GetClientParentAndBudget(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, *int, error) {
	var parentID uuid.NullUUID
	var budget sql.NullInt32
	err := q.db.QueryRowContext(ctx,
		`SELECT parent_client_id, allocated_tps_budget FROM clients WHERE id=$1`,
		clientID,
	).Scan(&parentID, &budget)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var pid *uuid.UUID
	if parentID.Valid {
		v := parentID.UUID
		pid = &v
	}
	var b *int
	if budget.Valid {
		v := int(budget.Int32)
		b = &v
	}
	return pid, b, nil
}

func (q *LimitQuerierDB) GetSubAccountAllocatedTPS(ctx context.Context, clientID, excludeProviderID uuid.UUID) (int, error) {
	var total sql.NullInt32
	err := q.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(tps_limit), 0)
		 FROM client_providers
		 WHERE client_id=$1 AND provider_id != $2 AND active=true AND tps_limit IS NOT NULL`,
		clientID, excludeProviderID,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return int(total.Int32), nil
}
