package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authinfra "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
)

// authCache caches (api-key-hash) → *domain.User for hot-path
// authentication. Enabled via HARD_CACHE_ENABLED env.
var authCache = cache.NewHardCache(60 * time.Second)

// lastUsedSampleRate — under hard-cache, only update api_keys.last_used_at
// on ~1% of hits to avoid lock contention on the row.
const lastUsedSampleDenominator = 100

// Sentinel errors, re-exported from repository for callers that don't import the repo package.
var (
	ErrUserNotFound   = authrepo.ErrUserNotFound
	ErrAPIKeyNotFound = authrepo.ErrAPIKeyNotFound
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user is inactive")
	ErrAPIKeyInvalid      = errors.New("api key is invalid")
)

// AuthService предоставляет методы для аутентификации
type AuthService struct {
	userRepo          UserRepository
	apiKeyRepo        APIKeyRepository
	roleRepo          RoleRepository
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
	userRepo UserRepository,
	apiKeyRepo APIKeyRepository,
	tokenService *TokenService,
	roleRepo ...RoleRepository,
) *AuthService {
	svc := &AuthService{
		userRepo:        userRepo,
		apiKeyRepo:      apiKeyRepo,
		tokenService:    tokenService,
		passwordHasher:  &authinfra.PasswordHasherImpl{},
		apiKeyGenerator: &authinfra.APIKeyGeneratorImpl{},
	}
	if len(roleRepo) > 0 {
		svc.roleRepo = roleRepo[0]
	}
	return svc
}

// NewAuthServiceWithDeps создает AuthService с явно указанными зависимостями (для тестов)
func NewAuthServiceWithDeps(
	userRepo UserRepository,
	apiKeyRepo APIKeyRepository,
	tokenService *TokenService,
	passwordHasher PasswordHasher,
	apiKeyGenerator APIKeyGenerator,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		apiKeyRepo:      apiKeyRepo,
		tokenService:    tokenService,
		passwordHasher:  passwordHasher,
		apiKeyGenerator: apiKeyGenerator,
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

	ip := ""
	if len(requestIP) > 0 {
		ip = requestIP[0]
	}

	// Hard-cache hit: skip DB entirely, но Active/Expired проверяются по
	// кешированному значению — deactivated/expired ключ не должен работать
	// в течение TTL. Невалидную запись инвалидируем, последующий запрос
	// пойдёт в БД за актуальным отказом. last_used_at update сэмплируется
	// ~1% для снятия hot-row lock contention.
	if authCache.Enabled() {
		if v, ok := authCache.Get(keyHash); ok {
			entry := v.(*cachedAuthEntry)
			if !entry.key.IsValid() || !entry.user.IsActive() {
				authCache.Invalidate(keyHash)
			} else if ip != "" && !entry.key.IsIPAllowed(ip) {
				return nil, ErrIPNotAllowed
			} else {
				if entry.counter.Add(1)%lastUsedSampleDenominator == 0 {
					go func(id uuid.UUID) {
						bg, cancel := context.WithTimeout(context.Background(), 2*time.Second)
						defer cancel()
						if err := s.apiKeyRepo.UpdateLastUsed(bg, id); err != nil {
							log.Debug().Err(err).Msg("sampled last_used update failed")
						}
					}(entry.key.ID)
				}
				return entry.user, nil
			}
		}
	}

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
	if ip != "" {
		if !key.IsIPAllowed(ip) {
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

	if authCache.Enabled() {
		authCache.Set(keyHash, &cachedAuthEntry{key: key, user: user})
	}

	return user, nil
}

type cachedAuthEntry struct {
	key     *domain.APIKey
	user    *domain.User
	counter atomic.Uint64
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

// RevokeAPIKey отзывает API ключ. Только владелец ключа может его отозвать.
func (s *AuthService) RevokeAPIKey(ctx context.Context, keyID, userID uuid.UUID) error {
	key, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return err
	}
	if key.UserID != userID {
		return ErrAPIKeyNotOwned
	}
	return s.apiKeyRepo.Revoke(ctx, keyID)
}

// ListAPIKeys получает список API ключей пользователя
func (s *AuthService) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return s.apiKeyRepo.ListByUserID(ctx, userID)
}

// Ошибки для UpdateAPIKey / ChangePassword
var (
	ErrAPIKeyNotOwned       = errors.New("api key does not belong to user")
	ErrAPIKeyRevoked        = errors.New("api key is revoked")
	ErrPasswordTooShort     = errors.New("password must be at least 8 characters")
	ErrPasswordSameAsOld    = errors.New("new password must differ from current")
	ErrCurrentPasswordWrong = errors.New("current password is incorrect")
)

// UpdateAPIKey обновляет API ключ (имя, scopes, allowed_ips, expires_at).
// Только владелец ключа может его обновить; ключ должен быть активным.
func (s *AuthService) UpdateAPIKey(
	ctx context.Context,
	keyID, userID uuid.UUID,
	name string,
	scopes, allowedIPs []string,
	expiresAt *time.Time,
) (*domain.APIKey, error) {
	key, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}

	if key.UserID != userID {
		return nil, ErrAPIKeyNotOwned
	}

	if !key.Active {
		return nil, ErrAPIKeyRevoked
	}

	// Обновляем поля
	key.Name = name
	key.Scopes = scopes
	key.AllowedIPs = allowedIPs
	key.ExpiresAt = expiresAt
	key.UpdatedAt = time.Now()

	if err := s.apiKeyRepo.Update(ctx, key); err != nil {
		return nil, err
	}

	return key, nil
}

// ChangePassword меняет пароль пользователя (требует текущий пароль).
func (s *AuthService) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	currentPassword, newPassword string,
) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Проверяем текущий пароль
	if !s.passwordHasher.CheckPassword(currentPassword, user.PasswordHash) {
		return ErrCurrentPasswordWrong
	}

	// Валидация нового пароля
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}
	if currentPassword == newPassword {
		return ErrPasswordSameAsOld
	}

	// Хешируем и сохраняем
	hash, err := s.passwordHasher.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	user.UpdatedAt = time.Now()

	return s.userRepo.Update(ctx, user)
}

// hashAPIKey хеширует API ключ используя SHA256
func (s *AuthService) hashAPIKey(key string) string {
	// Используем SHA256 для хеширования API ключей
	// В production можно использовать bcrypt, но для ключей SHA256 достаточно
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ============================================================
// User/Role Management methods
// ============================================================

// CreateUser создает нового пользователя (админ-операция)
func (s *AuthService) CreateUser(ctx context.Context, username, email, password string, roleID uuid.UUID, active bool) (*domain.User, error) {
	// Хешируем пароль
	passwordHash, err := s.passwordHasher.HashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &domain.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		RoleID:       roleID,
		Active:       active,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Загружаем пользователя с ролью
	return s.userRepo.GetByIDWithRole(ctx, user.ID)
}

// UpdateUser обновляет пользователя (email, роль, активность, client_id).
// clientID обновляется только если указатель не nil.
func (s *AuthService) UpdateUser(ctx context.Context, userID uuid.UUID, email string, roleID uuid.UUID, active bool, clientID *uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.Email = email
	user.RoleID = roleID
	user.Active = active
	user.UpdatedAt = time.Now()

	// Обновляем client_id только если явно передан
	if clientID != nil {
		user.ClientID = clientID
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return s.userRepo.GetByIDWithRole(ctx, user.ID)
}

// AssignClientToUser привязывает существующего клиента к пользователю.
// Остальные поля пользователя не затрагиваются.
func (s *AuthService) AssignClientToUser(ctx context.Context, userID, clientID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.ClientID = &clientID
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return s.userRepo.GetByIDWithRole(ctx, user.ID)
}

// DeactivateUser деактивирует пользователя
func (s *AuthService) DeactivateUser(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.Deactivate(ctx, userID)
}

// ResetUser2FA сбрасывает 2FA пользователя
func (s *AuthService) ResetUser2FA(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.ResetTOTP(ctx, userID)
}

// ResetUserPassword генерирует временный пароль, хеширует и сохраняет
func (s *AuthService) ResetUserPassword(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}

	// Генерируем случайный 12-символьный пароль
	tempPassword, err := generateRandomPassword(12)
	if err != nil {
		return "", err
	}

	// Хешируем и сохраняем
	passwordHash, err := s.passwordHasher.HashPassword(tempPassword)
	if err != nil {
		return "", err
	}

	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return "", err
	}

	return tempPassword, nil
}

// ListUsers возвращает список пользователей с фильтрацией
func (s *AuthService) ListUsers(ctx context.Context, search, roleID string, activeOnly bool, limit, offset int32) ([]*domain.User, int32, error) {
	return s.userRepo.List(ctx, search, roleID, activeOnly, limit, offset)
}

// GetUser возвращает пользователя по ID с ролью и правами
func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByIDWithRole(ctx, userID)
}

// CreateRole создает новую роль
func (s *AuthService) CreateRole(ctx context.Context, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	return s.roleRepo.Create(ctx, name, description, permissionIDs)
}

// UpdateRole обновляет роль
func (s *AuthService) UpdateRole(ctx context.Context, roleID uuid.UUID, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	return s.roleRepo.Update(ctx, roleID, name, description, permissionIDs)
}

// DeleteRole удаляет роль
func (s *AuthService) DeleteRole(ctx context.Context, roleID uuid.UUID) error {
	return s.roleRepo.Delete(ctx, roleID)
}

// ListRoles возвращает список ролей
func (s *AuthService) ListRoles(ctx context.Context, limit, offset int32) ([]*domain.Role, int32, error) {
	return s.roleRepo.List(ctx, limit, offset)
}

// GetRole возвращает роль по ID с правами и количеством пользователей
func (s *AuthService) GetRole(ctx context.Context, roleID uuid.UUID) (*domain.Role, error) {
	role, err := s.roleRepo.GetByIDWithPermissions(ctx, roleID)
	if err != nil {
		return nil, err
	}

	userCount, err := s.roleRepo.GetUserCount(ctx, roleID)
	if err != nil {
		log.Warn().Err(err).Msg("не удалось получить количество пользователей роли")
	}
	role.UserCount = userCount

	return role, nil
}

// ListAllPermissions возвращает все доступные права
func (s *AuthService) ListAllPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.roleRepo.ListAllPermissions(ctx)
}

// GetUserPermissions возвращает права конкретного пользователя
func (s *AuthService) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Permission, *domain.Role, error) {
	user, err := s.userRepo.GetByIDWithRole(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return user.Permissions, user.Role, nil
}

// generateRandomPassword генерирует случайный пароль указанной длины
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}
