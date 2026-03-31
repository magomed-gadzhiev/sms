package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	max_messenger "github.com/smpp-server/smpp-server/internal/services/cascade/channels/max_messenger"
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
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"channels": resp.Channels,
	})
}

// GetChannel обрабатывает GET /admin/channels/{id}
func (h *CascadeChannelHandlers) GetChannel(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ChannelType == "" || req.Name == "" {
		http.Error(w, "channel_type and name are required", http.StatusBadRequest)
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
	var req updateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
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
	var req toggleChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
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
	return max_messenger.ValidateConfig(cfg)
}
