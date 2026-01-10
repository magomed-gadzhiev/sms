package router

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RouteRepository описывает контракт репозитория маршрутов, нужный для Router.
type RouteRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error)
	GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error)
}

// ProviderRepository описывает контракт репозитория провайдеров, нужный для Router.
type ProviderRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error)
	GetAllActive(ctx context.Context) ([]*shared.Provider, error)
}

// Router управляет маршрутизацией сообщений к провайдерам
type Router struct {
	routeRepo  RouteRepository
	providerRepo ProviderRepository
	logger     zerolog.Logger
}

// NewRouter создает новый роутер
func NewRouter(routeRepo RouteRepository, providerRepo ProviderRepository) *Router {
	logger := log.With().Str("component", "router").Logger()
	return &Router{
		routeRepo:    routeRepo,
		providerRepo: providerRepo,
		logger:       logger,
	}
}

// RouteMessage определяет провайдера для сообщения
func (r *Router) RouteMessage(ctx context.Context, msg *shared.Message) (*shared.Provider, error) {
	// Если провайдер уже указан в сообщении, используем его
	if msg.ProviderID != nil {
		provider, err := r.providerRepo.GetByID(ctx, *msg.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения провайдера %s: %w", msg.ProviderID.String(), err)
		}
		if !provider.Active {
			return nil, fmt.Errorf("провайдер %s неактивен", provider.Name)
		}
		return provider, nil
	}

	// Если есть route_id, получаем маршрут и используем его провайдера
	if msg.RouteID != nil {
		route, err := r.routeRepo.GetByID(ctx, *msg.RouteID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения маршрута %s: %w", msg.RouteID.String(), err)
		}
		if !route.Active {
			return nil, fmt.Errorf("маршрут %s неактивен", route.Name)
		}

		provider, err := r.providerRepo.GetByID(ctx, route.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения провайдера маршрута: %w", err)
		}
		if !provider.Active {
			// Пробуем failover провайдера
			if route.FailoverProviderID != nil {
				failoverProvider, err := r.providerRepo.GetByID(ctx, *route.FailoverProviderID)
				if err != nil {
					return nil, fmt.Errorf("ошибка получения failover провайдера: %w", err)
				}
				if !failoverProvider.Active {
					return nil, fmt.Errorf("failover провайдер %s неактивен", failoverProvider.Name)
				}
				return failoverProvider, nil
			}
			return nil, fmt.Errorf("провайдер маршрута %s неактивен и нет failover провайдера", provider.Name)
		}
		return provider, nil
	}

	// Ищем маршрут по номеру назначения
	routes, err := r.routeRepo.GetActiveByDestination(ctx, msg.Destination)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения маршрутов: %w", err)
	}

	if len(routes) == 0 {
		// Если нет маршрутов, возвращаем первого активного провайдера по приоритету
		providers, err := r.providerRepo.GetAllActive(ctx)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения провайдеров: %w", err)
		}
		if len(providers) == 0 {
			return nil, fmt.Errorf("нет активных провайдеров")
		}
		return providers[0], nil
	}

	// Используем маршрут с наивысшим приоритетом (они уже отсортированы)
	route := routes[0]
	provider, err := r.providerRepo.GetByID(ctx, route.ProviderID)
	if err != nil {
		// Пробуем failover
		if route.FailoverProviderID != nil {
			provider, err = r.providerRepo.GetByID(ctx, *route.FailoverProviderID)
			if err != nil {
				return nil, fmt.Errorf("ошибка получения failover провайдера: %w", err)
			}
		} else {
			return nil, fmt.Errorf("ошибка получения провайдера маршрута: %w", err)
		}
	}

	if !provider.Active {
		// Пробуем failover
		if route.FailoverProviderID != nil && *route.FailoverProviderID != route.ProviderID {
			failoverProvider, err := r.providerRepo.GetByID(ctx, *route.FailoverProviderID)
			if err != nil {
				return nil, fmt.Errorf("ошибка получения failover провайдера: %w", err)
			}
			if !failoverProvider.Active {
				return nil, fmt.Errorf("failover провайдер %s неактивен", failoverProvider.Name)
			}
			provider = failoverProvider
		} else {
			return nil, fmt.Errorf("провайдер %s неактивен", provider.Name)
		}
	}

	r.logger.Debug().
		Str("message_id", msg.ID.String()).
		Str("destination", msg.Destination).
		Str("provider_id", provider.ID.String()).
		Str("provider_name", provider.Name).
		Str("route_id", route.ID.String()).
		Msg("сообщение маршрутизировано")

	return provider, nil
}

// GetFailoverProvider получает failover провайдера для маршрута
func (r *Router) GetFailoverProvider(ctx context.Context, routeID uuid.UUID) (*shared.Provider, error) {
	route, err := r.routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return nil, err
	}

	if route.FailoverProviderID == nil {
		return nil, fmt.Errorf("нет failover провайдера для маршрута %s", routeID.String())
	}

	provider, err := r.providerRepo.GetByID(ctx, *route.FailoverProviderID)
	if err != nil {
		return nil, err
	}

	if !provider.Active {
		return nil, fmt.Errorf("failover провайдер %s неактивен", provider.Name)
	}

	return provider, nil
}
