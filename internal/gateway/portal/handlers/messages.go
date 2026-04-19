package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	db              *pgxpool.Pool
}

// SetDB sets the database pool for direct SQL queries in GetMessage.
func (h *MessageHandlers) SetDB(db *pgxpool.Pool) {
	h.db = db
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
	if req.Source == "" {
		respondError(w, shared.ErrInvalidInput("Поле source обязательно"))
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

	if h.db == nil {
		if h.messagingClient == nil {
			respondError(w, shared.ErrServiceUnavailable("Сервис сообщений недоступен"))
			return
		}
		h.getMessageViaGRPC(w, r, id, clientID.String())
		return
	}

	ctx := r.Context()

	// Query 1 — message with provider/route names
	const msgQuery = `
SELECT
    m.id::text,
    COALESCE(m.source, '')        AS source,
    COALESCE(m.destination, '')   AS destination,
    COALESCE(m.text, '')          AS text,
    COALESCE(m.encoding, 'GSM7')  AS encoding,
    m.status::text,
    COALESCE(m.status_message, '') AS status_message,
    COALESCE(m.external_id, '')    AS external_id,
    COALESCE(m.segment_count, 0)   AS segment_count,
    COALESCE(m.retry_count, 0)     AS retry_count,
    COALESCE(m.max_retries, 0)     AS max_retries,
    COALESCE(m.provider_id::text, '') AS provider_id,
    COALESCE(m.route_id::text, '')    AS route_id,
    COALESCE(m.smpp_message_id, '')   AS smpp_message_id,
    m.created_at,
    m.submitted_at,
    m.delivered_at,
    m.failed_at,
    m.scheduled_at,
    m.expired_at,
    COALESCE(p.name, '') AS provider_name,
    COALESCE(r.name, '') AS route_name
FROM messages m
LEFT JOIN providers p ON p.id = m.provider_id
LEFT JOIN client_routes r ON r.id = m.route_id
WHERE m.id = $1::uuid AND m.client_id = $2::uuid
ORDER BY m.created_at DESC
LIMIT 1`

	var (
		msgID         string
		source        string
		destination   string
		text          string
		encoding      string
		status        string
		statusMessage string
		externalID    string
		segmentCount  int32
		retryCount    int32
		maxRetries    int32
		providerID    string
		routeID       string
		smppMessageID string
		createdAt     *time.Time
		submittedAt   *time.Time
		deliveredAt   *time.Time
		failedAt      *time.Time
		scheduledAt   *time.Time
		expiredAt     *time.Time
		providerName  string
		routeName     string
	)

	row := h.db.QueryRow(ctx, msgQuery, id, clientID.String())
	err := row.Scan(
		&msgID, &source, &destination, &text, &encoding,
		&status, &statusMessage, &externalID,
		&segmentCount, &retryCount, &maxRetries,
		&providerID, &routeID, &smppMessageID,
		&createdAt, &submittedAt, &deliveredAt, &failedAt, &scheduledAt, &expiredAt,
		&providerName, &routeName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Message may be in-flight (not yet persisted from Kafka pipeline).
			// If gRPC client is available, try it as a fallback before returning 404.
			if h.messagingClient != nil {
				h.getMessageViaGRPC(w, r, id, clientID.String())
				return
			}
			respondError(w, shared.ErrNotFound("Сообщение не найдено"))
		} else {
			log.Error().Err(err).Str("message_id", id).Msg("ошибка запроса сообщения из БД")
			respondError(w, shared.ErrInternalServer("Ошибка получения сообщения"))
		}
		return
	}

	result := map[string]interface{}{
		"message_id":    msgID,
		"source":        source,
		"destination":   destination,
		"text":          text,
		"encoding":      encoding,
		"status":        status,
		"segment_count": segmentCount,
		"retry_count":   retryCount,
		"max_retries":   maxRetries,
	}
	if createdAt != nil {
		result["created_at"] = *createdAt
	}
	if statusMessage != "" {
		result["status_message"] = statusMessage
	}
	if externalID != "" {
		result["external_id"] = externalID
	}
	if smppMessageID != "" {
		result["smpp_message_id"] = smppMessageID
	}
	if providerID != "" {
		result["provider_id"] = providerID
		result["provider_name"] = providerName
	}
	if routeID != "" {
		result["route_id"] = routeID
		result["route_name"] = routeName
	}
	if submittedAt != nil {
		result["submitted_at"] = *submittedAt
	}
	if deliveredAt != nil {
		result["delivered_at"] = *deliveredAt
	}
	if failedAt != nil {
		result["failed_at"] = *failedAt
	}
	if scheduledAt != nil {
		result["scheduled_at"] = *scheduledAt
	}
	if expiredAt != nil {
		result["expired_at"] = *expiredAt
	}

	// Query 2 — DLR receipt (most recent)
	const dlrQuery = `
SELECT stat, COALESCE(err, 0), COALESCE(text, ''),
       submit_date, done_date,
       COALESCE(receipted_message_id, '')
FROM dlr_receipts
WHERE message_id = $1::uuid
ORDER BY created_at DESC
LIMIT 1`

	var (
		dlrStat               string
		dlrErr                int32
		dlrText               string
		dlrSubmitDate         *time.Time
		dlrDoneDate           *time.Time
		dlrReceiptedMessageID string
	)
	dlrRow := h.db.QueryRow(ctx, dlrQuery, id)
	dlrScanErr := dlrRow.Scan(&dlrStat, &dlrErr, &dlrText, &dlrSubmitDate, &dlrDoneDate, &dlrReceiptedMessageID)
	if dlrScanErr == nil {
		dlr := map[string]interface{}{
			"stat": dlrStat,
			"err":  dlrErr,
			"text": dlrText,
		}
		if dlrSubmitDate != nil {
			dlr["submit_date"] = *dlrSubmitDate
		}
		if dlrDoneDate != nil {
			dlr["done_date"] = *dlrDoneDate
		}
		if dlrReceiptedMessageID != "" {
			dlr["receipted_message_id"] = dlrReceiptedMessageID
		}
		result["dlr"] = dlr
	} else if !errors.Is(dlrScanErr, pgx.ErrNoRows) {
		log.Warn().Err(dlrScanErr).Msg("ошибка получения DLR receipt")
	}

	// Query 3 — tarification log
	const billingQuery = `
SELECT segment_count, price_per_segment, total_amount,
       tariff_plan_id::text, source_rule_id::text, created_at
FROM tarification_log
WHERE message_id = $1::uuid
ORDER BY created_at DESC
LIMIT 1`

	var (
		billSegmentCount    int32
		billPricePerSegment float64
		billTotalAmount     float64
		billTariffPlanID    *string
		billSourceRuleID    *string
		billCreatedAt       *time.Time
	)
	billRow := h.db.QueryRow(ctx, billingQuery, id)
	billScanErr := billRow.Scan(
		&billSegmentCount, &billPricePerSegment, &billTotalAmount,
		&billTariffPlanID, &billSourceRuleID, &billCreatedAt,
	)
	if billScanErr == nil {
		billing := map[string]interface{}{
			"segment_count":     billSegmentCount,
			"price_per_segment": billPricePerSegment,
			"total_amount":      billTotalAmount,
			"tariff_plan_id":    billTariffPlanID, // *string → nil serialises as JSON null (unified rows)
			"source_rule_id":    billSourceRuleID, // new; set for unified-tarified messages
		}
		if billCreatedAt != nil {
			billing["billed_at"] = *billCreatedAt
		}
		result["billing"] = billing
	} else if !errors.Is(billScanErr, pgx.ErrNoRows) {
		log.Warn().Err(billScanErr).Msg("ошибка получения данных тарификации")
	}

	respondJSON(w, http.StatusOK, result)
}

// getMessageViaGRPC is the fallback implementation used when db is nil.
func (h *MessageHandlers) getMessageViaGRPC(w http.ResponseWriter, r *http.Request, id, clientIDStr string) {
	resp, err := h.messagingClient.GetMessageStatus(r.Context(), &messagingv1.GetMessageStatusRequest{
		MessageId: id,
		ClientId:  clientIDStr,
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

	if h.messagingClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис отправки сообщений недоступен"))
		return
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
