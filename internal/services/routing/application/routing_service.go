package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// RoutingService предоставляет бизнес-логику для маршрутизации сообщений
type RoutingService struct {
	routeRepo      domain.RouteRepository
	providerRepo   domain.ProviderRepository
	eventPublisher domain.EventPublisher
	selectors      map[domain.LoadBalanceStrategy]ProviderSelector
	hlrService     *HLRService
	smartRouter    *SmartRoutingService
	logger         zerolog.Logger
}

// NewRoutingService создает новый сервис маршрутизации
func NewRoutingService(
	routeRepo domain.RouteRepository,
	providerRepo domain.ProviderRepository,
	eventPublisher domain.EventPublisher,
) *RoutingService {
	selectors := make(map[domain.LoadBalanceStrategy]ProviderSelector)
	
	// Инициализируем все селекторы
	selectors[domain.LoadBalanceRoundRobin] = NewRoundRobinSelector()
	selectors[domain.LoadBalanceLeastLoaded] = NewLeastLoadedSelector(providerRepo)
	selectors[domain.LoadBalanceCheapest] = NewCheapestSelector()

	return &RoutingService{
		routeRepo:      routeRepo,
		providerRepo:   providerRepo,
		eventPublisher: eventPublisher,
		selectors:      selectors,
		logger:         log.With().Str("component", "routing-service").Logger(),
	}
}

// GetRoute получает маршрут для сообщения по номеру назначения
func (s *RoutingService) GetRoute(
	ctx context.Context,
	destination string,
	clientID *uuid.UUID,
) (*domain.Route, error) {
	// Ищем активные маршруты для этого номера
	routes, err := s.routeRepo.GetActiveByDestination(ctx, destination)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения маршрутов: %w", err)
	}

	if len(routes) == 0 {
		return nil, domain.ErrNoMatchingRoute
	}

	// Возвращаем маршрут с наивысшим приоритетом (они уже отсортированы)
	return routes[0], nil
}

// SelectProvider выбирает оптимального провайдера для маршрута
func (s *RoutingService) SelectProvider(
	ctx context.Context,
	route *domain.Route,
	clientID *uuid.UUID,
) (uuid.UUID, error) {
	if len(route.ProviderIDs) == 0 {
		return uuid.Nil, domain.ErrNoProviders
	}

	// Получаем информацию о провайдерах маршрута
	providerInfos := make([]*domain.ProviderInfo, 0, len(route.ProviderIDs))
	for _, providerID := range route.ProviderIDs {
		providerInfo, err := s.providerRepo.GetByID(ctx, providerID)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("provider_id", providerID.String()).
				Msg("не удалось получить информацию о провайдере, пропускаем")
			continue
		}
		providerInfos = append(providerInfos, providerInfo)
	}

	if len(providerInfos) == 0 {
		return uuid.Nil, domain.ErrNoProviders
	}

	// Получаем селектор для стратегии маршрута
	selector, ok := s.selectors[route.LoadBalanceStrategy]
	if !ok {
		// Если стратегия неизвестна, используем round-robin
		selector = s.selectors[domain.LoadBalanceRoundRobin]
		s.logger.Warn().
			Str("strategy", string(route.LoadBalanceStrategy)).
			Msg("неизвестная стратегия, используется round-robin")
	}

	// Выбираем провайдера
	selectedProvider, err := selector.SelectProvider(ctx, route, providerInfos)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка выбора провайдера: %w", err)
	}

	// Если включен failover и основной провайдер недоступен, пробуем резервный
	if route.FailoverEnabled && !selectedProvider.Active && len(providerInfos) > 1 {
		// Пробуем найти другой активный провайдер
		for _, provider := range providerInfos {
			if provider.ID != selectedProvider.ID && provider.Active {
				s.logger.Info().
					Str("route_id", route.ID.String()).
					Str("primary_provider_id", selectedProvider.ID.String()).
					Str("failover_provider_id", provider.ID.String()).
					Msg("использован failover провайдер")
				selectedProvider = provider
				break
			}
		}
	}

	if !selectedProvider.Active {
		return uuid.Nil, domain.ErrProviderUnavailable
	}

	return selectedProvider.ID, nil
}

// RouteMessage маршрутизирует сообщение (получает маршрут и выбирает провайдера)
func (s *RoutingService) RouteMessage(
	ctx context.Context,
	messageID uuid.UUID,
	destination string,
	clientID *uuid.UUID,
	existingRouteID *uuid.UUID,
	existingProviderID *uuid.UUID,
) (uuid.UUID, uuid.UUID, error) {
	var route *domain.Route
	var err error

	// Если маршрут уже указан, используем его
	if existingRouteID != nil {
		route, err = s.routeRepo.GetByID(ctx, *existingRouteID)
		if err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("ошибка получения маршрута: %w", err)
		}
		if !route.Active {
			return uuid.Nil, uuid.Nil, fmt.Errorf("маршрут %s неактивен", route.Name)
		}
	} else {
		// Ищем подходящий маршрут
		route, err = s.GetRoute(ctx, destination, clientID)
		if err != nil {
			// Если маршрут не найден, но есть провайдер - используем его
			if existingProviderID != nil {
				return uuid.Nil, *existingProviderID, nil
			}
			return uuid.Nil, uuid.Nil, fmt.Errorf("ошибка получения маршрута: %w", err)
		}
	}

	// Если провайдер уже указан и он в списке провайдеров маршрута, используем его
	if existingProviderID != nil {
		for _, providerID := range route.ProviderIDs {
			if providerID == *existingProviderID {
				providerInfo, err := s.providerRepo.GetByID(ctx, *existingProviderID)
				if err == nil && providerInfo.Active {
					// Публикуем событие маршрутизации
					if pubErr := s.eventPublisher.PublishMessageRouted(ctx, messageID, route.ID, *existingProviderID); pubErr != nil {
						s.logger.Warn().Err(pubErr).Msg("ошибка публикации события message.routed")
					}
					return route.ID, *existingProviderID, nil
				}
			}
		}
	}

	// Выбираем провайдера по стратегии
	providerID, err := s.SelectProvider(ctx, route, clientID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("ошибка выбора провайдера: %w", err)
	}

	// Публикуем событие маршрутизации
	if err := s.eventPublisher.PublishMessageRouted(ctx, messageID, route.ID, providerID); err != nil {
		s.logger.Warn().Err(err).Msg("ошибка публикации события message.routed")
	}

	s.logger.Debug().
		Str("message_id", messageID.String()).
		Str("route_id", route.ID.String()).
		Str("provider_id", providerID.String()).
		Str("strategy", string(route.LoadBalanceStrategy)).
		Msg("сообщение маршрутизировано")

	return route.ID, providerID, nil
}

// SetHLRService устанавливает HLR сервис для маршрутизации
func (s *RoutingService) SetHLRService(hlr *HLRService) {
	s.hlrService = hlr
}

// SetSmartRouter устанавливает сервис smart routing
func (s *RoutingService) SetSmartRouter(sr *SmartRoutingService) {
	s.smartRouter = sr
}

// RouteMessageResult представляет результат маршрутизации с HLR данными
type RouteMessageResult struct {
	RouteID      uuid.UUID
	ProviderID   uuid.UUID
	HLRResult    *domain.LookupResult
	HLRUsed      bool
	RoutingScore float64
}

// RouteMessageWithHLR маршрутизирует сообщение с предварительным HLR lookup
func (s *RoutingService) RouteMessageWithHLR(
	ctx context.Context,
	messageID uuid.UUID,
	destination string,
	clientID *uuid.UUID,
	existingRouteID *uuid.UUID,
	existingProviderID *uuid.UUID,
) (*RouteMessageResult, error) {
	result := &RouteMessageResult{}

	// HLR lookup если сервис сконфигурирован
	if s.hlrService != nil && clientID != nil {
		hlrResult, err := s.hlrService.LookupNumber(
			ctx, destination, false, *clientID,
			messageID.String(), domain.LookupSourceSMSRouting, &messageID,
		)
		if err == nil && hlrResult != nil {
			result.HLRResult = hlrResult
			result.HLRUsed = true

			// Блокируем невалидные номера
			if hlrResult.IsInvalid() {
				s.logger.Info().
					Str("message_id", messageID.String()).
					Str("destination", destination).
					Str("status", string(hlrResult.NumberStatus)).
					Msg("номер определён как invalid, отправка заблокирована")
				return nil, domain.ErrNumberInvalid
			}
		}
		// Если HLR не удался — продолжаем с prefix-based routing (fallback)
		if err != nil {
			s.logger.Debug().Err(err).
				Str("message_id", messageID.String()).
				Str("destination", destination).
				Msg("HLR lookup не удался, fallback на prefix routing")
		}
	}

	// Основная маршрутизация (существующая логика)
	routeID, providerID, err := s.RouteMessage(ctx, messageID, destination, clientID, existingRouteID, existingProviderID)
	if err != nil {
		return nil, err
	}

	result.RouteID = routeID
	result.ProviderID = providerID

	return result, nil
}

// CreateRoute создает новый маршрут
func (s *RoutingService) CreateRoute(
	ctx context.Context,
	name string,
	pattern string,
	patternType domain.PatternType,
	providerIDs []uuid.UUID,
	priority int,
	strategy domain.LoadBalanceStrategy,
	failoverEnabled bool,
	metadata map[string]string,
) (*domain.Route, error) {
	// Создаем доменную модель
	route := domain.NewRoute(name, pattern, patternType, providerIDs, priority, strategy)
	if failoverEnabled {
		route.EnableFailover()
	}
	if metadata != nil {
		route.Metadata = metadata
	}

	// Валидация правила
	rule := domain.NewRouteRule(pattern, patternType)
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("валидация правила: %w", err)
	}

	// Сохраняем в БД
	if err := s.routeRepo.Create(ctx, route); err != nil {
		return nil, fmt.Errorf("ошибка создания маршрута: %w", err)
	}

	s.logger.Info().
		Str("route_id", route.ID.String()).
		Str("name", route.Name).
		Msg("маршрут создан")

	return route, nil
}

// UpdateRoute обновляет маршрут
func (s *RoutingService) UpdateRoute(
	ctx context.Context,
	routeID uuid.UUID,
	updates *RouteUpdate,
) error {
	route, err := s.routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return fmt.Errorf("маршрут не найден: %w", err)
	}

	if updates.Name != nil {
		route.Name = *updates.Name
	}
	if updates.Pattern != nil && updates.PatternType != nil {
		route.UpdatePattern(*updates.Pattern, *updates.PatternType)
		// Валидация правила
		rule := domain.NewRouteRule(*updates.Pattern, *updates.PatternType)
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("валидация правила: %w", err)
		}
	}
	if updates.Priority != nil {
		route.Priority = *updates.Priority
	}
	if updates.ProviderIDs != nil {
		route.UpdateProviders(updates.ProviderIDs)
	}
	if updates.Strategy != nil {
		route.UpdateStrategy(*updates.Strategy)
	}
	if updates.FailoverEnabled != nil {
		if *updates.FailoverEnabled {
			route.EnableFailover()
		} else {
			route.DisableFailover()
		}
	}
	if updates.Active != nil {
		if *updates.Active {
			route.Activate()
		} else {
			route.Deactivate()
		}
	}
	if updates.Metadata != nil {
		route.Metadata = updates.Metadata
	}

	if err := s.routeRepo.Update(ctx, route); err != nil {
		return fmt.Errorf("ошибка обновления маршрута: %w", err)
	}

	s.logger.Info().
		Str("route_id", route.ID.String()).
		Msg("маршрут обновлен")

	return nil
}

// DeleteRoute удаляет маршрут
func (s *RoutingService) DeleteRoute(ctx context.Context, routeID uuid.UUID) error {
	if err := s.routeRepo.Delete(ctx, routeID); err != nil {
		return fmt.Errorf("ошибка удаления маршрута: %w", err)
	}

	s.logger.Info().
		Str("route_id", routeID.String()).
		Msg("маршрут удален")

	return nil
}

// ListRoutes получает список маршрутов
func (s *RoutingService) ListRoutes(
	ctx context.Context,
	activeOnly bool,
	limit, offset int,
) ([]*domain.Route, int, error) {
	return s.routeRepo.List(ctx, activeOnly, limit, offset)
}

// RouteUpdate представляет обновления для маршрута
type RouteUpdate struct {
	Name           *string
	Pattern        *string
	PatternType    *domain.PatternType
	Priority       *int
	ProviderIDs    []uuid.UUID
	Strategy       *domain.LoadBalanceStrategy
	FailoverEnabled *bool
	Active         *bool
	Metadata       map[string]string
}