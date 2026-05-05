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
