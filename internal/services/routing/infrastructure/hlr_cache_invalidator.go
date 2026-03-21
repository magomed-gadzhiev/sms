package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// DeliveryFailedEvent represents a message delivery failure event from Kafka
type DeliveryFailedEvent struct {
	MessageID   string `json:"message_id"`
	Destination string `json:"destination"`
	Reason      string `json:"reason"`
	ErrorCode   string `json:"error_code"`
}

// HLRCacheInvalidator listens for delivery failure events and invalidates HLR cache
type HLRCacheInvalidator struct {
	cache  domain.HLRCache
	logger zerolog.Logger
}

// NewHLRCacheInvalidator creates a new cache invalidator
func NewHLRCacheInvalidator(cache domain.HLRCache) *HLRCacheInvalidator {
	return &HLRCacheInvalidator{
		cache:  cache,
		logger: log.With().Str("component", "hlr-cache-invalidator").Logger(),
	}
}

// HandleDeliveryFailed processes a delivery failure event.
// If the reason is "wrong_operator", invalidate the HLR cache for the destination.
func (i *HLRCacheInvalidator) HandleDeliveryFailed(ctx context.Context, data []byte) error {
	var event DeliveryFailedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		i.logger.Warn().Err(err).Msg("ошибка десериализации события delivery_failed")
		return nil // don't retry on unmarshal errors
	}

	if event.Reason != "wrong_operator" {
		return nil // not our concern
	}

	i.logger.Info().
		Str("message_id", event.MessageID).
		Str("destination", event.Destination).
		Msg("инвалидация HLR кеша из-за wrong_operator")

	if err := i.cache.Delete(ctx, event.Destination); err != nil {
		i.logger.Error().Err(err).
			Str("destination", event.Destination).
			Msg("ошибка инвалидации HLR кеша")
		return err
	}

	return nil
}
