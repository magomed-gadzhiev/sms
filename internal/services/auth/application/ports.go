package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
)

// UserRepository интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByIDWithRole(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// APIKeyRepository интерфейс для работы с API ключами
type APIKeyRepository interface {
	Create(ctx context.Context, apiKey *domain.APIKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
	GetByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error)
	Update(ctx context.Context, apiKey *domain.APIKey) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Revoke(ctx context.Context, id uuid.UUID) error
}

// RefreshTokenRepository интерфейс для работы с refresh токенами
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
	RevokeByUserID(ctx context.Context, userID uuid.UUID) error
}

// TOTPRepository интерфейс для работы с TOTP
type TOTPRepository interface {
	SaveTOTPSecret(ctx context.Context, userID uuid.UUID, secretEncrypted []byte) error
	GetTOTPConfig(ctx context.Context, userID uuid.UUID) (*domain.TOTPConfig, error)
	EnableTOTP(ctx context.Context, userID uuid.UUID) error
	DisableTOTP(ctx context.Context, userID uuid.UUID) error
	SaveRecoveryCodes(ctx context.Context, codes []*domain.TOTPRecoveryCode) error
	UseRecoveryCode(ctx context.Context, userID uuid.UUID, codeHash string) error
	DeleteRecoveryCodes(ctx context.Context, userID uuid.UUID) error
}

// PasswordResetRepository интерфейс для работы с токенами сброса пароля
type PasswordResetRepository interface {
	Create(ctx context.Context, token *domain.PasswordResetToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	CountRecentByUserID(ctx context.Context, userID uuid.UUID, since time.Time) (int, error)
}
