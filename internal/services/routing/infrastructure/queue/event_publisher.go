package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/queue"
)

// EventPublisher реализует domain.EventPublisher
type EventPublisher struct {
	producer *queue.Producer
}

// NewEventPublisher создает новый publisher событий
func NewEventPublisher(producer *queue.Producer) *EventPublisher {
	return &EventPublisher{
		producer: producer,
	}
}

// PublishMessageRouted публикует событие маршрутизации сообщения
func (p *EventPublisher) PublishMessageRouted(
	ctx context.Context,
	messageID uuid.UUID,
	routeID uuid.UUID,
	providerID uuid.UUID,
) error {
	// Создаем сообщение для Kafka с информацией о маршрутизации
	// Используем существующий формат KafkaMessage, но добавляем route_id и provider_id
	kafkaMsg := &queue.KafkaMessage{
		ID:          messageID.String(),
		MessageID:   messageID,
		ProviderID:  &providerID,
		RouteID:     &routeID,
		CreatedAt:   time.Now(),
		Metadata: map[string]interface{}{
			"event_type": "message.routed",
			"route_id":   routeID.String(),
			"provider_id": providerID.String(),
		},
	}

	// Публикуем в очередь для отправки (sms.outgoing)
	// Provider Service будет читать это сообщение и отправлять через выбранного провайдера
	if err := p.producer.PublishOutgoing(ctx, kafkaMsg); err != nil {
		log.Error().Err(err).
			Str("message_id", messageID.String()).
			Str("route_id", routeID.String()).
			Str("provider_id", providerID.String()).
			Msg("ошибка публикации события message.routed")
		return err
	}

	log.Debug().
		Str("message_id", messageID.String()).
		Str("route_id", routeID.String()).
		Str("provider_id", providerID.String()).
		Msg("событие message.routed опубликовано")

	return nil
}