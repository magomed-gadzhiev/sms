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

var (
	ErrTOTPConfigNotFound    = errors.New("totp config not found")
	ErrRecoveryCodeNotFound  = errors.New("recovery code not found")
)

// TOTPRepository предоставляет методы для работы с TOTP
type TOTPRepository struct {
	db *sqlx.DB
}

// NewTOTPRepository создает новый репозиторий TOTP
func NewTOTPRepository(db *database.DB) *TOTPRepository {
	return &TOTPRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// SaveTOTPSecret сохраняет зашифрованный TOTP секрет пользователя
func (r *TOTPRepository) SaveTOTPSecret(ctx context.Context, userID uuid.UUID, secretEncrypted []byte) error {
	query := `
		UPDATE users SET totp_secret_encrypted = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID, secretEncrypted)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// GetTOTPConfig получает конфигурацию TOTP пользователя
func (r *TOTPRepository) GetTOTPConfig(ctx context.Context, userID uuid.UUID) (*domain.TOTPConfig, error) {
	var config domain.TOTPConfig
	query := `
		SELECT id AS user_id, totp_secret_encrypted, totp_enabled, totp_verified_at
		FROM users WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&config.UserID, &config.SecretEncrypted, &config.Enabled, &config.VerifiedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTOTPConfigNotFound
		}
		return nil, err
	}

	return &config, nil
}

// EnableTOTP включает TOTP для пользователя
func (r *TOTPRepository) EnableTOTP(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users SET totp_enabled = true, totp_verified_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// DisableTOTP отключает TOTP для пользователя
func (r *TOTPRepository) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users SET totp_enabled = false, totp_secret_encrypted = NULL, totp_verified_at = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// SaveRecoveryCodes сохраняет коды восстановления TOTP
func (r *TOTPRepository) SaveRecoveryCodes(ctx context.Context, codes []*domain.TOTPRecoveryCode) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO totp_recovery_codes (id, user_id, code_hash, used, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, code := range codes {
		_, err = tx.ExecContext(ctx, query,
			code.ID, code.UserID, code.CodeHash, code.Used, code.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UseRecoveryCode помечает код восстановления как использованный
func (r *TOTPRepository) UseRecoveryCode(ctx context.Context, userID uuid.UUID, codeHash string) error {
	query := `
		UPDATE totp_recovery_codes SET used = true, used_at = NOW()
		WHERE user_id = $1 AND code_hash = $2 AND used = false
	`

	result, err := r.db.ExecContext(ctx, query, userID, codeHash)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecoveryCodeNotFound
	}

	return nil
}

// DeleteRecoveryCodes удаляет все коды восстановления пользователя
func (r *TOTPRepository) DeleteRecoveryCodes(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM totp_recovery_codes WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
