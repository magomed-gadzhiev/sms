package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// SubAccountRoutingHandlers содержит HTTP обработчики для маршрутизации суб-аккаунтов
type SubAccountRoutingHandlers struct {
	routingClient routingv1.RoutingServiceClient
}

// NewSubAccountRoutingHandlers создаёт новый экземпляр SubAccountRoutingHandlers
func NewSubAccountRoutingHandlers(routingClient routingv1.RoutingServiceClient) *SubAccountRoutingHandlers {
	return &SubAccountRoutingHandlers{routingClient: routingClient}
}

// ---- Client Providers ----

// AssignProvider POST /portal/v1/sub-accounts/routing/providers
func (h *SubAccountRoutingHandlers) AssignProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req routingv1.AssignProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	resp, err := h.routingClient.AssignProviderToClient(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// ListProviders GET /portal/v1/sub-accounts/routing/providers
func (h *SubAccountRoutingHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	activeOnly := r.URL.Query().Get("active_only") == "true"

	resp, err := h.routingClient.ListClientProviders(r.Context(), &routingv1.ListClientProvidersRequest{
		ClientId:   clientID.String(),
		ActiveOnly: activeOnly,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// UpdateProvider PUT /portal/v1/sub-accounts/routing/providers/{id}
func (h *SubAccountRoutingHandlers) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req routingv1.UpdateClientProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.Id = mux.Vars(r)["id"]

	resp, err := h.routingClient.UpdateClientProvider(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// RevokeProvider DELETE /portal/v1/sub-accounts/routing/providers/{providerId}
func (h *SubAccountRoutingHandlers) RevokeProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	providerID := mux.Vars(r)["providerId"]

	_, err := h.routingClient.RevokeProviderFromClient(r.Context(), &routingv1.RevokeProviderRequest{
		ClientId:   clientID.String(),
		ProviderId: providerID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Provider Sharing ----

// ShareProvider POST /portal/v1/sub-accounts/routing/providers/share
func (h *SubAccountRoutingHandlers) ShareProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req routingv1.ShareProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ParentClientId = clientID.String()

	resp, err := h.routingClient.ShareProviderWithChild(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// RevokeSharedProvider DELETE /portal/v1/sub-accounts/routing/providers/shared/{id}
func (h *SubAccountRoutingHandlers) RevokeSharedProvider(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	_, err := h.routingClient.RevokeSharedProvider(r.Context(), &routingv1.RevokeSharedProviderRequest{
		Id: id,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Client Routes ----

// CreateRoute POST /portal/v1/sub-accounts/routing/routes
func (h *SubAccountRoutingHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
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

// ListRoutes GET /portal/v1/sub-accounts/routing/routes
func (h *SubAccountRoutingHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
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

// UpdateRoute PUT /portal/v1/sub-accounts/routing/routes/{id}
func (h *SubAccountRoutingHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
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

// DeleteRoute DELETE /portal/v1/sub-accounts/routing/routes/{id}
func (h *SubAccountRoutingHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
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

// SetStrategy PUT /portal/v1/sub-accounts/routing/strategy
func (h *SubAccountRoutingHandlers) SetStrategy(w http.ResponseWriter, r *http.Request) {
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

// GetStrategy GET /portal/v1/sub-accounts/routing/strategy
func (h *SubAccountRoutingHandlers) GetStrategy(w http.ResponseWriter, r *http.Request) {
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

// DeleteStrategy DELETE /portal/v1/sub-accounts/routing/strategy
func (h *SubAccountRoutingHandlers) DeleteStrategy(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	operatorID := r.URL.Query().Get("operator_id")

	_, err := h.routingClient.DeleteRoutingStrategy(r.Context(), &routingv1.DeleteRoutingStrategyRequest{
		ClientId:   clientID.String(),
		OperatorId: operatorID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
