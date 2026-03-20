package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSubscriptionNotFound    = errors.New("subscription not found")
	ErrMaxSubscriptionsReached = errors.New("maximum subscriptions per client reached")
	ErrInvalidURL              = errors.New("invalid webhook URL: must be HTTPS")
	ErrInvalidEventType        = errors.New("invalid event type")
)

var ValidEventTypes = map[string]bool{
	"delivered": true,
	"failed":    true,
	"expired":   true,
	"rejected":  true,
}

const MaxSubscriptionsPerClient = 10

// Subscription represents a webhook subscription
type Subscription struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	URL        string
	EventTypes []string
	Secret     string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// WebhookEvent represents an event to be delivered via webhook
type WebhookEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Data      EventData `json:"data"`
}

// EventData contains the message delivery status data
type EventData struct {
	MessageID     string     `json:"message_id"`
	ExternalID    string     `json:"external_id,omitempty"`
	Source        string     `json:"source"`
	Destination   string     `json:"destination"`
	Status        string     `json:"status"`
	StatusMessage string     `json:"status_message,omitempty"`
	ProviderID    string     `json:"provider_id,omitempty"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	FailedAt      *time.Time `json:"failed_at,omitempty"`
}

// SMPPStatToEventType maps SMPP DLR stat values to webhook event types
var SMPPStatToEventType = map[string]string{
	"DELIVRD": "delivered",
	"UNDELIV": "failed",
	"EXPIRED": "expired",
	"DELETED": "failed",
	"REJECTD": "rejected",
	"UNKNOWN": "failed",
}
