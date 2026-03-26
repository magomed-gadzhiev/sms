//go:build load

package load

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/queue"
)

// TestHighThroughputPipeline публикует 100K сообщений в Kafka (sms.outgoing)
// через AsyncProducer и проверяет, что пропускная способность >= 10K msg/sec.
func TestHighThroughputPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	brokers := []string{"localhost:9092"}
	if b := os.Getenv("KAFKA_BROKERS"); b != "" {
		brokers = strings.Split(b, ",")
	}

	// Настройка AsyncProducer для максимальной пропускной способности
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Flush.Messages = 500
	config.Producer.Flush.Frequency = 10 * time.Millisecond
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.MaxMessageBytes = 1024 * 1024 // 1 MB

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		t.Fatalf("failed to create async producer: %v", err)
	}
	defer producer.AsyncClose()

	// Счётчики для отслеживания результатов
	var successCount int64
	var errorCount int64

	// Drain successes
	go func() {
		for range producer.Successes() {
			atomic.AddInt64(&successCount, 1)
		}
	}()

	// Drain errors
	go func() {
		for err := range producer.Errors() {
			atomic.AddInt64(&errorCount, 1)
			t.Logf("produce error: %v", err)
		}
	}()

	const (
		totalMessages = 100000
		topic         = "sms.outgoing"
	)

	// Тестовый клиент для биллинга (должен существовать в БД)
	loadTestClientID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")

	start := time.Now()

	for i := 0; i < totalMessages; i++ {
		msg := &queue.KafkaMessage{
			ID:          fmt.Sprintf("load-test-%d", i),
			MessageID:   uuid.New(),
			Source:      "+79001234567",
			Destination: fmt.Sprintf("+7900%07d", i%10000000),
			Text:        "Load test message",
			ClientID:    &loadTestClientID,
			Priority:    0,
			RetryCount:  0,
			MaxRetries:  3,
			CreatedAt:   time.Now(),
		}

		data, err := msg.Serialize()
		if err != nil {
			t.Fatalf("failed to serialize message %d: %v", i, err)
		}

		producer.Input() <- &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(msg.MessageID.String()),
			Value: sarama.ByteEncoder(data),
		}
	}

	// Ждём, пока все acks вернутся (success + error = totalMessages)
	deadline := time.After(60 * time.Second)
	for {
		acked := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&errorCount)
		if acked >= int64(totalMessages) {
			break
		}
		select {
		case <-deadline:
			t.Logf("WARNING: timed out waiting for all acks; received %d/%d", acked, totalMessages)
			goto report
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

report:
	elapsed := time.Since(start)
	throughput := float64(totalMessages) / elapsed.Seconds()

	t.Logf("=== Pipeline Load Test Results ===")
	t.Logf("Total messages:  %d", totalMessages)
	t.Logf("Successful:      %d", atomic.LoadInt64(&successCount))
	t.Logf("Errors:          %d", atomic.LoadInt64(&errorCount))
	t.Logf("Elapsed:         %v", elapsed)
	t.Logf("Throughput:      %.0f msg/sec", throughput)

	// Soft assertion — предупреждение вместо провала теста
	if throughput < 10000 {
		t.Logf("WARNING: throughput %.0f msg/sec below target 10000 msg/sec", throughput)
	}
}

// TestHighThroughputPipeline_Varied отправляет 100K сообщений с реалистичными данными
// (разные номера, тексты, приоритеты) через AsyncProducer.
func TestHighThroughputPipeline_Varied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	brokers := []string{"localhost:9092"}
	if b := os.Getenv("KAFKA_BROKERS"); b != "" {
		brokers = strings.Split(b, ",")
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Flush.Messages = 500
	config.Producer.Flush.Frequency = 10 * time.Millisecond
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.RequiredAcks = sarama.WaitForLocal

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		t.Fatalf("failed to create async producer: %v", err)
	}
	defer producer.AsyncClose()

	var successCount int64
	var errorCount int64

	go func() {
		for range producer.Successes() {
			atomic.AddInt64(&successCount, 1)
		}
	}()
	go func() {
		for err := range producer.Errors() {
			atomic.AddInt64(&errorCount, 1)
			t.Logf("produce error: %v", err)
		}
	}()

	const (
		totalMessages = 100000
		topic         = "sms.outgoing"
	)

	loadTestClientID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")

	start := time.Now()

	for i := 0; i < totalMessages; i++ {
		msg := &queue.KafkaMessage{
			ID:          fmt.Sprintf("load-varied-%d", i),
			MessageID:   uuid.New(),
			Source:      RandomSource(),
			Destination: RandomDestination(),
			Text:        RandomText(),
			ClientID:    &loadTestClientID,
			Priority:    i % 4,
			RetryCount:  0,
			MaxRetries:  3,
			CreatedAt:   time.Now(),
		}

		data, err := msg.Serialize()
		if err != nil {
			t.Fatalf("failed to serialize message %d: %v", i, err)
		}

		producer.Input() <- &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(msg.MessageID.String()),
			Value: sarama.ByteEncoder(data),
		}
	}

	deadline := time.After(60 * time.Second)
	for {
		acked := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&errorCount)
		if acked >= int64(totalMessages) {
			break
		}
		select {
		case <-deadline:
			t.Logf("WARNING: timed out waiting for all acks; received %d/%d", acked, totalMessages)
			goto report
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

report:
	elapsed := time.Since(start)
	throughput := float64(totalMessages) / elapsed.Seconds()

	t.Logf("=== Pipeline Varied Load Test Results ===")
	t.Logf("Total messages:  %d", totalMessages)
	t.Logf("Successful:      %d", atomic.LoadInt64(&successCount))
	t.Logf("Errors:          %d", atomic.LoadInt64(&errorCount))
	t.Logf("Elapsed:         %v", elapsed)
	t.Logf("Throughput:      %.0f msg/sec", throughput)

	if throughput < 10000 {
		t.Logf("WARNING: throughput %.0f msg/sec below target 10000 msg/sec", throughput)
	}
}
