package domain

import "time"

// HealthStatus представляет статус здоровья провайдера
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
)

// Health представляет состояние здоровья провайдера
type Health struct {
	ProviderID         string
	Status             HealthStatus
	ActiveConnections  int
	TotalConnections   int
	SuccessRate        int // 0-100
	MessagesSent24h    int64
	MessagesFailed24h  int64
	LastSuccess        *time.Time
	LastFailure        *time.Time
	LastError          string
}

// CalculateStatus вычисляет статус на основе метрик
func (h *Health) CalculateStatus() HealthStatus {
	if h.TotalConnections == 0 {
		return HealthStatusUnhealthy
	}

	// Если нет активных соединений, провайдер нездоров
	if h.ActiveConnections == 0 {
		return HealthStatusUnhealthy
	}

	// Если активных соединений меньше половины от общего количества, статус degraded
	if h.ActiveConnections < h.TotalConnections/2 {
		return HealthStatusDegraded
	}

	// Если успешность меньше 50%, статус degraded
	if h.SuccessRate < 50 {
		return HealthStatusDegraded
	}

	// Если успешность меньше 20%, статус unhealthy
	if h.SuccessRate < 20 {
		return HealthStatusUnhealthy
	}

	return HealthStatusHealthy
}
