package grpc

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageProducer интерфейс для публикации сообщений
type MessageProducer interface {
	PublishOutgoing(ctx context.Context, msg *queue.KafkaMessage) error
}

// BatchMessagePublisher интерфейс для асинхронной пакетной публикации сообщений
// через AsyncProducer (неблокирующий PublishAsync)
type BatchMessagePublisher interface {
	PublishAsync(topic string, key string, value []byte, headers []sarama.RecordHeader)
}

// MessageRepository интерфейс для работы с сообщениями
type MessageRepository interface {
	Create(ctx context.Context, msg *shared.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Message, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error
}

// ClientRepository интерфейс для работы с клиентами
type ClientRepository interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*shared.Client, error)
}
