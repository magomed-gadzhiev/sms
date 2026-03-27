package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ClientRoutingHandlers struct {
	routingClient      routingv1.RoutingServiceClient
	tarificationClient tarificationv1.TarificationServiceClient
}

func NewClientRoutingHandlers(
	routingClient routingv1.RoutingServiceClient,
	tarificationClient tarificationv1.TarificationServiceClient,
) *ClientRoutingHandlers {
	return &ClientRoutingHandlers{
		routingClient:      routingClient,
		tarificationClient: tarificationClient,
	}
}

func (h *ClientRoutingHandlers) AssignProvider(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct {
		ProviderID     string `json:"provider_id"`
		Ownership      string `json:"ownership"`
		SharedPriority int32  `json:"shared_priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.AssignProviderToClient(r.Context(), &routingv1.AssignProviderRequest{
		ClientId:       clientID,
		ProviderId:     req.ProviderID,
		Ownership:      req.Ownership,
		SharedPriority: req.SharedPriority,
	})
	if err != nil {
		log.Error().Err(err).Msg("assign provider to client failed")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

func (h *ClientRoutingHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.routingClient.ListClientProviders(r.Context(), &routingv1.ListClientProvidersRequest{
		ClientId:   clientID,
		ActiveOnly: r.URL.Query().Get("active_only") == "true",
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ClientRoutingHandlers) RevokeProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_, err := h.routingClient.RevokeProviderFromClient(r.Context(), &routingv1.RevokeProviderRequest{
		ClientId:   vars["id"],
		ProviderId: vars["pid"],
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ClientRoutingHandlers) ShareProvider(w http.ResponseWriter, r *http.Request) {
	parentID := mux.Vars(r)["id"]
	var req struct {
		ChildClientID      string `json:"child_client_id"`
		ProviderID         string `json:"provider_id"`
		ExposeCost         bool   `json:"expose_cost"`
		ExposeProviderName bool   `json:"expose_provider_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.ShareProviderWithChild(r.Context(), &routingv1.ShareProviderRequest{
		ParentClientId:     parentID,
		ChildClientId:      req.ChildClientID,
		ProviderId:         req.ProviderID,
		ExposeCost:         req.ExposeCost,
		ExposeProviderName: req.ExposeProviderName,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

func (h *ClientRoutingHandlers) RevokeShared(w http.ResponseWriter, r *http.Request) {
	sid := mux.Vars(r)["sid"]
	_, err := h.routingClient.RevokeSharedProvider(r.Context(), &routingv1.RevokeSharedProviderRequest{
		Id: sid,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ClientRoutingHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct {
		OperatorID string `json:"operator_id"`
		ProviderID string `json:"provider_id"`
		Priority   int32  `json:"priority"`
		Weight     int32  `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.CreateClientRoute(r.Context(), &routingv1.CreateClientRouteRequest{
		ClientId:   clientID,
		OperatorId: req.OperatorID,
		ProviderId: req.ProviderID,
		Priority:   req.Priority,
		Weight:     req.Weight,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

func (h *ClientRoutingHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.routingClient.ListClientRoutes(r.Context(), &routingv1.ListClientRoutesRequest{
		ClientId:   clientID,
		OperatorId: r.URL.Query().Get("operator_id"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ClientRoutingHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	rid := mux.Vars(r)["rid"]
	var req struct {
		Priority int32 `json:"priority"`
		Weight   int32 `json:"weight"`
		Active   bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.UpdateClientRoute(r.Context(), &routingv1.UpdateClientRouteRequest{
		Id:       rid,
		Priority: req.Priority,
		Weight:   req.Weight,
		Active:   req.Active,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ClientRoutingHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	rid := mux.Vars(r)["rid"]
	_, err := h.routingClient.DeleteClientRoute(r.Context(), &routingv1.DeleteClientRouteRequest{
		Id: rid,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ClientRoutingHandlers) SetStrategy(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct {
		OperatorID string `json:"operator_id"`
		Strategy   string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.SetRoutingStrategy(r.Context(), &routingv1.SetRoutingStrategyRequest{
		ClientId:   clientID,
		OperatorId: req.OperatorID,
		Strategy:   req.Strategy,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ClientRoutingHandlers) GetStrategy(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.routingClient.GetRoutingStrategy(r.Context(), &routingv1.GetRoutingStrategyRequest{
		ClientId:   clientID,
		OperatorId: r.URL.Query().Get("operator_id"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ClientRoutingHandlers) GetMarginReport(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.tarificationClient.GetMarginReport(r.Context(), &tarificationv1.MarginReportRequest{
		ClientId: clientID,
		FromDate: r.URL.Query().Get("from"),
		ToDate:   r.URL.Query().Get("to"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}
