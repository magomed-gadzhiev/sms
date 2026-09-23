package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
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

	// Pipeline metrics
	PipelineMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_messages_processed_total",
			Help: "Общее количество обработанных сообщений pipeline",
		},
		[]string{"stage", "status"},
	)

	PipelineProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pipeline_processing_duration_seconds",
			Help:    "Длительность обработки сообщений pipeline в секундах",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"stage"},
	)

	PipelineBatchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pipeline_batch_size",
			Help:    "Размер batch в pipeline",
			Buckets: []float64{1, 10, 50, 100, 200, 500, 1000, 2000, 5000},
		},
		[]string{"stage"},
	)

	PipelineQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipeline_queue_depth",
			Help: "Глубина очереди (consumer lag)",
		},
		[]string{"topic", "partition"},
	)

	PipelineBackpressureActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipeline_backpressure_active",
			Help: "Активен ли backpressure для провайдера (0/1)",
		},
		[]string{"provider_id"},
	)

	PipelineConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipeline_connections_active",
			Help: "Количество активных SMPP соединений pipeline",
		},
		[]string{"provider_id"},
	)

	// Route cache metrics
	RouteCacheRefreshDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "route_cache_refresh_duration_seconds",
		Help:    "Time to refresh route/provider cache from DB",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
	})

	RouteCacheSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "route_cache_size",
		Help: "Number of items in route/provider cache",
	}, []string{"type"})

	// DLR delivery metrics
	DLREventsConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlr_events_consumed_total",
			Help: "Общее количество DLR событий из Kafka",
		},
		[]string{"status"},
	)

	DLREventsDispatched = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlr_events_dispatched_total",
			Help: "Общее количество DLR событий отправленных клиентам",
		},
		[]string{"status", "result"},
	)

	DLRDispatchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dlr_dispatch_duration_seconds",
			Help:    "Длительность отправки DLR клиенту",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{},
	)

	DLRRedisLookupMiss = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlr_redis_lookup_miss_total",
			Help: "Промахи при Redis lookup для DLR",
		},
		[]string{"key_type"},
	)

	SMPPDLRDelivered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smpp_dlr_delivered_total",
			Help: "Количество DLR успешно доставленных SMPP клиентам",
		},
		[]string{"system_id"},
	)

	SMPPDLRDeliveryFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smpp_dlr_delivery_failed_total",
			Help: "Количество неудачных доставок DLR SMPP клиентам",
		},
		[]string{"system_id", "reason"},
	)
)

// StartConsumerLagMonitor starts a goroutine that periodically polls consumer lag
// and updates the PipelineQueueDepth gauge.
func StartConsumerLagMonitor(ctx context.Context, brokers []string, groups []string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		saramaConfig := sarama.NewConfig()
		saramaConfig.Version = sarama.V2_6_0_0

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				admin, err := sarama.NewClusterAdmin(brokers, saramaConfig)
				if err != nil {
					log.Error().Err(err).Msg("failed to create Kafka cluster admin for lag monitoring")
					continue
				}

				for _, group := range groups {
					offsets, err := admin.ListConsumerGroupOffsets(group, nil)
					if err != nil {
						log.Error().Err(err).Str("group", group).Msg("failed to list consumer group offsets")
						continue
					}

					for topic, partitions := range offsets.Blocks {
						for partition, block := range partitions {
							latestOffset, err := getLatestOffset(brokers, saramaConfig, topic, partition)
							if err != nil {
								log.Error().Err(err).
									Str("topic", topic).
									Int32("partition", partition).
									Msg("failed to get latest offset")
								continue
							}
							lag := latestOffset - block.Offset
							if lag < 0 {
								lag = 0
							}
							PipelineQueueDepth.WithLabelValues(topic, fmt.Sprintf("%d", partition)).Set(float64(lag))
						}
					}
				}

				admin.Close()
			}
		}
	}()
}

// getLatestOffset returns the newest offset for a given topic/partition.
func getLatestOffset(brokers []string, config *sarama.Config, topic string, partition int32) (int64, error) {
	client, err := sarama.NewClient(brokers, config)
	if err != nil {
		return 0, fmt.Errorf("create sarama client: %w", err)
	}
	defer client.Close()

	offset, err := client.GetOffset(topic, partition, sarama.OffsetNewest)
	if err != nil {
		return 0, fmt.Errorf("get offset for %s/%d: %w", topic, partition, err)
	}
	return offset, nil
}
