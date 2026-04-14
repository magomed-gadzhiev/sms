package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Handler представляет HTTP handlers для API Gateway
type Handler struct {
	producer      MessageProducer
	asyncProducer BatchMessagePublisher
	topicOutgoing string
	messageRepo   MessageRepository
	clientRepo    ClientRepository
	healthChecker *monitoring.HealthChecker
}

// NewHandler создает новый HTTP handler
func NewHandler(
	producer MessageProducer,
	messageRepo MessageRepository,
	clientRepo ClientRepository,
	healthChecker *monitoring.HealthChecker,
) *Handler {
	return &Handler{
		producer:      producer,
		messageRepo:   messageRepo,
		clientRepo:    clientRepo,
		healthChecker: healthChecker,
	}
}


// SendSMS обрабатывает запрос на отправку SMS
func (h *Handler) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req SendSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		if appErr, ok := err.(*shared.AppError); ok {
			respondError(w, appErr)
		} else {
			respondError(w, shared.ErrInvalidInput(err.Error()))
		}
		return
	}

	if req.TemplateID != "" {
		respondError(w, shared.ErrInvalidInput("Отправка через шаблон не поддерживается в этом эндпоинте"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	now := time.Now()
	msg := &shared.Message{
		ID:                 uuid.New(),
		Source:             req.Source,
		Destination:        req.Destination,
		Text:               req.Text,
		ExternalID:         shared.NullString(req.ExternalID),
		PriorityFlag:       req.Priority,
		RegisteredDelivery: boolToInt(req.RegisteredDelivery),
		ValidityPeriod:     req.ValidityPeriod,
		ScheduledAt:        req.ScheduledAt,
		ServiceType:        req.ServiceType,
		SourceAddrTON:      req.SourceAddrTON,
		SourceAddrNPI:      req.SourceAddrNPI,
		DestAddrTON:        req.DestAddrTON,
		DestAddrNPI:        req.DestAddrNPI,
		DataCoding:         req.DataCoding,
		ClientID:           &clientID,
		MaxRetries:         5,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	msg.Encoding = detectEncoding(msg.Text)

	// Если указано время отправки — сохраняем как scheduled, не публикуем в Kafka
	if req.ScheduledAt != nil {
		msg.Status = shared.MessageStatusScheduled
		if err := h.messageRepo.Create(r.Context(), msg); err != nil {
			log.Error().Err(err).Msg("ошибка сохранения запланированного сообщения")
			respondError(w, shared.ErrDatabase("Ошибка сохранения сообщения", err))
			return
		}
		respondJSON(w, http.StatusAccepted, SendSMSResponse{
			MessageID:    msg.ID.String(),
			Status:       string(msg.Status),
			CreatedAt:    msg.CreatedAt,
			ScheduledAt:  msg.ScheduledAt,
			SegmentCount: calcSegmentCount(msg.Text),
		})
		return
	}

	msg.Status = shared.MessageStatusPending
	if err := h.messageRepo.Create(r.Context(), msg); err != nil {
		log.Error().Err(err).Msg("ошибка сохранения сообщения")
		respondError(w, shared.ErrDatabase("Ошибка сохранения сообщения", err))
		return
	}

	kafkaMsg := queue.FromMessage(msg)
	kafkaMsg.TraceID = shared.GetRequestID(r.Context())
	if err := h.producer.PublishOutgoing(r.Context(), kafkaMsg); err != nil {
		log.Error().Err(err).Msg("ошибка публикации сообщения в Kafka")
		respondError(w, shared.ErrKafkaProducer(err))
		return
	}

	msg.Status = shared.MessageStatusQueued
	msg.UpdatedAt = time.Now()
	if err := h.messageRepo.UpdateStatus(r.Context(), msg.ID, msg.Status, ""); err != nil {
		log.Warn().Err(err).Msg("ошибка обновления статуса сообщения")
	}

	respondJSON(w, http.StatusAccepted, SendSMSResponse{
		MessageID:    msg.ID.String(),
		Status:       string(msg.Status),
		CreatedAt:    msg.CreatedAt,
		SegmentCount: calcSegmentCount(msg.Text),
	})
}

// SendBatchSMS обрабатывает запрос на пакетную отправку SMS
func (h *Handler) SendBatchSMS(w http.ResponseWriter, r *http.Request) {
	var req SendBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if len(req.Messages) == 0 {
		respondError(w, shared.ErrInvalidInput("Список сообщений пуст"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	results := make([]SendSMSResponse, 0, len(req.Messages))
	successCount := 0
	failedCount := 0

	for _, msgReq := range req.Messages {
		if err := msgReq.Validate(); err != nil {
			var errMsg string
			if appErr, ok := err.(*shared.AppError); ok {
				errMsg = appErr.Message
			} else {
				errMsg = err.Error()
			}
			results = append(results, SendSMSResponse{Status: "failed", Error: errMsg})
			failedCount++
			continue
		}

		// Поле scheduled_at в пакетном запросе перекрывает поле из отдельного сообщения
		scheduledAt := msgReq.ScheduledAt
		if req.ScheduledAt != nil {
			scheduledAt = req.ScheduledAt
		}

		now := time.Now()
		msg := &shared.Message{
			ID:                 uuid.New(),
			Source:             msgReq.Source,
			Destination:        msgReq.Destination,
			Text:               msgReq.Text,
			ExternalID:         shared.NullString(msgReq.ExternalID),
			PriorityFlag:       msgReq.Priority,
			RegisteredDelivery: boolToInt(msgReq.RegisteredDelivery),
			ValidityPeriod:     msgReq.ValidityPeriod,
			ScheduledAt:        scheduledAt,
			ServiceType:        msgReq.ServiceType,
			SourceAddrTON:      msgReq.SourceAddrTON,
			SourceAddrNPI:      msgReq.SourceAddrNPI,
			DestAddrTON:        msgReq.DestAddrTON,
			DestAddrNPI:        msgReq.DestAddrNPI,
			DataCoding:         msgReq.DataCoding,
			ClientID:           &clientID,
			MaxRetries:         5,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		msg.Encoding = detectEncoding(msg.Text)

		if scheduledAt != nil {
			msg.Status = shared.MessageStatusScheduled
			if err := h.messageRepo.Create(r.Context(), msg); err != nil {
				log.Error().Err(err).Msg("ошибка сохранения запланированного сообщения")
				results = append(results, SendSMSResponse{Status: "failed", Error: "Ошибка сохранения сообщения"})
				failedCount++
				continue
			}
			results = append(results, SendSMSResponse{
				MessageID:    msg.ID.String(),
				Status:       string(msg.Status),
				CreatedAt:    msg.CreatedAt,
				ScheduledAt:  msg.ScheduledAt,
				SegmentCount: calcSegmentCount(msg.Text),
			})
			successCount++
			continue
		}

		msg.Status = shared.MessageStatusPending
		if err := h.messageRepo.Create(r.Context(), msg); err != nil {
			log.Error().Err(err).Msg("ошибка сохранения сообщения")
			results = append(results, SendSMSResponse{Status: "failed", Error: "Ошибка сохранения сообщения"})
			failedCount++
			continue
		}

		kafkaMsg := queue.FromMessage(msg)
		kafkaMsg.TraceID = shared.GetRequestID(r.Context())

		if h.asyncProducer != nil {
			data, err := kafkaMsg.Serialize()
			if err != nil {
				log.Error().Err(err).Msg("ошибка сериализации сообщения для Kafka")
				results = append(results, SendSMSResponse{
					MessageID: msg.ID.String(),
					Status:    "failed",
					Error:     "Ошибка сериализации сообщения",
				})
				failedCount++
				continue
			}
			headers := []sarama.RecordHeader{
				{Key: []byte("message_id"), Value: []byte(kafkaMsg.MessageID.String())},
				{Key: []byte("source"), Value: []byte(kafkaMsg.Source)},
				{Key: []byte("destination"), Value: []byte(kafkaMsg.Destination)},
			}
			h.asyncProducer.PublishAsync(h.topicOutgoing, kafkaMsg.MessageID.String(), data, headers)
		} else {
			if err := h.producer.PublishOutgoing(r.Context(), kafkaMsg); err != nil {
				log.Error().Err(err).Msg("ошибка публикации сообщения в Kafka")
				results = append(results, SendSMSResponse{
					MessageID: msg.ID.String(),
					Status:    "failed",
					Error:     "Ошибка публикации в очередь",
				})
				failedCount++
				continue
			}
		}

		msg.Status = shared.MessageStatusQueued
		msg.UpdatedAt = time.Now()
		if err := h.messageRepo.UpdateStatus(r.Context(), msg.ID, msg.Status, ""); err != nil {
			log.Warn().Err(err).Msg("ошибка обновления статуса сообщения")
		}

		results = append(results, SendSMSResponse{
			MessageID:    msg.ID.String(),
			Status:       string(msg.Status),
			CreatedAt:    msg.CreatedAt,
			SegmentCount: calcSegmentCount(msg.Text),
		})
		successCount++
	}

	respondJSON(w, http.StatusAccepted, SendBatchResponse{
		Results:      results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	})
}

// GetStatus обрабатывает запрос на получение статуса сообщения
// Параметр id передаётся как path variable: GET /api/v1/sms/status/{id}
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageIDStr := vars["id"]
	if messageIDStr == "" {
		respondError(w, shared.ErrInvalidInput("Параметр id обязателен"))
		return
	}

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат ID сообщения"))
		return
	}

	msg, err := h.messageRepo.GetByID(r.Context(), messageID)
	if err != nil {
		if err == storage.ErrNotFound {
			respondError(w, shared.ErrNotFound("Сообщение"))
			return
		}
		log.Error().Err(err).Msg("ошибка получения сообщения")
		respondError(w, shared.ErrDatabase("Ошибка получения сообщения", err))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if msg.ClientID != nil && *msg.ClientID != clientID {
		respondError(w, shared.ErrForbidden("Нет доступа к этому сообщению"))
		return
	}

	respondJSON(w, http.StatusOK, GetStatusResponse{
		MessageID:     msg.ID.String(),
		Status:        string(msg.Status),
		StatusMessage: string(msg.StatusMessage),
		CreatedAt:     msg.CreatedAt,
		SubmittedAt:   msg.SubmittedAt,
		DeliveredAt:   msg.DeliveredAt,
		FailedAt:      msg.FailedAt,
		SMPPMessageID: string(msg.SMPPMessageID),
	})
}

// GetHistory обрабатывает запрос на получение истории сообщений
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	filter := shared.MessageFilter{
		Limit:  parseIntParam(r, "limit", 100, 1, 1000),
		Offset: parseIntParam(r, "offset", 0, 0, -1),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		st := shared.MessageStatus(s)
		filter.Status = &st
	}
	if d := r.URL.Query().Get("destination"); d != "" {
		filter.Destination = d
	}
	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			filter.From = &t
		} else {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра from (ожидается RFC3339)"))
			return
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if pt, err := time.Parse(time.RFC3339, t); err == nil {
			filter.To = &pt
		} else {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра to (ожидается RFC3339)"))
			return
		}
	}

	messages, total, err := h.messageRepo.ListMessages(r.Context(), clientID, filter)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории сообщений")
		respondError(w, shared.ErrDatabase("Ошибка получения истории", err))
		return
	}

	results := make([]GetStatusResponse, len(messages))
	for i, msg := range messages {
		results[i] = GetStatusResponse{
			MessageID:     msg.ID.String(),
			Status:        string(msg.Status),
			StatusMessage: string(msg.StatusMessage),
			CreatedAt:     msg.CreatedAt,
			SubmittedAt:   msg.SubmittedAt,
			DeliveredAt:   msg.DeliveredAt,
			FailedAt:      msg.FailedAt,
			SMPPMessageID: string(msg.SMPPMessageID),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": results,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

// CancelSMS отменяет запланированное сообщение
func (h *Handler) CancelSMS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageIDStr := vars["id"]
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат ID сообщения"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if err := h.messageRepo.CancelByIDAndStatus(r.Context(), messageID, clientID); err != nil {
		if err == storage.ErrNotFound {
			respondError(w, shared.ErrNotFound("Сообщение не найдено или не может быть отменено"))
			return
		}
		// CancelByIDAndStatus возвращает fmt.Errorf при rows==0
		log.Error().Err(err).Msg("ошибка отмены сообщения")
		respondError(w, &shared.AppError{
			HTTPStatus: http.StatusNotFound,
			Code:       "NOT_FOUND",
			Message:    "Сообщение не найдено или не находится в статусе scheduled",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBalance возвращает баланс аккаунта
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	balance, currency, err := h.clientRepo.GetBalance(r.Context(), clientID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения баланса")
		respondError(w, shared.ErrDatabase("Ошибка получения баланса", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"balance":  balance,
		"currency": currency,
	})
}

// GetStats возвращает статистику отправок клиента
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	statuses := []shared.MessageStatus{
		shared.MessageStatusQueued,
		shared.MessageStatusSent,
		shared.MessageStatusDelivered,
		shared.MessageStatusFailed,
		shared.MessageStatusExpired,
		shared.MessageStatusRejected,
		shared.MessageStatusScheduled,
	}

	counts := make(map[string]int, len(statuses))
	total := 0
	for _, st := range statuses {
		s := st
		msgs, n, err := h.messageRepo.ListMessages(r.Context(), clientID, shared.MessageFilter{
			Status: &s,
			Limit:  1,
			Offset: 0,
		})
		_ = msgs
		if err != nil {
			log.Error().Err(err).Str("status", string(st)).Msg("ошибка получения статистики")
			respondError(w, shared.ErrDatabase("Ошибка получения статистики", err))
			return
		}
		counts[string(st)] = n
		total += n
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total":  total,
		"counts": counts,
	})
}

// GetScheduled возвращает список запланированных сообщений
func (h *Handler) GetScheduled(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	limit := parseIntParam(r, "limit", 100, 1, 1000)
	offset := parseIntParam(r, "offset", 0, 0, -1)

	messages, total, err := h.messageRepo.ListScheduled(r.Context(), clientID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения запланированных сообщений")
		respondError(w, shared.ErrDatabase("Ошибка получения запланированных сообщений", err))
		return
	}

	type item struct {
		MessageID   string     `json:"message_id"`
		Source      string     `json:"source"`
		Destination string     `json:"destination"`
		Text        string     `json:"text"`
		Status      string     `json:"status"`
		ExternalID  string     `json:"external_id,omitempty"`
		ScheduledAt *time.Time `json:"scheduled_at"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	results := make([]item, len(messages))
	for i, msg := range messages {
		results[i] = item{
			MessageID:   msg.ID.String(),
			Source:      msg.Source,
			Destination: msg.Destination,
			Text:        msg.Text,
			Status:      string(msg.Status),
			ExternalID:  string(msg.ExternalID),
			ScheduledAt: msg.ScheduledAt,
			CreatedAt:   msg.CreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": results,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// Health обрабатывает health check запрос
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker != nil {
		h.healthChecker.Handler()(w, r)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"service": "api-gateway",
	})
}

// Вспомогательные функции

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("ошибка кодирования JSON ответа")
	}
}

func respondError(w http.ResponseWriter, err *shared.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}
	if err.Details != "" {
		response["error"].(map[string]interface{})["details"] = err.Details
	}
	json.NewEncoder(w).Encode(response)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func detectEncoding(text string) shared.MessageEncoding {
	for _, r := range text {
		if r > 127 {
			return shared.MessageEncodingUCS2
		}
	}
	return shared.MessageEncodingGSM7
}

func calcSegmentCount(text string) int {
	if len(text) == 0 {
		return 1
	}
	isUCS2 := false
	for _, r := range text {
		if r > 127 {
			isUCS2 = true
			break
		}
	}
	runeCount := len([]rune(text))
	if isUCS2 {
		if runeCount <= 70 {
			return 1
		}
		return (runeCount + 66) / 67
	}
	if runeCount <= 160 {
		return 1
	}
	return (runeCount + 152) / 153
}

// parseIntParam парses a query param as int with default and bounds.
// maxVal <= 0 means no upper bound.
func parseIntParam(r *http.Request, name string, defaultVal, minVal, maxVal int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	if v < minVal {
		return minVal
	}
	if maxVal > 0 && v > maxVal {
		return maxVal
	}
	return v
}
