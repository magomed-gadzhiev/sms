package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// EventPublisher публикует события тарификации в Kafka
type EventPublisher struct {
	producer     sarama.SyncProducer
	resultsTopic string
	recalcTopic  string
	prepaidTopic string
}

// NewEventPublisher создает нового издателя событий
func NewEventPublisher(kafkaCfg *config.KafkaConfig, resultsTopic, recalcTopic, prepaidTopic string) (*EventPublisher, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Idempotent = true
	cfg.Producer.Return.Successes = true
	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.Retry.Max = kafkaCfg.MaxRetries
	cfg.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducer(kafkaCfg.Brokers, cfg)
	if err != nil {
		return nil, err
	}

	return &EventPublisher{
		producer:     producer,
		resultsTopic: resultsTopic,
		recalcTopic:  recalcTopic,
		prepaidTopic: prepaidTopic,
	}, nil
}

// PublishTarificationResult публикует результат тарификации
func (p *EventPublisher) PublishTarificationResult(_ context.Context, log *domain.TarificationLog) error {
	event := map[string]interface{}{
		"event_type":       "message.tarified",
		"message_id":       log.MessageID.String(),
		"client_id":        log.ClientID.String(),
		"operator_id":      log.OperatorID.String(),
		"sender_category":  string(log.SenderCategory),
		"strategy":         string(log.Strategy),
		"segment_count":    log.SegmentCount,
		"price_per_segment": log.PricePerSegment,
		"total_amount":     log.TotalAmount,
		"tariff_plan_id":   log.TariffPlanID.String(),
		"tariff_period_id": log.TariffPeriodID.String(),
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(p.resultsTopic, log.MessageID.String(), event)
}

// PublishRecalcEvent публикует событие пересчёта
func (p *EventPublisher) PublishRecalcEvent(_ context.Context, clientID, tariffPlanID, tariffPeriodID, oldPrice, newPrice string, affectedSegments int, recalcAmount, recalcType string) error {
	event := map[string]interface{}{
		"event_type":        "threshold.recalculated",
		"client_id":         clientID,
		"tariff_plan_id":    tariffPlanID,
		"tariff_period_id":  tariffPeriodID,
		"old_price":         oldPrice,
		"new_price":         newPrice,
		"affected_segments": affectedSegments,
		"recalc_amount":     recalcAmount,
		"recalc_type":       recalcType,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(p.recalcTopic, clientID, event)
}

// PublishPrepaidEvent публикует событие предоплаты
func (p *EventPublisher) PublishPrepaidEvent(_ context.Context, clientID, tariffPlanID, tariffPeriodID, amount, currency string) error {
	event := map[string]interface{}{
		"event_type":       "prepaid.charged",
		"client_id":        clientID,
		"tariff_plan_id":   tariffPlanID,
		"tariff_period_id": tariffPeriodID,
		"amount":           amount,
		"currency":         currency,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(p.prepaidTopic, clientID, event)
}

// Close закрывает producer
func (p *EventPublisher) Close() error {
	return p.producer.Close()
}

func (p *EventPublisher) publish(topic, key string, event map[string]interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	return err
}
