package domain

import "context"

// EventConsumer определяет интерфейс для подписки на события из Kafka
type EventConsumer interface {
	ConsumeMessageCreated(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error
	ConsumeMessageSent(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error
	ConsumeMessageDelivered(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error
	ConsumeMessageFailed(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64, reason string) error) error
}
