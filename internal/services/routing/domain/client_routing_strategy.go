package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type RoutingStrategy string

const (
	StrategyPriority RoutingStrategy = "priority"
	StrategyWeighted RoutingStrategy = "weighted"
	StrategySmart    RoutingStrategy = "smart"
)

type ClientRoutingStrategy struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	OperatorID *uuid.UUID // nil = account default
	Strategy   RoutingStrategy
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrInvalidRoutingStrategy  = errors.New("invalid routing strategy")
	ErrRoutingStrategyNotFound = errors.New("routing strategy not found")
)

func NewClientRoutingStrategy(clientID uuid.UUID, operatorID *uuid.UUID, strategy RoutingStrategy) *ClientRoutingStrategy {
	now := time.Now()
	return &ClientRoutingStrategy{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		Strategy:   strategy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (s *ClientRoutingStrategy) Validate() error {
	switch s.Strategy {
	case StrategyPriority, StrategyWeighted, StrategySmart:
		return nil
	default:
		return ErrInvalidRoutingStrategy
	}
}
