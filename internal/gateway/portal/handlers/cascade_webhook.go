package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
)

// CascadeWebhookHandlers обрабатывает входящие webhook от каналов (flash call, reverse call)
type CascadeWebhookHandlers struct {
	producer *cascadekafka.CascadeProducer
	logger   zerolog.Logger
}

// NewCascadeWebhookHandlers создаёт новый CascadeWebhookHandlers
func NewCascadeWebhookHandlers(producer *cascadekafka.CascadeProducer, logger zerolog.Logger) *CascadeWebhookHandlers {
	return &CascadeWebhookHandlers{
		producer: producer,
		logger:   logger.With().Str("component", "cascade_webhook").Logger(),
	}
}

// FlashCallWebhookBody — тело входящего webhook от flash call провайдера
type FlashCallWebhookBody struct {
	DeliveryID  string `json:"delivery_id"`
	ProviderRef string `json:"provider_ref"`
	Status      string `json:"status"`      // delivered | failed
	ErrorMsg    string `json:"error_msg,omitempty"`
}

// FlashCallWebhook обрабатывает POST /webhooks/cascade/flash-call/{attempt_id}
func (h *CascadeWebhookHandlers) FlashCallWebhook(w http.ResponseWriter, r *http.Request) {
	attemptID := mux.Vars(r)["attempt_id"]

	var body FlashCallWebhookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.logger.Warn().Err(err).Str("attempt_id", attemptID).Msg("неверное тело webhook")
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	status := body.Status
	if status != "delivered" && status != "failed" {
		status = "failed"
	}

	evt := cascadekafka.NewCascadeAttemptResultEvent(
		attemptID,
		body.DeliveryID,
		"flash_call",
		status,
		body.ProviderRef,
		body.ErrorMsg,
		time.Now().UTC().Format(time.RFC3339),
	)

	if err := h.producer.PublishAttemptResult(r.Context(), evt); err != nil {
		h.logger.Error().Err(err).Str("attempt_id", attemptID).Msg("ошибка публикации результата flash call")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info().
		Str("attempt_id", attemptID).
		Str("delivery_id", body.DeliveryID).
		Str("status", status).
		Msg("flash call webhook обработан")

	w.WriteHeader(http.StatusNoContent)
}
