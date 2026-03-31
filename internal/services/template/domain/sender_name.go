package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSenderNameNotFound      = errors.New("sender name not found")
	ErrDuplicateSenderName     = errors.New("sender name already exists for this client")
	ErrInvalidSenderNameFormat = errors.New("invalid sender name format: must be 1-11 alphanumeric chars or 1-15 digits")
	ErrInvalidSenderNameStatus = errors.New("invalid status transition for sender name")
	ErrSenderNameNotApproved   = errors.New("sender name is not approved")
)

const (
	SenderNameStatusPending     = "pending"
	SenderNameStatusApproved    = "approved"
	SenderNameStatusRejected    = "rejected"
	SenderNameStatusDeactivated = "deactivated"

	ActorTypeClient = "client"
	ActorTypeAdmin  = "admin"
	ActorTypeSystem = "system"
)

var (
	senderNameAlphanumericRegex = regexp.MustCompile(`^[A-Za-z0-9 ]{1,11}$`)
	senderNameNumericRegex      = regexp.MustCompile(`^\d{1,15}$`)
)

// SenderName represents a registered sender identifier belonging to a client.
type SenderName struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	Name            string
	Status          string
	RejectionReason string
	ReviewerID      *uuid.UUID
	ReviewedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SenderNameStatusHistory is one immutable record of a status transition.
type SenderNameStatusHistory struct {
	ID           uuid.UUID
	SenderNameID uuid.UUID
	OldStatus    *string
	NewStatus    string
	ActorID      *uuid.UUID
	ActorType    string
	Comment      string
	CreatedAt    time.Time
}

// ValidateSenderName checks that the name conforms to GSM 03.40 rules.
func ValidateSenderName(name string) error {
	if senderNameNumericRegex.MatchString(name) {
		return nil
	}
	if senderNameAlphanumericRegex.MatchString(name) && strings.TrimSpace(name) != "" {
		return nil
	}
	return ErrInvalidSenderNameFormat
}

// allowedTransitions maps (fromStatus -> set of valid toStatuses).
var allowedTransitions = map[string]map[string]bool{
	"":                          {SenderNameStatusPending: true},
	SenderNameStatusPending:     {SenderNameStatusApproved: true, SenderNameStatusRejected: true},
	SenderNameStatusRejected:    {SenderNameStatusPending: true},
	SenderNameStatusApproved:    {SenderNameStatusDeactivated: true},
	SenderNameStatusDeactivated: {},
}

// IsValidTransition returns true if transitioning from oldStatus to newStatus is allowed.
func IsValidTransition(oldStatus, newStatus string) bool {
	targets, ok := allowedTransitions[oldStatus]
	if !ok {
		return false
	}
	return targets[newStatus]
}
