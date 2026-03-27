package router

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	cache "github.com/smpp-server/smpp-server/internal/pipeline/cache"
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

// CachedRouter маршрутизирует сообщения используя in-memory кеш вместо БД.
type CachedRouter struct {
	cache  *cache.RouteCache
	logger zerolog.Logger
}

// NewCachedRouter создает новый роутер на основе кеша маршрутов.
func NewCachedRouter(c *cache.RouteCache) *CachedRouter {
	return &CachedRouter{
		cache:  c,
		logger: log.With().Str("component", "cached-router").Logger(),
	}
}

// RouteMessage определяет провайдера для сообщения используя in-memory кеш.
func (r *CachedRouter) RouteMessage(ctx context.Context, msg *shared.Message) (*shared.Provider, error) {
	// Если провайдер уже указан в сообщении, используем его
	if msg.ProviderID != nil {
		provider, ok := r.cache.GetProvider(*msg.ProviderID)
		if !ok {
			return nil, fmt.Errorf("ошибка получения провайдера %s: не найден в кеше", msg.ProviderID.String())
		}
		if !provider.Active {
			return nil, fmt.Errorf("провайдер %s неактивен", provider.Name)
		}
		return provider, nil
	}

	// Если есть route_id, получаем маршрут и используем его провайдера
	if msg.RouteID != nil {
		route, ok := r.cache.GetRouteByID(*msg.RouteID)
		if !ok {
			return nil, fmt.Errorf("ошибка получения маршрута %s: не найден в кеше", msg.RouteID.String())
		}
		if !route.Active {
			return nil, fmt.Errorf("маршрут %s неактивен", route.Name)
		}

		provider, ok := r.cache.GetProvider(route.ProviderID)
		if !ok {
			return nil, fmt.Errorf("ошибка получения провайдера маршрута: не найден в кеше")
		}
		if !provider.Active {
			// Пробуем failover провайдера
			if route.FailoverProviderID != nil {
				failoverProvider, ok := r.cache.GetProvider(*route.FailoverProviderID)
				if !ok {
					return nil, fmt.Errorf("ошибка получения failover провайдера: не найден в кеше")
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
	routes := r.cache.MatchRoutes(msg.Destination)

	if len(routes) == 0 {
		// Если нет маршрутов, возвращаем первого активного провайдера по приоритету
		allRoutes := r.cache.GetAllActiveRoutes()
		providers := make(map[uuid.UUID]*shared.Provider)
		for _, rt := range allRoutes {
			if p, ok := r.cache.GetProvider(rt.ProviderID); ok && p.Active {
				providers[p.ID] = p
			}
		}
		if len(providers) == 0 {
			return nil, fmt.Errorf("нет активных провайдеров")
		}
		// Возвращаем первого найденного активного провайдера
		for _, p := range providers {
			return p, nil
		}
	}

	// Используем маршрут с наивысшим приоритетом (они уже отсортированы)
	route := routes[0]
	provider, ok := r.cache.GetProvider(route.ProviderID)
	if !ok {
		// Пробуем failover
		if route.FailoverProviderID != nil {
			provider, ok = r.cache.GetProvider(*route.FailoverProviderID)
			if !ok {
				return nil, fmt.Errorf("ошибка получения failover провайдера: не найден в кеше")
			}
		} else {
			return nil, fmt.Errorf("ошибка получения провайдера маршрута: не найден в кеше")
		}
	}

	if !provider.Active {
		// Пробуем failover
		if route.FailoverProviderID != nil && *route.FailoverProviderID != route.ProviderID {
			failoverProvider, ok := r.cache.GetProvider(*route.FailoverProviderID)
			if !ok {
				return nil, fmt.Errorf("ошибка получения failover провайдера: не найден в кеше")
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

// GetFailoverProvider получает failover провайдера для маршрута используя кеш.
func (r *CachedRouter) GetFailoverProvider(ctx context.Context, routeID uuid.UUID) (*shared.Provider, error) {
	route, ok := r.cache.GetRouteByID(routeID)
	if !ok {
		return nil, fmt.Errorf("маршрут %s не найден в кеше", routeID.String())
	}

	if route.FailoverProviderID == nil {
		return nil, fmt.Errorf("нет failover провайдера для маршрута %s", routeID.String())
	}

	provider, ok := r.cache.GetProvider(*route.FailoverProviderID)
	if !ok {
		return nil, fmt.Errorf("failover провайдер не найден в кеше")
	}

	if !provider.Active {
		return nil, fmt.Errorf("failover провайдер %s неактивен", provider.Name)
	}

	return provider, nil
}
