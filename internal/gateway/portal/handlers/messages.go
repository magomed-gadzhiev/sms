package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/sse"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageHandlers содержит handlers для работы с сообщениями
type MessageHandlers struct {
	messagingClient messagingv1.MessagingServiceClient
	sseHub          *sse.Hub
}

// NewMessageHandlers создает новый MessageHandlers
func NewMessageHandlers(messagingClient messagingv1.MessagingServiceClient) *MessageHandlers {
	return &MessageHandlers{
		messagingClient: messagingClient,
	}
}

// SetSSEHub устанавливает SSE hub для стриминга статусов сообщений.
func (h *MessageHandlers) SetSSEHub(hub *sse.Hub) {
	h.sseHub = hub
}

type sendMessageRequest struct {
	Destination string `json:"destination"`
	Text        string `json:"text"`
	Source      string `json:"source"`
}

// SendMessage обрабатывает POST /messages
func (h *MessageHandlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Destination == "" {
		respondError(w, shared.ErrInvalidInput("Поле destination обязательно"))
		return
	}
	if req.Text == "" {
		respondError(w, shared.ErrInvalidInput("Поле text обязательно"))
		return
	}

	if h.messagingClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис отправки сообщений недоступен"))
		return
	}

	resp, err := h.messagingClient.SendMessage(r.Context(), &messagingv1.SendMessageRequest{
		ClientId:    clientID.String(),
		Source:      req.Source,
		Destination: req.Destination,
		Text:        req.Text,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка отправки SMS")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message_id": resp.MessageId,
		"status":     resp.Status,
	})
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

// GetMessage обрабатывает GET /messages/{id}
func (h *MessageHandlers) GetMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	resp, err := h.messagingClient.GetMessageStatus(r.Context(), &messagingv1.GetMessageStatusRequest{
		MessageId: id,
		ClientId:  clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"message_id":    resp.MessageId,
		"status":        resp.Status,
		"segment_count": resp.SegmentCount,
	}
	if resp.StatusMessage != "" {
		result["status_message"] = resp.StatusMessage
	}
	if resp.SmppMessageId != "" {
		result["smpp_message_id"] = resp.SmppMessageId
	}
	if resp.ErrorCode != "" {
		result["error_code"] = resp.ErrorCode
	}
	if resp.ErrorMessage != "" {
		result["error_message"] = resp.ErrorMessage
	}
	if resp.CreatedAt != nil {
		result["created_at"] = resp.CreatedAt.AsTime()
	}
	if resp.SubmittedAt != nil {
		result["submitted_at"] = resp.SubmittedAt.AsTime()
	}
	if resp.DeliveredAt != nil {
		result["delivered_at"] = resp.DeliveredAt.AsTime()
	}
	if resp.FailedAt != nil {
		result["failed_at"] = resp.FailedAt.AsTime()
	}
	if resp.ScheduledAt != nil {
		result["scheduled_at"] = resp.ScheduledAt.AsTime()
	}
	if resp.ExpiredAt != nil {
		result["expired_at"] = resp.ExpiredAt.AsTime()
	}

	respondJSON(w, http.StatusOK, result)
}

// StreamMessages обрабатывает GET /messages/stream — SSE endpoint для real-time статусов.
// Отправляет события типа "message.status" при каждом изменении статуса сообщения клиента.
func (h *MessageHandlers) StreamMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if h.sseHub == nil {
		http.Error(w, "SSE недоступен", http.StatusServiceUnavailable)
		return
	}

	// Ensure the ResponseWriter supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming не поддерживается", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	clientIDStr := clientID.String()
	ch := h.sseHub.Subscribe(clientIDStr)
	defer h.sseHub.Unsubscribe(clientIDStr, ch)

	// Send an initial "connected" event so the client knows the stream is live.
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	// Heartbeat ticker keeps the connection alive through proxies.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: message.status\ndata: %s\n\n", data)
			flusher.Flush()

		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

// ExportCSV обрабатывает GET /messages/export
func (h *MessageHandlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	query := r.URL.Query()
	statusFilter := query.Get("status")
	destination := query.Get("destination")

	var dateFrom, dateTo *timestamppb.Timestamp
	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			dateTo = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			dateTo = timestamppb.New(t.Add(24*time.Hour - time.Second))
		}
	}

	resp, err := h.messagingClient.GetMessageHistory(r.Context(), &messagingv1.GetMessageHistoryRequest{
		ClientId:    clientID.String(),
		From:        dateFrom,
		To:          dateTo,
		Status:      statusFilter,
		Destination: destination,
		Limit:       50000,
		Offset:      0,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка экспорта сообщений")
		respondGRPCError(w, err)
		return
	}

	now := time.Now().Format("2006-01-02")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=messages_"+now+".csv")

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"id", "source", "destination", "text", "status", "segment_count", "created_at", "delivered_at"})

	for _, msg := range resp.Messages {
		createdAt := ""
		if msg.CreatedAt != nil {
			createdAt = msg.CreatedAt.AsTime().Format(time.RFC3339)
		}
		deliveredAt := ""
		if msg.DeliveredAt != nil {
			deliveredAt = msg.DeliveredAt.AsTime().Format(time.RFC3339)
		}
		csvWriter.Write([]string{
			msg.MessageId, msg.Source, msg.Destination, msg.Text, msg.Status,
			fmt.Sprintf("%d", msg.SegmentCount), createdAt, deliveredAt,
		})
	}
	csvWriter.Flush()
}
