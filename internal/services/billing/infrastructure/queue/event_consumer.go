package queue

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// EventConsumer реализует domain.EventConsumer
// Это обертка над queue.Consumer для соответствия domain интерфейсу
type EventConsumer struct {
	consumer *queue.Consumer
	deliveredHandler domain.MessageDeliveredHandler
	failedHandler    domain.MessageFailedHandler
}

// NewEventConsumer создает новый consumer событий биллинга
func NewEventConsumer(
	consumer *queue.Consumer,
	deliveredHandler domain.MessageDeliveredHandler,
	failedHandler domain.MessageFailedHandler,
) *EventConsumer {
	return &EventConsumer{
		consumer:        consumer,
		deliveredHandler: deliveredHandler,
		failedHandler:   failedHandler,
	}
}

// ConsumeMessageDelivered подписывается на события message.delivered через DLR
// Реализация находится в main.go, где создается queue.Consumer с обработчиками
func (c *EventConsumer) ConsumeMessageDelivered(ctx context.Context) error {
	// Обработка реализована напрямую в main.go через queue.Consumer
	// Этот метод вызывается для запуска consumer
	return c.consumer.ConsumeDLR()
}

// ConsumeMessageFailed подписывается на события message.failed
func (c *EventConsumer) ConsumeMessageFailed(ctx context.Context) error {
	// Обработка реализована напрямую в main.go через queue.Consumer
	// Этот метод вызывается для запуска consumer
	return c.consumer.ConsumeFailed()
}
