package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authinfra "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user is inactive")
	ErrAPIKeyInvalid      = errors.New("api key is invalid")
)

// AuthService предоставляет методы для аутентификации
type AuthService struct {
	userRepo          *authrepo.UserRepository
	apiKeyRepo        *authrepo.APIKeyRepository
	tokenService      *TokenService
	passwordHasher    PasswordHasher
	apiKeyGenerator   APIKeyGenerator
}

// PasswordHasher интерфейс для хеширования паролей
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	CheckPassword(password, hash string) bool
}

// APIKeyGenerator интерфейс для генерации API ключей
type APIKeyGenerator interface {
	GenerateAPIKey() (string, error)
	GetKeyPrefix(key string) string
}

// NewAuthService создает новый сервис аутентификации
func NewAuthService(
	userRepo *authrepo.UserRepository,
	apiKeyRepo *authrepo.APIKeyRepository,
	tokenService *TokenService,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		apiKeyRepo:      apiKeyRepo,
		tokenService:    tokenService,
		passwordHasher:  &authinfra.PasswordHasherImpl{},
		apiKeyGenerator: &authinfra.APIKeyGeneratorImpl{},
	}
}

// AuthenticateByCredentials аутентифицирует пользователя по username/email и паролю
func (s *AuthService) AuthenticateByCredentials(
	ctx context.Context,
	usernameOrEmail string,
	password string,
) (*domain.User, string, string, error) {
	// Получаем пользователя по username или email
	var user *domain.User
	var err error

	// Пробуем сначала по username
	user, err = s.userRepo.GetByUsername(ctx, usernameOrEmail)
	if err != nil && err != authrepo.ErrUserNotFound {
		return nil, "", "", err
	}

	// Если не нашли, пробуем по email
	if user == nil {
		user, err = s.userRepo.GetByEmail(ctx, usernameOrEmail)
		if err != nil {
			if err == authrepo.ErrUserNotFound {
				return nil, "", "", ErrInvalidCredentials
			}
			return nil, "", "", err
		}
	}

	// Проверяем пароль
	if !s.passwordHasher.CheckPassword(password, user.PasswordHash) {
		return nil, "", "", ErrInvalidCredentials
	}

	// Проверяем активность
	if !user.IsActive() {
		return nil, "", "", ErrUserInactive
	}

	// Загружаем полную информацию с ролью и правами
	user, err = s.userRepo.GetByIDWithRole(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}

	// Генерируем токены
	accessToken, refreshToken, err := s.tokenService.GenerateTokenPair(ctx, user.ID, user.Role.Name)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// ErrIPNotAllowed ошибка при попытке доступа с запрещённого IP
var ErrIPNotAllowed = errors.New("ip address not allowed")

// AuthenticateByAPIKey аутентифицирует пользователя по API ключу.
// requestIP — IP адрес запроса (опционально, пустая строка = без проверки).
func (s *AuthService) AuthenticateByAPIKey(
	ctx context.Context,
	apiKey string,
	requestIP ...string,
) (*domain.User, error) {
	// Хешируем ключ для поиска
	keyHash := s.hashAPIKey(apiKey)

	// Получаем API ключ
	key, err := s.apiKeyRepo.GetByKeyHash(ctx, keyHash)
	if err != nil {
		if err == authrepo.ErrAPIKeyNotFound {
			return nil, ErrAPIKeyInvalid
		}
		return nil, err
	}

	// Проверяем валидность ключа
	if !key.IsValid() {
		return nil, ErrAPIKeyInvalid
	}

	// Проверяем IP whitelist
	if len(requestIP) > 0 && requestIP[0] != "" {
		if !key.IsIPAllowed(requestIP[0]) {
			return nil, ErrIPNotAllowed
		}
	}

	// Обновляем время последнего использования
	if err := s.apiKeyRepo.UpdateLastUsed(ctx, key.ID); err != nil {
		log.Warn().Err(err).Msg("не удалось обновить время использования API ключа")
	}

	// Получаем пользователя с ролью и правами
	user, err := s.userRepo.GetByIDWithRole(ctx, key.UserID)
	if err != nil {
		return nil, err
	}

	// Проверяем активность пользователя
	if !user.IsActive() {
		return nil, ErrUserInactive
	}

	return user, nil
}

// ValidateToken валидирует JWT токен и возвращает информацию о пользователе
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	// Валидируем токен
	claims, err := s.tokenService.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	// Получаем пользователя
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, errors.New("invalid user ID in token")
	}

	user, err := s.userRepo.GetByIDWithRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Проверяем активность
	if !user.IsActive() {
		return nil, ErrUserInactive
	}

	return user, nil
}

// CreateAPIKey создает новый API ключ для пользователя
func (s *AuthService) CreateAPIKey(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	expiresAt *time.Time,
	scopes []string,
	allowedIPs []string,
) (*domain.APIKey, string, error) {
	// Проверяем существование пользователя
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}

	// Генерируем ключ
	apiKey, err := s.apiKeyGenerator.GenerateAPIKey()
	if err != nil {
		return nil, "", err
	}

	// Хешируем ключ для хранения
	keyHash := s.hashAPIKey(apiKey)
	keyPrefix := s.apiKeyGenerator.GetKeyPrefix(apiKey)

	// Создаем запись API ключа
	key := &domain.APIKey{
		ID:         uuid.New(),
		UserID:     userID,
		Name:       name,
		KeyHash:    keyHash,
		KeyPrefix:  keyPrefix,
		Active:     true,
		ExpiresAt:  expiresAt,
		Scopes:     scopes,
		AllowedIPs: allowedIPs,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.apiKeyRepo.Create(ctx, key); err != nil {
		return nil, "", err
	}

	return key, apiKey, nil
}

// RevokeAPIKey отзывает API ключ
func (s *AuthService) RevokeAPIKey(ctx context.Context, keyID uuid.UUID) error {
	return s.apiKeyRepo.Revoke(ctx, keyID)
}

// ListAPIKeys получает список API ключей пользователя
func (s *AuthService) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return s.apiKeyRepo.ListByUserID(ctx, userID)
}

// hashAPIKey хеширует API ключ используя SHA256
func (s *AuthService) hashAPIKey(key string) string {
	// Используем SHA256 для хеширования API ключей
	// В production можно использовать bcrypt, но для ключей SHA256 достаточно
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
