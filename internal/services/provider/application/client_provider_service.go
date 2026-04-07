package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// ClientProviderService управляет SMPP-провайдерами, принадлежащими клиентам
type ClientProviderService struct {
	repo         domain.ProviderRepository
	maxPerClient int
}

func NewClientProviderService(repo domain.ProviderRepository, maxPerClient int) *ClientProviderService {
	return &ClientProviderService{repo: repo, maxPerClient: maxPerClient}
}

func (s *ClientProviderService) Create(ctx context.Context, clientID uuid.UUID, p *domain.Provider) (*domain.Provider, error) {
	count, err := s.repo.CountByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("проверка лимита провайдеров: %w", err)
	}
	if count >= s.maxPerClient {
		return nil, domain.ErrProviderLimitExceeded
	}

	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("валидация провайдера: %w", err)
	}

	now := time.Now()
	p.ID = uuid.New()
	p.ClientID = &clientID
	p.Active = true
	p.CreatedAt = now
	p.UpdatedAt = now

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("создание провайдера: %w", err)
	}

	if err := s.repo.LinkToClient(ctx, p.ID, clientID, "private"); err != nil {
		return nil, fmt.Errorf("привязка провайдера к клиенту: %w", err)
	}

	return p, nil
}

func (s *ClientProviderService) List(ctx context.Context, clientID uuid.UUID) ([]*domain.Provider, error) {
	return s.repo.ListByClientID(ctx, clientID)
}

func (s *ClientProviderService) Get(ctx context.Context, id, clientID uuid.UUID) (*domain.Provider, error) {
	return s.repo.GetByIDAndClientID(ctx, id, clientID)
}

func (s *ClientProviderService) Update(ctx context.Context, id, clientID uuid.UUID, updates *domain.Provider) (*domain.Provider, error) {
	existing, err := s.repo.GetByIDAndClientID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Description != "" {
		existing.Description = updates.Description
	}
	if len(updates.Tags) > 0 {
		existing.Tags = updates.Tags
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
	if updates.BindType != "" {
		existing.BindType = updates.BindType
	}
	if updates.WindowSize > 0 {
		existing.WindowSize = updates.WindowSize
	}
	if updates.MaxConnections > 0 {
		existing.MaxConnections = updates.MaxConnections
	}
	if updates.TPSLimit > 0 {
		existing.TPSLimit = updates.TPSLimit
	}
	if len(updates.RoutingRules) > 0 {
		existing.RoutingRules = updates.RoutingRules
	}

	existing.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("обновление провайдера: %w", err)
	}
	return existing, nil
}

func (s *ClientProviderService) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	existing, err := s.repo.GetByIDAndClientID(ctx, id, clientID)
	if err != nil {
		return err
	}
	existing.Active = false
	existing.UpdatedAt = time.Now()
	return s.repo.Update(ctx, existing)
}
