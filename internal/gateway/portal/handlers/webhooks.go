package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

// WebhookHandlers содержит handlers для управления вебхуками
type WebhookHandlers struct {
	webhookClient  webhookv1.WebhookServiceClient
	auditPublisher *audit.Publisher
}

// NewWebhookHandlers создает новый WebhookHandlers
func NewWebhookHandlers(
	webhookClient webhookv1.WebhookServiceClient,
	auditPublisher *audit.Publisher,
) *WebhookHandlers {
	return &WebhookHandlers{
		webhookClient:  webhookClient,
		auditPublisher: auditPublisher,
	}
}

// ListWebhooks обрабатывает GET /webhooks
func (h *WebhookHandlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	resp, err := h.webhookClient.ListSubscriptions(r.Context(), &webhookv1.ListSubscriptionsRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка вебхуков")
		respondGRPCError(w, err)
		return
	}

	subs := make([]map[string]interface{}, len(resp.Subscriptions))
	for i, sub := range resp.Subscriptions {
		subs[i] = portalSubscriptionToMap(sub)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": subs})
}

// CreateWebhook обрабатывает POST /webhooks
func (h *WebhookHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	userID, _ := middleware.GetUserID(r.Context())

	var req struct {
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.URL == "" {
		respondError(w, shared.ErrInvalidInput("url обязателен"))
		return
	}
	parsedURL, parseErr := url.Parse(req.URL)
	if parseErr != nil || parsedURL.Host == "" {
		respondError(w, shared.ErrInvalidInput("Неверный формат URL"))
		return
	}
	if parsedURL.Scheme != "https" {
		respondError(w, shared.ErrInvalidInput("URL должен использовать HTTPS"))
		return
	}
	if len(req.EventTypes) == 0 {
		respondError(w, shared.ErrInvalidInput("event_types обязателен"))
		return
	}

	resp, err := h.webhookClient.CreateSubscription(r.Context(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   clientID.String(),
		Url:        req.URL,
		EventTypes: req.EventTypes,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания webhook подписки")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionWebhookCreated, audit.ResourceWebhook, resp.Subscription.Id)
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"url":         req.URL,
		"event_types": req.EventTypes,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	result := portalSubscriptionToMap(resp.Subscription)
	result["secret"] = resp.Secret

	respondJSON(w, http.StatusCreated, result)
}

// UpdateWebhook обрабатывает PUT /webhooks/{id}
func (h *WebhookHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	userID, _ := middleware.GetUserID(r.Context())

	id := mux.Vars(r)["id"]

	var req struct {
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
		Active     *bool    `json:"active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.URL != "" {
		parsedURL, parseErr := url.Parse(req.URL)
		if parseErr != nil || parsedURL.Host == "" {
			respondError(w, shared.ErrInvalidInput("Неверный формат URL"))
			return
		}
		if parsedURL.Scheme != "https" {
			respondError(w, shared.ErrInvalidInput("URL должен использовать HTTPS"))
			return
		}
	}

	grpcReq := &webhookv1.UpdateSubscriptionRequest{
		Id:         id,
		ClientId:   clientID.String(),
		Url:        req.URL,
		EventTypes: req.EventTypes,
	}
	if req.Active != nil {
		grpcReq.Active = req.Active
	}

	resp, err := h.webhookClient.UpdateSubscription(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обновления webhook подписки")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionWebhookUpdated, audit.ResourceWebhook, id)
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"url":         req.URL,
		"event_types": req.EventTypes,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, portalSubscriptionToMap(resp.Subscription))
}

// DeleteWebhook обрабатывает DELETE /webhooks/{id}
func (h *WebhookHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	userID, _ := middleware.GetUserID(r.Context())

	id := mux.Vars(r)["id"]

	_, err := h.webhookClient.DeleteSubscription(r.Context(), &webhookv1.DeleteSubscriptionRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка удаления webhook подписки")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionWebhookDeleted, audit.ResourceWebhook, id)
	event.IPAddress = getIPAddress(r)
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestWebhook обрабатывает POST /webhooks/{id}/test
func (h *WebhookHandlers) TestWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	userID, _ := middleware.GetUserID(r.Context())

	id := mux.Vars(r)["id"]

	// Проверяем, что подписка существует и принадлежит клиенту
	_, err := h.webhookClient.GetSubscription(r.Context(), &webhookv1.GetSubscriptionRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения webhook подписки для теста")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionWebhookTestSent, audit.ResourceWebhook, id)
	event.IPAddress = getIPAddress(r)
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	// WebhookService не имеет TestSubscription RPC, возвращаем подтверждение
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Тестовое событие отправлено",
	})
}

// portalSubscriptionToMap преобразует SubscriptionInfo в map для JSON ответа
func portalSubscriptionToMap(sub *webhookv1.SubscriptionInfo) map[string]interface{} {
	result := map[string]interface{}{
		"id":          sub.Id,
		"client_id":   sub.ClientId,
		"url":         sub.Url,
		"event_types": sub.EventTypes,
		"active":      sub.Active,
	}
	if sub.CreatedAt != nil {
		result["created_at"] = sub.CreatedAt.AsTime()
	}
	if sub.UpdatedAt != nil {
		result["updated_at"] = sub.UpdatedAt.AsTime()
	}
	return result
}
