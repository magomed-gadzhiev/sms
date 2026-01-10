package queue

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// EventConsumer реализует domain.EventConsumer
type EventConsumer struct {
	consumer *queue.Consumer
	handler  domain.MessageQueuedHandler
}

// NewEventConsumer создает новый consumer событий
func NewEventConsumer(
	consumer *queue.Consumer,
	handler domain.MessageQueuedHandler,
) *EventConsumer {
	return &EventConsumer{
		consumer: consumer,
		handler:  handler,
	}
}

// ConsumeMessageQueued подписывается на события message.queued из топика sms.outgoing
func (c *EventConsumer) ConsumeMessageQueued(ctx context.Context) error {
	// Запускаем потребление из топика sms.outgoing
	// Routing Service обрабатывает только сообщения без route_id и provider_id
	return c.consumer.ConsumeOutgoing()
}

// HandleMessage обрабатывает сообщение из Kafka (вызывается из consumer handler)
func (c *EventConsumer) HandleMessage(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
	var clientID *uuid.UUID
	if kafkaMsg.ClientID != nil {
		clientID = kafkaMsg.ClientID
	}

	var routeID *uuid.UUID
	if kafkaMsg.RouteID != nil {
		routeID = kafkaMsg.RouteID
	}

	var providerID *uuid.UUID
	if kafkaMsg.ProviderID != nil {
		providerID = kafkaMsg.ProviderID
	}

	// Проверяем, не обработано ли уже сообщение (если есть provider_id и route_id, значит уже маршрутизировано)
	if providerID != nil && routeID != nil {
		log.Debug().
			Str("message_id", kafkaMsg.MessageID.String()).
			Msg("сообщение уже маршрутизировано, пропускаем")
		return nil
	}

	// Вызываем domain handler
	return c.handler(ctx, kafkaMsg.MessageID, kafkaMsg.Destination, clientID, routeID, providerID)
}
