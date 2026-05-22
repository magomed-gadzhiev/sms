// internal/gateway/admin/handlers/webhooks.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type WebhookHandlers struct {
	webhookClient webhookv1.WebhookServiceClient
}

func NewWebhookHandlers(webhookClient webhookv1.WebhookServiceClient) *WebhookHandlers {
	return &WebhookHandlers{webhookClient: webhookClient}
}

func (h *WebhookHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID   string   `json:"client_id"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ClientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}
	if req.URL == "" {
		respondError(w, shared.ErrInvalidInput("url обязателен"))
		return
	}
	if len(req.EventTypes) == 0 {
		respondError(w, shared.ErrInvalidInput("event_types обязателен"))
		return
	}

	resp, err := h.webhookClient.CreateSubscription(r.Context(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   req.ClientID,
		Url:        req.URL,
		EventTypes: req.EventTypes,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания webhook подписки")
		respondGRPCError(w, err)
		return
	}

	if resp.Subscription == nil {
		respondError(w, shared.ErrInternalServer("пустой ответ от webhook-сервиса"))
		return
	}
	result := map[string]interface{}{
		"id":          resp.Subscription.Id,
		"client_id":   resp.Subscription.ClientId,
		"url":         resp.Subscription.Url,
		"event_types": resp.Subscription.EventTypes,
		"active":      resp.Subscription.Active,
		"secret":      resp.Secret,
		"created_at":  safeTimestamp(resp.Subscription.CreatedAt),
	}
	respondJSON(w, http.StatusCreated, result)
}

func (h *WebhookHandlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	resp, err := h.webhookClient.ListSubscriptions(r.Context(), &webhookv1.ListSubscriptionsRequest{
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	subs := make([]map[string]interface{}, len(resp.Subscriptions))
	for i, sub := range resp.Subscriptions {
		subs[i] = adminSubscriptionToMap(sub)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": subs})
}

func (h *WebhookHandlers) GetWebhook(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	resp, err := h.webhookClient.GetSubscription(r.Context(), &webhookv1.GetSubscriptionRequest{
		Id:       id,
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	if resp.Subscription == nil {
		respondError(w, shared.ErrNotFound("webhook подписка не найдена"))
		return
	}
	respondJSON(w, http.StatusOK, adminSubscriptionToMap(resp.Subscription))
}

func (h *WebhookHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		ClientID   string   `json:"client_id"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
		Active     *bool    `json:"active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ClientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	grpcReq := &webhookv1.UpdateSubscriptionRequest{
		Id:         id,
		ClientId:   req.ClientID,
		Url:        req.URL,
		EventTypes: req.EventTypes,
	}
	if req.Active != nil {
		grpcReq.Active = req.Active
	}

	resp, err := h.webhookClient.UpdateSubscription(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	if resp.Subscription == nil {
		respondError(w, shared.ErrInternalServer("пустой ответ от webhook-сервиса"))
		return
	}
	respondJSON(w, http.StatusOK, adminSubscriptionToMap(resp.Subscription))
}

func (h *WebhookHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	_, err := h.webhookClient.DeleteSubscription(r.Context(), &webhookv1.DeleteSubscriptionRequest{
		Id:       id,
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminSubscriptionToMap(sub *webhookv1.SubscriptionInfo) map[string]interface{} {
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
