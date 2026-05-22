package domain

import (
	"regexp"
	"strings"
)

// RouteRule представляет правило маршрутизации
type RouteRule struct {
	Pattern     string
	PatternType PatternType
}

// NewRouteRule создает новое правило маршрутизации
func NewRouteRule(pattern string, patternType PatternType) *RouteRule {
	return &RouteRule{
		Pattern:     pattern,
		PatternType: patternType,
	}
}

// Matches проверяет, соответствует ли номер назначения правилу
func (rr *RouteRule) Matches(destination string) (bool, error) {
	switch rr.PatternType {
	case PatternTypePrefix:
		return strings.HasPrefix(destination, rr.Pattern), nil
	case PatternTypeExact:
		return destination == rr.Pattern, nil
	case PatternTypeRegex:
		matched, err := regexp.MatchString(rr.Pattern, destination)
		if err != nil {
			return false, err
		}
		return matched, nil
	default:
		return false, nil
	}
}

// Validate валидирует правило
func (rr *RouteRule) Validate() error {
	if rr.Pattern == "" {
		return ErrEmptyPattern
	}
	
	if rr.PatternType == PatternTypeRegex {
		_, err := regexp.Compile(rr.Pattern)
		if err != nil {
			return ErrInvalidRegex
		}
	}
	
	return nil
}

// PatternTypeFromString создает PatternType из строки
func PatternTypeFromString(s string) PatternType {
	switch s {
	case "prefix":
		return PatternTypePrefix
	case "exact":
		return PatternTypeExact
	case "regex":
		return PatternTypeRegex
	default:
		return PatternTypePrefix
	}
}

// LoadBalanceStrategyFromString создает LoadBalanceStrategy из строки
func LoadBalanceStrategyFromString(s string) LoadBalanceStrategy {
	switch s {
	case "round_robin":
		return LoadBalanceRoundRobin
	case "least_loaded":
		return LoadBalanceLeastLoaded
	case "cheapest":
		return LoadBalanceCheapest
	default:
		return LoadBalanceRoundRobin
	}
}