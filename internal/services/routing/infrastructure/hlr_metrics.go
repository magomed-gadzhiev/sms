package infrastructure

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HLRLookupDuration tracks the duration of HLR lookup operations
	HLRLookupDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "sms_gateway",
		Subsystem: "hlr",
		Name:      "lookup_duration_seconds",
		Help:      "Длительность HLR lookup операций",
		Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0},
	})

	// HLRCacheHits tracks cache hit count
	HLRCacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "sms_gateway",
		Subsystem: "hlr",
		Name:      "cache_hits_total",
		Help:      "Количество попаданий в кеш HLR",
	})

	// HLRCacheMisses tracks cache miss count
	HLRCacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "sms_gateway",
		Subsystem: "hlr",
		Name:      "cache_misses_total",
		Help:      "Количество промахов кеша HLR",
	})

	// HLRProviderRequests tracks requests to HLR providers
	HLRProviderRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "sms_gateway",
		Subsystem: "hlr",
		Name:      "provider_requests_total",
		Help:      "Количество запросов к HLR провайдерам",
	}, []string{"provider", "status"})

	// HLRProviderHealth tracks current health status of providers
	HLRProviderHealth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sms_gateway",
		Subsystem: "hlr",
		Name:      "provider_health",
		Help:      "Текущий статус здоровья HLR провайдера (1=healthy, 0.5=degraded, 0=unhealthy)",
	}, []string{"provider"})

	// SmartRoutingScore tracks the distribution of selected provider scores
	SmartRoutingScore = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "sms_gateway",
		Subsystem: "routing",
		Name:      "smart_routing_score",
		Help:      "Распределение scores выбранных провайдеров",
		Buckets:   []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
	})

	// HLRAllProvidersDown is a gauge set to 1 when ALL HLR providers are unavailable
	// Use this in Alertmanager rules: sms_gateway_hlr_all_providers_down == 1
	HLRAllProvidersDown = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sms_gateway",
		Subsystem: "hlr",
		Name:      "all_providers_down",
		Help:      "1 если все HLR провайдеры недоступны, 0 если хотя бы один доступен",
	})

	// LookupAPIRequests tracks API lookup requests
	LookupAPIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "sms_gateway",
		Subsystem: "lookup",
		Name:      "api_requests_total",
		Help:      "Количество API запросов на валидацию номеров",
	}, []string{"type"})
)
