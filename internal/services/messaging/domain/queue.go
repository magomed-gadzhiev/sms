package domain

import "context"

// EventPublisher определяет интерфейс для публикации событий в Kafka
type EventPublisher interface {
	PublishMessageCreated(ctx context.Context, msg *Message) error
	PublishMessageQueued(ctx context.Context, msg *Message) error
	PublishMessageStatusChanged(ctx context.Context, msg *Message, oldStatus string) error
}

// EventSubscriber определяет интерфейс для подписки на события из Kafka
type EventSubscriber interface {
	SubscribeToDLRReceived(ctx context.Context, handler func(dlr *DLRReceipt) error) error
	SubscribeToMessageStatusChanged(ctx context.Context, handler func(msg *Message, oldStatus string) error) error
}
