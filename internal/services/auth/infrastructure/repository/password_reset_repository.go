package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var ErrPasswordResetTokenNotFound = errors.New("password reset token not found")

// PasswordResetRepository предоставляет методы для работы с токенами сброса пароля
type PasswordResetRepository struct {
	db *sqlx.DB
}

// NewPasswordResetRepository создает новый репозиторий токенов сброса пароля
func NewPasswordResetRepository(db *database.DB) *PasswordResetRepository {
	return &PasswordResetRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает новый токен сброса пароля
func (r *PasswordResetRepository) Create(ctx context.Context, token *domain.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (
			id, user_id, token_hash, expires_at, used, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		token.ID, token.UserID, token.TokenHash,
		token.ExpiresAt, token.Used, token.CreatedAt,
	)

	return err
}

// GetByTokenHash получает токен сброса пароля по хешу
func (r *PasswordResetRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error) {
	var token domain.PasswordResetToken
	query := `
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM password_reset_tokens WHERE token_hash = $1
	`

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash,
		&token.ExpiresAt, &token.Used, &token.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPasswordResetTokenNotFound
		}
		return nil, err
	}

	return &token, nil
}

// MarkUsed помечает токен сброса пароля как использованный
func (r *PasswordResetRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE password_reset_tokens SET used = true WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPasswordResetTokenNotFound
	}

	return nil
}

// DeleteExpired удаляет истекшие и использованные токены
func (r *PasswordResetRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM password_reset_tokens WHERE expires_at < NOW() OR used = true`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// CountRecentByUserID подсчитывает количество токенов, созданных пользователем за указанный период (для rate limiting)
func (r *PasswordResetRepository) CountRecentByUserID(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM password_reset_tokens
		WHERE user_id = $1 AND created_at >= $2
	`

	err := r.db.QueryRowContext(ctx, query, userID, since).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
