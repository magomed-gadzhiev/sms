package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/canary"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
	"github.com/smpp-server/smpp-server/internal/shared/messagestatus"
)

// senderNameCache caches (client_id) → map[name]sender_name_id for approved
// senders. Hot-path: every POST /api/v1/sms/send calls resolveSenderName and
// without cache it fans out 1–2 gRPC calls each hitting Postgres.
var senderNameCache = cache.NewHardCache(60 * time.Second)

type cachedSenderMap struct {
	approved map[string]string // name → sender_name_id
}

// gRPC metadata keys matching internal/services/messaging/grpc/server.go. They
// ferry audit-linkage UUIDs from the HTTP handler into the messaging-service
// gRPC server without requiring a proto regeneration.
const (
	mdKeyTemplateID   = "x-sms-template-id"
	mdKeySenderNameID = "x-sms-sender-name-id"
)

// SMSHandlers содержит handlers для SMS операций
type SMSHandlers struct {
	messagingClient  messagingv1.MessagingServiceClient
	templateClient   templatev1.TemplateServiceClient
	clientClient     clientv1.ClientServiceClient
	senderNameClient sendernamev1.SenderNameServiceClient
}

// NewSMSHandlers создает новый SMSHandlers.
// senderNameClient may be nil in tests; when nil, sender-name authorization is
// DISABLED (legacy behaviour) — callers MUST pass a real client in production.
func NewSMSHandlers(
	messagingClient messagingv1.MessagingServiceClient,
	templateClient templatev1.TemplateServiceClient,
	clientClient clientv1.ClientServiceClient,
	senderNameClient sendernamev1.SenderNameServiceClient,
) *SMSHandlers {
	return &SMSHandlers{
		messagingClient:  messagingClient,
		templateClient:   templateClient,
		clientClient:     clientClient,
		senderNameClient: senderNameClient,
	}
}

// resolveSenderName looks up the sender_name row for (clientID, source) and
// enforces approval. It returns the UUID of the approved sender_name or an
// error suitable for responding to the HTTP caller.
//
// Errors:
//   - shared.ErrForbidden if the sender name is not registered for this client
//     or is registered but not approved (pending/rejected/deactivated).
//   - shared.ErrInternal on transport failures talking to sender-name service.
//
// When senderNameClient is nil (unit tests), it returns (nil, nil) so legacy
// tests continue to pass; production wiring always injects a real client.
func (h *SMSHandlers) resolveSenderName(ctx context.Context, clientID, source string) (string, *shared.AppError) {
	if h.senderNameClient == nil {
		return "", nil
	}

	// Hard-cache hit: all approved sender names for this client are in memory.
	if senderNameCache.Enabled() {
		if v, ok := senderNameCache.Get(clientID); ok {
			entry := v.(*cachedSenderMap)
			if id, found := entry.approved[source]; found {
				return id, nil
			}
			// Cached list is authoritative for approved senders; if source is
			// not there, fall through to the slow path only to distinguish
			// "unknown" from "exists-but-not-approved" for a useful error.
			return h.senderNotFoundResponse(ctx, clientID, source)
		}
	}

	// Fetch only approved sender names for this client. The API supports
	// server-side status filtering so we don't pull the full list.
	resp, err := h.senderNameClient.ListSenderNames(ctx, &sendernamev1.ListSenderNamesRequest{
		ClientId: clientID,
		Status:   "approved",
		Limit:    1000,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Str("source", source).
			Msg("ошибка запроса sender-name service")
		return "", shared.ErrInternalServer("ошибка проверки sender name")
	}

	// Populate cache with all approved senders for this client.
	if senderNameCache.Enabled() {
		m := make(map[string]string, len(resp.GetSenderNames()))
		for _, sn := range resp.GetSenderNames() {
			if sn.GetStatus() == "approved" {
				m[sn.GetName()] = sn.GetId()
			}
		}
		senderNameCache.Set(clientID, &cachedSenderMap{approved: m})
	}

	for _, sn := range resp.GetSenderNames() {
		if sn.GetName() == source {
			// Defence-in-depth: although we filtered by approved, verify.
			if sn.GetStatus() != "approved" {
				continue
			}
			return sn.GetId(), nil
		}
	}

	return h.senderNotFoundResponse(ctx, clientID, source)
}

// senderNotFoundResponse performs the distinguishing lookup between
// "sender unknown" and "sender exists but not approved" to return a clearer
// forbidden message. Slow path — not cached because it only fires on rejection.
func (h *SMSHandlers) senderNotFoundResponse(ctx context.Context, clientID, source string) (string, *shared.AppError) {
	allResp, err := h.senderNameClient.ListSenderNames(ctx, &sendernamev1.ListSenderNamesRequest{
		ClientId: clientID,
		Limit:    1000,
	})
	if err == nil {
		for _, sn := range allResp.GetSenderNames() {
			if sn.GetName() == source {
				return "", shared.ErrForbidden("sender name не одобрен (status=" + sn.GetStatus() + ")")
			}
		}
	}
	return "", shared.ErrForbidden("sender name не зарегистрирован")
}

// attachAuditMD adds template_id/sender_name_id to outgoing gRPC metadata.
func attachAuditMD(ctx context.Context, templateID, senderNameID string) context.Context {
	var pairs []string
	if templateID != "" {
		pairs = append(pairs, mdKeyTemplateID, templateID)
	}
	if senderNameID != "" {
		pairs = append(pairs, mdKeySenderNameID, senderNameID)
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// SendSMSRequest представляет запрос на отправку SMS
type SendSMSRequest struct {
	Source             string            `json:"source"`
	Destination        string            `json:"destination"`
	Text               string            `json:"text"`
	TemplateID         string            `json:"template_id,omitempty"`
	Variables          map[string]string `json:"variables,omitempty"`
	ExternalID         string            `json:"external_id,omitempty"`
	Priority           int32             `json:"priority,omitempty"`
	RegisteredDelivery bool              `json:"registered_delivery,omitempty"`
	ValidityPeriod     *time.Time        `json:"validity_period,omitempty"`
	ServiceType        string            `json:"service_type,omitempty"`
	SourceAddrTON      int32             `json:"source_addr_ton,omitempty"`
	SourceAddrNPI      int32             `json:"source_addr_npi,omitempty"`
	DestAddrTON        int32             `json:"dest_addr_ton,omitempty"`
	DestAddrNPI        int32             `json:"dest_addr_npi,omitempty"`
	DataCoding         int32             `json:"data_coding,omitempty"`
	ScheduledAt        *time.Time        `json:"scheduled_at,omitempty"`
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

	// Plan 8 Task 6 followup: canary mode allowlist (parity с gRPC SendMessage).
	// До Plan 8 HTTP send'ы шли мимо canary check'а — silent bypass.
	if !canary.IsAllowed(clientID.String()) {
		log.Warn().
			Str("client_id", clientID.String()).
			Msg("canary mode active: client not in allowlist, rejecting HTTP send")
		respondError(w, shared.ErrServiceUnavailable("service in canary mode — please retry after 24h"))
		return
	}

	// Валидация
	// Validate source and destination
	if req.Source == "" || req.Destination == "" {
		respondError(w, shared.ErrInvalidInput("Поля source и destination обязательны"))
		return
	}

	// Resolve text: either from template or direct
	text := req.Text
	if req.TemplateID != "" && req.Text != "" {
		respondError(w, shared.ErrInvalidInput("Нельзя указать одновременно text и template_id"))
		return
	}
	if req.TemplateID == "" && req.Text == "" {
		respondError(w, shared.ErrInvalidInput("Необходимо указать text или template_id"))
		return
	}
	if req.TemplateID != "" {
		renderResp, err := h.templateClient.RenderTemplate(r.Context(), &templatev1.RenderTemplateRequest{
			TemplateId: req.TemplateID,
			ClientId:   clientID.String(),
			Variables:  req.Variables,
		})
		if err != nil {
			respondGRPCError(w, err)
			return
		}
		text = renderResp.RenderedText
	}

	// Проверяем sandbox-режим клиента
	isSandbox := false
	if h.clientClient != nil {
		clientResp, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Warn().Err(err).Str("client_id", clientID.String()).Msg("не удалось получить данные клиента для sandbox-проверки")
		} else if clientResp.GetClient() != nil {
			isSandbox = clientResp.GetClient().IsSandbox
		}
	}

	// Sender name authorization (Bug #7): enforce that `source` matches a
	// sender_name row owned by this client and in status=approved. Skipped in
	// sandbox mode so smoke-testing doesn't require full onboarding. Skipped
	// when senderNameClient is nil (unit tests).
	senderNameID := ""
	if !isSandbox {
		id, appErr := h.resolveSenderName(r.Context(), clientID.String(), req.Source)
		if appErr != nil {
			log.Info().
				Str("client_id", clientID.String()).
				Str("source", req.Source).
				Str("reason", appErr.Message).
				Msg("отклонена отправка: sender name не авторизован")
			respondError(w, appErr)
			return
		}
		senderNameID = id
	}

	// Преобразуем в proto запрос
	protoReq := &messagingv1.SendMessageRequest{
		ClientId:           clientID.String(),
		Source:             req.Source,
		Destination:        req.Destination,
		Text:               text,
		ExternalId:         req.ExternalID,
		Priority:           req.Priority,
		RegisteredDelivery: req.RegisteredDelivery,
		ServiceType:        req.ServiceType,
		SourceAddrTon:      req.SourceAddrTON,
		SourceAddrNpi:      req.SourceAddrNPI,
		DestAddrTon:        req.DestAddrTON,
		DestAddrNpi:        req.DestAddrNPI,
		DataCoding:         req.DataCoding,
		IsSandbox:          isSandbox,
	}

	if req.ValidityPeriod != nil {
		protoReq.ValidityPeriod = timestamppb.New(*req.ValidityPeriod)
	}
	if req.ScheduledAt != nil {
		// US4 AC4: reject messages scheduled in the past.
		if req.ScheduledAt.Before(time.Now()) {
			respondError(w, shared.ErrInvalidInput("scheduled_at должно быть в будущем"))
			return
		}
		protoReq.ScheduledAt = timestamppb.New(*req.ScheduledAt)
	}

	// Вызываем Messaging Service. Audit linkage (template_id, sender_name_id)
	// is ferried as gRPC metadata because the messaging proto cannot be
	// regenerated in this environment (see docs/reports/2026-04-22-* for
	// context).
	sendCtx := attachAuditMD(r.Context(), req.TemplateID, senderNameID)
	resp, err := h.messagingClient.SendMessage(sendCtx, protoReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка отправки SMS через Messaging Service")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"message_id":    resp.MessageId,
		"status":        resp.Status,
		"segment_count": resp.SegmentCount,
	}
	if resp.CreatedAt != nil {
		response["created_at"] = resp.CreatedAt.AsTime()
	}
	if resp.Error != "" {
		response["error"] = resp.Error
	}
	if resp.ScheduledAt != nil {
		response["scheduled_at"] = resp.ScheduledAt.AsTime()
	}

	respondJSON(w, http.StatusOK, response)
}

// SendBatchSMSRequest представляет запрос на пакетную отправку SMS
type SendBatchSMSRequest struct {
	Messages    []SendSMSRequest `json:"messages"`
	ScheduledAt *time.Time       `json:"scheduled_at,omitempty"`
}

// MaxBatchSize — maximum messages accepted in a single POST /sms/batch
// (contracts/client-api.md). Batches larger than this are rejected wholesale
// with HTTP 400 batch_too_large (US3 AC3).
const MaxBatchSize = 10000

// SendBatch обрабатывает запрос на пакетную отправку SMS.
//
// Response contract (contracts/client-api.md):
//
//	{ "results": [...], "total": N, "accepted": K, "rejected": N-K }
//
// Each result is either {message_id, status, segment_count, created_at} for
// accepted messages OR {error, index} for entries rejected before the RPC
// (validation or sender-name failures). `index` is the 0-based position in the
// original request — clients need it to correlate errors with inputs.
func (h *SMSHandlers) SendBatch(w http.ResponseWriter, r *http.Request) {
	var req SendBatchSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Plan 8 Task 6 followup: canary mode allowlist (parity с gRPC SendBulkMessages).
	if !canary.IsAllowed(clientID.String()) {
		log.Warn().
			Str("client_id", clientID.String()).
			Msg("canary mode active: client not in allowlist, rejecting HTTP batch send")
		respondError(w, shared.ErrServiceUnavailable("service in canary mode — please retry after 24h"))
		return
	}

	if len(req.Messages) == 0 {
		respondError(w, shared.ErrInvalidInput("Список сообщений не может быть пустым"))
		return
	}

	// US3 AC3: oversize batches rejected wholesale before any processing.
	if len(req.Messages) > MaxBatchSize {
		respondError(w, &shared.AppError{
			Code:       "BATCH_TOO_LARGE",
			Message:    "Максимальный размер пакета — 10000 сообщений",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// US4 AC4: batch-level scheduled_at in the past → reject entire batch.
	if req.ScheduledAt != nil && req.ScheduledAt.Before(time.Now()) {
		respondError(w, shared.ErrInvalidInput("scheduled_at должно быть в будущем"))
		return
	}

	// Pre-RPC validation phase — build proto messages for accepted entries and
	// record rejections with their original indices so the client can map back.
	type rejection struct {
		index int
		err   string
	}
	protoMessages := make([]*messagingv1.SendMessageRequest, 0, len(req.Messages))
	// acceptedIndices[i] = original index of req.Messages that produced
	// protoMessages[i]. Needed to fill in rejections from downstream responses.
	acceptedIndices := make([]int, 0, len(req.Messages))
	rejections := make([]rejection, 0)

	for i, msg := range req.Messages {
		if msg.Source == "" || msg.Destination == "" {
			rejections = append(rejections, rejection{i, "source и destination обязательны"})
			continue
		}
		if msg.TemplateID != "" && msg.Text != "" {
			rejections = append(rejections, rejection{i, "нельзя указать одновременно text и template_id"})
			continue
		}
		if msg.TemplateID == "" && msg.Text == "" {
			rejections = append(rejections, rejection{i, "необходимо указать text или template_id"})
			continue
		}

		msgText := msg.Text
		if msg.TemplateID != "" {
			renderResp, err := h.templateClient.RenderTemplate(r.Context(), &templatev1.RenderTemplateRequest{
				TemplateId: msg.TemplateID,
				ClientId:   clientID.String(),
				Variables:  msg.Variables,
			})
			if err != nil {
				rejections = append(rejections, rejection{i, "ошибка рендера шаблона: " + err.Error()})
				continue
			}
			msgText = renderResp.RenderedText
		}

		// Bug #7: enforce sender-name authorization per message. Audit linkage
		// (template_id / sender_name_id) is NOT propagated through the batch
		// RPC because SendBatch proto carries one metadata map per RPC, not
		// per-message. Single-message path (SendSMS) is the authoritative
		// audit trail. TODO: extend SendBatch proto to carry per-message
		// audit metadata so batch+templates closes the compliance loop.
		if _, appErr := h.resolveSenderName(r.Context(), clientID.String(), msg.Source); appErr != nil {
			rejections = append(rejections, rejection{i, appErr.Message})
			continue
		}

		protoMsg := &messagingv1.SendMessageRequest{
			ClientId:           clientID.String(),
			Source:             msg.Source,
			Destination:        msg.Destination,
			Text:               msgText,
			ExternalId:         msg.ExternalID,
			Priority:           msg.Priority,
			RegisteredDelivery: msg.RegisteredDelivery,
			ServiceType:        msg.ServiceType,
			SourceAddrTon:      msg.SourceAddrTON,
			SourceAddrNpi:      msg.SourceAddrNPI,
			DestAddrTon:        msg.DestAddrTON,
			DestAddrNpi:        msg.DestAddrNPI,
			DataCoding:         msg.DataCoding,
		}
		if msg.ValidityPeriod != nil {
			protoMsg.ValidityPeriod = timestamppb.New(*msg.ValidityPeriod)
		}
		protoMessages = append(protoMessages, protoMsg)
		acceptedIndices = append(acceptedIndices, i)
	}

	// Build ordered results: insert accepted and rejected entries into a single
	// slice positioned by the original index to give the client a stable order.
	total := len(req.Messages)
	results := make([]map[string]interface{}, total)

	// Place rejections first.
	for _, rej := range rejections {
		results[rej.index] = map[string]interface{}{
			"error": rej.err,
			"index": rej.index,
		}
	}

	accepted := 0
	if len(protoMessages) > 0 {
		protoReq := &messagingv1.SendBatchRequest{
			ClientId: clientID.String(),
			Messages: protoMessages,
		}
		if req.ScheduledAt != nil {
			protoReq.ScheduledAt = timestamppb.New(*req.ScheduledAt)
		}

		resp, err := h.messagingClient.SendBatch(r.Context(), protoReq)
		if err != nil {
			log.Error().Err(err).Msg("ошибка пакетной отправки SMS через Messaging Service")
			respondGRPCError(w, err)
			return
		}

		// Fill in downstream results at their original positions. Assumption:
		// resp.Results aligns 1:1 with protoMessages in the order we sent.
		for i, result := range resp.Results {
			if i >= len(acceptedIndices) {
				break
			}
			origIdx := acceptedIndices[i]
			item := map[string]interface{}{
				"message_id":    result.MessageId,
				"status":        result.Status,
				"segment_count": result.SegmentCount,
			}
			if result.CreatedAt != nil {
				item["created_at"] = result.CreatedAt.AsTime()
			}
			if result.Error != "" {
				item["error"] = result.Error
				item["index"] = origIdx
			} else {
				accepted++
			}
			results[origIdx] = item
		}
	}

	response := map[string]interface{}{
		"results":  results,
		"total":    total,
		"accepted": accepted,
		"rejected": total - accepted,
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
		"message_id":     resp.MessageId,
		"status":         resp.Status,
		"status_message": resp.StatusMessage,
	}
	if resp.CreatedAt != nil {
		response["created_at"] = resp.CreatedAt.AsTime()
	}

	if resp.SubmittedAt != nil {
		// Contract (contracts/client-api.md) calls this field `sent_at`; proto
		// carries it as `submitted_at` for historical reasons. Emit both so
		// any existing clients relying on the internal name don't break during
		// the transition, but `sent_at` is the canonical public name.
		response["sent_at"] = resp.SubmittedAt.AsTime()
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
	if resp.ExpiredAt != nil {
		response["expired_at"] = resp.ExpiredAt.AsTime()
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

// ListScheduled returns paginated list of scheduled messages.
// GET /api/v1/sms/scheduled?limit=100&offset=0
func (h *SMSHandlers) ListScheduled(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	limit := 100
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	resp, err := h.messagingClient.ListScheduledMessages(r.Context(), &messagingv1.ListScheduledMessagesRequest{
		ClientId: clientID.String(),
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	type messageItem struct {
		MessageID   string  `json:"message_id"`
		Source      string  `json:"source"`
		Destination string  `json:"destination"`
		Text        string  `json:"text"`
		Status      string  `json:"status"`
		ExternalID  string  `json:"external_id,omitempty"`
		ScheduledAt *string `json:"scheduled_at,omitempty"`
		CreatedAt   string  `json:"created_at"`
	}

	items := make([]messageItem, len(resp.Messages))
	for i, m := range resp.Messages {
		item := messageItem{
			MessageID:   m.MessageId,
			Source:      m.Source,
			Destination: m.Destination,
			Text:        m.Text,
			Status:      m.Status,
			ExternalID:  m.ExternalId,
			CreatedAt:   m.CreatedAt.AsTime().Format(time.RFC3339),
		}
		if m.ScheduledAt != nil {
			t := m.ScheduledAt.AsTime().Format(time.RFC3339)
			item.ScheduledAt = &t
		}
		items[i] = item
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": items,
		"total":    resp.Total,
		"limit":    resp.Limit,
		"offset":   resp.Offset,
	})
}

// CancelSMS отменяет запланированное сообщение (legacy DELETE /api/v1/sms/{id}).
// Returns 204 No Content on success for backward compatibility with older SDK
// versions. Prefer CancelScheduled (POST /api/v1/sms/cancel/{id}) which returns
// a JSON body matching contracts/client-api.md.
func (h *SMSHandlers) CancelSMS(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	vars := mux.Vars(r)
	messageID := vars["id"]
	if messageID == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	_, err := h.messagingClient.CancelMessage(r.Context(), &messagingv1.CancelMessageRequest{
		MessageId: messageID,
		ClientId:  clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CancelScheduled отменяет запланированное сообщение по контракту
// POST /api/v1/sms/cancel/{id} → 200 {"message_id": "...", "status": "cancelled"}.
func (h *SMSHandlers) CancelScheduled(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	vars := mux.Vars(r)
	messageID := vars["id"]
	if messageID == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	_, err := h.messagingClient.CancelMessage(r.Context(), &messagingv1.CancelMessageRequest{
		MessageId: messageID,
		ClientId:  clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message_id": messageID,
		"status":     string(messagestatus.Cancelled),
	})
}
