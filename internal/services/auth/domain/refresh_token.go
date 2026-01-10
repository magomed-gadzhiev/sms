package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken представляет токен для обновления access token
type RefreshToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash string     `json:"-" db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	Revoked   bool       `json:"revoked" db:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// IsExpired проверяет, истек ли срок действия токена
func (t *RefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid проверяет, валиден ли токен (не отозван и не истек)
func (t *RefreshToken) IsValid() bool {
	return !t.Revoked && !t.IsExpired()
}
