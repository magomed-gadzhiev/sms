package max_messenger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
)

// WebhookHandler обрабатывает входящие webhook от Max Messenger API
type WebhookHandler struct {
	channelRepo domain.ChannelRepository
	attemptRepo domain.AttemptRepository
	producer    *cascadekafka.CascadeProducer
	metrics     *MaxMessengerMetrics
	logger      zerolog.Logger
}

// NewWebhookHandler создаёт новый WebhookHandler
func NewWebhookHandler(
	channelRepo domain.ChannelRepository,
	attemptRepo domain.AttemptRepository,
	producer *cascadekafka.CascadeProducer,
	metrics *MaxMessengerMetrics,
	logger zerolog.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		channelRepo: channelRepo,
		attemptRepo: attemptRepo,
		producer:    producer,
		metrics:     metrics,
		logger:      logger.With().Str("component", "max_messenger_webhook").Logger(),
	}
}

// Handle обрабатывает POST /webhooks/cascade/max_messenger
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Warn().Err(err).Msg("ошибка чтения тела webhook")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Загружаем webhook_secret из конфига канала
	ch, err := h.channelRepo.GetByType(r.Context(), domain.ChannelMaxMessenger)
	if err != nil {
		h.logger.Error().Err(err).Msg("не удалось загрузить конфиг канала Max Messenger")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	webhookSecret, _ := ch.Config["webhook_secret"].(string)

	// Верифицируем HMAC-SHA256 подпись
	signature := r.Header.Get("X-Max-Signature")
	if !verifyHMAC(body, signature, webhookSecret) {
		h.metrics.WebhooksReceived.WithLabelValues("invalid_signature").Inc()
		h.logger.Warn().Str("signature", signature).Msg("невалидная подпись webhook")
		respondWebhookJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}

	// Парсим payload
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Warn().Err(err).Msg("невалидное тело webhook")
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Маппинг статуса Max → internal
	status := mapMaxStatus(payload.Status)

	// Загружаем attempt для получения delivery_id
	attempt, err := h.attemptRepo.Get(r.Context(), uuidFromString(payload.AttemptID))
	if err != nil {
		h.metrics.WebhooksReceived.WithLabelValues("unknown_attempt").Inc()
		h.logger.Warn().Str("attempt_id", payload.AttemptID).Msg("attempt не найден")
		respondWebhookJSON(w, http.StatusNotFound, map[string]string{"error": "attempt not found"})
		return
	}

	// Проверяем late_duplicate: если delivery уже в terminal state
	if attempt.IsFinal() {
		h.metrics.WebhooksReceived.WithLabelValues("valid").Inc()
		h.logger.Info().
			Str("attempt_id", payload.AttemptID).
			Str("status", status).
			Msg("late_duplicate webhook для Max Messenger")

		// Публикуем как late_duplicate
		evt := cascadekafka.NewCascadeAttemptResultEvent(
			payload.AttemptID,
			attempt.DeliveryID.String(),
			"max_messenger",
			"late_duplicate",
			payload.MessageID,
			payload.Error,
			time.Now().UTC().Format(time.RFC3339),
		)
		_ = h.producer.PublishAttemptResult(r.Context(), evt)
		respondWebhookJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	// Публикуем событие результата
	evt := cascadekafka.NewCascadeAttemptResultEvent(
		payload.AttemptID,
		attempt.DeliveryID.String(),
		"max_messenger",
		status,
		payload.MessageID,
		payload.Error,
		time.Now().UTC().Format(time.RFC3339),
	)

	if err := h.producer.PublishAttemptResult(r.Context(), evt); err != nil {
		h.logger.Error().Err(err).Str("attempt_id", payload.AttemptID).Msg("ошибка публикации результата webhook")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.metrics.WebhooksReceived.WithLabelValues("valid").Inc()
	h.logger.Info().
		Str("attempt_id", payload.AttemptID).
		Str("message_id", payload.MessageID).
		Str("status", status).
		Msg("max messenger webhook обработан")

	respondWebhookJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// mapMaxStatus маппит статус Max Messenger → internal attempt status
func mapMaxStatus(maxStatus string) string {
	switch maxStatus {
	case "delivered", "read":
		return "delivered"
	case "error":
		return "failed"
	default:
		return "failed"
	}
}

// verifyHMAC проверяет HMAC-SHA256 подпись
func verifyHMAC(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func respondWebhookJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func uuidFromString(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
