package sse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/pipeline"
)

// Event represents a real-time status update sent to SSE clients.
type Event struct {
	MessageID string    `json:"message_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Hub manages SSE subscriptions and fans out Kafka status updates to connected clients.
// A single Kafka consumer group reads from the sms.status topic; events are routed
// per client_id after a DB lookup.
type Hub struct {
	mu     sync.RWMutex
	subs   map[string]map[chan Event]struct{} // client_id → set of subscriber channels

	brokers []string
	topic   string
	dbPool  *pgxpool.Pool
	logger  zerolog.Logger
}

// NewHub creates a new Hub. Call Run(ctx) in a goroutine to start consuming.
func NewHub(brokers []string, topic string, dbPool *pgxpool.Pool, logger zerolog.Logger) *Hub {
	return &Hub{
		subs:    make(map[string]map[chan Event]struct{}),
		brokers: brokers,
		topic:   topic,
		dbPool:  dbPool,
		logger:  logger,
	}
}

// Subscribe registers a new SSE client for the given clientID and returns a channel
// that will receive events. The caller must call Unsubscribe when done.
func (h *Hub) Subscribe(clientID string) chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	if _, ok := h.subs[clientID]; !ok {
		h.subs[clientID] = make(map[chan Event]struct{})
	}
	h.subs[clientID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the hub and closes it.
func (h *Hub) Unsubscribe(clientID string, ch chan Event) {
	h.mu.Lock()
	if subs, ok := h.subs[clientID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subs, clientID)
		}
	}
	h.mu.Unlock()
	close(ch)
}

// Run starts the Kafka consumer loop. Blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_6_0_0
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.Consumer.Return.Errors = true

	cg, err := sarama.NewConsumerGroup(h.brokers, "portal-sse-hub", cfg)
	if err != nil {
		h.logger.Error().Err(err).Msg("SSE hub: не удалось создать Kafka consumer group")
		return
	}
	defer cg.Close()

	handler := &consumerGroupHandler{hub: h}

	for {
		if err := cg.Consume(ctx, []string{h.topic}, handler); err != nil {
			h.logger.Error().Err(err).Msg("SSE hub: ошибка Kafka consumer group")
		}
		if ctx.Err() != nil {
			return
		}
		// Brief pause before reconnect attempt.
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// route delivers the event to all subscribers of clientID.
func (h *Hub) route(clientID string, event Event) {
	h.mu.RLock()
	subs, ok := h.subs[clientID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Copy channels to avoid holding the lock while sending.
	channels := make([]chan Event, 0, len(subs))
	for ch := range subs {
		channels = append(channels, ch)
	}
	h.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- event:
		default:
			// Drop the event if the subscriber is not keeping up.
		}
	}
}

// lookupClientID returns the client_id for a given message_id from the DB.
func (h *Hub) lookupClientID(ctx context.Context, messageID string) (string, error) {
	var clientID string
	err := h.dbPool.QueryRow(ctx, "SELECT client_id FROM messages WHERE id = $1", messageID).Scan(&clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("lookupClientID(%s): %w", messageID, err)
	}
	return clientID, nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler for the SSE hub.
type consumerGroupHandler struct {
	hub *Hub
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.handleMessage(session.Context(), msg)
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *consumerGroupHandler) handleMessage(ctx context.Context, msg *sarama.ConsumerMessage) {
	update, err := pipeline.DeserializeStatusUpdate(msg.Value)
	if err != nil {
		h.hub.logger.Debug().Err(err).Msg("SSE hub: ошибка десериализации StatusUpdate")
		return
	}

	// Check if there are any active subscribers before doing the DB lookup.
	h.hub.mu.RLock()
	totalSubs := len(h.hub.subs)
	h.hub.mu.RUnlock()
	if totalSubs == 0 {
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	clientID, err := h.hub.lookupClientID(lookupCtx, update.MessageID.String())
	if err != nil {
		h.hub.logger.Debug().Err(err).Str("message_id", update.MessageID.String()).Msg("SSE hub: ошибка поиска client_id")
		return
	}
	if clientID == "" {
		return
	}

	event := Event{
		MessageID: update.MessageID.String(),
		Status:    update.Status,
		UpdatedAt: update.UpdatedAt,
	}
	h.hub.route(clientID, event)
}
