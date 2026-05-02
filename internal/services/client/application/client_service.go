package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	clientrepo "github.com/smpp-server/smpp-server/internal/services/client/infrastructure/repository"
)

var (
	ErrClientNotFound     = errors.New("client not found")
	ErrInvalidClientData  = errors.New("invalid client data")
	ErrConfigNotFound     = errors.New("client config not found")
)

// PlanRepositoryInterface определяет интерфейс для работы с тарифными планами
type PlanRepositoryInterface interface {
	ListActive(ctx context.Context) ([]*domain.Plan, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error)
}

// ClientRepositoryInterface определяет интерфейс для работы с клиентами
type ClientRepositoryInterface interface {
	Create(ctx context.Context, client *domain.Client) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Client, error)
	Update(ctx context.Context, client *domain.Client) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, activeOnly bool, search string, limit, offset int) ([]*domain.Client, int, error)
	AssignPlan(ctx context.Context, clientID uuid.UUID, planID uuid.UUID) error
}

// ConfigRepositoryInterface определяет интерфейс для работы с конфигурациями
type ConfigRepositoryInterface interface {
	Create(ctx context.Context, config *domain.ClientConfig) error
	GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.ClientConfig, error)
	Update(ctx context.Context, config *domain.ClientConfig) error
	Upsert(ctx context.Context, config *domain.ClientConfig) error
	UpdateRateLimits(ctx context.Context, clientID uuid.UUID, limits *domain.RateLimits) error
}

// ClientService предоставляет методы для управления клиентами
type ClientService struct {
	clientRepo ClientRepositoryInterface
	configRepo ConfigRepositoryInterface
	planRepo   PlanRepositoryInterface
}

// NewClientService создает новый сервис управления клиентами
func NewClientService(
	clientRepo ClientRepositoryInterface,
	configRepo ConfigRepositoryInterface,
	planRepo PlanRepositoryInterface,
) *ClientService {
	return &ClientService{
		clientRepo: clientRepo,
		configRepo: configRepo,
		planRepo:   planRepo,
	}
}

// CreateClient создает нового клиента.
// isReseller=true допустим только для top-level клиентов (parent_client_id IS NULL).
// БД-инвариант защищён CHECK-constraint в migrations/000016_add_sub_accounts.up.sql.
func (s *ClientService) CreateClient(
	ctx context.Context,
	name, email, contactPerson, phone string,
	active bool,
	isSandbox bool,
	isReseller bool,
	maxSubAccounts int,
	metadata map[string]string,
) (*domain.Client, error) {
	// Валидация
	if name == "" {
		return nil, ErrInvalidClientData
	}
	if maxSubAccounts < 0 {
		return nil, ErrInvalidClientData
	}
	// Реселлер без слотов — мусорное состояние (флаг есть, толку нет). БД CHECK
	// этого не ловит (constraint только для top-level invariant).
	if isReseller && maxSubAccounts < 1 {
		return nil, ErrInvalidClientData
	}

	// Генерируем API key и secret
	apiKeyBytes := make([]byte, 16)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(apiKeyBytes); err != nil {
		return nil, err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}

	// Создаем клиента
	client := &domain.Client{
		ID:             uuid.New(),
		Name:           name,
		APIKey:         "ak-" + hex.EncodeToString(apiKeyBytes),
		Secret:         hex.EncodeToString(secretBytes),
		Email:          email,
		ContactPerson:  contactPerson,
		Phone:          phone,
		Active:         active,
		IsSandbox:      isSandbox,
		IsReseller:     isReseller,
		MaxSubAccounts: maxSubAccounts,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Устанавливаем метаданные
	if metadata != nil {
		if err := client.SetMetadata(metadata); err != nil {
			return nil, err
		}
	} else {
		client.Metadata = json.RawMessage("{}")
	}

	// Сохраняем клиента
	if err := s.clientRepo.Create(ctx, client); err != nil {
		return nil, err
	}

	// Создаем конфигурацию по умолчанию
	defaultConfig := &domain.ClientConfig{
		ID:                 uuid.New(),
		ClientID:           client.ID,
		RateLimitPerSecond: 10,
		RateLimitPerMinute: 100,
		RateLimitPerHour:   1000,
		RateLimitPerDay:    10000,
		AllowedSources:     []string{},
		BlockedDestinations: []string{},
		Settings:           json.RawMessage("{}"),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.configRepo.Create(ctx, defaultConfig); err != nil {
		log.Warn().Err(err).Msg("не удалось создать конфигурацию по умолчанию")
	}

	client.Config = defaultConfig

	return client, nil
}

// UpdateClient обновляет клиента.
// isReseller и maxSubAccounts опциональны — nil означает "не менять".
// БД-инвариант (is_reseller только для top-level) защищён CHECK-constraint.
func (s *ClientService) UpdateClient(
	ctx context.Context,
	clientID uuid.UUID,
	name, email, contactPerson, phone *string,
	active *bool,
	isReseller *bool,
	maxSubAccounts *int,
	metadata map[string]string,
) (*domain.Client, error) {
	// Получаем текущего клиента
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return nil, ErrClientNotFound
		}
		return nil, err
	}

	// Обновляем поля
	if name != nil {
		client.Name = *name
	}
	if email != nil {
		client.Email = *email
	}
	if contactPerson != nil {
		client.ContactPerson = *contactPerson
	}
	if phone != nil {
		client.Phone = *phone
	}
	if active != nil {
		client.Active = *active
	}
	if isReseller != nil {
		client.IsReseller = *isReseller
	}
	if maxSubAccounts != nil {
		if *maxSubAccounts < 0 {
			return nil, ErrInvalidClientData
		}
		client.MaxSubAccounts = *maxSubAccounts
	}
	// Та же бизнес-инварианта что и в CreateClient: реселлер требует ≥1 слота.
	// Проверяем итоговое состояние клиента (после применения PATCH-полей).
	if client.IsReseller && client.MaxSubAccounts < 1 {
		return nil, ErrInvalidClientData
	}
	// БД-инвариант chk_reseller_is_top_level: is_reseller=true допустим только
	// для top-level (parent_client_id IS NULL). Без pre-validation БД CHECK
	// проброс через repo даёт codes.Internal → HTTP 500. Здесь — 400 явно.
	if client.IsReseller && client.ParentClientID != nil {
		return nil, ErrInvalidClientData
	}
	if metadata != nil {
		if err := client.SetMetadata(metadata); err != nil {
			return nil, err
		}
	}

	client.UpdatedAt = time.Now()

	// Сохраняем изменения
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return nil, err
	}

	return client, nil
}

// GetClient получает клиента по ID
func (s *ClientService) GetClient(ctx context.Context, clientID uuid.UUID) (*domain.Client, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return nil, ErrClientNotFound
		}
		return nil, err
	}

	// Загружаем конфигурацию
	config, err := s.configRepo.GetByClientID(ctx, clientID)
	if err != nil && err != clientrepo.ErrConfigNotFound {
		return nil, err
	}
	if config != nil {
		client.Config = config
	}

	return client, nil
}

// ListClients получает список клиентов
func (s *ClientService) ListClients(
	ctx context.Context,
	activeOnly bool,
	search string,
	limit, offset int,
) ([]*domain.Client, int, error) {
	return s.clientRepo.List(ctx, activeOnly, search, limit, offset)
}

// DeleteClient удаляет клиента
func (s *ClientService) DeleteClient(ctx context.Context, clientID uuid.UUID) error {
	return s.clientRepo.Delete(ctx, clientID)
}

// ToggleSandbox включает или отключает sandbox-режим для клиента
func (s *ClientService) ToggleSandbox(ctx context.Context, clientID uuid.UUID, enable bool) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return ErrClientNotFound
		}
		return err
	}
	client.IsSandbox = enable
	client.UpdatedAt = time.Now()
	return s.clientRepo.Update(ctx, client)
}

// GetClientConfig получает конфигурацию клиента
func (s *ClientService) GetClientConfig(ctx context.Context, clientID uuid.UUID) (*domain.ClientConfig, error) {
	config, err := s.configRepo.GetByClientID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrConfigNotFound {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	return config, nil
}

// UpdateClientConfig обновляет конфигурацию клиента
func (s *ClientService) UpdateClientConfig(
	ctx context.Context,
	clientID uuid.UUID,
	config *domain.ClientConfig,
) error {
	// Проверяем существование клиента
	_, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return ErrClientNotFound
		}
		return err
	}

	// Устанавливаем client_id и обновляем время
	config.ClientID = clientID
	config.UpdatedAt = time.Now()

	// Используем upsert для создания или обновления
	return s.configRepo.Upsert(ctx, config)
}

// ListPlans возвращает список активных тарифных планов
func (s *ClientService) ListPlans(ctx context.Context) ([]*domain.Plan, error) {
	return s.planRepo.ListActive(ctx)
}

// AssignPlan назначает тарифный план клиенту
func (s *ClientService) AssignPlan(ctx context.Context, clientID uuid.UUID, planID uuid.UUID) error {
	// Проверяем что клиент существует
	_, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return ErrClientNotFound
		}
		return err
	}
	// Проверяем что план существует и активен
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}
	if !plan.Active {
		return fmt.Errorf("plan is not active")
	}
	return s.clientRepo.AssignPlan(ctx, clientID, planID)
}

// UpdateClientRateLimits обновляет rate limits клиента
func (s *ClientService) UpdateClientRateLimits(
	ctx context.Context,
	clientID uuid.UUID,
	limits *domain.RateLimits,
) error {
	// Проверяем существование клиента
	_, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return ErrClientNotFound
		}
		return err
	}

	// Пробуем обновить
	err = s.configRepo.UpdateRateLimits(ctx, clientID, limits)
	if err == clientrepo.ErrConfigNotFound {
		// Если конфигурации нет, создаем новую
		config := &domain.ClientConfig{
			ID:                 uuid.New(),
			ClientID:           clientID,
			RateLimitPerSecond: limits.PerSecond,
			RateLimitPerMinute: limits.PerMinute,
			RateLimitPerHour:   limits.PerHour,
			RateLimitPerDay:    limits.PerDay,
			AllowedSources:     []string{},
			BlockedDestinations: []string{},
			Settings:           json.RawMessage("{}"),
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		return s.configRepo.Create(ctx, config)
	}

	return err
}
