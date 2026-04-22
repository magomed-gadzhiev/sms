package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	OperatorBindingStatusPending  = "pending"
	OperatorBindingStatusApproved = "approved"
	OperatorBindingStatusRejected = "rejected"
)

var (
	ErrOperatorBindingNotFound          = errors.New("operator template binding not found")
	ErrDuplicateOperatorBinding         = errors.New("binding already exists for (template, operator)")
	ErrInvalidOperatorBindingTransition = errors.New("invalid binding status transition")
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

// NewOperatorTemplateBinding creates a new pending binding with fresh ID and timestamps.
func NewOperatorTemplateBinding(templateID, senderNameID, operatorID uuid.UUID) *OperatorTemplateBinding {
	now := time.Now().UTC()
	return &OperatorTemplateBinding{
		ID:           uuid.New(),
		TemplateID:   templateID,
		SenderNameID: senderNameID,
		OperatorID:   operatorID,
		Status:       OperatorBindingStatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

var operatorBindingTransitions = map[string]map[string]bool{
	"":                               {OperatorBindingStatusPending: true},
	OperatorBindingStatusPending:     {OperatorBindingStatusApproved: true, OperatorBindingStatusRejected: true},
	OperatorBindingStatusApproved:    {},
	OperatorBindingStatusRejected:    {OperatorBindingStatusPending: true}, // allow re-submission
}

// IsValidOperatorBindingTransition returns true if moving between statuses is allowed.
func IsValidOperatorBindingTransition(from, to string) bool {
	targets, ok := operatorBindingTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}
