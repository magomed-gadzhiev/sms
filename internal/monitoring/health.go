package monitoring

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"

	"github.com/smpp-server/smpp-server/internal/config"
)

// HealthStatus представляет статус здоровья сервиса
type HealthStatus struct {
	Status      string                 `json:"status"`
	Service     string                 `json:"service"`
	Version     string                 `json:"version,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Checks      map[string]CheckResult `json:"checks,omitempty"`
}

// CheckResult представляет результат проверки зависимости
type CheckResult struct {
	Status      string        `json:"status"`
	Message     string        `json:"message,omitempty"`
	ResponseTime time.Duration `json:"response_time_ms,omitempty"`
}

// HealthChecker представляет проверщик здоровья сервиса
type HealthChecker struct {
	serviceName string
	version     string
	db          *sql.DB
	redis       *redis.Client
	kafka       sarama.Client
}

// NewHealthChecker создает новый health checker
func NewHealthChecker(serviceName, version string) *HealthChecker {
	return &HealthChecker{
		serviceName: serviceName,
		version:     version,
	}
}

// SetDatabase устанавливает подключение к БД для проверки
func (h *HealthChecker) SetDatabase(db *sql.DB) {
	h.db = db
}

// SetRedis устанавливает подключение к Redis для проверки
func (h *HealthChecker) SetRedis(redis *redis.Client) {
	h.redis = redis
}

// SetKafka устанавливает клиент Kafka для проверки
func (h *HealthChecker) SetKafka(kafka sarama.Client) {
	h.kafka = kafka
}

// Check выполняет проверку здоровья сервиса
func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:    "ok",
		Service:   h.serviceName,
		Version:   h.version,
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
	}

	// Проверка базы данных
	if h.db != nil {
		check := h.checkDatabase(ctx)
		status.Checks["database"] = check
		if check.Status != "ok" {
			status.Status = "degraded"
		}
	}

	// Проверка Redis
	if h.redis != nil {
		check := h.checkRedis(ctx)
		status.Checks["redis"] = check
		if check.Status != "ok" && status.Status == "ok" {
			status.Status = "degraded"
		}
	}

	// Проверка Kafka
	if h.kafka != nil {
		check := h.checkKafka(ctx)
		status.Checks["kafka"] = check
		if check.Status != "ok" && status.Status == "ok" {
			status.Status = "degraded"
		}
	}

	return status
}

// checkDatabase проверяет подключение к базе данных
func (h *HealthChecker) checkDatabase(ctx context.Context) CheckResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := h.db.PingContext(ctx)
	duration := time.Since(start)

	if err != nil {
		return CheckResult{
			Status:      "error",
			Message:     err.Error(),
			ResponseTime: duration,
		}
	}

	return CheckResult{
		Status:      "ok",
		ResponseTime: duration,
	}
}

// checkRedis проверяет подключение к Redis
func (h *HealthChecker) checkRedis(ctx context.Context) CheckResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := h.redis.Ping(ctx).Err()
	duration := time.Since(start)

	if err != nil {
		return CheckResult{
			Status:      "error",
			Message:     err.Error(),
			ResponseTime: duration,
		}
	}

	return CheckResult{
		Status:      "ok",
		ResponseTime: duration,
	}
}

// checkKafka проверяет подключение к Kafka
func (h *HealthChecker) checkKafka(ctx context.Context) CheckResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	brokers := h.kafka.Brokers()
	if len(brokers) == 0 {
		return CheckResult{
			Status:      "error",
			Message:     "нет доступных брокеров",
			ResponseTime: time.Since(start),
		}
	}

	// Проверяем подключение к первому брокеру
	broker := brokers[0]
	if err := broker.Open(h.kafka.Config()); err != nil {
		return CheckResult{
			Status:      "error",
			Message:     err.Error(),
			ResponseTime: time.Since(start),
		}
	}
	defer broker.Close()

	connected, err := broker.Connected()
	duration := time.Since(start)

	if err != nil || !connected {
		return CheckResult{
			Status:      "error",
			Message:     "не удалось подключиться к брокеру",
			ResponseTime: duration,
		}
	}

	return CheckResult{
		Status:      "ok",
		ResponseTime: duration,
	}
}

// Handler возвращает HTTP handler для health check
func (h *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHealthCheckTimeout)
		defer cancel()

		status := h.Check(ctx)

		statusCode := http.StatusOK
		if status.Status == "error" {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(status)
	}
}

// LivenessHandler возвращает простой liveness probe
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	}
}

// ReadinessHandler возвращает readiness probe с проверкой зависимостей
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return h.Handler()
}
