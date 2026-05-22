package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientRepository предоставляет методы для работы с клиентами
type ClientRepository struct {
	db *sqlx.DB
}

// NewClientRepository создает новый репозиторий клиентов
func NewClientRepository(db *DB) *ClientRepository {
	return &ClientRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает нового клиента
func (r *ClientRepository) Create(ctx context.Context, client *shared.Client) error {
	query := `
		INSERT INTO clients (
			id, name, api_key, secret, active,
			rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour,
			allowed_source_addresses, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		client.ID, client.Name, client.APIKey, client.Secret, client.Active,
		client.RateLimitPerSecond, client.RateLimitPerMinute, client.RateLimitPerHour,
		client.AllowedSourceAddresses, client.CreatedAt, client.UpdatedAt,
	)

	return err
}

// GetByID получает клиента по ID
func (r *ClientRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Client, error) {
	var client shared.Client
	var allowedAddrsStr string
	query := `
		SELECT id, name, api_key, secret, active,
		       rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour,
		       allowed_source_addresses::text, created_at, updated_at
		FROM clients WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&client.ID, &client.Name, &client.APIKey, &client.Secret, &client.Active,
		&client.RateLimitPerSecond, &client.RateLimitPerMinute, &client.RateLimitPerHour,
		&allowedAddrsStr, &client.CreatedAt, &client.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Парсим строку PostgreSQL массива в наш StringArray
	if err := client.AllowedSourceAddresses.Scan(allowedAddrsStr); err != nil {
		client.AllowedSourceAddresses = shared.StringArray{}
	}

	return &client, nil
}

// GetByAPIKey получает клиента по API ключу
func (r *ClientRepository) GetByAPIKey(ctx context.Context, apiKey string) (*shared.Client, error) {
	var client shared.Client
	var allowedAddrsStr string
	query := `
		SELECT id, name, api_key, secret, active,
		       rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour,
		       allowed_source_addresses::text, created_at, updated_at
		FROM clients WHERE api_key = $1
	`

	err := r.db.QueryRowContext(ctx, query, apiKey).Scan(
		&client.ID, &client.Name, &client.APIKey, &client.Secret, &client.Active,
		&client.RateLimitPerSecond, &client.RateLimitPerMinute, &client.RateLimitPerHour,
		&allowedAddrsStr, &client.CreatedAt, &client.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Парсим строку PostgreSQL массива в наш StringArray
	if err := client.AllowedSourceAddresses.Scan(allowedAddrsStr); err != nil {
		client.AllowedSourceAddresses = shared.StringArray{}
	}

	return &client, nil
}

// GetAllActive получает всех активных клиентов
func (r *ClientRepository) GetAllActive(ctx context.Context) ([]*shared.Client, error) {
	var clients []*shared.Client
	query := `
		SELECT * FROM clients
		WHERE active = true
		ORDER BY name ASC
	`

	err := r.db.SelectContext(ctx, &clients, query)
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// GetAll получает всех клиентов
func (r *ClientRepository) GetAll(ctx context.Context) ([]*shared.Client, error) {
	var clients []*shared.Client
	query := `
		SELECT * FROM clients
		ORDER BY name ASC
	`

	err := r.db.SelectContext(ctx, &clients, query)
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// Update обновляет клиента
func (r *ClientRepository) Update(ctx context.Context, client *shared.Client) error {
	query := `
		UPDATE clients SET
			name = $2, api_key = $3, secret = $4, active = $5,
			rate_limit_per_second = $6, rate_limit_per_minute = $7,
			rate_limit_per_hour = $8, allowed_source_addresses = $9,
			updated_at = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		client.ID, client.Name, client.APIKey, client.Secret, client.Active,
		client.RateLimitPerSecond, client.RateLimitPerMinute, client.RateLimitPerHour,
		client.AllowedSourceAddresses, client.UpdatedAt,
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

// GetBalance возвращает текущий баланс клиента из таблицы accounts.
func (r *ClientRepository) GetBalance(ctx context.Context, clientID uuid.UUID) (float64, string, error) {
	var balance float64
	var currency string
	query := `SELECT balance, currency FROM accounts WHERE client_id = $1`
	err := r.db.QueryRowContext(ctx, query, clientID).Scan(&balance, &currency)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "RUB", nil
		}
		return 0, "", err
	}
	return balance, currency, nil
}

// IncrementMonthlySMSCount увеличивает счётчик SMS за месяц (с авто-сбросом в начале нового месяца)
func (r *ClientRepository) IncrementMonthlySMSCount(ctx context.Context, clientID uuid.UUID, count int) error {
	query := `UPDATE clients
		SET monthly_sms_count = CASE
			WHEN monthly_sms_reset_at IS NOT NULL AND monthly_sms_reset_at <= NOW() THEN $2
			ELSE COALESCE(monthly_sms_count, 0) + $2
		END,
		monthly_sms_reset_at = CASE
			WHEN monthly_sms_reset_at IS NOT NULL AND monthly_sms_reset_at <= NOW() THEN date_trunc('month', NOW()) + INTERVAL '1 month'
			ELSE COALESCE(monthly_sms_reset_at, date_trunc('month', NOW()) + INTERVAL '1 month')
		END,
		updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, clientID, count)
	return err
}

// Delete удаляет клиента (мягкое удаление через active = false)
func (r *ClientRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE clients SET active = false, updated_at = NOW()
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

