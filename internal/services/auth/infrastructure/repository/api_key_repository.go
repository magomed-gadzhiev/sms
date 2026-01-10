package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

// APIKeyRepository предоставляет методы для работы с API ключами
type APIKeyRepository struct {
	db *sqlx.DB
}

// NewAPIKeyRepository создает новый репозиторий API ключей
func NewAPIKeyRepository(db *database.DB) *APIKeyRepository {
	return &APIKeyRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает новый API ключ
func (r *APIKeyRepository) Create(ctx context.Context, apiKey *domain.APIKey) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Вставляем API ключ
	query := `
		INSERT INTO api_keys (
			id, user_id, name, key_hash, key_prefix, active, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	_, err = tx.ExecContext(ctx, query,
		apiKey.ID, apiKey.UserID, apiKey.Name, apiKey.KeyHash, apiKey.KeyPrefix,
		apiKey.Active, apiKey.ExpiresAt, apiKey.CreatedAt, apiKey.UpdatedAt,
	)
	if err != nil {
		return err
	}

	// Вставляем scopes
	if len(apiKey.Scopes) > 0 {
		scopeQuery := `INSERT INTO api_key_scopes (api_key_id, scope) VALUES ($1, $2)`
		for _, scope := range apiKey.Scopes {
			_, err = tx.ExecContext(ctx, scopeQuery, apiKey.ID, scope)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// GetByID получает API ключ по ID
func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	query := `
		SELECT id, user_id, name, key_hash, key_prefix, active, expires_at, last_used_at, created_at, updated_at
		FROM api_keys WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.KeyPrefix,
		&apiKey.Active, &apiKey.ExpiresAt, &apiKey.LastUsedAt,
		&apiKey.CreatedAt, &apiKey.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}

	// Загружаем scopes
	scopes, err := r.getScopesByKeyID(ctx, id)
	if err != nil {
		return nil, err
	}
	apiKey.Scopes = scopes

	return &apiKey, nil
}

// GetByKeyHash получает API ключ по хешу ключа
func (r *APIKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	query := `
		SELECT id, user_id, name, key_hash, key_prefix, active, expires_at, last_used_at, created_at, updated_at
		FROM api_keys WHERE key_hash = $1
	`

	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.KeyPrefix,
		&apiKey.Active, &apiKey.ExpiresAt, &apiKey.LastUsedAt,
		&apiKey.CreatedAt, &apiKey.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}

	// Загружаем scopes
	scopes, err := r.getScopesByKeyID(ctx, apiKey.ID)
	if err != nil {
		return nil, err
	}
	apiKey.Scopes = scopes

	return &apiKey, nil
}

// ListByUserID получает список API ключей пользователя
func (r *APIKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	var apiKeys []*domain.APIKey
	query := `
		SELECT id, user_id, name, key_hash, key_prefix, active, expires_at, last_used_at, created_at, updated_at
		FROM api_keys WHERE user_id = $1
		ORDER BY created_at DESC
	`

	err := r.db.SelectContext(ctx, &apiKeys, query, userID)
	if err != nil {
		return nil, err
	}

	// Загружаем scopes для каждого ключа
	for _, key := range apiKeys {
		scopes, err := r.getScopesByKeyID(ctx, key.ID)
		if err != nil {
			return nil, err
		}
		key.Scopes = scopes
	}

	return apiKeys, nil
}

// UpdateLastUsed обновляет время последнего использования ключа
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_keys SET last_used_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// Revoke отзывает API ключ (устанавливает active = false)
func (r *APIKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_keys SET active = false, updated_at = NOW() WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// getScopesByKeyID получает scopes API ключа
func (r *APIKeyRepository) getScopesByKeyID(ctx context.Context, keyID uuid.UUID) ([]string, error) {
	var scopes []string
	query := `SELECT scope FROM api_key_scopes WHERE api_key_id = $1 ORDER BY scope`

	err := r.db.SelectContext(ctx, &scopes, query, keyID)
	if err != nil {
		return nil, err
	}

	return scopes, nil
}
