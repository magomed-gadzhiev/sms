package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MessageRepository определяет интерфейс репозитория сообщений
type MessageRepository interface {
	Create(ctx context.Context, msg *Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*Message, error)
	GetByMessageID(ctx context.Context, messageID string) (*Message, error)
	GetByExternalID(ctx context.Context, externalID string) (*Message, error)
	GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*Message, error)
	Update(ctx context.Context, msg *Message) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, statusMessage string) error
	GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, status *string) ([]*Message, error)
	GetPendingForRetry(ctx context.Context, limit int) ([]*Message, error)
	GetScheduledReady(ctx context.Context, limit int) ([]*Message, error)
	GetStuckPending(ctx context.Context, threshold time.Duration, limit int) ([]*Message, error)
	CancelByIDAndStatus(ctx context.Context, id uuid.UUID, clientID uuid.UUID) error
}

// DLRRepository определяет интерфейс репозитория DLR receipts
type DLRRepository interface {
	Create(ctx context.Context, dlr *DLRReceipt) error
	GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*DLRReceipt, error)
	GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*DLRReceipt, error)
}
