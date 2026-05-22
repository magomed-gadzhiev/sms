package domain

import "context"

// EventPublisher определяет интерфейс для публикации событий биллинга
type EventPublisher interface {
	PublishBalanceChanged(ctx context.Context, clientID string, balance, currency string) error
	PublishTransactionCompleted(ctx context.Context, transactionID, clientID, transactionType, amount, currency string) error
	PublishBalanceLow(ctx context.Context, clientID, balance, threshold, currency string) error
}

// EventConsumer определяет интерфейс для подписки на события
type EventConsumer interface {
	ConsumeMessageDelivered(ctx context.Context) error
	ConsumeMessageFailed(ctx context.Context) error
}

// MessageDeliveredHandler обрабатывает событие доставки сообщения
type MessageDeliveredHandler func(ctx context.Context, messageID, clientID string) error

// MessageFailedHandler обрабатывает событие неудачной отправки сообщения
type MessageFailedHandler func(ctx context.Context, messageID, clientID string) error
