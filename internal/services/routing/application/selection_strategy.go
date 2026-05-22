package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)


// ProviderSelector представляет интерфейс для выбора провайдера
type ProviderSelector interface {
	SelectProvider(ctx context.Context, route *domain.Route, providerInfos []*domain.ProviderInfo) (*domain.ProviderInfo, error)
}

// RoundRobinSelector реализует стратегию round-robin
type RoundRobinSelector struct {
	currentIndex map[uuid.UUID]int
}

// NewRoundRobinSelector создает новый round-robin селектор
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{
		currentIndex: make(map[uuid.UUID]int),
	}
}

// PurgeStaleRoutes удаляет из currentIndex записи для маршрутов, не входящих в activeRouteIDs.
func (s *RoundRobinSelector) PurgeStaleRoutes(activeRouteIDs []uuid.UUID) {
	active := make(map[uuid.UUID]struct{}, len(activeRouteIDs))
	for _, id := range activeRouteIDs {
		active[id] = struct{}{}
	}
	for id := range s.currentIndex {
		if _, ok := active[id]; !ok {
			delete(s.currentIndex, id)
		}
	}
}

// SelectProvider выбирает провайдера по стратегии round-robin
func (s *RoundRobinSelector) SelectProvider(
	ctx context.Context,
	route *domain.Route,
	providerInfos []*domain.ProviderInfo,
) (*domain.ProviderInfo, error) {
	if len(providerInfos) == 0 {
		return nil, domain.ErrNoProviders
	}

	// Фильтруем только активные провайдеры
	activeProviders := make([]*domain.ProviderInfo, 0, len(providerInfos))
	for _, provider := range providerInfos {
		if provider.Active {
			activeProviders = append(activeProviders, provider)
		}
	}

	if len(activeProviders) == 0 {
		return nil, domain.ErrNoProviders
	}

	// Получаем текущий индекс для этого маршрута
	idx := s.currentIndex[route.ID]
	if idx >= len(activeProviders) {
		idx = 0
	}

	selected := activeProviders[idx]
	
	// Увеличиваем индекс для следующего вызова
	s.currentIndex[route.ID] = (idx + 1) % len(activeProviders)

	log.Debug().
		Str("route_id", route.ID.String()).
		Str("provider_id", selected.ID.String()).
		Str("strategy", "round_robin").
		Msg("выбран провайдер по round-robin")

	return selected, nil
}

// LeastLoadedSelector реализует стратегию least-loaded
type LeastLoadedSelector struct {
	providerRepo domain.ProviderRepository
}

// NewLeastLoadedSelector создает новый least-loaded селектор
func NewLeastLoadedSelector(providerRepo domain.ProviderRepository) *LeastLoadedSelector {
	return &LeastLoadedSelector{
		providerRepo: providerRepo,
	}
}

// SelectProvider выбирает провайдера с наименьшей загрузкой
func (s *LeastLoadedSelector) SelectProvider(
	ctx context.Context,
	route *domain.Route,
	providerInfos []*domain.ProviderInfo,
) (*domain.ProviderInfo, error) {
	if len(providerInfos) == 0 {
		return nil, domain.ErrNoProviders
	}

	// Фильтруем только активные провайдеры
	activeProviders := make([]*domain.ProviderInfo, 0, len(providerInfos))
	for _, provider := range providerInfos {
		if provider.Active {
			activeProviders = append(activeProviders, provider)
		}
	}

	if len(activeProviders) == 0 {
		return nil, domain.ErrNoProviders
	}

	// Получаем health всех провайдеров одним запросом (batch, без N+1)
	ids := make([]uuid.UUID, len(activeProviders))
	for i, p := range activeProviders {
		ids[i] = p.ID
	}
	healthMap, err := s.providerRepo.GetHealthBatch(ctx, ids)
	if err != nil {
		log.Warn().Err(err).Msg("не удалось получить health провайдеров пакетом, используем первого")
		healthMap = map[uuid.UUID]*domain.ProviderHealth{}
	}

	var bestProvider *domain.ProviderInfo
	var bestLoad float64 = -1

	for _, provider := range activeProviders {
		health, ok := healthMap[provider.ID]
		if !ok {
			log.Warn().
				Str("provider_id", provider.ID.String()).
				Msg("health провайдера отсутствует в ответе, пропускаем")
			continue
		}

		var load float64
		if health.TotalConnections > 0 {
			load = float64(health.ActiveConnections) / float64(health.TotalConnections)
		}

		if bestProvider == nil || load < bestLoad {
			bestProvider = provider
			bestLoad = load
		}
	}

	if bestProvider == nil {
		return nil, domain.ErrNoProviders
	}

	log.Debug().
		Str("route_id", route.ID.String()).
		Str("provider_id", bestProvider.ID.String()).
		Float64("load", bestLoad).
		Str("strategy", "least_loaded").
		Msg("выбран провайдер по least-loaded")

	return bestProvider, nil
}

// CheapestSelector реализует стратегию cheapest (использует приоритет провайдера)
type CheapestSelector struct{}

// NewCheapestSelector создает новый cheapest селектор
func NewCheapestSelector() *CheapestSelector {
	return &CheapestSelector{}
}

// SelectProvider выбирает провайдера с наивысшим приоритетом (приоритет = цена)
func (s *CheapestSelector) SelectProvider(
	ctx context.Context,
	route *domain.Route,
	providerInfos []*domain.ProviderInfo,
) (*domain.ProviderInfo, error) {
	if len(providerInfos) == 0 {
		return nil, domain.ErrNoProviders
	}

	// Фильтруем только активные провайдеры
	activeProviders := make([]*domain.ProviderInfo, 0, len(providerInfos))
	for _, provider := range providerInfos {
		if provider.Active {
			activeProviders = append(activeProviders, provider)
		}
	}

	if len(activeProviders) == 0 {
		return nil, domain.ErrNoProviders
	}

	// Выбираем провайдера с наивысшим приоритетом (чем выше приоритет, тем дешевле)
	var bestProvider *domain.ProviderInfo
	for _, provider := range activeProviders {
		if bestProvider == nil || provider.Priority > bestProvider.Priority {
			bestProvider = provider
		}
	}

	if bestProvider == nil {
		return nil, domain.ErrNoProviders
	}

	log.Debug().
		Str("route_id", route.ID.String()).
		Str("provider_id", bestProvider.ID.String()).
		Int("priority", bestProvider.Priority).
		Str("strategy", "cheapest").
		Msg("выбран провайдер по cheapest")

	return bestProvider, nil
}

// StrategyFactory создает селектор по стратегии
func StrategyFactory(strategy domain.LoadBalanceStrategy, providerRepo domain.ProviderRepository) (ProviderSelector, error) {
	switch strategy {
	case domain.LoadBalanceRoundRobin:
		return NewRoundRobinSelector(), nil
	case domain.LoadBalanceLeastLoaded:
		return NewLeastLoadedSelector(providerRepo), nil
	case domain.LoadBalanceCheapest:
		return NewCheapestSelector(), nil
	default:
		return nil, fmt.Errorf("неизвестная стратегия: %s", strategy)
	}
}