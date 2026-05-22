// internal/services/webhook/application/delivery_service.go
package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	webhookhttp "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/http"
	"github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/repository"
)

var (
	deliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_deliveries_total",
		Help: "Total webhook delivery attempts",
	}, []string{"event_type", "status"})

	deliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webhook_delivery_duration_seconds",
		Help:    "Webhook HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"event_type"})

	retryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_retry_total",
		Help: "Webhook retry attempts",
	}, []string{"attempt"})

	deliveryFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_delivery_failed",
		Help: "Webhooks that exhausted all retries",
	}, []string{"event_type"})

	workerPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_worker_pool_size",
		Help: "Current worker pool utilization",
	})

	subscriptionsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_subscriptions_total",
		Help: "Active subscriptions count",
	})
)

var retryBackoff = []time.Duration{15 * time.Second, 30 * time.Second, 1 * time.Minute, 5 * time.Minute, 15 * time.Minute}

type DeliveryService struct {
	subRepo    *repository.SubscriptionRepository
	msgRepo    *repository.MessageRepository
	httpClient *webhookhttp.DeliveryClient
	logger     zerolog.Logger

	// In-memory subscription cache
	cacheMu  sync.RWMutex
	cache    map[uuid.UUID]cachedSubs
	cacheTTL time.Duration

	// Worker pool
	workCh chan deliveryJob
	wg     sync.WaitGroup
}

type cachedSubs struct {
	subs      []*domain.Subscription
	expiresAt time.Time
}

type deliveryJob struct {
	sub   *domain.Subscription
	event *domain.WebhookEvent
}

func NewDeliveryService(
	subRepo *repository.SubscriptionRepository,
	msgRepo *repository.MessageRepository,
	httpClient *webhookhttp.DeliveryClient,
	poolSize int,
) *DeliveryService {
	ds := &DeliveryService{
		subRepo:    subRepo,
		msgRepo:    msgRepo,
		httpClient: httpClient,
		logger:     log.With().Str("component", "webhook-delivery").Logger(),
		cache:      make(map[uuid.UUID]cachedSubs),
		cacheTTL:   60 * time.Second,
		workCh:     make(chan deliveryJob, poolSize*2),
	}

	// Start worker pool
	for i := 0; i < poolSize; i++ {
		ds.wg.Add(1)
		go ds.worker()
	}

	return ds
}

func (ds *DeliveryService) worker() {
	defer ds.wg.Done()
	for job := range ds.workCh {
		workerPoolSize.Inc()
		ds.deliverWithRetry(job.sub, job.event, 0)
		workerPoolSize.Dec()
	}
}

func (ds *DeliveryService) deliverWithRetry(sub *domain.Subscription, event *domain.WebhookEvent, attempt int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := ds.httpClient.Deliver(ctx, sub, event)
	duration := time.Since(start)

	deliveryDuration.WithLabelValues(event.EventType).Observe(duration.Seconds())

	if err == nil {
		deliveriesTotal.WithLabelValues(event.EventType, "success").Inc()
		return
	}

	deliveriesTotal.WithLabelValues(event.EventType, "failed").Inc()

	if attempt >= len(retryBackoff) {
		deliveryFailed.WithLabelValues(event.EventType).Inc()
		ds.logger.Error().
			Err(err).
			Str("subscription_id", sub.ID.String()).
			Str("url", sub.URL).
			Str("event_type", event.EventType).
			Int("attempts", attempt+1).
			Msg("webhook delivery exhausted all retries")
		return
	}

	retryTotal.WithLabelValues(fmt.Sprintf("%d", attempt+1)).Inc()
	delay := retryBackoff[attempt]

	ds.logger.Warn().
		Err(err).
		Str("subscription_id", sub.ID.String()).
		Dur("retry_after", delay).
		Int("attempt", attempt+1).
		Msg("webhook delivery failed, scheduling retry")

	time.AfterFunc(delay, func() {
		ds.deliverWithRetry(sub, event, attempt+1)
	})
}

// HandleDLR processes a DLR Kafka message
func (ds *DeliveryService) HandleDLR(ctx context.Context, dlr *queue.DLRMessage) error {
	eventType, ok := domain.SMPPStatToEventType[dlr.Stat]
	if !ok {
		// ACCEPTED and other non-final states are ignored
		return nil
	}

	enrichment, err := ds.msgRepo.GetEnrichment(ctx, dlr.MessageID)
	if err != nil {
		ds.logger.Error().Err(err).Str("message_id", dlr.MessageID.String()).Msg("failed to enrich DLR")
		return nil // don't block consumer on enrichment errors
	}
	if enrichment == nil || enrichment.ClientID == nil {
		ds.logger.Warn().Str("message_id", dlr.MessageID.String()).Msg("message not found for DLR, skipping")
		return nil
	}

	// Prefer enriched values from DB (always populated); DLR fields may be empty (omitempty)
	source := enrichment.Source
	if source == "" {
		source = dlr.Source
	}
	destination := enrichment.Destination
	if destination == "" {
		destination = dlr.Destination
	}

	event := &domain.WebhookEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now(),
		Data: domain.EventData{
			MessageID:    dlr.MessageID.String(),
			ExternalID:   enrichment.ExternalID,
			Source:       source,
			Destination:  destination,
			Status:       eventType,
			SegmentCount: enrichment.SegmentCount,
		},
	}

	if enrichment.SubmittedAt != nil {
		event.Data.SubmittedAt = enrichment.SubmittedAt
	}
	if dlr.ProviderID != nil {
		event.Data.ProviderID = dlr.ProviderID.String()
	}
	if dlr.DoneDate != nil {
		if eventType == "delivered" {
			event.Data.DeliveredAt = dlr.DoneDate
		} else {
			event.Data.FailedAt = dlr.DoneDate
		}
	}
	if dlr.Text != "" {
		event.Data.StatusMessage = dlr.Text
	}

	ds.dispatchToSubscriptions(ctx, *enrichment.ClientID, eventType, event)
	return nil
}

// HandleFailed processes a failed message from Kafka
func (ds *DeliveryService) HandleFailed(ctx context.Context, failed *queue.FailedMessage) error {
	enrichment, err := ds.msgRepo.GetEnrichment(ctx, failed.MessageID)
	if err != nil {
		ds.logger.Error().Err(err).Str("message_id", failed.MessageID.String()).Msg("failed to enrich failed message")
		return nil
	}
	if enrichment == nil || enrichment.ClientID == nil {
		ds.logger.Warn().Str("message_id", failed.MessageID.String()).Msg("message not found for failed event, skipping")
		return nil
	}

	event := &domain.WebhookEvent{
		EventID:   uuid.New().String(),
		EventType: "failed",
		Timestamp: time.Now(),
		Data: domain.EventData{
			MessageID:     failed.MessageID.String(),
			Source:        enrichment.Source,
			Destination:   enrichment.Destination,
			Status:        "failed",
			StatusMessage: failed.Error,
			SegmentCount:  enrichment.SegmentCount,
			FailedAt:      &failed.FailedAt,
		},
	}
	if enrichment.ExternalID != "" {
		event.Data.ExternalID = enrichment.ExternalID
	}
	if enrichment.SubmittedAt != nil {
		event.Data.SubmittedAt = enrichment.SubmittedAt
	}

	ds.dispatchToSubscriptions(ctx, *enrichment.ClientID, "failed", event)
	return nil
}

func (ds *DeliveryService) dispatchToSubscriptions(ctx context.Context, clientID uuid.UUID, eventType string, event *domain.WebhookEvent) {
	subs, err := ds.getSubscriptions(ctx, clientID)
	if err != nil {
		ds.logger.Error().Err(err).Str("client_id", clientID.String()).Msg("failed to get subscriptions")
		return
	}

	for _, sub := range subs {
		if !sub.Active {
			continue
		}
		// Check if subscription is interested in this event type
		matched := false
		for _, et := range sub.EventTypes {
			if et == eventType {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		select {
		case ds.workCh <- deliveryJob{sub: sub, event: event}:
		default:
			ds.logger.Warn().
				Str("subscription_id", sub.ID.String()).
				Msg("worker pool full, dropping webhook delivery")
		}
	}
}

func (ds *DeliveryService) getSubscriptions(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error) {
	ds.cacheMu.RLock()
	if cached, ok := ds.cache[clientID]; ok && time.Now().Before(cached.expiresAt) {
		ds.cacheMu.RUnlock()
		return cached.subs, nil
	}
	ds.cacheMu.RUnlock()

	subs, err := ds.subRepo.ListActiveByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	ds.cacheMu.Lock()
	ds.cache[clientID] = cachedSubs{
		subs:      subs,
		expiresAt: time.Now().Add(ds.cacheTTL),
	}
	ds.cacheMu.Unlock()

	return subs, nil
}

// InvalidateCache removes cached subscriptions for a client (called by WebhookService on CRUD)
func (ds *DeliveryService) InvalidateCache(clientID uuid.UUID) {
	ds.cacheMu.Lock()
	delete(ds.cache, clientID)
	ds.cacheMu.Unlock()
}

// Close drains the worker pool
func (ds *DeliveryService) Close() {
	close(ds.workCh)
	ds.wg.Wait()
}
