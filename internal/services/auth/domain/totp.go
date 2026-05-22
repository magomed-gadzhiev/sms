package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Ошибки TOTP
var (
	ErrTOTPNotEnabled     = errors.New("totp is not enabled")
	ErrTOTPAlreadyEnabled = errors.New("totp is already enabled")
	ErrInvalidTOTPCode    = errors.New("invalid totp code")
	ErrInvalidRecoveryCode = errors.New("invalid recovery code")
)

// TOTPConfig представляет конфигурацию TOTP для пользователя
type TOTPConfig struct {
	UserID          uuid.UUID  `db:"user_id" json:"-"`
	SecretEncrypted []byte     `db:"totp_secret_encrypted" json:"-"`
	Enabled         bool       `db:"totp_enabled" json:"totp_enabled"`
	VerifiedAt      *time.Time `db:"totp_verified_at" json:"verified_at,omitempty"`
}

// TOTPRecoveryCode представляет код восстановления TOTP
type TOTPRecoveryCode struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"-"`
	CodeHash  string     `db:"code_hash" json:"-"`
	Used      bool       `db:"used" json:"used"`
	UsedAt    *time.Time `db:"used_at" json:"used_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// IsEnabled проверяет, включен ли TOTP
func (t *TOTPConfig) IsEnabled() bool {
	return t.Enabled
}

// IsVerified проверяет, верифицирован ли TOTP
func (t *TOTPConfig) IsVerified() bool {
	return t.VerifiedAt != nil
}
