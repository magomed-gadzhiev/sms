package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ProviderOwnership string

const (
	OwnershipPlatform  ProviderOwnership = "platform"
	OwnershipPrivate   ProviderOwnership = "private"
	OwnershipInherited ProviderOwnership = "inherited"
)

type ClientProvider struct {
	ID                 uuid.UUID
	ClientID           uuid.UUID
	ProviderID         uuid.UUID
	Ownership          ProviderOwnership
	SourceClientID     *uuid.UUID
	SharedPriority     int
	ExposeCost         bool
	ExposeProviderName bool
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

var (
	ErrClientProviderNotFound      = errors.New("client provider not found")
	ErrClientProviderAlreadyExists = errors.New("client already has this provider")
	ErrInvalidOwnership            = errors.New("invalid ownership type")
	ErrSourceClientRequired        = errors.New("source_client_id required for inherited ownership")
	ErrProviderNotAssigned         = errors.New("provider not assigned to client")
)

func NewClientProvider(clientID, providerID uuid.UUID, ownership ProviderOwnership) *ClientProvider {
	now := time.Now()
	return &ClientProvider{
		ID:                 uuid.New(),
		ClientID:           clientID,
		ProviderID:         providerID,
		Ownership:          ownership,
		SharedPriority:     0,
		ExposeCost:         false,
		ExposeProviderName: true,
		Active:             true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (cp *ClientProvider) Validate() error {
	switch cp.Ownership {
	case OwnershipPlatform, OwnershipPrivate, OwnershipInherited:
	default:
		return ErrInvalidOwnership
	}
	if cp.Ownership == OwnershipInherited && cp.SourceClientID == nil {
		return ErrSourceClientRequired
	}
	return nil
}
