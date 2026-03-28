package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

// APIKeyHandlers содержит handlers для управления API ключами
type APIKeyHandlers struct {
	authClient     authv1.AuthServiceClient
	auditPublisher *audit.Publisher
}

// NewAPIKeyHandlers создает новый APIKeyHandlers
func NewAPIKeyHandlers(authClient authv1.AuthServiceClient, auditPublisher *audit.Publisher) *APIKeyHandlers {
	return &APIKeyHandlers{
		authClient:     authClient,
		auditPublisher: auditPublisher,
	}
}

// createAPIKeyRequest представляет запрос на создание API ключа
type createAPIKeyRequest struct {
	Name       string   `json:"name"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	ExpiresAt  *string  `json:"expires_at,omitempty"` // RFC3339 формат
}

// ListAPIKeys обрабатывает GET /api-keys
func (h *APIKeyHandlers) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	resp, err := h.authClient.ListAPIKeys(r.Context(), &authv1.ListAPIKeysRequest{
		UserId: userID.String(),
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("ошибка получения списка API ключей")
		respondGRPCError(w, err)
		return
	}

	keys := make([]map[string]interface{}, len(resp.Keys))
	for i, key := range resp.Keys {
		keys[i] = map[string]interface{}{
			"id":         key.Id,
			"name":       key.Name,
			"prefix":     key.Prefix,
			"active":     key.Active,
			"scopes":     key.Scopes,
			"allowed_ips": key.AllowedIps,
			"created_at": key.CreatedAt.AsTime(),
		}
		if key.ExpiresAt != nil {
			keys[i]["expires_at"] = key.ExpiresAt.AsTime()
		}
		if key.LastUsedAt != nil {
			keys[i]["last_used_at"] = key.LastUsedAt.AsTime()
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}

// CreateAPIKey обрабатывает POST /api-keys
func (h *APIKeyHandlers) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	if len(req.Name) > 255 {
		respondError(w, shared.ErrInvalidInput("Имя не может превышать 255 символов"))
		return
	}

	grpcReq := &authv1.CreateAPIKeyRequest{
		UserId:     userID.String(),
		Name:       req.Name,
		Scopes:     req.Scopes,
		AllowedIps: req.AllowedIPs,
	}

	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат expires_at, ожидается RFC3339"))
			return
		}
		grpcReq.ExpiresAt = timestamppb.New(t)
	}

	resp, err := h.authClient.CreateAPIKey(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("ошибка создания API ключа")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	if h.auditPublisher != nil {
		event := audit.NewAuditEvent("", userID.String(), audit.ActionAPIKeyCreated, audit.ResourceAPIKey, resp.ApiKeyId)
		event.IPAddress = getIPAddress(r)
		if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
			log.Error().Err(err).Msg("ошибка публикации audit event")
		}
	}

	result := map[string]interface{}{
		"api_key":    resp.ApiKey,
		"api_key_id": resp.ApiKeyId,
		"created_at": resp.CreatedAt.AsTime(),
	}
	if resp.ExpiresAt != nil {
		result["expires_at"] = resp.ExpiresAt.AsTime()
	}

	respondJSON(w, http.StatusCreated, result)
}

// GetAPIKey обрабатывает GET /api-keys/{id}
func (h *APIKeyHandlers) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	keyID := mux.Vars(r)["id"]
	if keyID == "" {
		respondError(w, shared.ErrInvalidInput("ID ключа обязателен"))
		return
	}

	resp, err := h.authClient.ListAPIKeys(r.Context(), &authv1.ListAPIKeysRequest{
		UserId: userID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	for _, key := range resp.Keys {
		if key.Id == keyID {
			result := map[string]interface{}{
				"id":          key.Id,
				"name":        key.Name,
				"prefix":      key.Prefix,
				"active":      key.Active,
				"scopes":      key.Scopes,
				"allowed_ips": key.AllowedIps,
				"created_at":  key.CreatedAt.AsTime(),
			}
			if key.ExpiresAt != nil {
				result["expires_at"] = key.ExpiresAt.AsTime()
			}
			if key.LastUsedAt != nil {
				result["last_used_at"] = key.LastUsedAt.AsTime()
			}
			respondJSON(w, http.StatusOK, result)
			return
		}
	}

	respondError(w, shared.ErrNotFound("API ключ не найден"))
}

// RevokeAPIKey обрабатывает DELETE /api-keys/{id}
func (h *APIKeyHandlers) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	vars := mux.Vars(r)
	keyID := vars["id"]
	if keyID == "" {
		respondError(w, shared.ErrInvalidInput("ID ключа обязателен"))
		return
	}

	_, err := h.authClient.RevokeAPIKey(r.Context(), &authv1.RevokeAPIKeyRequest{
		ApiKeyId: keyID,
	})
	if err != nil {
		log.Error().Err(err).Str("api_key_id", keyID).Msg("ошибка отзыва API ключа")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	if h.auditPublisher != nil {
		event := audit.NewAuditEvent("", userID.String(), audit.ActionAPIKeyRevoked, audit.ResourceAPIKey, keyID)
		event.IPAddress = getIPAddress(r)
		if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
			log.Error().Err(err).Msg("ошибка публикации audit event")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateAPIKeyRequest представляет запрос на обновление API ключа
type updateAPIKeyRequest struct {
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	AllowedIPs []string `json:"allowed_ips"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
}

// UpdateAPIKey обрабатывает PUT /api-keys/{id}
func (h *APIKeyHandlers) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	keyID := mux.Vars(r)["id"]
	if keyID == "" {
		respondError(w, shared.ErrInvalidInput("ID ключа обязателен"))
		return
	}

	var req updateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}

	grpcReq := &authv1.UpdateAPIKeyRequest{
		KeyId:      keyID,
		UserId:     userID.String(),
		Name:       req.Name,
		Scopes:     req.Scopes,
		AllowedIps: req.AllowedIPs,
	}

	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат expires_at"))
			return
		}
		grpcReq.ExpiresAt = timestamppb.New(t)
	}

	resp, err := h.authClient.UpdateAPIKey(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"id":          resp.Key.Id,
		"name":        resp.Key.Name,
		"prefix":      resp.Key.Prefix,
		"active":      resp.Key.Active,
		"scopes":      resp.Key.Scopes,
		"allowed_ips": resp.Key.AllowedIps,
		"created_at":  resp.Key.CreatedAt.AsTime(),
	}
	if resp.Key.ExpiresAt != nil {
		result["expires_at"] = resp.Key.ExpiresAt.AsTime()
	}
	if resp.Key.LastUsedAt != nil {
		result["last_used_at"] = resp.Key.LastUsedAt.AsTime()
	}

	// Публикуем audit event
	if h.auditPublisher != nil {
		event := audit.NewAuditEvent("", userID.String(), "api_key.updated", audit.ResourceAPIKey, keyID)
		event.IPAddress = getIPAddress(r)
		h.auditPublisher.Publish(r.Context(), event)
	}

	respondJSON(w, http.StatusOK, result)
}
