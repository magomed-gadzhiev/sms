package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	maxmessenger "github.com/smpp-server/smpp-server/internal/services/cascade/channels/maxmessenger"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CascadeChannelHandlers обрабатывает admin-запросы для управления каналами
type CascadeChannelHandlers struct {
	client cascadev1.ChannelAdminServiceClient
}

// NewCascadeChannelHandlers создаёт новый CascadeChannelHandlers
func NewCascadeChannelHandlers(client cascadev1.ChannelAdminServiceClient) *CascadeChannelHandlers {
	return &CascadeChannelHandlers{client: client}
}

// ListChannels обрабатывает GET /admin/channels
func (h *CascadeChannelHandlers) ListChannels(w http.ResponseWriter, r *http.Request) {
	resp, err := h.client.ListChannels(r.Context(), &cascadev1.ListChannelsRequest{})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	// BUG-65: nil-slice → JSON `null` ломает frontend .map. Подменяем на пустой массив.
	channels := resp.Channels
	if channels == nil {
		channels = []*cascadev1.ChannelResponse{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channels,
	})
}

// GetChannel обрабатывает GET /admin/channels/{id}
func (h *CascadeChannelHandlers) GetChannel(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// BUG-64: до фикса invalid UUID уходил в gRPC, который возвращал Internal 500
	// вместо InvalidArgument. Проверяем формат на handler-уровне.
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid channel_id format"))
		return
	}
	resp, err := h.client.GetChannel(r.Context(), &cascadev1.GetChannelRequest{ChannelId: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

type createChannelRequest struct {
	ChannelType string `json:"channel_type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ConfigJSON  string `json:"config_json,omitempty"`
}

// CreateChannel обрабатывает POST /admin/channels
func (h *CascadeChannelHandlers) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req createChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// BUG-66: до фикса http.Error возвращал plain-text, frontend крэшил JSON-парсер.
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.ChannelType == "" || req.Name == "" {
		respondError(w, shared.ErrInvalidInput("channel_type and name are required"))
		return
	}
	if err := validateChannelConfig(req.ChannelType, req.ConfigJSON); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	resp, err := h.client.CreateChannel(r.Context(), &cascadev1.CreateChannelRequest{
		ChannelType: req.ChannelType,
		Name:        req.Name,
		Description: req.Description,
		ConfigJson:  req.ConfigJSON,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

type updateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ConfigJSON  string `json:"config_json,omitempty"`
}

// UpdateChannel обрабатывает PUT /admin/channels/{id}
func (h *CascadeChannelHandlers) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid channel_id format"))
		return
	}
	var req updateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// BUG-66: до фикса http.Error возвращал plain-text, frontend крэшил JSON-парсер.
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	// Для UpdateChannel нужен channel_type — загрузим канал
	chResp, chErr := h.client.GetChannel(r.Context(), &cascadev1.GetChannelRequest{ChannelId: id})
	if chErr == nil && req.ConfigJSON != "" {
		if err := validateChannelConfig(chResp.GetChannelType(), req.ConfigJSON); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			return
		}
	}
	resp, err := h.client.UpdateChannel(r.Context(), &cascadev1.UpdateChannelRequest{
		ChannelId:   id,
		Name:        req.Name,
		Description: req.Description,
		ConfigJson:  req.ConfigJSON,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

type toggleChannelRequest struct {
	Active bool `json:"active"`
}

// ToggleChannel обрабатывает PUT /admin/channels/{id}/toggle
func (h *CascadeChannelHandlers) ToggleChannel(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid channel_id format"))
		return
	}
	var req toggleChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// BUG-66: до фикса http.Error возвращал plain-text, frontend крэшил JSON-парсер.
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	resp, err := h.client.ToggleChannel(r.Context(), &cascadev1.ToggleChannelRequest{
		ChannelId: id,
		Active:    req.Active,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// validateChannelConfig выполняет channel-specific валидацию конфигурации
func validateChannelConfig(channelType, configJSON string) error {
	if channelType != "max_messenger" || configJSON == "" {
		return nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return err
	}
	return maxmessenger.ValidateConfig(cfg)
}
