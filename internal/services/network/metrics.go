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
// Plan 5 Task 2 (A5): добавлен label `source` (initial|retry). Initial — первый
// PUT/Bulk-сбой материализации (новая деградация); retry — per-tick re-attempt
// retry_loop'ом по уже стоящему stuck-row (амплификатор cardinality, не новый
// сбой). Ops-алёрты должны фильтровать `source="initial"`, иначе stuck row на
// неделю → ~10K инкрементов и alert-fatigue. Grafana-дашборды без фильтра
// автоматически суммируют initial+retry — для total-counter это корректно, но
// rate()-панели надо пересмотреть.
var MaterializeFailureTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "portal",
		Name:      "materialize_failure_total",
		Help:      "Materializer ApplyToClient partial-failures (SRA committed, materialize failed). Label source=initial|retry distinguishes first-time PUT/Bulk failures from retry-loop ticks for the same stuck row.",
	},
	[]string{"operation", "source"}, // operation: provider|route; source: initial|retry
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

// SRARetryBacklogOverflowTotal — count of RetryPendingOnce ticks where the
// batch reached the LIMIT 100 cap. Sustained increases imply pending queue
// exceeds tick capacity (1m * 100 = 6000/h ceiling). Plan 5 Task 4 (B4):
// gauge alone underreports backlog beyond 100; this counter gives ops a
// trend-line for "tick is at capacity".
var SRARetryBacklogOverflowTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "portal",
		Name:      "sra_retry_backlog_overflow_total",
		Help:      "RetryPendingOnce ticks where batch reached LIMIT 100 (possible backlog overflow). Sustained increases imply pending queue exceeds tick capacity.",
	},
)
