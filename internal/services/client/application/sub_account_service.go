package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	clientrepo "github.com/smpp-server/smpp-server/internal/services/client/infrastructure/repository"
)

var (
	ErrNotReseller        = errors.New("client is not a reseller")
	ErrMaxSubAccounts     = errors.New("maximum number of sub-accounts reached")
	ErrSubAccountNotFound = errors.New("sub-account not found")
	// ErrInvalidEmail / ErrEmailExists нужны, чтобы grpc-сервер мог отделить
	// 400 INVALID_INPUT (битый формат) и 409 ALREADY_EXISTS (дубликат) от 500.
	ErrInvalidEmail   = errors.New("invalid email format")
	ErrEmailExists    = errors.New("email already used by another sub-account")
)

// emailRegex — простая проверка формата `local@domain.tld`. RFC-полный регекс
// громоздкий и для UI-проверки избыточен; цель — отсеять явный мусор вроде
// "notanemail", который сейчас принимается без вопросов.
var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// SubAccountRepositoryInterface определяет интерфейс для работы с суб-аккаунтами
type SubAccountRepositoryInterface interface {
	ListByParentID(ctx context.Context, parentID uuid.UUID) ([]*domain.Client, error)
	CountByParentID(ctx context.Context, parentID uuid.UUID) (int, error)
	GetSubAccount(ctx context.Context, subAccountID, parentID uuid.UUID) (*domain.Client, error)
	DeleteSubAccount(ctx context.Context, subAccountID uuid.UUID) error
	ExistsByEmailUnderParent(ctx context.Context, parentID uuid.UUID, email string) (bool, error)
}

// SubAccountService предоставляет методы для управления суб-аккаунтами
type SubAccountService struct {
	clientRepo     ClientRepositoryInterface
	subAccountRepo SubAccountRepositoryInterface
	configRepo     ConfigRepositoryInterface
}

// NewSubAccountService создает новый сервис управления суб-аккаунтами
func NewSubAccountService(
	clientRepo ClientRepositoryInterface,
	subAccountRepo SubAccountRepositoryInterface,
	configRepo ConfigRepositoryInterface,
) *SubAccountService {
	return &SubAccountService{
		clientRepo:     clientRepo,
		subAccountRepo: subAccountRepo,
		configRepo:     configRepo,
	}
}

// CreateSubAccount создает новый суб-аккаунт для реселлера
func (s *SubAccountService) CreateSubAccount(
	ctx context.Context,
	parentClientID uuid.UUID,
	name, email, contactPerson string,
	dailyLimit, monthlyLimit int,
) (*domain.Client, error) {
	// Получаем родительского клиента
	parent, err := s.clientRepo.GetByID(ctx, parentClientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return nil, ErrClientNotFound
		}
		return nil, err
	}

	// Проверяем, что родитель — реселлер
	if !parent.IsReseller {
		return nil, ErrNotReseller
	}

	// Проверяем лимит на количество суб-аккаунтов
	currentCount, err := s.subAccountRepo.CountByParentID(ctx, parentClientID)
	if err != nil {
		return nil, err
	}

	if !parent.CanCreateSubAccount(currentCount) {
		return nil, ErrMaxSubAccounts
	}

	// Валидация
	if name == "" {
		return nil, ErrInvalidClientData
	}
	// Email опционален, но если передан — должен быть валидным форматом
	// и уникальным в пределах родительского реселлера. Иначе раньше один
	// агрегатор мог завести несколько субакков с одним email — это ломает
	// возможность login/email-уведомлений и проходит валидацию проверкой
	// "поле непустое" (см. handler-уровень).
	//
	// Нормализуем (trim + lower) ДО dup-check и до записи в БД, чтобы
	// "Foo@x" и "foo@x" не создавали ambiguity. Без нормализации dup-check
	// в SQL делает `lower(email) = lower($2)`, но запись могла оставаться
	// в смешанном регистре, и downstream lookups (без lower) видели разные
	// записи.
	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" {
		if !emailRegex.MatchString(email) {
			return nil, ErrInvalidEmail
		}
		// Не фильтруем active=true: soft-deleted субаккаунт держит email
		// "забронированным" — иначе после delete+create под тем же email
		// получаем 2 строки в БД и downstream login-by-email падает.
		exists, err := s.subAccountRepo.ExistsByEmailUnderParent(ctx, parentClientID, email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
	}

	// Генерируем уникальный API key и secret — БД требует UNIQUE(api_key),
	// иначе второй субаккаунт с пустым ключом упадёт с SQLSTATE 23505.
	apiKeyBytes := make([]byte, 16)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(apiKeyBytes); err != nil {
		return nil, err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}

	// Создаем суб-аккаунт
	subAccount := &domain.Client{
		ID:             uuid.New(),
		Name:           name,
		APIKey:         "ak-" + hex.EncodeToString(apiKeyBytes),
		Secret:         hex.EncodeToString(secretBytes),
		Email:          email,
		ContactPerson:  contactPerson,
		Active:         true,
		ParentClientID: &parentClientID,
		IsReseller:     false,
		MaxSubAccounts: 0,
		Metadata:       json.RawMessage("{}"),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Сохраняем суб-аккаунт
	if err := s.clientRepo.Create(ctx, subAccount); err != nil {
		return nil, err
	}

	// Создаем конфигурацию по умолчанию с лимитами
	defaultConfig := &domain.ClientConfig{
		ID:                  uuid.New(),
		ClientID:            subAccount.ID,
		RateLimitPerSecond:  10,
		RateLimitPerMinute:  100,
		RateLimitPerHour:    1000,
		RateLimitPerDay:     dailyLimit,
		AllowedSources:      []string{},
		BlockedDestinations: []string{},
		Settings:            json.RawMessage("{}"),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	// Сохраняем месячный лимит в settings
	if monthlyLimit > 0 {
		settings := map[string]string{
			"monthly_limit": fmt.Sprintf("%d", monthlyLimit),
		}
		settingsJSON, _ := json.Marshal(settings)
		defaultConfig.Settings = json.RawMessage(settingsJSON)
	}

	if err := s.configRepo.Create(ctx, defaultConfig); err != nil {
		log.Warn().Err(err).Msg("не удалось создать конфигурацию для суб-аккаунта")
	}

	subAccount.Config = defaultConfig

	log.Info().
		Str("parent_client_id", parentClientID.String()).
		Str("sub_account_id", subAccount.ID.String()).
		Str("name", name).
		Msg("суб-аккаунт создан")

	return subAccount, nil
}

// ListSubAccounts получает список суб-аккаунтов реселлера
func (s *SubAccountService) ListSubAccounts(ctx context.Context, parentClientID uuid.UUID) ([]*domain.Client, error) {
	// Проверяем существование родительского клиента
	parent, err := s.clientRepo.GetByID(ctx, parentClientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return nil, ErrClientNotFound
		}
		return nil, err
	}

	if !parent.IsReseller {
		return nil, ErrNotReseller
	}

	return s.subAccountRepo.ListByParentID(ctx, parentClientID)
}

// GetSubAccount получает суб-аккаунт с проверкой принадлежности к родителю
func (s *SubAccountService) GetSubAccount(ctx context.Context, subAccountID, parentClientID uuid.UUID) (*domain.Client, error) {
	subAccount, err := s.subAccountRepo.GetSubAccount(ctx, subAccountID, parentClientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return nil, ErrSubAccountNotFound
		}
		return nil, err
	}

	return subAccount, nil
}

// DeleteSubAccount удаляет суб-аккаунт
func (s *SubAccountService) DeleteSubAccount(ctx context.Context, subAccountID, parentClientID uuid.UUID) error {
	// Проверяем, что суб-аккаунт принадлежит родителю
	_, err := s.subAccountRepo.GetSubAccount(ctx, subAccountID, parentClientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return ErrSubAccountNotFound
		}
		return err
	}

	return s.subAccountRepo.DeleteSubAccount(ctx, subAccountID)
}

// UpdateSubAccountLimits обновляет лимиты суб-аккаунта
func (s *SubAccountService) UpdateSubAccountLimits(
	ctx context.Context,
	subAccountID, parentClientID uuid.UUID,
	dailyLimit, monthlyLimit int,
) (*domain.Client, error) {
	// Проверяем принадлежность суб-аккаунта к родителю
	subAccount, err := s.subAccountRepo.GetSubAccount(ctx, subAccountID, parentClientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return nil, ErrSubAccountNotFound
		}
		return nil, err
	}

	// Обновляем rate limits через configRepo
	limits := &domain.RateLimits{
		PerDay: dailyLimit,
	}

	// Получаем текущую конфигурацию для сохранения остальных полей
	config, err := s.configRepo.GetByClientID(ctx, subAccountID)
	if err != nil {
		if err == clientrepo.ErrConfigNotFound {
			// Создаем новую конфигурацию
			config = &domain.ClientConfig{
				ID:                  uuid.New(),
				ClientID:            subAccountID,
				RateLimitPerSecond:  10,
				RateLimitPerMinute:  100,
				RateLimitPerHour:    1000,
				AllowedSources:      []string{},
				BlockedDestinations: []string{},
				Settings:            json.RawMessage("{}"),
				CreatedAt:           time.Now(),
			}
		} else {
			return nil, err
		}
	}

	config.RateLimitPerDay = limits.PerDay
	config.UpdatedAt = time.Now()

	// Обновляем месячный лимит в settings
	if monthlyLimit > 0 {
		settings := config.GetSettings()
		settings["monthly_limit"] = fmt.Sprintf("%d", monthlyLimit)
		config.SetSettings(settings)
	}

	if err := s.configRepo.Upsert(ctx, config); err != nil {
		return nil, err
	}

	subAccount.Config = config

	log.Info().
		Str("sub_account_id", subAccountID.String()).
		Int("daily_limit", dailyLimit).
		Int("monthly_limit", monthlyLimit).
		Msg("лимиты суб-аккаунта обновлены")

	return subAccount, nil
}

// CountByParentID возвращает количество суб-аккаунтов для родительского клиента
func (s *SubAccountService) CountByParentID(ctx context.Context, parentClientID uuid.UUID) (int, error) {
	return s.subAccountRepo.CountByParentID(ctx, parentClientID)
}
