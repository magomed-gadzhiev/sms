package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type DomainHandlers struct {
	domainClient linkv1.DomainServiceClient
}

func NewDomainHandlers(domainClient linkv1.DomainServiceClient) *DomainHandlers {
	return &DomainHandlers{domainClient: domainClient}
}

func (h *DomainHandlers) AddDomain(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}
	resp, err := h.domainClient.AddDomain(r.Context(), &linkv1.AddDomainRequest{
		ClientId: clientID.String(),
		Domain:   req.Domain,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

func (h *DomainHandlers) ListDomains(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	resp, err := h.domainClient.ListDomains(r.Context(), &linkv1.ListDomainsRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *DomainHandlers) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	domainID := mux.Vars(r)["id"]
	_, err := h.domainClient.DeleteDomain(r.Context(), &linkv1.DeleteDomainRequest{
		ClientId: clientID.String(),
		DomainId: domainID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
