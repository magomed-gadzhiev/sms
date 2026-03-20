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

// CountryRepository определяет интерфейс репозитория стран
type CountryRepository interface {
	Create(ctx context.Context, country *Country) error
	GetByID(ctx context.Context, id uuid.UUID) (*Country, error)
	GetByISOCode(ctx context.Context, isoCode string) (*Country, error)
	Update(ctx context.Context, country *Country) error
	List(ctx context.Context, limit, offset int) ([]*Country, int, error)
}

// OperatorRepository определяет интерфейс репозитория операторов
type OperatorRepository interface {
	Create(ctx context.Context, operator *Operator) error
	GetByID(ctx context.Context, id uuid.UUID) (*Operator, error)
	GetByCode(ctx context.Context, code string) (*Operator, error)
	Update(ctx context.Context, operator *Operator) error
	List(ctx context.Context, countryID *uuid.UUID, activeOnly bool, limit, offset int) ([]*Operator, int, error)
}

// OperatorPrefixRepository определяет интерфейс репозитория номерных префиксов
type OperatorPrefixRepository interface {
	Create(ctx context.Context, prefix *OperatorPrefix) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOperatorID(ctx context.Context, operatorID uuid.UUID) ([]*OperatorPrefix, error)
	FindByNumber(ctx context.Context, phoneNumber string) (*OperatorPrefix, error)
}