package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ClientRoute struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	OperatorID uuid.UUID
	ProviderID uuid.UUID
	Priority   int
	Weight     int
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrClientRouteNotFound      = errors.New("client route not found")
	ErrClientRouteAlreadyExists = errors.New("route already exists for this client/operator/provider")
)

func NewClientRoute(clientID, operatorID, providerID uuid.UUID, priority, weight int) *ClientRoute {
	now := time.Now()
	return &ClientRoute{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		ProviderID: providerID,
		Priority:   priority,
		Weight:     weight,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
