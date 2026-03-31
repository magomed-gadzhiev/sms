package sms

import (
	"context"
	"fmt"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
)

// CascadeProducer — минимальный интерфейс producer для SMS адаптера
type CascadeProducer interface {
	PublishAttemptSend(ctx context.Context, cmd cascadekafka.CascadeAttemptSendCommand) error
}

// Adapter — SMS адаптер, публикует команду на отправку в Kafka
type Adapter struct {
	producer CascadeProducer
}

// NewAdapter создаёт новый SMS адаптер
func NewAdapter(producer CascadeProducer) *Adapter {
	return &Adapter{producer: producer}
}

// Type возвращает тип канала
func (a *Adapter) Type() domain.ChannelType {
	return domain.ChannelSMS
}

// Send публикует команду на отправку SMS через Kafka
func (a *Adapter) Send(ctx context.Context, attempt *domain.DeliveryAttempt, cfg *domain.ChannelConfig) error {
	var channelConfig map[string]interface{}
	if cfg != nil {
		channelConfig = cfg.Config
	}

	delivery, ok := ctx.Value(deliveryContextKey{}).(*domain.Delivery)
	if !ok || delivery == nil {
		return fmt.Errorf("delivery not found in context")
	}

	timeoutS := 30
	if cfg != nil {
		if v, ok := cfg.Config["timeout_s"]; ok {
			if t, ok := v.(float64); ok {
				timeoutS = int(t)
			}
		}
	}

	cmd := cascadekafka.NewCascadeAttemptSendCommand(
		attempt.ID.String(),
		attempt.DeliveryID.String(),
		string(domain.ChannelSMS),
		channelConfig,
		delivery.Recipient,
		delivery.Text,
		timeoutS,
		delivery.RequestID,
	)

	return a.producer.PublishAttemptSend(ctx, cmd)
}

// deliveryContextKey — ключ для хранения Delivery в контексте
type deliveryContextKey struct{}

// WithDelivery добавляет Delivery в контекст
func WithDelivery(ctx context.Context, d *domain.Delivery) context.Context {
	return context.WithValue(ctx, deliveryContextKey{}, d)
}
