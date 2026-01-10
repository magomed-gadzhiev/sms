package http

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageProducer интерфейс для публикации сообщений
type MessageProducer interface {
	PublishOutgoing(ctx context.Context, msg *queue.KafkaMessage) error
}

// MessageRepository интерфейс для работы с сообщениями
type MessageRepository interface {
	Create(ctx context.Context, msg *shared.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Message, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error
	GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error)
	GetAll(ctx context.Context, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error)
}

// ClientRepository интерфейс для работы с клиентами
type ClientRepository interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*shared.Client, error)
}
