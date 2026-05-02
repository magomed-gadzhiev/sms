package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientHandlers обрабатывает HTTP запросы для управления клиентами
type ClientHandlers struct {
	clientClient clientv1.ClientServiceClient
}

// NewClientHandlers создает новый экземпляр ClientHandlers
func NewClientHandlers(clientClient clientv1.ClientServiceClient) *ClientHandlers {
	return &ClientHandlers{
		clientClient: clientClient,
	}
}

// CreateClient обрабатывает POST /admin/v1/clients
func (h *ClientHandlers) CreateClient(w http.ResponseWriter, r *http.Request) {
	var req CreateClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &clientv1.CreateClientRequest{
		Name:           req.Name,
		Email:          req.Email,
		ContactPerson:  req.ContactPerson,
		Phone:          req.Phone,
		Active:         req.Active,
		IsReseller:     req.IsReseller,
		MaxSubAccounts: req.MaxSubAccounts,
		Metadata:       req.Metadata,
	}

	resp, err := h.clientClient.CreateClient(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания клиента")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, CreateClientResponse{
		ClientID:  resp.ClientId,
		CreatedAt: safeTimestamp(resp.CreatedAt),
	})
}

// GetClient обрабатывает GET /admin/v1/clients/:id
func (h *ClientHandlers) GetClient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	resp, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{
		ClientId: clientID,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка получения клиента")
		respondGRPCError(w, err)
		return
	}

	if resp.Client == nil {
		respondError(w, shared.ErrNotFound("клиент не найден"))
		return
	}
	respondJSON(w, http.StatusOK, clientInfoToResponse(resp.Client))
}

// ListClients обрабатывает GET /admin/v1/clients
func (h *ClientHandlers) ListClients(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") == "true"
	search := r.URL.Query().Get("search")
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	resp, err := h.clientClient.ListClients(r.Context(), &clientv1.ListClientsRequest{
		ActiveOnly: activeOnly,
		Search:     search,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка клиентов")
		respondGRPCError(w, err)
		return
	}

	clients := make([]ClientInfo, len(resp.Clients))
	for i, c := range resp.Clients {
		clients[i] = clientInfoToResponse(c)
	}

	respondJSON(w, http.StatusOK, ListClientsResponse{
		Clients: clients,
		Total:   int(resp.Total),
		Limit:   limit,
		Offset:  offset,
	})
}

// UpdateClient обрабатывает PUT /admin/v1/clients/:id
func (h *ClientHandlers) UpdateClient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	var req UpdateClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Name != nil {
		if err := validateTextField("name", *req.Name, 255); err != nil {
			respondError(w, err.(*shared.AppError))
			return
		}
	}
	if req.Email != nil {
		if err := validateTextField("email", *req.Email, 255); err != nil {
			respondError(w, err.(*shared.AppError))
			return
		}
	}
	if req.ContactPerson != nil {
		if err := validateTextField("contact_person", *req.ContactPerson, 255); err != nil {
			respondError(w, err.(*shared.AppError))
			return
		}
	}
	if req.Phone != nil {
		if err := validateTextField("phone", *req.Phone, 50); err != nil {
			respondError(w, err.(*shared.AppError))
			return
		}
	}
	if req.MaxSubAccounts != nil && *req.MaxSubAccounts < 0 {
		respondError(w, shared.ErrInvalidInput("max_sub_accounts не может быть отрицательным"))
		return
	}
	// Reseller-без-слотов проверяется на application-уровне (требует знания
	// итогового состояния после применения PATCH; admin handler видит только delta).

	var name, email, contactPerson, phone string
	if req.Name != nil {
		name = *req.Name
	}
	if req.Email != nil {
		email = *req.Email
	}
	if req.ContactPerson != nil {
		contactPerson = *req.ContactPerson
	}
	if req.Phone != nil {
		phone = *req.Phone
	}

	grpcReq := &clientv1.UpdateClientRequest{
		ClientId:       clientID,
		Name:           name,
		Email:          email,
		ContactPerson:  contactPerson,
		Phone:          phone,
		Active:         req.Active,
		IsReseller:     req.IsReseller,
		MaxSubAccounts: req.MaxSubAccounts,
		Metadata:       req.Metadata,
	}

	resp, err := h.clientClient.UpdateClient(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка обновления клиента")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, UpdateClientResponse{
		Success: resp.Success,
	})
}

// DeleteClient обрабатывает DELETE /admin/v1/clients/:id
func (h *ClientHandlers) DeleteClient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	resp, err := h.clientClient.DeleteClient(r.Context(), &clientv1.DeleteClientRequest{
		ClientId: clientID,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка удаления клиента")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, DeleteClientResponse{
		Success: resp.Success,
	})
}

// GetClientConfig обрабатывает GET /admin/v1/clients/:id/config
func (h *ClientHandlers) GetClientConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	resp, err := h.clientClient.GetClientConfig(r.Context(), &clientv1.GetClientConfigRequest{
		ClientId: clientID,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка получения конфигурации клиента")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, clientConfigToResponse(resp.Config))
}

// UpdateClientConfig обрабатывает PUT /admin/v1/clients/:id/config
func (h *ClientHandlers) UpdateClientConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	var req UpdateClientConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &clientv1.UpdateClientConfigRequest{
		ClientId: clientID,
		Config: &clientv1.ClientConfig{
			ClientId:           clientID,
			RateLimits:         rateLimitsToProto(req.RateLimits),
			AllowedSources:     req.AllowedSources,
			BlockedDestinations: req.BlockedDestinations,
			Settings:           req.Settings,
		},
	}

	resp, err := h.clientClient.UpdateClientConfig(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка обновления конфигурации клиента")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, UpdateClientConfigResponse{
		Success: resp.Success,
	})
}

// UpdateClientRateLimits обрабатывает PUT /admin/v1/clients/:id/rate-limits
func (h *ClientHandlers) UpdateClientRateLimits(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	var req UpdateClientRateLimitsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &clientv1.UpdateClientRateLimitsRequest{
		ClientId: clientID,
		RateLimits: rateLimitsToProto(req.RateLimits),
	}

	resp, err := h.clientClient.UpdateClientRateLimits(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка обновления rate limits клиента")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, UpdateClientRateLimitsResponse{
		Success: resp.Success,
	})
}

// Вспомогательные функции и типы

type CreateClientRequest struct {
	Name           string            `json:"name"`
	Email          string            `json:"email"`
	ContactPerson  string            `json:"contact_person,omitempty"`
	Phone          string            `json:"phone,omitempty"`
	Active         bool              `json:"active"`
	IsReseller     bool              `json:"is_reseller,omitempty"`
	MaxSubAccounts int32             `json:"max_sub_accounts,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

func (r *CreateClientRequest) Validate() error {
	if r.Name == "" {
		return shared.ErrInvalidInput("name обязателен")
	}
	if err := validateTextField("name", r.Name, 255); err != nil {
		return err
	}
	if r.Email == "" {
		return shared.ErrInvalidInput("email обязателен")
	}
	if err := validateTextField("email", r.Email, 255); err != nil {
		return err
	}
	if err := validateTextField("contact_person", r.ContactPerson, 255); err != nil {
		return err
	}
	if err := validateTextField("phone", r.Phone, 50); err != nil {
		return err
	}
	if r.MaxSubAccounts < 0 {
		return shared.ErrInvalidInput("max_sub_accounts не может быть отрицательным")
	}
	if r.IsReseller && r.MaxSubAccounts < 1 {
		return shared.ErrInvalidInput("реселлер требует max_sub_accounts >= 1")
	}
	return nil
}

// validateTextField checks length and rejects HTML tags to prevent stored XSS
func validateTextField(field, value string, maxLen int) error {
	if len(value) > maxLen {
		return shared.ErrInvalidInput(field + " не должен превышать " + strconv.Itoa(maxLen) + " символов")
	}
	if strings.ContainsAny(value, "<>") {
		return shared.ErrInvalidInput(field + " содержит недопустимые символы")
	}
	return nil
}

type CreateClientResponse struct {
	ClientID  string    `json:"client_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateClientRequest struct {
	Name           *string           `json:"name,omitempty"`
	Email          *string           `json:"email,omitempty"`
	ContactPerson  *string           `json:"contact_person,omitempty"`
	Phone          *string           `json:"phone,omitempty"`
	Active         *bool             `json:"active,omitempty"`
	IsReseller     *bool             `json:"is_reseller,omitempty"`
	MaxSubAccounts *int32            `json:"max_sub_accounts,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type UpdateClientResponse struct {
	Success bool `json:"success"`
}

type DeleteClientResponse struct {
	Success bool `json:"success"`
}

type ClientInfo struct {
	ClientID       string            `json:"client_id"`
	Name           string            `json:"name"`
	Email          string            `json:"email"`
	ContactPerson  string            `json:"contact_person"`
	Phone          string            `json:"phone"`
	Active         bool              `json:"active"`
	IsReseller     bool              `json:"is_reseller"`
	MaxSubAccounts int32             `json:"max_sub_accounts"`
	RateLimits     RateLimits        `json:"rate_limits"`
	Metadata       map[string]string `json:"metadata"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type ListClientsResponse struct {
	Clients []ClientInfo `json:"clients"`
	Total   int          `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
}

type RateLimits struct {
	MessagesPerSecond int `json:"messages_per_second"`
	MessagesPerMinute int `json:"messages_per_minute"`
	MessagesPerHour   int `json:"messages_per_hour"`
	MessagesPerDay    int `json:"messages_per_day"`
}

type UpdateClientConfigRequest struct {
	RateLimits         RateLimits        `json:"rate_limits"`
	AllowedSources     []string          `json:"allowed_sources,omitempty"`
	BlockedDestinations []string         `json:"blocked_destinations,omitempty"`
	Settings           map[string]string `json:"settings,omitempty"`
}

type UpdateClientConfigResponse struct {
	Success bool `json:"success"`
}

type UpdateClientRateLimitsRequest struct {
	RateLimits RateLimits `json:"rate_limits"`
}

type UpdateClientRateLimitsResponse struct {
	Success bool `json:"success"`
}

type ClientConfig struct {
	ClientID           string            `json:"client_id"`
	RateLimits         RateLimits        `json:"rate_limits"`
	AllowedSources     []string          `json:"allowed_sources"`
	BlockedDestinations []string         `json:"blocked_destinations"`
	Settings           map[string]string `json:"settings"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func clientInfoToResponse(c *clientv1.ClientInfo) ClientInfo {
	info := ClientInfo{
		ClientID:       c.ClientId,
		Name:           c.Name,
		Email:          c.Email,
		ContactPerson:  c.ContactPerson,
		Phone:          c.Phone,
		Active:         c.Active,
		IsReseller:     c.IsReseller,
		MaxSubAccounts: c.MaxSubAccounts,
		Metadata:       c.Metadata,
	}
	
	if c.CreatedAt != nil {
		info.CreatedAt = c.CreatedAt.AsTime()
	}
	if c.UpdatedAt != nil {
		info.UpdatedAt = c.UpdatedAt.AsTime()
	}
	if c.RateLimits != nil {
		info.RateLimits = RateLimits{
			MessagesPerSecond: int(c.RateLimits.MessagesPerSecond),
			MessagesPerMinute: int(c.RateLimits.MessagesPerMinute),
			MessagesPerHour:   int(c.RateLimits.MessagesPerHour),
			MessagesPerDay:    int(c.RateLimits.MessagesPerDay),
		}
	}
	
	return info
}

func clientConfigToResponse(c *clientv1.ClientConfig) ClientConfig {
	config := ClientConfig{
		ClientID:            c.ClientId,
		AllowedSources:      c.AllowedSources,
		BlockedDestinations: c.BlockedDestinations,
		Settings:            c.Settings,
	}
	
	if c.RateLimits != nil {
		config.RateLimits = RateLimits{
			MessagesPerSecond: int(c.RateLimits.MessagesPerSecond),
			MessagesPerMinute: int(c.RateLimits.MessagesPerMinute),
			MessagesPerHour:   int(c.RateLimits.MessagesPerHour),
			MessagesPerDay:    int(c.RateLimits.MessagesPerDay),
		}
	}
	
	if c.UpdatedAt != nil {
		config.UpdatedAt = c.UpdatedAt.AsTime()
	}
	
	return config
}

func rateLimitsToProto(rl RateLimits) *clientv1.RateLimits {
	return &clientv1.RateLimits{
		MessagesPerSecond: int32(rl.MessagesPerSecond),
		MessagesPerMinute: int32(rl.MessagesPerMinute),
		MessagesPerHour:   int32(rl.MessagesPerHour),
		MessagesPerDay:    int32(rl.MessagesPerDay),
	}
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return val
}
