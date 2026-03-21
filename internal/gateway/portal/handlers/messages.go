package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageHandlers содержит handlers для работы с сообщениями
type MessageHandlers struct {
	messagingClient messagingv1.MessagingServiceClient
}

// NewMessageHandlers создает новый MessageHandlers
func NewMessageHandlers(messagingClient messagingv1.MessagingServiceClient) *MessageHandlers {
	return &MessageHandlers{
		messagingClient: messagingClient,
	}
}

// ListMessages обрабатывает GET /messages
func (h *MessageHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Парсим параметры пагинации
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	// Парсим параметры фильтрации
	query := r.URL.Query()
	status := query.Get("status")
	destination := query.Get("destination")

	// Парсим даты
	var dateFrom, dateTo *timestamppb.Timestamp
	if fromStr := query.Get("date_from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		} else {
			// Попробуем формат YYYY-MM-DD
			if t, err := time.Parse("2006-01-02", fromStr); err == nil {
				dateFrom = timestamppb.New(t)
			}
		}
	}
	if toStr := query.Get("date_to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			dateTo = timestamppb.New(t)
		} else {
			// Попробуем формат YYYY-MM-DD
			if t, err := time.Parse("2006-01-02", toStr); err == nil {
				// Конец дня
				dateTo = timestamppb.New(t.Add(24*time.Hour - time.Second))
			}
		}
	}

	if h.messagingClient == nil {
		// Placeholder: messaging client не настроен
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"messages":   []interface{}{},
			"total":      0,
			"page":       page,
			"per_page":   perPage,
			"total_pages": 0,
		})
		return
	}

	resp, err := h.messagingClient.GetMessageHistory(r.Context(), &messagingv1.GetMessageHistoryRequest{
		ClientId:    clientID.String(),
		From:        dateFrom,
		To:          dateTo,
		Status:      status,
		Destination: destination,
		Limit:       perPage,
		Offset:      offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории сообщений")
		respondGRPCError(w, err)
		return
	}

	// Формируем список сообщений
	messages := make([]map[string]interface{}, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		m := map[string]interface{}{
			"message_id":    msg.MessageId,
			"source":        msg.Source,
			"destination":   msg.Destination,
			"text":          msg.Text,
			"status":        msg.Status,
			"segment_count": msg.SegmentCount,
		}
		if msg.ExternalId != "" {
			m["external_id"] = msg.ExternalId
		}
		if msg.CreatedAt != nil {
			m["created_at"] = msg.CreatedAt.AsTime()
		}
		if msg.SubmittedAt != nil {
			m["submitted_at"] = msg.SubmittedAt.AsTime()
		}
		if msg.DeliveredAt != nil {
			m["delivered_at"] = msg.DeliveredAt.AsTime()
		}
		if msg.FailedAt != nil {
			m["failed_at"] = msg.FailedAt.AsTime()
		}
		if msg.ScheduledAt != nil {
			m["scheduled_at"] = msg.ScheduledAt.AsTime()
		}
		messages = append(messages, m)
	}

	// Вычисляем total_pages
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages":    messages,
		"total":       resp.Total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}
