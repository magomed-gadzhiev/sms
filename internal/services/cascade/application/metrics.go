package application

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CascadeMetrics содержит Prometheus-метрики для cascade-service
type CascadeMetrics struct {
	DeliveriesTotal    *prometheus.CounterVec
	AttemptsTotal      *prometheus.CounterVec
	DeliveryDuration   *prometheus.HistogramVec
	ActiveDeliveries   prometheus.Gauge
}

// NewCascadeMetrics регистрирует и возвращает все метрики cascade-service
func NewCascadeMetrics() *CascadeMetrics {
	return &CascadeMetrics{
		DeliveriesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cascade_deliveries_total",
			Help: "Общее количество каскадных доставок по статусу",
		}, []string{"status"}),

		AttemptsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cascade_attempts_total",
			Help: "Общее количество попыток доставки по типу канала и статусу",
		}, []string{"channel_type", "status"}),

		DeliveryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "cascade_delivery_duration_seconds",
			Help:    "Длительность каскадной доставки от создания до финального статуса",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300},
		}, []string{"status"}),

		ActiveDeliveries: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cascade_active_deliveries",
			Help: "Текущее количество доставок в статусе in_progress",
		}),
	}
}
