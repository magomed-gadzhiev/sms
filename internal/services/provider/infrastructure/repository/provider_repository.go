package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// ProviderRepositoryAdapter адаптирует storage.ProviderRepository к domain.ProviderRepository
type ProviderRepositoryAdapter struct {
	repo *storage.ProviderRepository
}

// NewProviderRepositoryAdapter создает новый адаптер репозитория
func NewProviderRepositoryAdapter(repo *storage.ProviderRepository) *ProviderRepositoryAdapter {
	return &ProviderRepositoryAdapter{
		repo: repo,
	}
}

// Create создает нового провайдера
func (a *ProviderRepositoryAdapter) Create(ctx context.Context, p *domain.Provider) error {
	sharedProvider := domainToShared(p)
	return a.repo.Create(ctx, sharedProvider)
}

// Update обновляет провайдера
func (a *ProviderRepositoryAdapter) Update(ctx context.Context, p *domain.Provider) error {
	sharedProvider := domainToShared(p)
	return a.repo.Update(ctx, sharedProvider)
}

// GetByID получает провайдера по ID
func (a *ProviderRepositoryAdapter) GetByID(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	sharedProvider, err := a.repo.GetByID(ctx, id)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, domain.ErrProviderNotFound
		}
		return nil, err
	}
	return sharedToDomain(sharedProvider), nil
}

// GetByName получает провайдера по имени
func (a *ProviderRepositoryAdapter) GetByName(ctx context.Context, name string) (*domain.Provider, error) {
	sharedProvider, err := a.repo.GetByName(ctx, name)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, domain.ErrProviderNotFound
		}
		return nil, err
	}
	return sharedToDomain(sharedProvider), nil
}

// List получает список провайдеров
func (a *ProviderRepositoryAdapter) List(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Provider, int, error) {
	var sharedProviders []*shared.Provider
	var err error

	if activeOnly {
		sharedProviders, err = a.repo.GetAllActive(ctx)
	} else {
		sharedProviders, err = a.repo.GetAll(ctx)
	}

	if err != nil {
		return nil, 0, err
	}

	total := len(sharedProviders)

	// Применяем пагинацию
	start := offset
	end := offset + limit
	if start > total {
		return []*domain.Provider{}, total, nil
	}
	if end > total {
		end = total
	}

	providers := make([]*domain.Provider, 0, end-start)
	for i := start; i < end; i++ {
		providers = append(providers, sharedToDomain(sharedProviders[i]))
	}

	return providers, total, nil
}

// Delete удаляет провайдера
func (a *ProviderRepositoryAdapter) Delete(ctx context.Context, id uuid.UUID) error {
	return a.repo.Delete(ctx, id)
}

// domainToShared преобразует domain.Provider в shared.Provider
func domainToShared(p *domain.Provider) *shared.Provider {
	rulesJSON, _ := json.Marshal(p.RoutingRules)
	return &shared.Provider{
		ID:               p.ID,
		Name:             p.Name,
		Host:             p.Host,
		Port:             p.Port,
		SystemID:         p.SystemID,
		Password:         p.Password,
		SystemType:       p.SystemType,
		BindType:         string(p.BindType),
		BindTON:          p.BindTON,
		BindNPI:          p.BindNPI,
		AddrTON:          p.AddrTON,
		AddrNPI:          p.AddrNPI,
		AddressRange:     p.AddressRange,
		MaxConnections:   p.MaxConnections,
		Active:           p.Active,
		Priority:         p.Priority,
		ThroughputPerSec: p.ThroughputPerSec,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		ClientID:         p.ClientID,
		Description:      p.Description,
		Tags:             shared.StringArray(p.Tags),
		TPSLimit:         p.TPSLimit,
		RoutingRules:     rulesJSON,
	}
}

// sharedToDomain преобразует shared.Provider в domain.Provider
func sharedToDomain(p *shared.Provider) *domain.Provider {
	d := &domain.Provider{
		ID:               p.ID,
		Name:             p.Name,
		Host:             p.Host,
		Port:             p.Port,
		SystemID:         p.SystemID,
		Password:         p.Password,
		SystemType:       p.SystemType,
		BindType:         domain.BindType(p.BindType),
		BindTON:          p.BindTON,
		BindNPI:          p.BindNPI,
		AddrTON:          p.AddrTON,
		AddrNPI:          p.AddrNPI,
		AddressRange:     p.AddressRange,
		MaxConnections:   p.MaxConnections,
		Active:           p.Active,
		Priority:         p.Priority,
		ThroughputPerSec: p.ThroughputPerSec,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		ClientID:         p.ClientID,
		Description:      p.Description,
		Tags:             []string(p.Tags),
		TPSLimit:         p.TPSLimit,
	}
	if len(p.RoutingRules) > 0 {
		_ = json.Unmarshal(p.RoutingRules, &d.RoutingRules)
	}
	return d
}

// LinkToClient создаёт связь провайдера с клиентом
func (a *ProviderRepositoryAdapter) LinkToClient(ctx context.Context, providerID, clientID uuid.UUID, ownership string) error {
	return a.repo.LinkToClient(ctx, providerID, clientID, ownership)
}

// ListByClientID возвращает провайдеров клиента
func (a *ProviderRepositoryAdapter) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Provider, error) {
	providers, err := a.repo.ListByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Provider, len(providers))
	for i, p := range providers {
		result[i] = sharedToDomain(p)
	}
	return result, nil
}

// CountByClientID считает провайдеров клиента
func (a *ProviderRepositoryAdapter) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	return a.repo.CountByClientID(ctx, clientID)
}

// GetByIDAndClientID получает провайдера с проверкой принадлежности клиенту
func (a *ProviderRepositoryAdapter) GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*domain.Provider, error) {
	p, err := a.repo.GetByIDAndClientID(ctx, id, clientID)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, domain.ErrProviderNotFound
		}
		return nil, err
	}
	return sharedToDomain(p), nil
}
