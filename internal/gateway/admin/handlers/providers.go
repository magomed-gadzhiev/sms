package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/api/proto/providerv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ProviderHandlers обрабатывает HTTP запросы для управления провайдерами
type ProviderHandlers struct {
	providerClient providerv1.ProviderServiceClient
}

// NewProviderHandlers создает новый экземпляр ProviderHandlers
func NewProviderHandlers(providerClient providerv1.ProviderServiceClient) *ProviderHandlers {
	return &ProviderHandlers{
		providerClient: providerClient,
	}
}

// CreateProvider обрабатывает POST /admin/v1/providers
func (h *ProviderHandlers) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &providerv1.CreateProviderRequest{
		Name:           req.Name,
		Host:           req.Host,
		Port:           req.Port,
		SystemId:       req.SystemID,
		Password:       req.Password,
		SystemType:     req.SystemType,
		AddrTon:        req.AddrTON,
		AddrNpi:        req.AddrNPI,
		BindType:       req.BindType,
		MaxConnections: req.MaxConnections,
		WindowSize:     req.WindowSize,
		Active:         req.Active,
		Settings:       req.Settings,
	}

	resp, err := h.providerClient.CreateProvider(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, CreateProviderResponse{
		ProviderID: resp.ProviderId,
		CreatedAt:  safeTimestamp(resp.CreatedAt),
	})
}

// GetProvider обрабатывает GET /admin/v1/providers/:id
func (h *ProviderHandlers) GetProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	resp, err := h.providerClient.GetProvider(r.Context(), &providerv1.GetProviderRequest{
		ProviderId: providerID,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка получения провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, providerInfoToResponse(resp.Provider))
}

// ListProviders обрабатывает GET /admin/v1/providers
func (h *ProviderHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") == "true"
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	resp, err := h.providerClient.ListProviders(r.Context(), &providerv1.ListProvidersRequest{
		ActiveOnly: activeOnly,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка провайдеров")
		respondGRPCError(w, err)
		return
	}

	providers := make([]ProviderInfo, len(resp.Providers))
	for i, p := range resp.Providers {
		providers[i] = providerInfoToResponse(p)
	}

	respondJSON(w, http.StatusOK, ListProvidersResponse{
		Providers: providers,
		Total:     int(resp.Total),
		Limit:     limit,
		Offset:    offset,
	})
}

// UpdateProvider обрабатывает PUT /admin/v1/providers/:id
func (h *ProviderHandlers) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	var name, host, systemID, password string
	var port, maxConnections int32
	var active bool
	if req.Name != nil {
		name = *req.Name
	}
	if req.Host != nil {
		host = *req.Host
	}
	if req.Port != nil {
		port = *req.Port
	}
	if req.SystemID != nil {
		systemID = *req.SystemID
	}
	if req.Password != nil {
		password = *req.Password
	}
	if req.MaxConnections != nil {
		maxConnections = *req.MaxConnections
	}
	if req.Active != nil {
		active = *req.Active
	}

	grpcReq := &providerv1.UpdateProviderRequest{
		ProviderId:     providerID,
		Name:           name,
		Host:           host,
		Port:           port,
		SystemId:       systemID,
		Password:       password,
		MaxConnections: maxConnections,
		Active:         active,
		Settings:       req.Settings,
	}

	resp, err := h.providerClient.UpdateProvider(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка обновления провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, UpdateProviderResponse{
		Success: resp.Success,
	})
}

// DeleteProvider обрабатывает DELETE /admin/v1/providers/:id
func (h *ProviderHandlers) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	resp, err := h.providerClient.DeleteProvider(r.Context(), &providerv1.DeleteProviderRequest{
		ProviderId: providerID,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка удаления провайдера")
		respondGRPCError(w, err)
		return
	}

	// DeleteProvider возвращает DeleteProviderRequest (это ошибка в proto, но обработаем)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp != nil,
	})
}

// GetProviderHealth обрабатывает GET /admin/v1/providers/:id/health
func (h *ProviderHandlers) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	resp, err := h.providerClient.GetProviderHealth(r.Context(), &providerv1.GetProviderHealthRequest{
		ProviderId: providerID,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка получения статуса здоровья провайдера")
		respondGRPCError(w, err)
		return
	}

	health := ProviderHealth{
		ProviderID:        resp.ProviderId,
		Status:            resp.Status,
		ActiveConnections: int(resp.ActiveConnections),
		TotalConnections:  int(resp.TotalConnections),
		SuccessRate:       int(resp.SuccessRate),
		MessagesSent24h:   resp.MessagesSent_24H,
		MessagesFailed24h: resp.MessagesFailed_24H,
	}

	if resp.LastSuccess != nil {
		t := resp.LastSuccess.AsTime()
		health.LastSuccess = &t
	}
	if resp.LastFailure != nil {
		t := resp.LastFailure.AsTime()
		health.LastFailure = &t
	}
	if resp.LastError != "" {
		health.LastError = &resp.LastError
	}

	respondJSON(w, http.StatusOK, health)
}

// Типы запросов и ответов

type CreateProviderRequest struct {
	Name           string            `json:"name"`
	Host           string            `json:"host"`
	Port           int32             `json:"port"`
	SystemID       string            `json:"system_id"`
	Password       string            `json:"password"`
	SystemType     string            `json:"system_type,omitempty"`
	AddrTON        int32             `json:"addr_ton,omitempty"`
	AddrNPI        int32             `json:"addr_npi,omitempty"`
	BindType       int32             `json:"bind_type"`
	MaxConnections int32             `json:"max_connections"`
	WindowSize     int32             `json:"window_size,omitempty"`
	Active         bool              `json:"active"`
	Settings       map[string]string `json:"settings,omitempty"`
}

func (r *CreateProviderRequest) Validate() error {
	if r.Name == "" {
		return shared.ErrInvalidInput("name обязателен")
	}
	if r.Host == "" {
		return shared.ErrInvalidInput("host обязателен")
	}
	if r.Port <= 0 || r.Port > 65535 {
		return shared.ErrInvalidInput("port должен быть в диапазоне 1-65535")
	}
	if r.SystemID == "" {
		return shared.ErrInvalidInput("system_id обязателен")
	}
	if r.Password == "" {
		return shared.ErrInvalidInput("password обязателен")
	}
	return nil
}

type CreateProviderResponse struct {
	ProviderID string    `json:"provider_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type UpdateProviderRequest struct {
	Name           *string           `json:"name,omitempty"`
	Host           *string           `json:"host,omitempty"`
	Port           *int32            `json:"port,omitempty"`
	SystemID       *string           `json:"system_id,omitempty"`
	Password       *string           `json:"password,omitempty"`
	MaxConnections *int32            `json:"max_connections,omitempty"`
	Active         *bool             `json:"active,omitempty"`
	Settings       map[string]string `json:"settings,omitempty"`
}

type UpdateProviderResponse struct {
	Success bool `json:"success"`
}

type ProviderInfo struct {
	ProviderID    string            `json:"provider_id"`
	Name          string            `json:"name"`
	Host          string            `json:"host"`
	Port          int32             `json:"port"`
	SystemID      string            `json:"system_id"`
	SystemType    string            `json:"system_type"`
	BindType      int32             `json:"bind_type"`
	MaxConnections int32            `json:"max_connections"`
	WindowSize    int32             `json:"window_size"`
	Active        bool              `json:"active"`
	Settings      map[string]string `json:"settings"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type ListProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
}

type ProviderHealth struct {
	ProviderID        string     `json:"provider_id"`
	Status            string     `json:"status"`
	ActiveConnections int        `json:"active_connections"`
	TotalConnections  int        `json:"total_connections"`
	SuccessRate       int        `json:"success_rate"`
	MessagesSent24h   int64      `json:"messages_sent_24h"`
	MessagesFailed24h int64      `json:"messages_failed_24h"`
	LastSuccess       *time.Time `json:"last_success,omitempty"`
	LastFailure       *time.Time `json:"last_failure,omitempty"`
	LastError         *string    `json:"last_error,omitempty"`
}

func providerInfoToResponse(p *providerv1.ProviderInfo) ProviderInfo {
	info := ProviderInfo{
		ProviderID:     p.ProviderId,
		Name:           p.Name,
		Host:           p.Host,
		Port:           p.Port,
		SystemID:       p.SystemId,
		SystemType:     p.SystemType,
		BindType:       p.BindType,
		MaxConnections: p.MaxConnections,
		WindowSize:     p.WindowSize,
		Active:         p.Active,
		Settings:       p.Settings,
	}

	if p.CreatedAt != nil {
		info.CreatedAt = p.CreatedAt.AsTime()
	}
	if p.UpdatedAt != nil {
		info.UpdatedAt = p.UpdatedAt.AsTime()
	}

	return info
}
