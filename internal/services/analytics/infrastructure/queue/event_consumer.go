package queue

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/queue"
)

// EventConsumer реализует domain.EventConsumer
// Это обертка над queue.Consumer для соответствия domain интерфейсу
type EventConsumer struct {
	consumer *queue.Consumer
}

// NewEventConsumer создает новый consumer событий
func NewEventConsumer(consumer *queue.Consumer) *EventConsumer {
	return &EventConsumer{
		consumer: consumer,
	}
}

// ConsumeMessageCreated подписывается на события message.created
// Реализация находится в main.go, где создается queue.Consumer с обработчиками
func (c *EventConsumer) ConsumeMessageCreated(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error {
	// Обработка реализована напрямую в main.go через queue.Consumer
	return nil
}

// ConsumeMessageSent подписывается на события message.sent
func (c *EventConsumer) ConsumeMessageSent(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error {
	// Обработка реализована напрямую в main.go через queue.Consumer
	return nil
}

// ConsumeMessageDelivered подписывается на события message.delivered через DLR
func (c *EventConsumer) ConsumeMessageDelivered(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error {
	// Обработка реализована напрямую в main.go через queue.Consumer
	return nil
}

// ConsumeMessageFailed подписывается на события message.failed
func (c *EventConsumer) ConsumeMessageFailed(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64, reason string) error) error {
	// Обработка реализована напрямую в main.go через queue.Consumer
	return nil
}
