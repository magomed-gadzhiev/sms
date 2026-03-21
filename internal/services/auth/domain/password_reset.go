package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Ошибки сброса пароля
var (
	ErrPasswordResetTokenNotFound = errors.New("password reset token not found")
	ErrPasswordResetTokenExpired  = errors.New("password reset token expired")
	ErrPasswordResetTokenUsed     = errors.New("password reset token already used")
)

// PasswordResetToken представляет токен для сброса пароля
type PasswordResetToken struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"-"`
	TokenHash string    `db:"token_hash" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	Used      bool      `db:"used" json:"used"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// IsExpired проверяет, истек ли срок действия токена
func (t *PasswordResetToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid проверяет, валиден ли токен (не использован и не истек)
func (t *PasswordResetToken) IsValid() bool {
	return !t.Used && !t.IsExpired()
}
