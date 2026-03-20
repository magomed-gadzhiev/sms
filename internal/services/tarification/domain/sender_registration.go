package domain

import (
	"time"

	"github.com/google/uuid"
)

type SenderRegistrationType string

const (
	SenderTypePaid SenderRegistrationType = "paid"
	SenderTypeFree SenderRegistrationType = "free"
)

type SenderRegistrationStatus string

const (
	SenderStatusActive  SenderRegistrationStatus = "active"
	SenderStatusPending SenderRegistrationStatus = "pending"
	SenderStatusExpired SenderRegistrationStatus = "expired"
)

type SenderCategory string

const (
	CategoryShared         SenderCategory = "shared"
	CategoryPaidRegistered SenderCategory = "paid_registered"
	CategoryFreeRegistered SenderCategory = "free_registered"
)

type SenderRegistration struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	OperatorID uuid.UUID
	SenderName string
	Type       SenderRegistrationType
	Status     SenderRegistrationStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewSenderRegistration(clientID, operatorID uuid.UUID, senderName string, regType SenderRegistrationType) *SenderRegistration {
	now := time.Now()
	return &SenderRegistration{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		SenderName: senderName,
		Type:       regType,
		Status:     SenderStatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (s *SenderRegistration) Validate() error {
	if s.ClientID == uuid.Nil {
		return ErrSenderRegistrationNotFound
	}
	if s.OperatorID == uuid.Nil {
		return ErrSenderRegistrationNotFound
	}
	if s.SenderName == "" {
		return ErrSenderRegistrationNotFound
	}
	if s.Type != SenderTypePaid && s.Type != SenderTypeFree {
		return ErrSenderRegistrationInvalidType
	}
	return nil
}
