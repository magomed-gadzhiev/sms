package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RoutingHandlers обрабатывает HTTP запросы для управления маршрутизацией
type RoutingHandlers struct {
	routingClient routingv1.RoutingServiceClient
}

// NewRoutingHandlers создает новый экземпляр RoutingHandlers
func NewRoutingHandlers(routingClient routingv1.RoutingServiceClient) *RoutingHandlers {
	return &RoutingHandlers{
		routingClient: routingClient,
	}
}

// CreateRoute обрабатывает POST /admin/v1/routes
func (h *RoutingHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &routingv1.CreateRouteRequest{
		Name:              req.Name,
		Pattern:           req.Pattern,
		Priority:          req.Priority,
		ProviderIds:       req.ProviderIDs,
		LoadBalanceStrategy: req.LoadBalanceStrategy,
		FailoverEnabled:   req.FailoverEnabled,
		Metadata:          req.Metadata,
	}

	resp, err := h.routingClient.CreateRoute(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания маршрута")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, CreateRouteResponse{
		RouteID:   resp.RouteId,
		CreatedAt: safeTimestamp(resp.CreatedAt),
	})
}

// GetRoute обрабатывает GET /admin/v1/routes/:id (через destination)
func (h *RoutingHandlers) GetRoute(w http.ResponseWriter, r *http.Request) {
	destination := r.URL.Query().Get("destination")
	clientID := r.URL.Query().Get("client_id")
	source := r.URL.Query().Get("source")

	if destination == "" {
		respondError(w, shared.ErrInvalidInput("destination обязателен"))
		return
	}

	grpcReq := &routingv1.GetRouteRequest{
		Destination: destination,
		ClientId:    clientID,
		Source:      source,
	}

	resp, err := h.routingClient.GetRoute(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения маршрута")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, routeInfoToResponse(resp.Route))
}

// ListRoutes обрабатывает GET /admin/v1/routes
func (h *RoutingHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") == "true"
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	resp, err := h.routingClient.ListRoutes(r.Context(), &routingv1.ListRoutesRequest{
		ActiveOnly: activeOnly,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка маршрутов")
		respondGRPCError(w, err)
		return
	}

	routes := make([]RouteInfo, len(resp.Routes))
	for i, route := range resp.Routes {
		routes[i] = routeInfoToResponse(route)
	}

	respondJSON(w, http.StatusOK, ListRoutesResponse{
		Routes: routes,
		Total:  int(resp.Total),
		Limit:  limit,
		Offset: offset,
	})
}

// UpdateRoute обрабатывает PUT /admin/v1/routes/:id
func (h *RoutingHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	routeID := vars["id"]

	var req UpdateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	var name, pattern, loadBalanceStrategy string
	var priority int32
	var failoverEnabled, active bool
	if req.Name != nil {
		name = *req.Name
	}
	if req.Pattern != nil {
		pattern = *req.Pattern
	}
	if req.Priority != nil {
		priority = *req.Priority
	}
	if req.LoadBalanceStrategy != nil {
		loadBalanceStrategy = *req.LoadBalanceStrategy
	}
	if req.FailoverEnabled != nil {
		failoverEnabled = *req.FailoverEnabled
	}
	if req.Active != nil {
		active = *req.Active
	}

	grpcReq := &routingv1.UpdateRouteRequest{
		RouteId:             routeID,
		Name:                name,
		Pattern:             pattern,
		Priority:            priority,
		ProviderIds:         req.ProviderIDs,
		LoadBalanceStrategy: loadBalanceStrategy,
		FailoverEnabled:     failoverEnabled,
		Active:              active,
		Metadata:            req.Metadata,
	}

	resp, err := h.routingClient.UpdateRoute(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("route_id", routeID).Msg("ошибка обновления маршрута")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, UpdateRouteResponse{
		Success: resp.Success,
	})
}

// DeleteRoute обрабатывает DELETE /admin/v1/routes/:id
func (h *RoutingHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	routeID := vars["id"]

	resp, err := h.routingClient.DeleteRoute(r.Context(), &routingv1.DeleteRouteRequest{
		RouteId: routeID,
	})
	if err != nil {
		log.Error().Err(err).Str("route_id", routeID).Msg("ошибка удаления маршрута")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, DeleteRouteResponse{
		Success: resp.Success,
	})
}

// Типы запросов и ответов

type CreateRouteRequest struct {
	Name                string            `json:"name"`
	Pattern             string            `json:"pattern"`
	Priority            int32             `json:"priority"`
	ProviderIDs         []string          `json:"provider_ids"`
	LoadBalanceStrategy string            `json:"load_balance_strategy"`
	FailoverEnabled     bool              `json:"failover_enabled"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

func (r *CreateRouteRequest) Validate() error {
	if r.Name == "" {
		return shared.ErrInvalidInput("name обязателен")
	}
	if r.Pattern == "" {
		return shared.ErrInvalidInput("pattern обязателен")
	}
	if len(r.ProviderIDs) == 0 {
		return shared.ErrInvalidInput("provider_ids обязателен и не может быть пустым")
	}
	return nil
}

type CreateRouteResponse struct {
	RouteID   string    `json:"route_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateRouteRequest struct {
	Name                *string           `json:"name,omitempty"`
	Pattern             *string           `json:"pattern,omitempty"`
	Priority            *int32            `json:"priority,omitempty"`
	ProviderIDs         []string          `json:"provider_ids,omitempty"`
	LoadBalanceStrategy *string           `json:"load_balance_strategy,omitempty"`
	FailoverEnabled     *bool             `json:"failover_enabled,omitempty"`
	Active              *bool             `json:"active,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

type UpdateRouteResponse struct {
	Success bool `json:"success"`
}

type DeleteRouteResponse struct {
	Success bool `json:"success"`
}

type RouteInfo struct {
	RouteID             string            `json:"route_id"`
	Name                string            `json:"name"`
	Pattern             string            `json:"pattern"`
	Priority            int32             `json:"priority"`
	ProviderIDs         []string          `json:"provider_ids"`
	LoadBalanceStrategy string            `json:"load_balance_strategy"`
	FailoverEnabled     bool              `json:"failover_enabled"`
	Active              bool              `json:"active"`
	Metadata            map[string]string `json:"metadata"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type ListRoutesResponse struct {
	Routes []RouteInfo `json:"routes"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func routeInfoToResponse(route *routingv1.RouteInfo) RouteInfo {
	info := RouteInfo{
		RouteID:             route.RouteId,
		Name:                route.Name,
		Pattern:             route.Pattern,
		Priority:            route.Priority,
		ProviderIDs:         route.ProviderIds,
		LoadBalanceStrategy: route.LoadBalanceStrategy,
		FailoverEnabled:     route.FailoverEnabled,
		Active:              route.Active,
		Metadata:            route.Metadata,
	}

	if route.CreatedAt != nil {
		info.CreatedAt = route.CreatedAt.AsTime()
	}
	if route.UpdatedAt != nil {
		info.UpdatedAt = route.UpdatedAt.AsTime()
	}

	return info
}
