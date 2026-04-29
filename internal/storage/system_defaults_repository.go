package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jmoiron/sqlx"
)

type SystemDefaultsRepository struct {
	db *sqlx.DB
}

func NewSystemDefaultsRepository(db *DB) *SystemDefaultsRepository {
	return &SystemDefaultsRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *SystemDefaultsRepository) GetInt(ctx context.Context, key string) (int, error) {
	var raw json.RawMessage
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_defaults WHERE key = $1`, key).Scan(&raw)
	if err != nil {
		return 0, fmt.Errorf("system_defaults.%s: %w", key, err)
	}
	var s string
	if jsonErr := json.Unmarshal(raw, &s); jsonErr == nil {
		return strconv.Atoi(s)
	}
	var n int
	if jsonErr := json.Unmarshal(raw, &n); jsonErr == nil {
		return n, nil
	}
	return 0, fmt.Errorf("system_defaults.%s: не удалось распарсить значение %s", key, string(raw))
}

func (r *SystemDefaultsRepository) Set(ctx context.Context, key string, value int, updatedBy *string) error {
	encoded, _ := json.Marshal(strconv.Itoa(value))
	// BUG-54: колонка `value` имеет тип jsonb. database/sql + pgx-stdlib
	// передают []byte как `bytea`, и Postgres не умеет неявно приводить bytea→jsonb
	// (`invalid input syntax for type json`). Преобразуем в string — pgx тогда
	// шлёт значение как text, и cast `$2::jsonb` отрабатывает корректно.
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO system_defaults (key, value, updated_at, updated_by)
		VALUES ($1, $2::jsonb, now(), $3)
		ON CONFLICT (key) DO UPDATE
		    SET value = EXCLUDED.value, updated_at = now(), updated_by = EXCLUDED.updated_by`,
		key, string(encoded), updatedBy)
	return err
}

type SystemDefaultsAll struct {
	RateLimitPerSecond    int `json:"rate_limit_per_second"`
	RateLimitPerMinute    int `json:"rate_limit_per_minute"`
	RateLimitPerHour      int `json:"rate_limit_per_hour"`
	DefaultTPSPerProvider int `json:"default_tps_per_provider"`
	MaxProvidersPerClient int `json:"max_providers_per_client"`
	MaxSubAccounts        int `json:"max_sub_accounts"`
}

func (r *SystemDefaultsRepository) GetAll(ctx context.Context) (*SystemDefaultsAll, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM system_defaults`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int)
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if scanErr := rows.Scan(&key, &raw); scanErr != nil {
			continue
		}
		var s string
		if jsonErr := json.Unmarshal(raw, &s); jsonErr == nil {
			if n, convErr := strconv.Atoi(s); convErr == nil {
				m[key] = n
			}
			continue
		}
		var n int
		if jsonErr := json.Unmarshal(raw, &n); jsonErr == nil {
			m[key] = n
		}
	}

	return &SystemDefaultsAll{
		RateLimitPerSecond:    m["rate_limit_per_second"],
		RateLimitPerMinute:    m["rate_limit_per_minute"],
		RateLimitPerHour:      m["rate_limit_per_hour"],
		DefaultTPSPerProvider: m["default_tps_per_provider"],
		MaxProvidersPerClient: m["max_providers_per_client"],
		MaxSubAccounts:        m["max_sub_accounts"],
	}, rows.Err()
}
