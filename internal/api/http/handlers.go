package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Handler представляет HTTP handlers для API Gateway
type Handler struct {
	producer       MessageProducer
	asyncProducer  BatchMessagePublisher
	topicOutgoing  string
	messageRepo    MessageRepository
	clientRepo     ClientRepository
	healthChecker  *monitoring.HealthChecker
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

// SetAsyncProducer устанавливает AsyncProducer для пакетной публикации.
// Когда asyncProducer задан, SendBatchSMS использует неблокирующий PublishAsync
// вместо синхронного PublishOutgoing для каждого сообщения.
func (h *Handler) SetAsyncProducer(ap BatchMessagePublisher, topicOutgoing string) {
	h.asyncProducer = ap
	h.topicOutgoing = topicOutgoing
}

// SendSMS обрабатывает запрос на отправку SMS
func (h *Handler) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req SendSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Валидация
	if err := req.Validate(); err != nil {
		if appErr, ok := err.(*shared.AppError); ok {
			respondError(w, appErr)
		} else {
			respondError(w, shared.ErrInvalidInput(err.Error()))
		}
		return
	}

	// Получаем клиента из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Создаем сообщение
	msg := &shared.Message{
		ID:                uuid.New(),
		Source:            req.Source,
		Destination:       req.Destination,
		Text:              req.Text,
		ExternalID:        shared.NullString(req.ExternalID),
		PriorityFlag:      req.Priority,
		RegisteredDelivery: boolToInt(req.RegisteredDelivery),
		ValidityPeriod:    req.ValidityPeriod,
		ServiceType:       req.ServiceType,
		SourceAddrTON:     req.SourceAddrTON,
		SourceAddrNPI:     req.SourceAddrNPI,
		DestAddrTON:       req.DestAddrTON,
		DestAddrNPI:       req.DestAddrNPI,
		DataCoding:        req.DataCoding,
		Status:            shared.MessageStatusPending,
		ClientID:          &clientID,
		MaxRetries:        5,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Определяем кодировку
	msg.Encoding = detectEncoding(msg.Text)

	// Сохраняем в БД
	if err := h.messageRepo.Create(r.Context(), msg); err != nil {
		log.Error().Err(err).Msg("ошибка сохранения сообщения")
		respondError(w, shared.ErrDatabase("Ошибка сохранения сообщения", err))
		return
	}

	// Публикуем в Kafka
	kafkaMsg := queue.FromMessage(msg)
	kafkaMsg.TraceID = shared.GetRequestID(r.Context())
	if err := h.producer.PublishOutgoing(r.Context(), kafkaMsg); err != nil {
		log.Error().Err(err).Msg("ошибка публикации сообщения в Kafka")
		respondError(w, shared.ErrKafkaProducer(err))
		return
	}

	// Обновляем статус на queued
	msg.Status = shared.MessageStatusQueued
	msg.UpdatedAt = time.Now()
	if err := h.messageRepo.UpdateStatus(r.Context(), msg.ID, msg.Status, ""); err != nil {
		log.Warn().Err(err).Msg("ошибка обновления статуса сообщения")
	}

	respondJSON(w, http.StatusAccepted, SendSMSResponse{
		MessageID: msg.ID.String(),
		Status:    string(msg.Status),
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

	// Получаем клиента из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	results := make([]SendSMSResponse, 0, len(req.Messages))
	successCount := 0
	failedCount := 0

	for _, msgReq := range req.Messages {
		// Валидация
		if err := msgReq.Validate(); err != nil {
			var errMsg string
			if appErr, ok := err.(*shared.AppError); ok {
				errMsg = appErr.Message
			} else {
				errMsg = err.Error()
			}
			results = append(results, SendSMSResponse{
				Status: "failed",
				Error:  errMsg,
			})
			failedCount++
			continue
		}

		// Создаем сообщение
		msg := &shared.Message{
			ID:                uuid.New(),
			Source:            msgReq.Source,
			Destination:       msgReq.Destination,
			Text:              msgReq.Text,
			ExternalID:        shared.NullString(msgReq.ExternalID),
			PriorityFlag:      msgReq.Priority,
			RegisteredDelivery: boolToInt(msgReq.RegisteredDelivery),
			ValidityPeriod:    msgReq.ValidityPeriod,
			ServiceType:       msgReq.ServiceType,
			SourceAddrTON:     msgReq.SourceAddrTON,
			SourceAddrNPI:     msgReq.SourceAddrNPI,
			DestAddrTON:       msgReq.DestAddrTON,
			DestAddrNPI:       msgReq.DestAddrNPI,
			DataCoding:        msgReq.DataCoding,
			Status:            shared.MessageStatusPending,
			ClientID:          &clientID,
			MaxRetries:        5,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		msg.Encoding = detectEncoding(msg.Text)

		// Сохраняем в БД
		if err := h.messageRepo.Create(r.Context(), msg); err != nil {
			log.Error().Err(err).Msg("ошибка сохранения сообщения")
			results = append(results, SendSMSResponse{
				Status: "failed",
				Error:  "Ошибка сохранения сообщения",
			})
			failedCount++
			continue
		}

		// Публикуем в Kafka
		kafkaMsg := queue.FromMessage(msg)
		kafkaMsg.TraceID = shared.GetRequestID(r.Context())

		if h.asyncProducer != nil {
			// Асинхронная пакетная публикация через AsyncProducer (T032)
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
			// Синхронная публикация (fallback)
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

		// Обновляем статус
		msg.Status = shared.MessageStatusQueued
		msg.UpdatedAt = time.Now()
		if err := h.messageRepo.UpdateStatus(r.Context(), msg.ID, msg.Status, ""); err != nil {
			log.Warn().Err(err).Msg("ошибка обновления статуса сообщения")
		}

		results = append(results, SendSMSResponse{
			MessageID: msg.ID.String(),
			Status:    string(msg.Status),
		})
		successCount++
	}

	respondJSON(w, http.StatusAccepted, SendBatchResponse{
		Results:     results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	})
}

// GetStatus обрабатывает запрос на получение статуса сообщения
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	messageIDStr := r.URL.Query().Get("id")
	if messageIDStr == "" {
		respondError(w, shared.ErrInvalidInput("Параметр id обязателен"))
		return
	}

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат ID сообщения"))
		return
	}

	// Получаем сообщение из БД
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

	// Проверяем права доступа (клиент может видеть только свои сообщения)
	clientID, ok := middleware.GetClientID(r.Context())
	if ok && msg.ClientID != nil && *msg.ClientID != clientID {
		respondError(w, shared.ErrForbidden("Нет доступа к этому сообщению"))
		return
	}

	respondJSON(w, http.StatusOK, GetStatusResponse{
		MessageID:    msg.ID.String(),
		Status:       string(msg.Status),
		StatusMessage: string(msg.StatusMessage),
		CreatedAt:    msg.CreatedAt,
		SubmittedAt: msg.SubmittedAt,
		DeliveredAt: msg.DeliveredAt,
		FailedAt:    msg.FailedAt,
		SMPPMessageID: string(msg.SMPPMessageID),
	})
}

// GetHistory обрабатывает запрос на получение истории сообщений
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры запроса
	clientID, _ := middleware.GetClientID(r.Context())
	
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 1000 {
			limit = 100
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
	}

	statusStr := r.URL.Query().Get("status")
	var status *shared.MessageStatus
	if statusStr != "" {
		s := shared.MessageStatus(statusStr)
		status = &s
	}

	// Получаем сообщения
	var messages []*shared.Message
	var err error
	
	if clientID != uuid.Nil {
		messages, err = h.messageRepo.GetByClientID(r.Context(), clientID, limit, offset, status)
	} else {
		// Административный доступ (если нужно)
		messages, err = h.messageRepo.GetAll(r.Context(), limit, offset, status)
	}

	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории сообщений")
		respondError(w, shared.ErrDatabase("Ошибка получения истории", err))
		return
	}

	// Преобразуем в формат ответа
	results := make([]GetStatusResponse, len(messages))
	for i, msg := range messages {
		results[i] = GetStatusResponse{
			MessageID:    msg.ID.String(),
			Status:       string(msg.Status),
			StatusMessage: string(msg.StatusMessage),
			CreatedAt:    msg.CreatedAt,
			SubmittedAt: msg.SubmittedAt,
			DeliveredAt: msg.DeliveredAt,
			FailedAt:    msg.FailedAt,
			SMPPMessageID: string(msg.SMPPMessageID),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": results,
		"limit":    limit,
		"offset":   offset,
		"count":    len(results),
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
	// Простая проверка - если все символы в ASCII диапазоне, используем GSM7
	for _, r := range text {
		if r > 127 {
			return shared.MessageEncodingUCS2
		}
	}
	return shared.MessageEncodingGSM7
}
