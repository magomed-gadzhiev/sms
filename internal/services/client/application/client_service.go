package application

import (
	"context"
	"encoding/json"
	"errors"
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

// ClientService предоставляет методы для управления клиентами
type ClientService struct {
	clientRepo *clientrepo.ClientRepository
	configRepo *clientrepo.ConfigRepository
}

// NewClientService создает новый сервис управления клиентами
func NewClientService(
	clientRepo *clientrepo.ClientRepository,
	configRepo *clientrepo.ConfigRepository,
) *ClientService {
	return &ClientService{
		clientRepo: clientRepo,
		configRepo: configRepo,
	}
}

// CreateClient создает нового клиента
func (s *ClientService) CreateClient(
	ctx context.Context,
	name, email, contactPerson, phone string,
	active bool,
	metadata map[string]string,
) (*domain.Client, error) {
	// Валидация
	if name == "" {
		return nil, ErrInvalidClientData
	}

	// Создаем клиента
	client := &domain.Client{
		ID:            uuid.New(),
		Name:          name,
		Email:         email,
		ContactPerson: contactPerson,
		Phone:         phone,
		Active:        active,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
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

// UpdateClient обновляет клиента
func (s *ClientService) UpdateClient(
	ctx context.Context,
	clientID uuid.UUID,
	name, email, contactPerson, phone *string,
	active *bool,
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
