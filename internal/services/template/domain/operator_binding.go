package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	BindingStatusPending  = "pending"
	BindingStatusApproved = "approved"
	BindingStatusRejected = "rejected"
)

var (
	ErrBindingNotFound          = errors.New("operator template binding not found")
	ErrDuplicateBinding         = errors.New("binding already exists for (template, operator)")
	ErrInvalidBindingTransition = errors.New("invalid binding status transition")
)

// OperatorTemplateBinding is the moderation record for a (template, sender_name, operator) tuple.
type OperatorTemplateBinding struct {
	ID              uuid.UUID
	TemplateID      uuid.UUID
	SenderNameID    uuid.UUID
	OperatorID      uuid.UUID
	Status          string
	RejectionReason string
	ReviewedBy      *uuid.UUID
	ReviewedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewOperatorBinding creates a new pending binding with fresh ID and timestamps.
func NewOperatorBinding(templateID, senderNameID, operatorID uuid.UUID) *OperatorTemplateBinding {
	now := time.Now().UTC()
	return &OperatorTemplateBinding{
		ID:           uuid.New(),
		TemplateID:   templateID,
		SenderNameID: senderNameID,
		OperatorID:   operatorID,
		Status:       BindingStatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

var bindingTransitions = map[string]map[string]bool{
	"":                    {BindingStatusPending: true},
	BindingStatusPending:  {BindingStatusApproved: true, BindingStatusRejected: true},
	BindingStatusApproved: {},
	BindingStatusRejected: {BindingStatusPending: true}, // allow re-submission
}

// IsValidBindingTransition returns true if moving between statuses is allowed.
func IsValidBindingTransition(from, to string) bool {
	targets, ok := bindingTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}
