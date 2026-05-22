package domain

import (
	"context"

	"github.com/google/uuid"
)

type ClientProviderRepository interface {
	Create(ctx context.Context, cp *ClientProvider) error
	GetByID(ctx context.Context, id uuid.UUID) (*ClientProvider, error)
	GetByClientAndProvider(ctx context.Context, clientID, providerID uuid.UUID) (*ClientProvider, error)
	ListByClient(ctx context.Context, clientID uuid.UUID, activeOnly bool) ([]*ClientProvider, error)
	ListBySourceClient(ctx context.Context, sourceClientID uuid.UUID) ([]*ClientProvider, error)
	Update(ctx context.Context, cp *ClientProvider) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ClientRouteRepository interface {
	Create(ctx context.Context, route *ClientRoute) error
	GetByID(ctx context.Context, id uuid.UUID) (*ClientRoute, error)
	ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID, activeOnly bool) ([]*ClientRoute, error)
	ListByClient(ctx context.Context, clientID uuid.UUID) ([]*ClientRoute, error)
	Update(ctx context.Context, route *ClientRoute) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ClientRoutingStrategyRepository interface {
	Upsert(ctx context.Context, strategy *ClientRoutingStrategy) error
	Get(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) (*ClientRoutingStrategy, error)
	Delete(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) error
}
