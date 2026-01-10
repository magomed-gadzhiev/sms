package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// SMSHandlers содержит handlers для SMS операций
type SMSHandlers struct {
	messagingClient messagingv1.MessagingServiceClient
}

// NewSMSHandlers создает новый SMSHandlers
func NewSMSHandlers(messagingClient messagingv1.MessagingServiceClient) *SMSHandlers {
	return &SMSHandlers{
		messagingClient: messagingClient,
	}
}

// SendSMSRequest представляет запрос на отправку SMS
type SendSMSRequest struct {
	Source            string    `json:"source"`
	Destination       string    `json:"destination"`
	Text              string    `json:"text"`
	ExternalID        string    `json:"external_id,omitempty"`
	Priority          int32     `json:"priority,omitempty"`
	RegisteredDelivery bool     `json:"registered_delivery,omitempty"`
	ValidityPeriod    *time.Time `json:"validity_period,omitempty"`
	ServiceType       string    `json:"service_type,omitempty"`
	SourceAddrTON     int32     `json:"source_addr_ton,omitempty"`
	SourceAddrNPI     int32     `json:"source_addr_npi,omitempty"`
	DestAddrTON       int32     `json:"dest_addr_ton,omitempty"`
	DestAddrNPI       int32     `json:"dest_addr_npi,omitempty"`
	DataCoding        int32     `json:"data_coding,omitempty"`
}

// SendSMS обрабатывает запрос на отправку одного SMS
func (h *SMSHandlers) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req SendSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Валидация
	if req.Source == "" || req.Destination == "" || req.Text == "" {
		respondError(w, shared.ErrInvalidInput("Поля source, destination и text обязательны"))
		return
	}

	// Преобразуем в proto запрос
	protoReq := &messagingv1.SendMessageRequest{
		ClientId:          clientID.String(),
		Source:            req.Source,
		Destination:       req.Destination,
		Text:              req.Text,
		ExternalId:        req.ExternalID,
		Priority:          req.Priority,
		RegisteredDelivery: req.RegisteredDelivery,
		ServiceType:       req.ServiceType,
		SourceAddrTon:     req.SourceAddrTON,
		SourceAddrNpi:     req.SourceAddrNPI,
		DestAddrTon:       req.DestAddrTON,
		DestAddrNpi:       req.DestAddrNPI,
		DataCoding:        req.DataCoding,
	}

	if req.ValidityPeriod != nil {
		protoReq.ValidityPeriod = timestamppb.New(*req.ValidityPeriod)
	}

	// Вызываем Messaging Service
	resp, err := h.messagingClient.SendMessage(r.Context(), protoReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка отправки SMS через Messaging Service")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"message_id": resp.MessageId,
		"status":     resp.Status,
		"created_at": resp.CreatedAt.AsTime(),
	}
	if resp.Error != "" {
		response["error"] = resp.Error
	}

	respondJSON(w, http.StatusOK, response)
}

// SendBatchSMSRequest представляет запрос на пакетную отправку SMS
type SendBatchSMSRequest struct {
	Messages []SendSMSRequest `json:"messages"`
}

// SendBatch обрабатывает запрос на пакетную отправку SMS
func (h *SMSHandlers) SendBatch(w http.ResponseWriter, r *http.Request) {
	var req SendBatchSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if len(req.Messages) == 0 {
		respondError(w, shared.ErrInvalidInput("Список сообщений не может быть пустым"))
		return
	}

	// Преобразуем в proto сообщения
	protoMessages := make([]*messagingv1.SendMessageRequest, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Source == "" || msg.Destination == "" || msg.Text == "" {
			continue // Пропускаем невалидные сообщения
		}

		protoMsg := &messagingv1.SendMessageRequest{
			ClientId:          clientID.String(),
			Source:            msg.Source,
			Destination:       msg.Destination,
			Text:              msg.Text,
			ExternalId:        msg.ExternalID,
			Priority:          msg.Priority,
			RegisteredDelivery: msg.RegisteredDelivery,
			ServiceType:       msg.ServiceType,
			SourceAddrTon:     msg.SourceAddrTON,
			SourceAddrNpi:     msg.SourceAddrNPI,
			DestAddrTon:       msg.DestAddrTON,
			DestAddrNpi:       msg.DestAddrNPI,
			DataCoding:        msg.DataCoding,
		}

		if msg.ValidityPeriod != nil {
			protoMsg.ValidityPeriod = timestamppb.New(*msg.ValidityPeriod)
		}

		protoMessages = append(protoMessages, protoMsg)
	}

	// Вызываем Messaging Service
	protoReq := &messagingv1.SendBatchRequest{
		ClientId:  clientID.String(),
		Messages:  protoMessages,
	}

	resp, err := h.messagingClient.SendBatch(r.Context(), protoReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка пакетной отправки SMS через Messaging Service")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	results := make([]map[string]interface{}, 0, len(resp.Results))
	for _, result := range resp.Results {
		r := map[string]interface{}{
			"message_id": result.MessageId,
			"status":     result.Status,
			"created_at": result.CreatedAt.AsTime(),
		}
		if result.Error != "" {
			r["error"] = result.Error
		}
		results = append(results, r)
	}

	response := map[string]interface{}{
		"results":       results,
		"success_count": resp.SuccessCount,
		"failed_count":  resp.FailedCount,
	}

	respondJSON(w, http.StatusOK, response)
}

// GetStatus обрабатывает запрос на получение статуса сообщения
func (h *SMSHandlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageID := vars["id"]

	if messageID == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Вызываем Messaging Service
	resp, err := h.messagingClient.GetMessageStatus(r.Context(), &messagingv1.GetMessageStatusRequest{
		MessageId: messageID,
		ClientId:  clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Str("message_id", messageID).Msg("ошибка получения статуса сообщения")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"message_id":    resp.MessageId,
		"status":        resp.Status,
		"status_message": resp.StatusMessage,
		"created_at":    resp.CreatedAt.AsTime(),
	}

	if resp.SubmittedAt != nil {
		response["submitted_at"] = resp.SubmittedAt.AsTime()
	}
	if resp.DeliveredAt != nil {
		response["delivered_at"] = resp.DeliveredAt.AsTime()
	}
	if resp.FailedAt != nil {
		response["failed_at"] = resp.FailedAt.AsTime()
	}
	if resp.SmppMessageId != "" {
		response["smpp_message_id"] = resp.SmppMessageId
	}
	if resp.ErrorCode != "" {
		response["error_code"] = resp.ErrorCode
	}
	if resp.ErrorMessage != "" {
		response["error_message"] = resp.ErrorMessage
	}

	respondJSON(w, http.StatusOK, response)
}

// GetHistory обрабатывает запрос на получение истории сообщений
func (h *SMSHandlers) GetHistory(w http.ResponseWriter, r *http.Request) {
	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Парсим query параметры
	query := r.URL.Query()
	
	var from, to *time.Time
	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &t
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &t
		}
	}

	status := query.Get("status")
	destination := query.Get("destination")
	
	limit := 100
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Формируем proto запрос
	protoReq := &messagingv1.GetMessageHistoryRequest{
		ClientId:    clientID.String(),
		Status:      status,
		Destination: destination,
		Limit:       int32(limit),
		Offset:      int32(offset),
	}

	if from != nil {
		protoReq.From = timestamppb.New(*from)
	}
	if to != nil {
		protoReq.To = timestamppb.New(*to)
	}

	// Вызываем Messaging Service
	resp, err := h.messagingClient.GetMessageHistory(r.Context(), protoReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории сообщений")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	messages := make([]map[string]interface{}, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		m := map[string]interface{}{
			"message_id":  msg.MessageId,
			"client_id":   msg.ClientId,
			"source":      msg.Source,
			"destination": msg.Destination,
			"text":        msg.Text,
			"status":      msg.Status,
			"external_id": msg.ExternalId,
			"created_at":  msg.CreatedAt.AsTime(),
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
		if msg.ProviderId != "" {
			m["provider_id"] = msg.ProviderId
		}
		if msg.RouteId != "" {
			m["route_id"] = msg.RouteId
		}

		messages = append(messages, m)
	}

	response := map[string]interface{}{
		"messages": messages,
		"total":    resp.Total,
		"limit":    resp.Limit,
		"offset":   resp.Offset,
	}

	respondJSON(w, http.StatusOK, response)
}
