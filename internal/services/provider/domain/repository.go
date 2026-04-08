package domain

import (
	"context"

	"github.com/google/uuid"
)

// ProviderRepository определяет интерфейс для работы с провайдерами
type ProviderRepository interface {
	Create(ctx context.Context, provider *Provider) error
	Update(ctx context.Context, provider *Provider) error
	GetByID(ctx context.Context, id uuid.UUID) (*Provider, error)
	GetByName(ctx context.Context, name string) (*Provider, error)
	List(ctx context.Context, activeOnly bool, limit, offset int) ([]*Provider, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// Методы для client-провайдеров
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*Provider, error)
	CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error)
	GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*Provider, error)
	LinkToClient(ctx context.Context, providerID, clientID uuid.UUID, ownership string) error
	UnlinkFromClient(ctx context.Context, providerID, clientID uuid.UUID) error
}
