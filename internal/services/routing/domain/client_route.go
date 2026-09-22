package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ClientRoute struct {
	ID         uuid.UUID
	ClientID   *uuid.UUID       // nil = default route
	OperatorID *uuid.UUID       // legacy field, kept for backward compat
	ProviderID uuid.UUID
	Priority   int
	Weight     int
	Active     bool
	Name       string
	Comment    string
	Status     RouteStatus
	Share      int
	RouteType  string           // sms, hlr, max
	Shared     bool             // reseller route set shared with sub accounts
	Groups     []ConditionGroup
	Schedules  []Schedule
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrClientRouteNotFound      = errors.New("client route not found")
	ErrClientRouteAlreadyExists = errors.New("route already exists for this client/operator/provider")
)

func NewClientRoute(clientID, operatorID, providerID uuid.UUID, priority, weight int) *ClientRoute {
	now := time.Now()
	cID := clientID
	oID := operatorID
	return &ClientRoute{
		ID:         uuid.New(),
		ClientID:   &cID,
		OperatorID: &oID,
		ProviderID: providerID,
		Priority:   priority,
		Weight:     weight,
		Active:     true,
		Status:     RouteStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func NewManagedRoute(clientID *uuid.UUID, providerID uuid.UUID, name string, routeType string, priority, share int, status RouteStatus) *ClientRoute {
	now := time.Now()
	return &ClientRoute{
		ID:         uuid.New(),
		ClientID:   clientID,
		ProviderID: providerID,
		Name:       name,
		RouteType:  routeType,
		Priority:   priority,
		Share:      share,
		Status:     status,
		Weight:     share,
		Active:     status == RouteStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
