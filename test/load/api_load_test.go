//go:build load

package load

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/queue"
)

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var (
	apiBaseURL = getEnv("LOAD_TEST_BASE_URL", "http://localhost:8080")
	apiKey     = getEnv("LOAD_TEST_API_KEY", "test-api-key")
)

// drainBody читает и закрывает тело ответа для корректного переиспользования соединений
func drainBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// TestAPIGateway_Load отправляет множество запросов для нагрузочного тестирования
func TestAPIGateway_Load(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 10
	requestsPerWorker := 100

	results := make(chan bool, concurrency*requestsPerWorker)
	errors := make(chan error, concurrency*requestsPerWorker)

	start := time.Now()

	// Запускаем горутины для параллельных запросов
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			for j := 0; j < requestsPerWorker; j++ {
				req := map[string]interface{}{
					"source":      fmt.Sprintf("12345%d", workerID),
					"destination": fmt.Sprintf("7900123456%d", j%10),
					"text":        fmt.Sprintf("Load test message #%d from worker %d", j, workerID),
				}

				body, err := json.Marshal(req)
				if err != nil {
					errors <- err
					continue
				}

				httpReq, err := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
				if err != nil {
					errors <- err
					continue
				}

				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("X-API-Key", apiKey)

				resp, err := client.Do(httpReq)
				if err != nil {
					errors <- err
					continue
				}

				if resp.StatusCode == http.StatusOK {
					results <- true
				} else {
					results <- false
				}

				drainBody(resp)
			}
		}(i)
	}

	// Собираем результаты
	successCount := 0
	failureCount := 0
	totalRequests := concurrency * requestsPerWorker

	for i := 0; i < totalRequests; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			} else {
				failureCount++
			}
		case err := <-errors:
			t.Logf("Error: %v", err)
			failureCount++
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for results")
		}
	}

	duration := time.Since(start)
	throughput := float64(totalRequests) / duration.Seconds()

	t.Logf("Load test results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Failed: %d", failureCount)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f req/s", throughput)
	t.Logf("  Success rate: %.2f%%", float64(successCount)/float64(totalRequests)*100)

	// Проверяем, что успешных запросов достаточно
	successRate := float64(successCount) / float64(totalRequests)
	if successRate < 0.95 {
		t.Errorf("Success rate too low: %.2f%%", successRate*100)
	}
}

// TestAPIGateway_Stress отправляет запросы с увеличивающейся нагрузкой
func TestAPIGateway_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	stages := []struct {
		name        string
		concurrency int
		duration    time.Duration
	}{
		{"Warm up", 5, 10 * time.Second},
		{"Normal load", 20, 30 * time.Second},
		{"High load", 50, 30 * time.Second},
		{"Peak load", 100, 20 * time.Second},
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			results := make(chan bool, stage.concurrency*100)
			errors := make(chan error, stage.concurrency*100)

			start := time.Now()
			stop := make(chan bool)

			// Запускаем горутины
			for i := 0; i < stage.concurrency; i++ {
				go func(workerID int) {
					requestID := 0
					for {
						select {
						case <-stop:
							return
						default:
							req := map[string]interface{}{
								"source":      fmt.Sprintf("12345%d", workerID),
								"destination": fmt.Sprintf("7900123456%d", requestID%10),
								"text":        fmt.Sprintf("Stress test message #%d", requestID),
							}

							body, err := json.Marshal(req)
							if err != nil {
								errors <- err
								continue
							}

							httpReq, err := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
							if err != nil {
								errors <- err
								continue
							}

							httpReq.Header.Set("Content-Type", "application/json")
							httpReq.Header.Set("X-API-Key", apiKey)

							resp, err := client.Do(httpReq)
							if err != nil {
								errors <- err
								continue
							}

							results <- resp.StatusCode == http.StatusOK
							drainBody(resp)

							requestID++
							time.Sleep(100 * time.Millisecond) // Небольшая задержка между запросами
						}
					}
				}(i)
			}

			// Ждем указанное время
			time.Sleep(stage.duration)
			close(stop)

			// Собираем результаты
			successCount := 0
			failureCount := 0

			timeout := time.After(5 * time.Second)
			for {
				select {
				case success := <-results:
					if success {
						successCount++
					} else {
						failureCount++
					}
				case err := <-errors:
					t.Logf("Error: %v", err)
					failureCount++
				case <-timeout:
					goto done
				}
			}
		done:
			duration := time.Since(start)
			totalRequests := successCount + failureCount
			if totalRequests > 0 {
				throughput := float64(totalRequests) / duration.Seconds()
				successRate := float64(successCount) / float64(totalRequests) * 100

				t.Logf("  Requests: %d", totalRequests)
				t.Logf("  Successful: %d", successCount)
				t.Logf("  Failed: %d", failureCount)
				t.Logf("  Throughput: %.2f req/s", throughput)
				t.Logf("  Success rate: %.2f%%", successRate)
			}
		})
	}
}

// BenchmarkAPIGateway_SendSMS бенчмарк для отправки SMS
func BenchmarkAPIGateway_SendSMS(b *testing.B) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req := map[string]interface{}{
		"source":      "12345",
		"destination": "79001234567",
		"text":        "Benchmark test message",
	}

	body, err := json.Marshal(req)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpReq, err := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
		if err != nil {
			b.Fatal(err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-API-Key", apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			b.Fatal(err)
		}

		drainBody(resp)
	}
}

// TestAPIGateway_SendBatchSMS_Load нагрузочный тест для batch endpoint
func TestAPIGateway_SendBatchSMS_Load(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 5
	batchSize := 10
	requestsPerWorker := 20

	results := make(chan bool, concurrency*requestsPerWorker)
	errors := make(chan error, concurrency*requestsPerWorker)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			for j := 0; j < requestsPerWorker; j++ {
				messages := make([]map[string]interface{}, batchSize)
				for k := 0; k < batchSize; k++ {
					messages[k] = map[string]interface{}{
						"source":      fmt.Sprintf("12345%d", workerID),
						"destination": fmt.Sprintf("7900123456%d", k%10),
						"text":        fmt.Sprintf("Batch load test message #%d from worker %d", k, workerID),
					}
				}

				req := map[string]interface{}{
					"messages": messages,
				}

				body, err := json.Marshal(req)
				if err != nil {
					errors <- err
					continue
				}

				httpReq, err := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/batch", bytes.NewBuffer(body))
				if err != nil {
					errors <- err
					continue
				}

				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("X-API-Key", apiKey)

				resp, err := client.Do(httpReq)
				if err != nil {
					errors <- err
					continue
				}

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
					results <- true
				} else {
					results <- false
				}

				drainBody(resp)
			}
		}(i)
	}

	successCount := 0
	failureCount := 0
	totalRequests := concurrency * requestsPerWorker

	for i := 0; i < totalRequests; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			} else {
				failureCount++
			}
		case err := <-errors:
			t.Logf("Error: %v", err)
			failureCount++
		case <-time.After(60 * time.Second):
			t.Fatal("Timeout waiting for results")
		}
	}

	duration := time.Since(start)
	throughput := float64(totalRequests) / duration.Seconds()

	t.Logf("Batch load test results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Failed: %d", failureCount)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f req/s", throughput)
	t.Logf("  Success rate: %.2f%%", float64(successCount)/float64(totalRequests)*100)

	successRate := float64(successCount) / float64(totalRequests)
	if successRate < 0.95 {
		t.Errorf("Success rate too low: %.2f%%", successRate*100)
	}
}

// TestAPIGateway_GetStatus_Load нагрузочный тест для получения статуса
func TestAPIGateway_GetStatus_Load(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	// Сначала создаем несколько сообщений
	messageIDs := make([]string, 100)
	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 100; i++ {
		req := map[string]interface{}{
			"source":      "12345",
			"destination": fmt.Sprintf("7900123456%d", i%10),
			"text":        fmt.Sprintf("Load test message #%d", i),
		}

		body, _ := json.Marshal(req)
		httpReq, _ := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-API-Key", apiKey)

		resp, err := client.Do(httpReq)
		if err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted) {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			if msgID, ok := result["message_id"].(string); ok {
				messageIDs[i] = msgID
			}
		}
		drainBody(resp)
	}

	concurrency := 20
	requestsPerWorker := 50

	results := make(chan bool, concurrency*requestsPerWorker)
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			client := &http.Client{Timeout: 10 * time.Second}

			for j := 0; j < requestsPerWorker; j++ {
				messageID := messageIDs[j%len(messageIDs)]
				if messageID == "" {
					continue
				}

				httpReq, err := http.NewRequest("GET", apiBaseURL+"/api/v1/sms/status/"+messageID, nil)
				if err != nil {
					continue
				}

				httpReq.Header.Set("X-API-Key", apiKey)

				resp, err := client.Do(httpReq)
				if err != nil {
					continue
				}

				results <- resp.StatusCode == http.StatusOK
				drainBody(resp)
			}
		}(i)
	}

	successCount := 0
	failureCount := 0
	totalRequests := concurrency * requestsPerWorker

	for i := 0; i < totalRequests; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			} else {
				failureCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for results")
		}
	}

	duration := time.Since(start)
	throughput := float64(totalRequests) / duration.Seconds()

	t.Logf("GetStatus load test results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Failed: %d", failureCount)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f req/s", throughput)
}

// BenchmarkBatchAPIAsyncProducer сравнивает sync vs async публикацию
// батча из 1000 сообщений в Kafka.
func BenchmarkBatchAPIAsyncProducer(b *testing.B) {
	brokers := []string{"localhost:9092"}
	if br := os.Getenv("KAFKA_BROKERS"); br != "" {
		brokers = strings.Split(br, ",")
	}

	const (
		batchSize = 1000
		topic     = "sms.outgoing"
	)

	// Подготовим батч сообщений один раз
	messages := make([]*sarama.ProducerMessage, batchSize)
	for i := 0; i < batchSize; i++ {
		msg := &queue.KafkaMessage{
			ID:          fmt.Sprintf("bench-%d", i),
			MessageID:   uuid.New(),
			Source:      "+79001234567",
			Destination: fmt.Sprintf("+7900%07d", i),
			Text:        "Benchmark test message",
			Priority:    0,
			RetryCount:  0,
			MaxRetries:  3,
			CreatedAt:   time.Now(),
		}
		data, err := msg.Serialize()
		if err != nil {
			b.Fatalf("failed to serialize message %d: %v", i, err)
		}
		messages[i] = &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(msg.MessageID.String()),
			Value: sarama.ByteEncoder(data),
		}
	}

	b.Run("SyncProducer", func(b *testing.B) {
		config := sarama.NewConfig()
		config.Producer.Return.Successes = true
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Compression = sarama.CompressionSnappy

		producer, err := sarama.NewSyncProducer(brokers, config)
		if err != nil {
			b.Fatalf("failed to create sync producer: %v", err)
		}
		defer producer.Close()

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			for _, msg := range messages {
				// Создаём копию, т.к. sarama может мутировать сообщение
				m := &sarama.ProducerMessage{
					Topic: msg.Topic,
					Key:   msg.Key,
					Value: msg.Value,
				}
				if _, _, err := producer.SendMessage(m); err != nil {
					b.Fatalf("sync send error: %v", err)
				}
			}
		}
		b.StopTimer()
		b.ReportMetric(float64(batchSize*b.N)/b.Elapsed().Seconds(), "msg/sec")
	})

	b.Run("AsyncProducer", func(b *testing.B) {
		config := sarama.NewConfig()
		config.Producer.Return.Successes = true
		config.Producer.Return.Errors = true
		config.Producer.Flush.Messages = 500
		config.Producer.Flush.Frequency = 10 * time.Millisecond
		config.Producer.Compression = sarama.CompressionSnappy
		config.Producer.RequiredAcks = sarama.WaitForLocal

		producer, err := sarama.NewAsyncProducer(brokers, config)
		if err != nil {
			b.Fatalf("failed to create async producer: %v", err)
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
			for range producer.Errors() {
				atomic.AddInt64(&errorCount, 1)
			}
		}()

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			atomic.StoreInt64(&successCount, 0)
			atomic.StoreInt64(&errorCount, 0)

			for _, msg := range messages {
				m := &sarama.ProducerMessage{
					Topic: msg.Topic,
					Key:   msg.Key,
					Value: msg.Value,
				}
				producer.Input() <- m
			}

			// Ждём, пока все acks вернутся
			deadline := time.After(30 * time.Second)
			for {
				acked := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&errorCount)
				if acked >= int64(batchSize) {
					break
				}
				select {
				case <-deadline:
					b.Fatalf("timeout waiting for acks: got %d/%d", acked, batchSize)
				default:
					time.Sleep(1 * time.Millisecond)
				}
			}
		}
		b.StopTimer()
		b.ReportMetric(float64(batchSize*b.N)/b.Elapsed().Seconds(), "msg/sec")
	})
}

// TestAPIGateway_MixedLoad смешанная нагрузка на все эндпоинты
func TestAPIGateway_MixedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	concurrency := 10
	requestsPerWorker := 30

	results := make(chan map[string]interface{}, concurrency*requestsPerWorker*3)
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			client := &http.Client{Timeout: 10 * time.Second}

			for j := 0; j < requestsPerWorker; j++ {
				// SendSMS
				req := map[string]interface{}{
					"source":      fmt.Sprintf("12345%d", workerID),
					"destination": fmt.Sprintf("7900123456%d", j%10),
					"text":        fmt.Sprintf("Mixed load test #%d", j),
				}

				body, _ := json.Marshal(req)
				httpReq, _ := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("X-API-Key", apiKey)

				resp, err := client.Do(httpReq)
				if err == nil {
					var result map[string]interface{}
					json.NewDecoder(resp.Body).Decode(&result)
					results <- map[string]interface{}{"endpoint": "send", "success": resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted}
					drainBody(resp)
				}

				// GetHistory
				httpReq, _ = http.NewRequest("GET", apiBaseURL+"/api/v1/sms/history?limit=10", nil)
				httpReq.Header.Set("X-API-Key", apiKey)
				resp, err = client.Do(httpReq)
				if err == nil {
					results <- map[string]interface{}{"endpoint": "history", "success": resp.StatusCode == http.StatusOK}
					drainBody(resp)
				}

				// Health
				httpReq, _ = http.NewRequest("GET", apiBaseURL+"/health", nil)
				resp, err = client.Do(httpReq)
				if err == nil {
					results <- map[string]interface{}{"endpoint": "health", "success": resp.StatusCode == http.StatusOK}
					drainBody(resp)
				}
			}
		}(i)
	}

	successCount := 0
	failureCount := 0
	endpointStats := make(map[string]int)

	timeout := time.After(60 * time.Second)
	for {
		select {
		case result := <-results:
			if success, ok := result["success"].(bool); ok && success {
				successCount++
			} else {
				failureCount++
			}
			if endpoint, ok := result["endpoint"].(string); ok {
				endpointStats[endpoint]++
			}
		case <-timeout:
			goto done
		}
	}

done:
	duration := time.Since(start)
	totalRequests := successCount + failureCount
	if totalRequests > 0 {
		throughput := float64(totalRequests) / duration.Seconds()

		t.Logf("Mixed load test results:")
		t.Logf("  Total requests: %d", totalRequests)
		t.Logf("  Successful: %d", successCount)
		t.Logf("  Failed: %d", failureCount)
		t.Logf("  Duration: %v", duration)
		t.Logf("  Throughput: %.2f req/s", throughput)
		t.Logf("  Endpoint stats: %v", endpointStats)
	}
}