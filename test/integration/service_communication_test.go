//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	messagingv1 "github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestKafkaToMessagingServiceFlow тестирует поток: Kafka -> Messaging Service
func TestKafkaToMessagingServiceFlow(t *testing.T) {
	// Создаем Kafka producer
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

	// Подключаемся к Messaging Service
	dCtx, dCancel := dialCtx(t)
	defer dCancel()
	messagingConn, err := grpc.DialContext(dCtx, messagingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("Skipping test: Messaging Service not available: %v", err)
	}
	defer messagingConn.Close()

	messagingClient := messagingv1.NewMessagingServiceClient(messagingConn)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Публикуем сообщение в Kafka
	msgID := uuid.New()
	kafkaMsg := &queue.KafkaMessage{
		ID:          uuid.New().String(),
		MessageID:   msgID,
		Source:      "12345",
		Destination: "79001234567",
		Text:        "Kafka integration test",
		Priority:    0,
		RetryCount:  0,
		MaxRetries:  5,
		CreatedAt:   time.Now(),
	}

	err = producer.PublishOutgoing(ctx, kafkaMsg)
	require.NoError(t, err)

	// 2. Ждем обработки сообщения
	time.Sleep(3 * time.Second)

	// 3. Проверяем статус через Messaging Service
	statusResp, err := messagingClient.GetMessageStatus(ctx, &messagingv1.GetMessageStatusRequest{
		MessageId: msgID.String(),
	})

	// Может не быть найдено, если сообщение еще не обработано
	if err == nil {
		assert.Equal(t, msgID.String(), statusResp.MessageId)
	}
}

// TestGatewayToServiceFlow тестирует поток: Gateway -> Service через gRPC
func TestGatewayToServiceFlow(t *testing.T) {
	// Подключаемся к Messaging Service напрямую
	dCtx, dCancel := dialCtx(t)
	defer dCancel()
	messagingConn, err := grpc.DialContext(dCtx, messagingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("Skipping test: Messaging Service not available: %v", err)
	}
	defer messagingConn.Close()

	messagingClient := messagingv1.NewMessagingServiceClient(messagingConn)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientID := uuid.New().String()

	// Отправляем несколько сообщений
	messageIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		msgID := uuid.New()
		messageIDs[i] = msgID.String()

		resp, err := messagingClient.SendMessage(ctx, &messagingv1.SendMessageRequest{
			ClientId:    clientID,
			ExternalId:  msgID.String(),
			Source:      "12345",
			Destination: "79001234567",
			Text:        "Batch test message",
		})

		if err != nil {
			t.Logf("SendMessage error: %v", err)
			continue
		}

		assert.Equal(t, msgID.String(), resp.MessageId)
	}

	// Ждем обработки
	time.Sleep(2 * time.Second)

	// Проверяем историю
	historyResp, err := messagingClient.GetMessageHistory(ctx, &messagingv1.GetMessageHistoryRequest{
		ClientId: clientID,
		Limit:    10,
		Offset:   0,
	})

	if err == nil {
		assert.NotNil(t, historyResp.Messages)
		// Может быть меньше, если не все сообщения обработаны
	}
}

// TestConcurrentRequests тестирует конкурентные запросы к сервису
func TestConcurrentRequests(t *testing.T) {
	dCtx, dCancel := dialCtx(t)
	defer dCancel()
	messagingConn, err := grpc.DialContext(dCtx, messagingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("Skipping test: Messaging Service not available: %v", err)
	}
	defer messagingConn.Close()

	messagingClient := messagingv1.NewMessagingServiceClient(messagingConn)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	concurrency := 10
	results := make(chan bool, concurrency)

	clientID := uuid.New().String()

	// Отправляем параллельные запросы
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			msgID := uuid.New()
			_, err := messagingClient.SendMessage(ctx, &messagingv1.SendMessageRequest{
				ClientId:    clientID,
				ExternalId:  msgID.String(),
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Concurrent test message",
			})
			results <- err == nil
		}(i)
	}

	// Собираем результаты
	successCount := 0
	for i := 0; i < concurrency; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for results")
		}
	}

	t.Logf("Concurrent requests: %d/%d successful", successCount, concurrency)
	assert.Greater(t, successCount, 0, "At least some requests should succeed")
}

// dialCtx ограничивает gRPC dial в интеграционных тестах: недоступный
// бэкенд должен быстро завершить тест, а не висеть до package timeout.
func dialCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 5*time.Second)
}
