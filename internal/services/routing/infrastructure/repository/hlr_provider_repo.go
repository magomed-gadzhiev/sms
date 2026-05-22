package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type hlrProviderRow struct {
	ID               uuid.UUID       `db:"id"`
	Name             string          `db:"name"`
	AdapterType      string          `db:"adapter_type"`
	Config           json.RawMessage `db:"config"`
	Priority         int             `db:"priority"`
	SupportedRegions pq.StringArray  `db:"supported_regions"`
	CostPerLookup    float64         `db:"cost_per_lookup"`
	Status           string          `db:"status"`
	SuccessRate      float64         `db:"success_rate"`
	LastSuccessAt    *int64          `db:"last_success_at"`
	LastFailureAt    *int64          `db:"last_failure_at"`
	Active           bool            `db:"active"`
	CreatedAt        int64           `db:"created_at"`
	UpdatedAt        int64           `db:"updated_at"`
}

// HLRProviderRepository implements domain.HLRProviderRepository
type HLRProviderRepository struct {
	db *sqlx.DB
}

// NewHLRProviderRepository creates a new HLR provider repository
func NewHLRProviderRepository(db *sqlx.DB) *HLRProviderRepository {
	return &HLRProviderRepository{db: db}
}

func (r *HLRProviderRepository) Create(ctx context.Context, provider *domain.HLRProvider) error {
	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return fmt.Errorf("ошибка сериализации конфигурации: %w", err)
	}

	query := `
		INSERT INTO hlr_providers (id, name, adapter_type, config, priority, supported_regions, cost_per_lookup, status, success_rate, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	// database/sql encodes []byte как bytea, и Postgres не имеет implicit
	// bytea→jsonb cast → SQLSTATE 22P02. Передаём как string, чтобы драйвер
	// послал text, который jsonb принимает через implicit cast. (Драйвер тут
	// pgx-stdlib, см. cmd/services/routing-service/main.go; lib/pq в этом
	// файле используется только для pq.Array, не как SQL-driver.)
	_, err = r.db.ExecContext(ctx, query,
		provider.ID, provider.Name, provider.AdapterType, string(configJSON),
		provider.Priority, pq.Array(provider.SupportedRegions), provider.CostPerLookup,
		string(provider.Status), provider.SuccessRate, provider.Active,
		provider.CreatedAt, provider.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания HLR провайдера: %w", err)
	}
	return nil
}

func (r *HLRProviderRepository) Update(ctx context.Context, provider *domain.HLRProvider) error {
	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return fmt.Errorf("ошибка сериализации конфигурации: %w", err)
	}

	query := `
		UPDATE hlr_providers SET
			name = $2, adapter_type = $3, config = $4, priority = $5,
			supported_regions = $6, cost_per_lookup = $7, status = $8,
			success_rate = $9, last_success_at = $10, last_failure_at = $11,
			active = $12, updated_at = $13
		WHERE id = $1`

	// last_success_at и last_failure_at — TIMESTAMPTZ. Передаём *time.Time
	// напрямую (Unix-seconds через *int64 вызывает SQLSTATE 22P02, потому что
	// нет implicit-cast int→timestamp в Postgres — это не специфика драйвера).
	// config: см. комментарий в Create — string, не []byte.
	_, err = r.db.ExecContext(ctx, query,
		provider.ID, provider.Name, provider.AdapterType, string(configJSON),
		provider.Priority, pq.Array(provider.SupportedRegions), provider.CostPerLookup,
		string(provider.Status), provider.SuccessRate,
		provider.LastSuccessAt, provider.LastFailureAt,
		provider.Active, provider.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка обновления HLR провайдера: %w", err)
	}
	return nil
}

func (r *HLRProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "UPDATE hlr_providers SET active = false WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка удаления HLR провайдера: %w", err)
	}
	return nil
}

func (r *HLRProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.HLRProvider, error) {
	query := `SELECT id, name, adapter_type, config, priority, supported_regions,
		cost_per_lookup, status, success_rate,
		EXTRACT(EPOCH FROM last_success_at)::bigint as last_success_at,
		EXTRACT(EPOCH FROM last_failure_at)::bigint as last_failure_at,
		active,
		EXTRACT(EPOCH FROM created_at)::bigint as created_at,
		EXTRACT(EPOCH FROM updated_at)::bigint as updated_at
	FROM hlr_providers WHERE id = $1`

	var row hlrProviderRow
	if err := r.db.GetContext(ctx, &row, query, id); err != nil {
		return nil, domain.ErrHLRProviderNotFound
	}
	return rowToHLRProvider(&row), nil
}

func (r *HLRProviderRepository) ListActive(ctx context.Context) ([]*domain.HLRProvider, error) {
	query := `SELECT id, name, adapter_type, config, priority, supported_regions,
		cost_per_lookup, status, success_rate,
		EXTRACT(EPOCH FROM last_success_at)::bigint as last_success_at,
		EXTRACT(EPOCH FROM last_failure_at)::bigint as last_failure_at,
		active,
		EXTRACT(EPOCH FROM created_at)::bigint as created_at,
		EXTRACT(EPOCH FROM updated_at)::bigint as updated_at
	FROM hlr_providers WHERE active = true ORDER BY priority ASC`

	var rows []hlrProviderRow
	if err := r.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("ошибка получения HLR провайдеров: %w", err)
	}

	providers := make([]*domain.HLRProvider, len(rows))
	for i, row := range rows {
		providers[i] = rowToHLRProvider(&row)
	}
	return providers, nil
}

func (r *HLRProviderRepository) GetByPriority(ctx context.Context, countryCode string) ([]*domain.HLRProvider, error) {
	query := `SELECT id, name, adapter_type, config, priority, supported_regions,
		cost_per_lookup, status, success_rate,
		EXTRACT(EPOCH FROM last_success_at)::bigint as last_success_at,
		EXTRACT(EPOCH FROM last_failure_at)::bigint as last_failure_at,
		active,
		EXTRACT(EPOCH FROM created_at)::bigint as created_at,
		EXTRACT(EPOCH FROM updated_at)::bigint as updated_at
	FROM hlr_providers
	WHERE active = true AND $1 = ANY(supported_regions)
	ORDER BY priority ASC`

	var rows []hlrProviderRow
	if err := r.db.SelectContext(ctx, &rows, query, countryCode); err != nil {
		return nil, fmt.Errorf("ошибка получения HLR провайдеров по приоритету: %w", err)
	}

	providers := make([]*domain.HLRProvider, len(rows))
	for i, row := range rows {
		providers[i] = rowToHLRProvider(&row)
	}
	return providers, nil
}

func rowToHLRProvider(row *hlrProviderRow) *domain.HLRProvider {
	provider := &domain.HLRProvider{
		ID:               row.ID,
		Name:             row.Name,
		AdapterType:      row.AdapterType,
		Priority:         row.Priority,
		SupportedRegions: []string(row.SupportedRegions),
		CostPerLookup:    row.CostPerLookup,
		Status:           domain.HLRProviderStatus(row.Status),
		SuccessRate:      row.SuccessRate,
		Active:           row.Active,
	}

	// Parse config
	if row.Config != nil {
		_ = json.Unmarshal(row.Config, &provider.Config)
	}

	// Parse timestamps
	if row.CreatedAt > 0 {
		t := time.Unix(row.CreatedAt, 0)
		provider.CreatedAt = t
	}
	if row.UpdatedAt > 0 {
		t := time.Unix(row.UpdatedAt, 0)
		provider.UpdatedAt = t
	}
	if row.LastSuccessAt != nil {
		t := time.Unix(*row.LastSuccessAt, 0)
		provider.LastSuccessAt = &t
	}
	if row.LastFailureAt != nil {
		t := time.Unix(*row.LastFailureAt, 0)
		provider.LastFailureAt = &t
	}

	return provider
}
