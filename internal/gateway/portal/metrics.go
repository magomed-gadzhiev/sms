package portal

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// LoginAttemptsTotal — счётчик попыток входа
	LoginAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "portal",
			Name:      "login_attempts_total",
			Help:      "Общее количество попыток входа в портал",
		},
		[]string{"status"}, // success, failed, blocked
	)

	// ActiveSessionsGauge — количество активных сессий
	ActiveSessionsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "portal",
			Name:      "active_sessions",
			Help:      "Количество активных сессий в портале",
		},
	)

	// APIRequestsTotal — счётчик API-запросов по эндпоинтам
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "portal",
			Name:      "api_requests_total",
			Help:      "Общее количество API-запросов к порталу",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// AuditEventsPublishedTotal — счётчик опубликованных audit-событий
	AuditEventsPublishedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "portal",
			Name:      "audit_events_published_total",
			Help:      "Общее количество опубликованных событий аудита",
		},
	)

	// TOTPSetupTotal — счётчик настроек 2FA
	TOTPSetupTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "portal",
			Name:      "totp_operations_total",
			Help:      "Количество операций с 2FA",
		},
		[]string{"operation"}, // setup, verify, disable
	)

	// SubAccountOperationsTotal — счётчик операций с sub-accounts
	SubAccountOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "portal",
			Name:      "sub_account_operations_total",
			Help:      "Количество операций с sub-accounts",
		},
		[]string{"operation"}, // create, delete, transfer
	)
)

// NB: MaterializeFailureTotal перенесён в internal/services/network/metrics.go
// в Plan 4 Task 5 — устранение циклического импорта между этим пакетом и
// helper'ом ApplyAssignmentMaterializers. Namespace "portal" сохранён, метрика
// в Grafana по-прежнему доступна как portal_materialize_failure_total.
