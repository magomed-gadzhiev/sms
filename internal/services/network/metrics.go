package network

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MaterializeFailureTotal — счётчик partial-failure'ов в materializer'ах
// (provider_set / route_set ApplyToClient). Plan 3 Task 4: SRA committed,
// materialize failed → 200+warnings вместо 500; этот counter alerter'у
// сообщает реальную деградацию.
//
// Plan 4 Task 5: метрика перенесена сюда из internal/gateway/portal/metrics.go
// для устранения циклического импорта (helper в этом пакете → metrics; handlers
// → этот пакет → handlers недопустим). Namespace "portal" сохранён, чтобы
// существующие Grafana-запросы не сломались.
var MaterializeFailureTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "portal",
		Name:      "materialize_failure_total",
		Help:      "Materializer ApplyToClient partial-failures (SRA committed, materialize failed)",
	},
	[]string{"operation"}, // provider, route
)

// SRAPendingRetryGauge — count of subaccount_routing_assignment rows with
// last_materialize_error_at IS NOT NULL (Plan 3 Task 4 → Plan 4 Task 6 retry
// queue). Set per-tick by retry_loop.RetryPendingOnce. Capped at 100 because
// RetryPendingOnce reads with LIMIT 100 — if backlog > 100, this gauge
// underreports. Combined with rate(MaterializeFailureTotal[5m]) it gives ops
// a clear view: gauge bumping into 100 ⇒ backlog overflow, alert via
// dedicated query (e.g., always-100 trend).
var SRAPendingRetryGauge = promauto.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "portal",
		Name:      "sra_pending_retry_count",
		Help:      "Number of subaccount_routing_assignment rows pending materialize retry.",
	},
)
