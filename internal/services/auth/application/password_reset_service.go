package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
)

var (
	ErrPasswordResetRateLimit = errors.New("password reset rate limit exceeded")
	ErrPasswordResetInvalid   = errors.New("password reset token is invalid")
)

const (
	resetTokenLength    = 32 // 32 байта = 256 бит
	resetTokenExpiry    = 1 * time.Hour
	maxResetPerHour     = 3
)

// PasswordResetService предоставляет методы для сброса пароля
type PasswordResetService struct {
	resetRepo      PasswordResetRepository
	userRepo       UserRepository
	passwordHasher PasswordHasher
}

// NewPasswordResetService создает новый сервис сброса пароля
func NewPasswordResetService(
	resetRepo PasswordResetRepository,
	userRepo UserRepository,
	passwordHasher PasswordHasher,
) *PasswordResetService {
	return &PasswordResetService{
		resetRepo:      resetRepo,
		userRepo:       userRepo,
		passwordHasher: passwordHasher,
	}
}

// RequestReset запрашивает сброс пароля для пользователя по email
func (s *PasswordResetService) RequestReset(ctx context.Context, email string) (string, uuid.UUID, error) {
	// Получаем пользователя по email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return "", uuid.Nil, authrepo.ErrUserNotFound
		}
		return "", uuid.Nil, err
	}

	// Проверяем rate limit (максимум 3 запроса в час)
	since := time.Now().Add(-1 * time.Hour)
	count, err := s.resetRepo.CountRecentByUserID(ctx, user.ID, since)
	if err != nil {
		return "", uuid.Nil, err
	}

	if count >= maxResetPerHour {
		return "", uuid.Nil, ErrPasswordResetRateLimit
	}

	// Генерируем случайный токен
	tokenBytes := make([]byte, resetTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", uuid.Nil, err
	}
	tokenStr := base64.URLEncoding.EncodeToString(tokenBytes)

	// Хешируем токен для хранения
	tokenHash := s.hashToken(tokenStr)

	// Создаем запись токена сброса пароля
	resetToken := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(resetTokenExpiry),
		Used:      false,
		CreatedAt: time.Now(),
	}

	if err := s.resetRepo.Create(ctx, resetToken); err != nil {
		return "", uuid.Nil, err
	}

	return tokenStr, user.ID, nil
}

// ResetPassword сбрасывает пароль пользователя по токену
func (s *PasswordResetService) ResetPassword(ctx context.Context, tokenStr string, newPassword string) error {
	// Хешируем токен для поиска
	tokenHash := s.hashToken(tokenStr)

	// Получаем токен из БД
	resetToken, err := s.resetRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if err == authrepo.ErrPasswordResetTokenNotFound {
			return ErrPasswordResetInvalid
		}
		return err
	}

	// Проверяем валидность токена
	if !resetToken.IsValid() {
		if resetToken.Used {
			return domain.ErrPasswordResetTokenUsed
		}
		if resetToken.IsExpired() {
			return domain.ErrPasswordResetTokenExpired
		}
		return ErrPasswordResetInvalid
	}

	// Хешируем новый пароль
	passwordHash, err := s.passwordHasher.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, resetToken.UserID)
	if err != nil {
		return err
	}

	// Обновляем пароль пользователя
	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Помечаем токен как использованный
	if err := s.resetRepo.MarkUsed(ctx, resetToken.ID); err != nil {
		return err
	}

	return nil
}

// hashToken хеширует токен с помощью SHA256
func (s *PasswordResetService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
