package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientRoutingHandlers содержит HTTP обработчики для управления маршрутизацией клиента
type ClientRoutingHandlers struct {
	routingClient routingv1.RoutingServiceClient
	pool          *pgxpool.Pool
}

// NewClientRoutingHandlers создаёт новый экземпляр ClientRoutingHandlers
func NewClientRoutingHandlers(routingClient routingv1.RoutingServiceClient, pool *pgxpool.Pool) *ClientRoutingHandlers {
	return &ClientRoutingHandlers{routingClient: routingClient, pool: pool}
}

// ---- Routing Mode ----

// GetRoutingMode GET /portal/v1/routing/mode
func (h *ClientRoutingHandlers) GetRoutingMode(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var routingMode string
	err := h.pool.QueryRow(r.Context(),
		`SELECT routing_mode FROM clients WHERE id = $1`, clientID,
	).Scan(&routingMode)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"routing_mode": routingMode})
}

// SetRoutingMode PUT /portal/v1/routing/mode
func (h *ClientRoutingHandlers) SetRoutingMode(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req struct {
		RoutingMode string `json:"routing_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.RoutingMode != "legacy" && req.RoutingMode != "new" && req.RoutingMode != "hybrid" {
		respondError(w, shared.ErrInvalidInput("routing_mode должен быть legacy, new или hybrid"))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`UPDATE clients SET routing_mode = $1, updated_at = now() WHERE id = $2`,
		req.RoutingMode, clientID,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"routing_mode": req.RoutingMode})
}

// ---- Operators ----

// ListOperators GET /portal/v1/routing/operators
func (h *ClientRoutingHandlers) ListOperators(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	resp, err := h.routingClient.ListOperators(r.Context(), &routingv1.ListOperatorsRequest{
		ActiveOnly: true,
		Limit:      1000,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// ---- Client Routes ----

// CreateRoute POST /portal/v1/routing/routes
func (h *ClientRoutingHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req routingv1.CreateClientRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	resp, err := h.routingClient.CreateClientRoute(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// ListRoutes GET /portal/v1/routing/routes
func (h *ClientRoutingHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	operatorID := r.URL.Query().Get("operator_id")

	resp, err := h.routingClient.ListClientRoutes(r.Context(), &routingv1.ListClientRoutesRequest{
		ClientId:   clientID.String(),
		OperatorId: operatorID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// UpdateRoute PUT /portal/v1/routing/routes/{id}
func (h *ClientRoutingHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req routingv1.UpdateClientRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.Id = mux.Vars(r)["id"]

	resp, err := h.routingClient.UpdateClientRoute(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DeleteRoute DELETE /portal/v1/routing/routes/{id}
func (h *ClientRoutingHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	_, err := h.routingClient.DeleteClientRoute(r.Context(), &routingv1.DeleteClientRouteRequest{
		Id: id,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Routing Strategy ----

// SetStrategy PUT /portal/v1/routing/strategy
func (h *ClientRoutingHandlers) SetStrategy(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req routingv1.SetRoutingStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	resp, err := h.routingClient.SetRoutingStrategy(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetStrategy GET /portal/v1/routing/strategy
func (h *ClientRoutingHandlers) GetStrategy(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	operatorID := r.URL.Query().Get("operator_id")

	resp, err := h.routingClient.GetRoutingStrategy(r.Context(), &routingv1.GetRoutingStrategyRequest{
		ClientId:   clientID.String(),
		OperatorId: operatorID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}
