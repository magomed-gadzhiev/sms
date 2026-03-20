//go:build load

package load

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	targetTPS = 10000 // Цель: 10,000 сообщений в секунду
)

// TestHighPerformanceLoad тест для достижения 10K сообщений/сек
func TestHighPerformanceLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high performance load test in short mode")
	}

	// Параметры теста для достижения 10K msg/s
	const (
		warmupDuration    = 30 * time.Second
		testDuration      = 2 * time.Minute
		workers           = 200  // Количество параллельных воркеров
		messagesPerSecond = 50   // Сообщений на воркера в секунду (200 * 50 = 10K)
	)

	var (
		successCount    int64
		failureCount    int64
		totalRequests   int64
		requestDuration int64
		mu              sync.Mutex
		latencies       []time.Duration
	)

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 500,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	stop := make(chan bool)
	startTime := time.Now()

	// Warm-up фаза
	t.Log("Starting warm-up phase...")
	warmupStop := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(workerID int) {
			ticker := time.NewTicker(200 * time.Millisecond) // 5 req/sec на воркера
			defer ticker.Stop()
			for {
				select {
				case <-warmupStop:
					return
				case <-ticker.C:
					sendRequest(client, workerID, &successCount, &failureCount, &totalRequests, &requestDuration, &mu, &latencies)
				}
			}
		}(i)
	}

	time.Sleep(warmupDuration)
	close(warmupStop)
	time.Sleep(2 * time.Second)

	// Основной тест
	t.Log("Starting main load test...")
	successCount = 0
	failureCount = 0
	totalRequests = 0
	requestDuration = 0
	latencies = make([]time.Duration, 0, 100000)

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			ticker := time.NewTicker(time.Second / messagesPerSecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					sendRequest(client, workerID, &successCount, &failureCount, &totalRequests, &requestDuration, &mu, &latencies)
				}
			}
		}(i)
	}

	// Запускаем основной тест
	time.Sleep(testDuration)
	close(stop)

	// Ждем завершения всех горутин
	time.Sleep(5 * time.Second)

	duration := time.Since(startTime)
	actualTPS := float64(successCount) / duration.Seconds()

	t.Logf("\n=== High Performance Load Test Results ===")
	t.Logf("Test Duration: %v", duration)
	t.Logf("Total Requests: %d", totalRequests)
	t.Logf("Successful: %d", successCount)
	t.Logf("Failed: %d", failureCount)
	t.Logf("Success Rate: %.2f%%", float64(successCount)/float64(totalRequests)*100)
	t.Logf("Throughput: %.2f msg/s", actualTPS)
	t.Logf("Average Latency: %.2f ms", float64(requestDuration)/float64(totalRequests)/1000000)

	// Статистика по латентности
	mu.Lock()
	if len(latencies) > 0 {
		p50, p95, p99 := calculatePercentiles(latencies)
		t.Logf("Latency p50: %.2f ms", float64(p50)/float64(time.Millisecond))
		t.Logf("Latency p95: %.2f ms", float64(p95)/float64(time.Millisecond))
		t.Logf("Latency p99: %.2f ms", float64(p99)/float64(time.Millisecond))
	}
	mu.Unlock()

	// Проверяем, достигли ли мы цели
	if actualTPS < targetTPS*0.8 {
		t.Errorf("Throughput too low: %.2f msg/s (target: %d msg/s)", actualTPS, targetTPS)
	}

	successRate := float64(successCount) / float64(totalRequests)
	if successRate < 0.95 {
		t.Errorf("Success rate too low: %.2f%%", successRate*100)
	}
}

func sendRequest(client *http.Client, workerID int, successCount, failureCount, totalRequests, requestDuration *int64,
	mu *sync.Mutex, latencies *[]time.Duration) {
	atomic.AddInt64(totalRequests, 1)

	req := map[string]interface{}{
		"source":      fmt.Sprintf("12345%d", workerID%10),
		"destination": fmt.Sprintf("7900123456%d", atomic.LoadInt64(totalRequests)%10),
		"text":        fmt.Sprintf("High perf test #%d", atomic.LoadInt64(totalRequests)),
	}

	body, err := json.Marshal(req)
	if err != nil {
		atomic.AddInt64(failureCount, 1)
		return
	}

	start := time.Now()
	httpReq, err := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
	if err != nil {
		atomic.AddInt64(failureCount, 1)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(httpReq)
	latency := time.Since(start)

	if err != nil {
		atomic.AddInt64(failureCount, 1)
		return
	}

	atomic.AddInt64(requestDuration, latency.Nanoseconds())

	mu.Lock()
	*latencies = append(*latencies, latency)
	if len(*latencies) > 100000 {
		*latencies = (*latencies)[len(*latencies)-100000:]
	}
	mu.Unlock()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		atomic.AddInt64(successCount, 1)
	} else {
		atomic.AddInt64(failureCount, 1)
	}

	resp.Body.Close()
}

func calculatePercentiles(latencies []time.Duration) (p50, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Простая сортировка (можно оптимизировать с помощью heap)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p50Idx := int(float64(len(sorted)) * 0.50)
	p95Idx := int(float64(len(sorted)) * 0.95)
	p99Idx := int(float64(len(sorted)) * 0.99)

	if p50Idx >= len(sorted) {
		p50Idx = len(sorted) - 1
	}
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}

	return sorted[p50Idx], sorted[p95Idx], sorted[p99Idx]
}

// TestSustainedLoad тестирует устойчивую нагрузку 10K msg/s в течение длительного времени
func TestSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	const (
		duration = 10 * time.Minute
		workers  = 200
		msgPerSec = 50
	)

	var successCount int64
	var failureCount int64

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 500,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	stop := make(chan bool)
	start := time.Now()

	// Мониторинг производительности
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				elapsed := time.Since(start).Seconds()
				currentTPS := float64(atomic.LoadInt64(&successCount)) / elapsed
				t.Logf("[%v] Current TPS: %.2f, Success: %d, Failed: %d",
					time.Since(start), currentTPS,
					atomic.LoadInt64(&successCount), atomic.LoadInt64(&failureCount))
			}
		}
	}()

	// Запускаем воркеры
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			ticker := time.NewTicker(time.Second / msgPerSec)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					req := map[string]interface{}{
						"source":      fmt.Sprintf("12345%d", workerID%10),
						"destination": fmt.Sprintf("7900123456%d", time.Now().UnixNano()%10),
						"text":        "Sustained load test",
					}

					body, _ := json.Marshal(req)
					httpReq, _ := http.NewRequest("POST", apiBaseURL+"/api/v1/sms/send", bytes.NewBuffer(body))
					httpReq.Header.Set("Content-Type", "application/json")
					httpReq.Header.Set("X-API-Key", apiKey)

					resp, err := client.Do(httpReq)
					if err == nil {
						if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
							atomic.AddInt64(&successCount, 1)
						} else {
							atomic.AddInt64(&failureCount, 1)
						}
						resp.Body.Close()
					} else {
						atomic.AddInt64(&failureCount, 1)
					}
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stop)
	time.Sleep(5 * time.Second)

	elapsed := time.Since(start).Seconds()
	finalTPS := float64(atomic.LoadInt64(&successCount)) / elapsed
	successRate := float64(atomic.LoadInt64(&successCount)) / 
		float64(atomic.LoadInt64(&successCount)+atomic.LoadInt64(&failureCount))

	t.Logf("\n=== Sustained Load Test Results ===")
	t.Logf("Duration: %v", duration)
	t.Logf("Total Successful: %d", atomic.LoadInt64(&successCount))
	t.Logf("Total Failed: %d", atomic.LoadInt64(&failureCount))
	t.Logf("Average TPS: %.2f msg/s", finalTPS)
	t.Logf("Success Rate: %.2f%%", successRate*100)

	if finalTPS < targetTPS*0.8 {
		t.Errorf("Sustained throughput too low: %.2f msg/s (target: %d msg/s)", finalTPS, targetTPS)
	}

	if successRate < 0.95 {
		t.Errorf("Success rate too low: %.2f%%", successRate*100)
	}
}