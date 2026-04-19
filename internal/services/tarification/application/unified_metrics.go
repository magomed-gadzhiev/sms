// unified_metrics.go
package application

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Метрики unified hot path. Имена с префиксом tarification_unified_*
// чтобы отделять от legacy метрик и от общих tarification_*.
var (
	unifiedResolveTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_unified_resolve_total",
			Help: "Resolve-вызовы hot-path unified. cache=hit|miss, branch=subaccount|margin.",
		},
		[]string{"cache", "branch"},
	)

	unifiedResolveLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tarification_unified_resolve_duration_seconds",
			Help:    "Длительность resolve в unified hot-path.",
			Buckets: []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"branch"}, // subaccount | margin
	)

	unifiedPriceNotFoundTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tarification_unified_price_not_found_total",
			Help: "Количество случаев, когда для запроса не нашлось применимого правила. Должно быть 0 — триггер алерта.",
		},
	)

	unifiedFallbackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_unified_fallback_total",
			Help: "Fallback unified → legacy в hot-path. reason=not_found|resolve_error|calc_error|counter_error.",
		},
		[]string{"reason"},
	)

	unifiedTarifyTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_unified_tarify_total",
			Help: "TarifyMessage через unified path. result=approved|rejected.",
		},
		[]string{"result"},
	)

	unifiedMarginPerHourRub = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tarification_unified_margin_rub_total",
			Help: "Суммарная агрегаторская маржа в RUB через unified путь. Per-hour rate считается в Grafana через rate().",
		},
	)
)
