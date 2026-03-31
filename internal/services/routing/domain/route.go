package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

func normalizeDialString(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "+")
}

// Route представляет доменную модель маршрута
type Route struct {
	ID                  uuid.UUID
	Name                string
	Pattern             string
	PatternType         PatternType
	ProviderIDs         []uuid.UUID
	Priority            int
	Active              bool
	FailoverEnabled     bool
	LoadBalanceStrategy LoadBalanceStrategy
	Metadata            map[string]string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PatternType представляет тип паттерна маршрута
type PatternType string

const (
	PatternTypePrefix PatternType = "prefix"
	PatternTypeExact  PatternType = "exact"
	PatternTypeRegex  PatternType = "regex"
)

// LoadBalanceStrategy представляет стратегию балансировки нагрузки
type LoadBalanceStrategy string

const (
	LoadBalanceRoundRobin  LoadBalanceStrategy = "round_robin"
	LoadBalanceLeastLoaded LoadBalanceStrategy = "least_loaded"
	LoadBalanceCheapest    LoadBalanceStrategy = "cheapest"
)

// NewRoute создает новый маршрут
func NewRoute(
	name string,
	pattern string,
	patternType PatternType,
	providerIDs []uuid.UUID,
	priority int,
	strategy LoadBalanceStrategy,
) *Route {
	now := time.Now()

	return &Route{
		ID:                  uuid.New(),
		Name:                name,
		Pattern:             pattern,
		PatternType:         patternType,
		ProviderIDs:         providerIDs,
		Priority:            priority,
		Active:              true,
		FailoverEnabled:     false,
		LoadBalanceStrategy: strategy,
		Metadata:            make(map[string]string),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// Matches проверяет, соответствует ли номер назначения паттерну маршрута
func (r *Route) Matches(destination string) bool {
	normalizedDestination := normalizeDialString(destination)
	normalizedPattern := normalizeDialString(r.Pattern)

	switch r.PatternType {
	case PatternTypePrefix:
		if len(normalizedDestination) >= len(normalizedPattern) {
			return normalizedDestination[:len(normalizedPattern)] == normalizedPattern
		}
		return false
	case PatternTypeExact:
		return normalizedDestination == normalizedPattern
	case PatternTypeRegex:
		// Регулярное выражение проверяется в репозитории
		// Здесь возвращаем true, т.к. валидация уже выполнена
		return true
	default:
		return false
	}
}

// Activate активирует маршрут
func (r *Route) Activate() {
	r.Active = true
	r.UpdatedAt = time.Now()
}

// Deactivate деактивирует маршрут
func (r *Route) Deactivate() {
	r.Active = false
	r.UpdatedAt = time.Now()
}

// EnableFailover включает failover
func (r *Route) EnableFailover() {
	r.FailoverEnabled = true
	r.UpdatedAt = time.Now()
}

// DisableFailover выключает failover
func (r *Route) DisableFailover() {
	r.FailoverEnabled = false
	r.UpdatedAt = time.Now()
}

// UpdatePattern обновляет паттерн маршрута
func (r *Route) UpdatePattern(pattern string, patternType PatternType) {
	r.Pattern = pattern
	r.PatternType = patternType
	r.UpdatedAt = time.Now()
}

// UpdateProviders обновляет список провайдеров
func (r *Route) UpdateProviders(providerIDs []uuid.UUID) {
	r.ProviderIDs = providerIDs
	r.UpdatedAt = time.Now()
}

// UpdateStrategy обновляет стратегию балансировки
func (r *Route) UpdateStrategy(strategy LoadBalanceStrategy) {
	r.LoadBalanceStrategy = strategy
	r.UpdatedAt = time.Now()
}

// ToShared преобразует доменную модель в shared.Route (для обратной совместимости)
func (r *Route) ToShared() *shared.Route {
	var providerID uuid.UUID
	if len(r.ProviderIDs) > 0 {
		providerID = r.ProviderIDs[0]
	}

	var failoverProviderID *uuid.UUID
	if r.FailoverEnabled && len(r.ProviderIDs) > 1 {
		failoverID := r.ProviderIDs[1]
		failoverProviderID = &failoverID
	}

	return &shared.Route{
		ID:                 r.ID,
		Name:               r.Name,
		Pattern:            r.Pattern,
		PatternType:        string(r.PatternType),
		ProviderID:         providerID,
		Priority:           r.Priority,
		Active:             r.Active,
		FailoverProviderID: failoverProviderID,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

// RouteFromShared создает доменную модель из shared.Route
func RouteFromShared(route *shared.Route) *Route {
	providerIDs := []uuid.UUID{route.ProviderID}
	if route.FailoverProviderID != nil {
		providerIDs = append(providerIDs, *route.FailoverProviderID)
	}

	failoverEnabled := route.FailoverProviderID != nil

	return &Route{
		ID:                  route.ID,
		Name:                route.Name,
		Pattern:             route.Pattern,
		PatternType:         PatternType(route.PatternType),
		ProviderIDs:         providerIDs,
		Priority:            route.Priority,
		Active:              route.Active,
		FailoverEnabled:     failoverEnabled,
		LoadBalanceStrategy: LoadBalanceRoundRobin, // По умолчанию
		Metadata:            make(map[string]string),
		CreatedAt:           route.CreatedAt,
		UpdatedAt:           route.UpdatedAt,
	}
}
