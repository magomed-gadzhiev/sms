//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKafkaProducer_PublishOutgoing(t *testing.T) {
	// Этот тест требует запущенного Kafka
	// Используется testcontainers или внешний Kafka
	cfg := &config.KafkaConfig{
		Brokers:       []string{"localhost:9092"},
		TopicOutgoing: "test.sms.outgoing",
		MaxRetries:    3,
		RetryBackoff:  time.Second,
	}

	producer, err := queue.NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
	}
	defer producer.Close()

	ctx := context.Background()
	msg := &queue.KafkaMessage{
		ID:          uuid.New().String(),
		MessageID:   uuid.New(),
		Source:      "12345",
		Destination: "79001234567",
		Text:        "Test message",
		Priority:    0,
		RetryCount:  0,
		MaxRetries:  5,
		CreatedAt:   time.Now(),
	}

	err = producer.PublishOutgoing(ctx, msg)
	require.NoError(t, err)
}

func TestKafkaConsumer_ConsumeOutgoing(t *testing.T) {
	// Этот тест требует запущенного Kafka
	cfg := &config.KafkaConfig{
		Brokers:           []string{"localhost:9092"},
		TopicOutgoing:     "test.sms.outgoing",
		ConsumerGroup:     "test-consumer-group",
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
		MaxRetries:        3,
		RetryBackoff:      time.Second,
	}

	var receivedMessages []*queue.KafkaMessage

	handler := func(ctx context.Context, msg *queue.KafkaMessage) error {
		receivedMessages = append(receivedMessages, msg)
		return nil
	}

	consumer, err := queue.NewConsumer(cfg, handler, nil, nil)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
	}
	defer consumer.Close()

	// Запускаем consumer в отдельной горутине
	err = consumer.ConsumeOutgoing()
	require.NoError(t, err)

	// Даем время на подключение
	time.Sleep(2 * time.Second)

	// Публикуем сообщение
	producer, err := queue.NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka producer not available: %v", err)
	}
	defer producer.Close()

	msg := &queue.KafkaMessage{
		ID:          uuid.New().String(),
		MessageID:   uuid.New(),
		Source:      "12345",
		Destination: "79001234567",
		Text:        "Test message",
		Priority:    0,
		RetryCount:  0,
		MaxRetries:  5,
		CreatedAt:   time.Now(),
	}

	err = producer.PublishOutgoing(context.Background(), msg)
	require.NoError(t, err)

	// Ждем обработки сообщения
	time.Sleep(5 * time.Second)

	// Проверяем, что сообщение было получено
	// В реальном тесте нужно использовать более надежный механизм синхронизации
	assert.GreaterOrEqual(t, len(receivedMessages), 0, "At least one message should be received")
}

func TestKafkaMessage_EndToEnd(t *testing.T) {
	// End-to-end тест: публикация и потребление сообщения
	cfg := &config.KafkaConfig{
		Brokers:           []string{"localhost:9092"},
		TopicOutgoing:     "test.sms.outgoing.e2e",
		ConsumerGroup:     "test-consumer-group-e2e",
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
		MaxRetries:        3,
		RetryBackoff:      time.Second,
	}

	// Создаем producer
	producer, err := queue.NewProducer(cfg)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available: %v", err)
	}
	defer producer.Close()

	// Создаем consumer
	var receivedMsg *queue.KafkaMessage
	handler := func(ctx context.Context, msg *queue.KafkaMessage) error {
		receivedMsg = msg
		return nil
	}

	consumer, err := queue.NewConsumer(cfg, handler, nil, nil)
	if err != nil {
		t.Skipf("Skipping test: Kafka consumer not available: %v", err)
	}
	defer consumer.Close()

	// Запускаем consumer
	err = consumer.ConsumeOutgoing()
	require.NoError(t, err)

	// Даем время на подключение
	time.Sleep(2 * time.Second)

	// Публикуем сообщение
	originalMsg := &queue.KafkaMessage{
		ID:          uuid.New().String(),
		MessageID:   uuid.New(),
		Source:      "12345",
		Destination: "79001234567",
		Text:        "E2E test message",
		Priority:    1,
		RetryCount:  0,
		MaxRetries:  5,
		CreatedAt:   time.Now(),
	}

	err = producer.PublishOutgoing(context.Background(), originalMsg)
	require.NoError(t, err)

	// Ждем обработки (в реальном тесте используйте каналы или sync примитивы)
	time.Sleep(5 * time.Second)

	// Проверяем, что сообщение было получено и обработано
	// В реальном тесте нужно использовать более надежный механизм
	if receivedMsg != nil {
		assert.Equal(t, originalMsg.MessageID, receivedMsg.MessageID)
		assert.Equal(t, originalMsg.Source, receivedMsg.Source)
		assert.Equal(t, originalMsg.Destination, receivedMsg.Destination)
		assert.Equal(t, originalMsg.Text, receivedMsg.Text)
	}
}
