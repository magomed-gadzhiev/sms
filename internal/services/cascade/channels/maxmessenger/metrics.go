package maxmessenger

import "github.com/prometheus/client_golang/prometheus"

// MaxMessengerMetrics — channel-specific Prometheus метрики для Max Messenger
type MaxMessengerMetrics struct {
	WebhooksReceived   *prometheus.CounterVec
	ReachabilityChecks *prometheus.CounterVec
	SendLatency        *prometheus.HistogramVec
	ActiveAttempts     prometheus.Gauge
}

// NewMaxMessengerMetrics создаёт и регистрирует метрики Max Messenger
func NewMaxMessengerMetrics() *MaxMessengerMetrics {
	m := &MaxMessengerMetrics{
		WebhooksReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "max_messenger_webhooks_received_total",
				Help: "Total number of webhooks received from Max Messenger API",
			},
			[]string{"status"}, // valid | invalid_signature | unknown_attempt
		),
		ReachabilityChecks: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "max_messenger_reachability_checks_total",
				Help: "Total number of reachability checks for Max Messenger",
			},
			[]string{"result"}, // registered | not_registered | error | cache_hit
		),
		SendLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "max_messenger_send_latency_seconds",
				Help:    "Latency of sending messages via Max Messenger API",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"status"}, // success | rate_limited | error
		),
		ActiveAttempts: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "max_messenger_active_attempts",
				Help: "Number of currently active Max Messenger delivery attempts",
			},
		),
	}

	prometheus.MustRegister(
		m.WebhooksReceived,
		m.ReachabilityChecks,
		m.SendLatency,
		m.ActiveAttempts,
	)

	return m
}
