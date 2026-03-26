package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP метрики
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Общее количество HTTP запросов",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Длительность HTTP запросов в секундах",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// gRPC метрики
	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Общее количество gRPC запросов",
		},
		[]string{"method", "status"},
	)

	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Длительность gRPC запросов в секундах",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// SMS метрики
	SMSMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_messages_received_total",
			Help: "Общее количество полученных SMS сообщений",
		},
		[]string{"source", "client_id"},
	)

	SMSMessagesQueued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_messages_queued_total",
			Help: "Общее количество сообщений, добавленных в очередь",
		},
		[]string{"client_id"},
	)

	SMSMessagesFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_messages_failed_total",
			Help: "Общее количество неудачных SMS сообщений",
		},
		[]string{"client_id", "reason"},
	)

	// Kafka метрики
	KafkaPublishDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_publish_duration_seconds",
			Help:    "Длительность публикации сообщений в Kafka",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"topic"},
	)

	KafkaPublishErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_publish_errors_total",
			Help: "Общее количество ошибок публикации в Kafka",
		},
		[]string{"topic"},
	)

	// Rate limiting метрики
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_hits_total",
			Help: "Общее количество срабатываний rate limit",
		},
		[]string{"client_id", "period"},
	)

	// SMPP Server метрики
	SMPPMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smpp_messages_received_total",
			Help: "Общее количество полученных SMS сообщений через SMPP",
		},
		[]string{"client_id", "session_id"},
	)

	SMPPMessagesSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smpp_messages_sent_total",
			Help: "Общее количество отправленных SMS сообщений",
		},
		[]string{"provider_id", "provider_name", "status"},
	)

	SMPPMessagesFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smpp_messages_failed_total",
			Help: "Общее количество неудачных SMS сообщений",
		},
		[]string{"provider_id", "provider_name", "reason"},
	)

	SMPPConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "smpp_connections_active",
			Help: "Количество активных SMPP соединений",
		},
		[]string{"type"}, // type: "bound", "unbound", "total"
	)

	SMPPProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "smpp_processing_duration_seconds",
			Help:    "Длительность обработки сообщений в секундах",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"operation"}, // operation: "submit_sm", "deliver_sm", "send_to_provider"
	)

	SMPPProviderThroughput = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "smpp_provider_throughput",
			Help: "Пропускная способность провайдера (сообщений в секунду)",
		},
		[]string{"provider_id", "provider_name"},
	)

	SMPPMessagesDelivered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smpp_messages_delivered_total",
			Help: "Общее количество доставленных SMS сообщений",
		},
		[]string{"provider_id", "provider_name"},
	)

	// Kafka queue метрики
	KafkaQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "smpp_queue_size",
			Help: "Размер очереди Kafka (lag)",
		},
		[]string{"topic", "consumer_group"},
	)

	// Worker метрики
	WorkerMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_messages_processed_total",
			Help: "Общее количество обработанных сообщений worker",
		},
		[]string{"worker_id", "status"},
	)

	WorkerProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_processing_duration_seconds",
			Help:    "Длительность обработки сообщений worker в секундах",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"worker_id", "operation"},
	)

	// Multipart SMS метрики
	SMSSegmentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_segments_total",
			Help: "Общее количество SMS-сегментов",
		},
		[]string{"client_id"},
	)

	SMSSegmentsPerMessage = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "sms_segments_per_message",
			Help:    "Распределение количества сегментов на сообщение",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	)

	// DLR Expiry метрики
	DLRExpiredMessagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "dlr_expired_messages_total",
			Help: "Общее количество сообщений с истёкшим DLR таймаутом",
		},
	)

	// Data Purging метрики
	DataPurgePartitionsDropped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "data_purge_partitions_dropped_total",
			Help: "Количество удалённых партиций при очистке данных",
		},
		[]string{"table"},
	)

	// Database метрики
	DatabaseConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "database_connections_active",
			Help: "Количество активных соединений к базе данных",
		},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Длительность выполнения запросов к БД в секундах",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"operation"},
	)
)
