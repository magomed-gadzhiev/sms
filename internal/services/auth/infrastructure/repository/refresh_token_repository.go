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

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

// RefreshTokenRepository предоставляет методы для работы с refresh токенами
type RefreshTokenRepository struct {
	db *sqlx.DB
}

// NewRefreshTokenRepository создает новый репозиторий refresh токенов
func NewRefreshTokenRepository(db *database.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает новый refresh токен
func (r *RefreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (
			id, user_id, token_hash, expires_at, revoked, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		token.ID, token.UserID, token.TokenHash,
		token.ExpiresAt, token.Revoked, token.CreatedAt,
	)

	return err
}

// GetByTokenHash получает refresh токен по хешу
func (r *RefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1
	`

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash,
		&token.ExpiresAt, &token.Revoked, &token.RevokedAt, &token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, err
	}

	return &token, nil
}

// Revoke отзывает refresh токен
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens 
		SET revoked = true, revoked_at = NOW()
		WHERE token_hash = $1
	`

	result, err := r.db.ExecContext(ctx, query, tokenHash)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRefreshTokenNotFound
	}

	return nil
}

// RevokeByUserID отзывает все refresh токены пользователя
func (r *RefreshTokenRepository) RevokeByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens 
		SET revoked = true, revoked_at = NOW()
		WHERE user_id = $1 AND revoked = false
	`

	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// DeleteExpired удаляет истекшие токены (опционально, для очистки)
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`
	_, err := r.db.ExecContext(ctx, query)
	return err
}
