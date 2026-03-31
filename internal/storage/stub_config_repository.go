package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/smsc"
)

type StubConfigRepository struct {
	db *sqlx.DB
}

func NewStubConfigRepository(db *DB) *StubConfigRepository {
	return &StubConfigRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *StubConfigRepository) GetByProviderID(ctx context.Context, providerID uuid.UUID) (*smsc.StubProviderConfig, error) {
	type row struct {
		ProviderID     uuid.UUID       `db:"provider_id"`
		MinDelayMs     int             `db:"min_delay_ms"`
		MaxDelayMs     int             `db:"max_delay_ms"`
		FailureRatePct int             `db:"failure_rate_pct"`
		DLRDelayMs     int             `db:"dlr_delay_ms"`
		DLRSuccessRate int             `db:"dlr_success_rate"`
		DLRStatuses    json.RawMessage `db:"dlr_statuses"`
	}
	var r2 row
	err := r.db.QueryRowxContext(ctx,
		`SELECT provider_id, min_delay_ms, max_delay_ms, failure_rate_pct,
		        dlr_delay_ms, dlr_success_rate, dlr_statuses
		 FROM stub_provider_config WHERE provider_id = $1`, providerID,
	).StructScan(&r2)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var statuses []string
	_ = json.Unmarshal(r2.DLRStatuses, &statuses)
	return &smsc.StubProviderConfig{
		ProviderID:     r2.ProviderID,
		MinDelayMs:     r2.MinDelayMs,
		MaxDelayMs:     r2.MaxDelayMs,
		FailureRatePct: r2.FailureRatePct,
		DLRDelayMs:     r2.DLRDelayMs,
		DLRSuccessRate: r2.DLRSuccessRate,
		DLRStatuses:    statuses,
	}, nil
}

func (r *StubConfigRepository) Upsert(ctx context.Context, cfg *smsc.StubProviderConfig) error {
	statuses, _ := json.Marshal(cfg.DLRStatuses)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stub_provider_config
		    (provider_id, min_delay_ms, max_delay_ms, failure_rate_pct, dlr_delay_ms, dlr_success_rate, dlr_statuses)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider_id) DO UPDATE SET
		    min_delay_ms = EXCLUDED.min_delay_ms,
		    max_delay_ms = EXCLUDED.max_delay_ms,
		    failure_rate_pct = EXCLUDED.failure_rate_pct,
		    dlr_delay_ms = EXCLUDED.dlr_delay_ms,
		    dlr_success_rate = EXCLUDED.dlr_success_rate,
		    dlr_statuses = EXCLUDED.dlr_statuses,
		    updated_at = now()`,
		cfg.ProviderID, cfg.MinDelayMs, cfg.MaxDelayMs, cfg.FailureRatePct,
		cfg.DLRDelayMs, cfg.DLRSuccessRate, statuses,
	)
	return err
}
