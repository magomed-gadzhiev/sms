package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ProviderHandlers содержит HTTP обработчики для провайдеров
type ProviderHandlers struct {
	providerClient cpv1.ClientProviderServiceClient
}

// NewProviderHandlers создаёт новый экземпляр ProviderHandlers
func NewProviderHandlers(providerClient cpv1.ClientProviderServiceClient) *ProviderHandlers {
	return &ProviderHandlers{providerClient: providerClient}
}

// ListProviders GET /portal/v1/providers
func (h *ProviderHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	req := &cpv1.ListClientProvidersRequest{}
	if clientID != uuid.Nil {
		req.ClientId = clientID.String()
	}

	resp, err := h.providerClient.ListClientProviders(r.Context(), req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// CreateProvider POST /portal/v1/providers
func (h *ProviderHandlers) CreateProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req cpv1.CreateClientProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	resp, err := h.providerClient.CreateClientProvider(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// GetProvider GET /portal/v1/providers/{id}
func (h *ProviderHandlers) GetProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id := mux.Vars(r)["id"]
	resp, err := h.providerClient.GetClientProvider(r.Context(), &cpv1.GetClientProviderRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// UpdateProvider PUT /portal/v1/providers/{id}
func (h *ProviderHandlers) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req cpv1.UpdateClientProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.Id = mux.Vars(r)["id"]
	req.ClientId = clientID.String()

	resp, err := h.providerClient.UpdateClientProvider(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DeleteProvider DELETE /portal/v1/providers/{id}
func (h *ProviderHandlers) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id := mux.Vars(r)["id"]
	_, err := h.providerClient.DeleteClientProvider(r.Context(), &cpv1.DeleteClientProviderRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestProviderConnection POST /portal/v1/providers/test-connection
func (h *ProviderHandlers) TestProviderConnection(w http.ResponseWriter, r *http.Request) {
	var req cpv1.TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.providerClient.TestClientProviderConnection(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}
