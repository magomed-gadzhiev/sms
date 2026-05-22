package application

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// parseDecimal — конвертирует decimal-строку в float64 для метрик.
// Потеря точности допустима: метрика маржи — агрегат, а не источник истины.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// billingAggregatorMarginByMode — суммарная маржа агрегатора по charge_mode.
// Инкремент один раз за успешный ChargeMessageDual, sum = (sub_total - agg_total).
// label aggregator_id имеет cardinality = кол-во агрегаторов, charge_mode ∈
// {pool, overage, split}. При масштабе тысяч агрегаторов — пересмотреть
// (см. комментарий в дизайн-спеке, раздел Риски).
var billingAggregatorMarginByMode = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "billing_aggregator_margin_by_mode_total",
		Help: "Суммарная маржа агрегатора (sub_total - agg_total) в единицах валюты по charge_mode.",
	},
	[]string{"aggregator_id", "charge_mode"},
)
