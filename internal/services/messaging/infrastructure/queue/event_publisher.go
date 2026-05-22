package queue

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
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

// PublishMessageCreated публикует событие создания сообщения
func (p *EventPublisher) PublishMessageCreated(ctx context.Context, msg *domain.Message) error {
	// Пока используем существующий механизм публикации в sms.outgoing
	// В будущем можно добавить отдельный топик для message.created
	sharedMsg := msg.ToShared()
	kafkaMsg := queue.FromMessage(sharedMsg)
	kafkaMsg.TraceID = shared.GetRequestID(ctx)

	// Публикуем в очередь для отправки
	if err := p.producer.PublishOutgoing(ctx, kafkaMsg); err != nil {
		log.Error().Err(err).Msg("ошибка публикации события message.created")
		return err
	}

	log.Debug().
		Str("message_id", msg.ID.String()).
		Msg("событие message.created опубликовано")

	return nil
}

// PublishMessageQueued публикует событие добавления сообщения в очередь
func (p *EventPublisher) PublishMessageQueued(ctx context.Context, msg *domain.Message) error {
	// Используем существующий механизм публикации в sms.outgoing
	sharedMsg := msg.ToShared()
	kafkaMsg := queue.FromMessage(sharedMsg)
	kafkaMsg.TraceID = shared.GetRequestID(ctx)

	// Публикуем в очередь для отправки
	if err := p.producer.PublishOutgoing(ctx, kafkaMsg); err != nil {
		log.Error().Err(err).Msg("ошибка публикации события message.queued")
		return err
	}

	log.Debug().
		Str("message_id", msg.ID.String()).
		Msg("событие message.queued опубликовано")

	return nil
}

// PublishMessageStatusChanged публикует событие изменения статуса сообщения
func (p *EventPublisher) PublishMessageStatusChanged(ctx context.Context, msg *domain.Message, oldStatus string) error {
	// Для изменения статуса можно добавить отдельный топик или использовать существующий механизм
	// Пока просто логируем событие, в будущем можно публиковать в отдельный топик message.status.changed
	log.Debug().
		Str("message_id", msg.ID.String()).
		Str("old_status", oldStatus).
		Str("new_status", string(msg.Status)).
		Msg("событие message.status.changed")

	return nil
}
