package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// ProviderService предоставляет бизнес-логику для работы с провайдерами
type ProviderService struct {
	repo domain.ProviderRepository
}

// NewProviderService создает новый сервис провайдеров
func NewProviderService(repo domain.ProviderRepository) *ProviderService {
	return &ProviderService{
		repo: repo,
	}
}

// CreateProvider создает нового провайдера
func (s *ProviderService) CreateProvider(ctx context.Context, p *domain.Provider) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("валидация провайдера: %w", err)
	}

	// Проверяем, существует ли провайдер с таким именем
	existing, err := s.repo.GetByName(ctx, p.Name)
	if err == nil && existing != nil {
		return fmt.Errorf("провайдер с именем %s уже существует", p.Name)
	}

	now := time.Now()
	p.ID = uuid.New()
	p.CreatedAt = now
	p.UpdatedAt = now

	if err := s.repo.Create(ctx, p); err != nil {
		return fmt.Errorf("создание провайдера: %w", err)
	}

	log.Info().
		Str("provider_id", p.ID.String()).
		Str("name", p.Name).
		Msg("провайдер создан")

	return nil
}

// UpdateProvider обновляет провайдера
func (s *ProviderService) UpdateProvider(ctx context.Context, id uuid.UUID, updates *domain.Provider) error {
	// Получаем существующего провайдера
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("получение провайдера: %w", err)
	}

	// Обновляем поля
	if updates.Name != "" {
		// Проверяем уникальность имени, если оно изменилось
		if updates.Name != existing.Name {
			duplicate, err := s.repo.GetByName(ctx, updates.Name)
			if err == nil && duplicate != nil && duplicate.ID != id {
				return fmt.Errorf("провайдер с именем %s уже существует", updates.Name)
			}
		}
		existing.Name = updates.Name
	}
	if updates.Host != "" {
		existing.Host = updates.Host
	}
	if updates.Port > 0 {
		existing.Port = updates.Port
	}
	if updates.SystemID != "" {
		existing.SystemID = updates.SystemID
	}
	if updates.Password != "" {
		existing.Password = updates.Password
	}
	if updates.SystemType != "" {
		existing.SystemType = updates.SystemType
	}
	if updates.BindType != "" {
		existing.BindType = updates.BindType
	}
	if updates.MaxConnections > 0 {
		existing.MaxConnections = updates.MaxConnections
	}
	// Active и Priority могут быть явно установлены в false/0
	existing.Active = updates.Active
	existing.Priority = updates.Priority
	existing.ThroughputPerSec = updates.ThroughputPerSec

	if err := existing.Validate(); err != nil {
		return fmt.Errorf("валидация обновленного провайдера: %w", err)
	}

	existing.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, existing); err != nil {
		return fmt.Errorf("обновление провайдера: %w", err)
	}

	log.Info().
		Str("provider_id", id.String()).
		Msg("провайдер обновлен")

	return nil
}

// GetProvider получает провайдера по ID
func (s *ProviderService) GetProvider(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("получение провайдера: %w", err)
	}
	return provider, nil
}

// ListProviders получает список провайдеров
func (s *ProviderService) ListProviders(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Provider, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	providers, total, err := s.repo.List(ctx, activeOnly, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("получение списка провайдеров: %w", err)
	}

	return providers, total, nil
}

// DeleteProvider удаляет провайдера (мягкое удаление)
func (s *ProviderService) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("получение провайдера: %w", err)
	}

	// Мягкое удаление - помечаем как неактивный
	provider.Active = false
	provider.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, provider); err != nil {
		return fmt.Errorf("удаление провайдера: %w", err)
	}

	log.Info().
		Str("provider_id", id.String()).
		Msg("провайдер удален (деактивирован)")

	return nil
}
