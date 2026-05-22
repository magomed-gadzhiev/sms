package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ProviderRepository предоставляет методы для работы с провайдерами
type ProviderRepository struct {
	db *sqlx.DB
}

// NewProviderRepository создает новый репозиторий провайдеров
func NewProviderRepository(db *DB) *ProviderRepository {
	return &ProviderRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает нового провайдера
func (r *ProviderRepository) Create(ctx context.Context, provider *shared.Provider) error {
	query := `
		INSERT INTO providers (
			id, name, host, port, system_id, password, system_type,
			bind_type, bind_ton, bind_npi, addr_ton, addr_npi,
			address_range, max_connections, active, priority,
			throughput_per_second, created_at, updated_at,
			client_id, description, tags, tps_limit, routing_rules
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24
		)
	`

	routingRules := string(provider.RoutingRules)
	if routingRules == "" || routingRules == "null" {
		routingRules = "[]"
	}
	tags := tagsArg(provider.Tags)

	_, err := r.db.ExecContext(ctx, query,
		provider.ID, provider.Name, provider.Host, provider.Port,
		provider.SystemID, provider.Password, provider.SystemType,
		provider.BindType, provider.BindTON, provider.BindNPI,
		provider.AddrTON, provider.AddrNPI, provider.AddressRange,
		provider.MaxConnections, provider.Active, provider.Priority,
		provider.ThroughputPerSec, provider.CreatedAt, provider.UpdatedAt,
		provider.ClientID, provider.Description, tags, provider.TPSLimit, routingRules,
	)

	return err
}

// GetByID получает провайдера по ID
func (r *ProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
	var provider shared.Provider
	query := `
		SELECT * FROM providers WHERE id = $1
	`

	err := r.db.GetContext(ctx, &provider, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &provider, nil
}

// GetByName получает провайдера по имени
func (r *ProviderRepository) GetByName(ctx context.Context, name string) (*shared.Provider, error) {
	var provider shared.Provider
	query := `
		SELECT * FROM providers WHERE name = $1
	`

	err := r.db.GetContext(ctx, &provider, query, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &provider, nil
}

// GetAllActive получает всех активных провайдеров, отсортированных по приоритету
func (r *ProviderRepository) GetAllActive(ctx context.Context) ([]*shared.Provider, error) {
	var providers []*shared.Provider
	query := `
		SELECT * FROM providers
		WHERE active = true
		ORDER BY priority DESC, name ASC
	`

	err := r.db.SelectContext(ctx, &providers, query)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

// GetAll получает всех провайдеров
func (r *ProviderRepository) GetAll(ctx context.Context) ([]*shared.Provider, error) {
	var providers []*shared.Provider
	query := `
		SELECT * FROM providers
		ORDER BY priority DESC, name ASC
	`

	err := r.db.SelectContext(ctx, &providers, query)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

// Update обновляет провайдера
func (r *ProviderRepository) Update(ctx context.Context, provider *shared.Provider) error {
	query := `
		UPDATE providers SET
			name = $2, host = $3, port = $4, system_id = $5, password = $6,
			system_type = $7, bind_type = $8, bind_ton = $9, bind_npi = $10,
			addr_ton = $11, addr_npi = $12, address_range = $13,
			max_connections = $14, active = $15, priority = $16,
			throughput_per_second = $17, updated_at = $18,
			description = $19, tags = $20, tps_limit = $21, routing_rules = $22
		WHERE id = $1
	`

	routingRules := string(provider.RoutingRules)
	if routingRules == "" || routingRules == "null" {
		routingRules = "[]"
	}
	tags := tagsArg(provider.Tags)

	result, err := r.db.ExecContext(ctx, query,
		provider.ID, provider.Name, provider.Host, provider.Port,
		provider.SystemID, provider.Password, provider.SystemType,
		provider.BindType, provider.BindTON, provider.BindNPI,
		provider.AddrTON, provider.AddrNPI, provider.AddressRange,
		provider.MaxConnections, provider.Active, provider.Priority,
		provider.ThroughputPerSec, provider.UpdatedAt,
		provider.Description, tags, provider.TPSLimit, routingRules,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete удаляет провайдера (мягкое удаление через active = false)
func (r *ProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE providers SET active = false, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// LinkToClient создаёт связь провайдера с клиентом в client_providers
func (r *ProviderRepository) LinkToClient(ctx context.Context, providerID, clientID uuid.UUID, ownership string) error {
	query := `INSERT INTO client_providers (id, client_id, provider_id, ownership, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW(), NOW())`
	_, err := r.db.ExecContext(ctx, query, uuid.New(), clientID, providerID, ownership)
	return err
}

// UnlinkFromClient деактивирует связь провайдера с клиентом в client_providers
func (r *ProviderRepository) UnlinkFromClient(ctx context.Context, providerID, clientID uuid.UUID) error {
	query := `UPDATE client_providers SET active = false, updated_at = NOW()
		WHERE provider_id = $1 AND client_id = $2`
	result, err := r.db.ExecContext(ctx, query, providerID, clientID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByClientID возвращает провайдеров, принадлежащих клиенту
func (r *ProviderRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Provider, error) {
	var providers []*shared.Provider
	query := `SELECT p.id, p.name, p.host, p.port, p.system_id, p.password, p.system_type,
		p.bind_type, p.bind_ton, p.bind_npi, p.addr_ton, p.addr_npi, p.address_range,
		p.max_connections, p.active, p.priority, p.throughput_per_second,
		p.created_at, p.updated_at, p.daily_quota, p.monthly_quota
		FROM providers p
		INNER JOIN client_providers cp ON p.id = cp.provider_id
		WHERE cp.client_id = $1 AND cp.active = true
		ORDER BY p.created_at DESC`
	err := r.db.SelectContext(ctx, &providers, query, clientID)
	if err != nil {
		return nil, err
	}
	return providers, nil
}

// CountByClientID считает провайдеров клиента
func (r *ProviderRepository) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM providers p
		INNER JOIN client_providers cp ON p.id = cp.provider_id
		WHERE cp.client_id = $1 AND cp.active = true`
	err := r.db.GetContext(ctx, &count, query, clientID)
	return count, err
}

// GetByIDAndClientID получает провайдера по ID с проверкой принадлежности клиенту
func (r *ProviderRepository) GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*shared.Provider, error) {
	var provider shared.Provider
	query := `SELECT p.id, p.name, p.host, p.port, p.system_id, p.password, p.system_type,
		p.bind_type, p.bind_ton, p.bind_npi, p.addr_ton, p.addr_npi, p.address_range,
		p.max_connections, p.active, p.priority, p.throughput_per_second,
		p.created_at, p.updated_at, p.daily_quota, p.monthly_quota
		FROM providers p
		INNER JOIN client_providers cp ON p.id = cp.provider_id
		WHERE p.id = $1 AND cp.client_id = $2 AND cp.active = true`
	err := r.db.GetContext(ctx, &provider, query, id, clientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &provider, nil
}

// tagsArg преобразует StringArray в pq.Array с гарантированным непустым значением.
func tagsArg(tags shared.StringArray) interface{} {
	s := []string(tags)
	if s == nil {
		s = []string{}
	}
	return pq.Array(s)
}
