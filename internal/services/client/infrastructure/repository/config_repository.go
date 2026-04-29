package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var (
	ErrConfigNotFound = errors.New("client config not found")
)

// ConfigRepository предоставляет методы для работы с конфигурациями клиентов
type ConfigRepository struct {
	db *sqlx.DB
}

// NewConfigRepository создает новый репозиторий конфигураций
func NewConfigRepository(db *database.DB) *ConfigRepository {
	return &ConfigRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает новую конфигурацию клиента
func (r *ConfigRepository) Create(ctx context.Context, config *domain.ClientConfig) error {
	query := `
		INSERT INTO client_configs (
			id, client_id, rate_limit_per_second, rate_limit_per_minute,
			rate_limit_per_hour, rate_limit_per_day,
			allowed_sources, blocked_destinations, settings, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	// settings — jsonb. database/sql encodes []byte/json.RawMessage как bytea,
	// что Postgres не кастует в jsonb (SQLSTATE 22P02). Передаём как string,
	// тогда драйвер шлёт text и Postgres сам кастует. См. также BUG-15
	// (HLR provider config).
	_, err := r.db.ExecContext(ctx, query,
		config.ID, config.ClientID,
		config.RateLimitPerSecond, config.RateLimitPerMinute,
		config.RateLimitPerHour, config.RateLimitPerDay,
		pq.Array(config.AllowedSources),
		pq.Array(config.BlockedDestinations),
		string(config.Settings),
		config.CreatedAt, config.UpdatedAt,
	)

	if err != nil {
		return err
	}

	return nil
}

// GetByClientID получает конфигурацию по client_id
func (r *ConfigRepository) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.ClientConfig, error) {
	var config domain.ClientConfig
	var allowedSources pq.StringArray
	var blockedDestinations pq.StringArray

	query := `
		SELECT id, client_id, rate_limit_per_second, rate_limit_per_minute,
		       rate_limit_per_hour, rate_limit_per_day,
		       allowed_sources, blocked_destinations, settings, created_at, updated_at
		FROM client_configs WHERE client_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, clientID).Scan(
		&config.ID, &config.ClientID,
		&config.RateLimitPerSecond, &config.RateLimitPerMinute,
		&config.RateLimitPerHour, &config.RateLimitPerDay,
		&allowedSources, &blockedDestinations,
		&config.Settings, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	// Преобразуем pq.StringArray в []string
	if allowedSources != nil {
		config.AllowedSources = []string(allowedSources)
	} else {
		config.AllowedSources = []string{}
	}
	if blockedDestinations != nil {
		config.BlockedDestinations = []string(blockedDestinations)
	} else {
		config.BlockedDestinations = []string{}
	}

	return &config, nil
}

// Update обновляет конфигурацию клиента
func (r *ConfigRepository) Update(ctx context.Context, config *domain.ClientConfig) error {
	query := `
		UPDATE client_configs SET
			rate_limit_per_second = $2, rate_limit_per_minute = $3,
			rate_limit_per_hour = $4, rate_limit_per_day = $5,
			allowed_sources = $6, blocked_destinations = $7,
			settings = $8, updated_at = $9
		WHERE client_id = $1
	`

	// settings — jsonb. См. комментарий в Create — string, не []byte.
	result, err := r.db.ExecContext(ctx, query,
		config.ClientID,
		config.RateLimitPerSecond, config.RateLimitPerMinute,
		config.RateLimitPerHour, config.RateLimitPerDay,
		pq.Array(config.AllowedSources),
		pq.Array(config.BlockedDestinations),
		string(config.Settings),
		config.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrConfigNotFound
	}

	return nil
}

// Upsert создает или обновляет конфигурацию клиента
func (r *ConfigRepository) Upsert(ctx context.Context, config *domain.ClientConfig) error {
	// Пробуем обновить
	err := r.Update(ctx, config)
	if err == nil {
		return nil
	}

	if err != ErrConfigNotFound {
		return err
	}

	// Если не найдено, создаем
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	return r.Create(ctx, config)
}

// UpdateRateLimits обновляет только rate limits
func (r *ConfigRepository) UpdateRateLimits(ctx context.Context, clientID uuid.UUID, limits *domain.RateLimits) error {
	query := `
		UPDATE client_configs SET
			rate_limit_per_second = $2,
			rate_limit_per_minute = $3,
			rate_limit_per_hour = $4,
			rate_limit_per_day = $5,
			updated_at = NOW()
		WHERE client_id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		clientID, limits.PerSecond, limits.PerMinute, limits.PerHour, limits.PerDay,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrConfigNotFound
	}

	return nil
}
