package domain

import (
	"context"

	"github.com/google/uuid"
)

// RouteRepository определяет интерфейс репозитория маршрутов
type RouteRepository interface {
	Create(ctx context.Context, route *Route) error
	GetByID(ctx context.Context, id uuid.UUID) (*Route, error)
	Update(ctx context.Context, route *Route) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, activeOnly bool, limit, offset int) ([]*Route, int, error)
	GetActiveByDestination(ctx context.Context, destination string) ([]*Route, error)
}

// ProviderRepository определяет интерфейс репозитория провайдеров (используется из внешнего сервиса)
type ProviderRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderInfo, error)
	GetAllActive(ctx context.Context) ([]*ProviderInfo, error)
	GetHealth(ctx context.Context, id uuid.UUID) (*ProviderHealth, error)
}

// ProviderInfo представляет информацию о провайдере
type ProviderInfo struct {
	ID              uuid.UUID
	Name            string
	Active          bool
	Priority        int
	ThroughputPerSec int
}

// ProviderHealth представляет информацию о здоровье провайдера
type ProviderHealth struct {
	Status           string
	ActiveConnections int
	TotalConnections  int
	SuccessRate      int
	LastSuccess      *int64
	LastFailure      *int64
}