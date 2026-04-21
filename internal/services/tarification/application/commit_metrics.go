package application

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Метрики commit-on-submit flow и retry worker'а.
// Префикс tarification_commit_charge_* для handler-метрик,
// tarification_commit_retry_* для worker'а.
var (
	commitChargeDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tarification_commit_charge_duration_seconds",
			Help:    "Длительность CommitCharge от начала до возврата. branch=direct|subaccount.",
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"branch"},
	)

	commitChargeResult = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_commit_charge_result_total",
			Help: "Итог CommitCharge. result=committed|already_committed|insufficient_balance_subaccount|insufficient_balance_aggregator|quota_not_configured|no_tariff|transport_error.",
		},
		[]string{"result"},
	)

	// aggregator_id может быть сотни; при тысячах пересмотреть.
	aggregatorMarginByMode = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_aggregator_margin_by_mode_total",
			Help: "Суммарная маржа агрегатора в единицах валюты по charge_mode. charge_mode=pool|overage|split.",
		},
		[]string{"aggregator_id", "charge_mode"},
	)

	commitRetryEnqueuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tarification_commit_retry_enqueued_total",
			Help: "Количество CommitCharge-неудач, добавленных в retry queue.",
		},
	)

	commitRetryExhaustedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tarification_commit_retry_exhausted_total",
			Help: "Записи, удалённые из retry queue после max_attempts без успеха. Требует ручного разбора — потерянные деньги.",
		},
	)

	commitRetryProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_commit_retry_processed_total",
			Help: "Результат retry из worker. outcome=success|terminal|transient.",
		},
		[]string{"outcome"},
	)
)
